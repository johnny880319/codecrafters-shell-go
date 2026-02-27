package shell

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"

	"github.com/chzyer/readline"
)

const prompt = "$ "

var readlineInitMutex sync.Mutex

// Shell represents the core state of the interactive shell.
type Shell struct {
	workingDir     string
	commandFuncMap map[string]func(args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) error
	stream         shellStream
	env            shellEnv
	history        shellHistory
}

type shellStream struct {
	stdin  io.Reader
	stdout io.Writer
	stderr io.Writer
}

type shellEnv struct {
	path     string
	histfile string
}

type shellHistory struct {
	lines      []string
	startLine  int
	appendLine int
}

// Option defines a functional parameter for configuring a Shell instance.
type Option func(*Shell)

// WithSysPath overrides the default system PATH used by the Shell.
func WithSysPath(p string) Option {
	return func(s *Shell) { s.env.path = p }
}

// WithWorkingDir overrides the default initial working directory for the Shell.
func WithWorkingDir(d string) Option {
	return func(s *Shell) { s.workingDir = d }
}

// NewShell creates a new Shell instance with default OS environment variables.
func NewShell(in io.Reader, out io.Writer, opts ...Option) *Shell {
	wd, err := os.Getwd()
	if err != nil {
		wd = "/"
	}

	s := &Shell{
		stream:     shellStream{stdin: in, stdout: out, stderr: out},
		env:        shellEnv{path: os.Getenv("PATH"), histfile: os.Getenv("HISTFILE")},
		workingDir: wd,
	}

	for _, opt := range opts {
		opt(s)
	}

	s.commandFuncMap = s.getCommandFuncMap()
	if err := s.readHistoryFromFile(s.env.histfile, s.stream.stderr); err != nil {
		_, _ = fmt.Fprintf(s.stream.stderr, "read history from file error: %v\n", err)
	}
	s.history.startLine = len(s.history.lines)
	s.history.appendLine = len(s.history.lines)

	return s
}

// Repl starts a read-eval-print loop that reads commands from in, executes them, and writes output to out.
func (s *Shell) Repl() error {
	readlineInitMutex.Lock()
	rl, err := readline.NewEx(&readline.Config{
		Prompt:          prompt,
		AutoComplete:    &customCompleter{shell: s, tabCount: 0},
		InterruptPrompt: "^C",
		EOFPrompt:       "exit",
		Stdin:           io.NopCloser(s.stream.stdin),
		Stdout:          s.stream.stdout,
		Stderr:          s.stream.stderr,
	})
	readlineInitMutex.Unlock()

	if err != nil {
		return fmt.Errorf("failed to initialize readline: %w", err)
	}
	defer func() {
		_ = rl.Close()
	}()

	for {
		if _, err := fmt.Fprint(s.stream.stdout, prompt); err != nil {
			return fmt.Errorf("write prompt: %w", err)
		}
		input, err := rl.Readline()
		if err != nil {
			if errors.Is(err, readline.ErrInterrupt) {
				continue
			}
			if errors.Is(err, io.EOF) {
				return nil
			}
			return fmt.Errorf("read line error: %w", err)
		}

		if err := s.execute(input); err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
		}
	}
}

//nolint:gocognit // Will be refactored in a future exercise
func (s *Shell) execute(input string) error {
	s.history.lines = append(s.history.lines, input)
	splitedInput := handleQuotesAndEscapes(input)
	pipelineStr := splitPipeline(splitedInput)
	var wg sync.WaitGroup

	prevReader := s.stream.stdin
	for i, cmdStr := range pipelineStr {
		command, args := cmdStr[0], cmdStr[1:]

		if command == "" {
			continue
		}

		cmdStdin := prevReader
		cmdStdout := s.stream.stdout
		cmdStderr := s.stream.stderr

		var currentPipeWriter io.Closer
		if i < len(pipelineStr)-1 {
			pr, pw := io.Pipe()
			cmdStdout = pw
			prevReader = pr
			currentPipeWriter = pw
		}

		args, closer, err := handleRedirect(args, &cmdStdout, &cmdStderr)
		if closer != nil {
			defer func(c io.Closer) {
				_ = c.Close()
			}(closer)
		}
		if err != nil {
			_, _ = fmt.Fprintln(s.stream.stderr, err)
			if currentPipeWriter != nil {
				_ = currentPipeWriter.Close()
			}
			continue
		}

		if commandFunc, ok := s.commandFuncMap[command]; ok {
			if command == "cd" || command == "exit" {
				err = commandFunc(args, cmdStdin, cmdStdout, cmdStderr)
				if errors.Is(err, io.EOF) {
					return io.EOF
				}
				if currentPipeWriter != nil {
					_ = currentPipeWriter.Close()
				}
				continue
			}
			wg.Add(1)
			go func(in io.Reader, stdout io.Writer, stderr io.Writer, pw io.Closer) {
				defer wg.Done()
				_ = commandFunc(args, in, stdout, stderr)
				if pw != nil {
					_ = pw.Close()
				}
				if closer, ok := in.(io.ReadCloser); ok && in != s.stream.stdin {
					_ = closer.Close()
				}
			}(cmdStdin, cmdStdout, cmdStderr, currentPipeWriter)

			continue
		}

		if path, found := s.findExecutable(command); found {
			//nolint:gosec // Executing dynamic user input is the intended behavior of a shell
			cmd := exec.CommandContext(context.Background(), path, args...)
			cmd.Args[0] = command
			cmd.Dir = s.workingDir
			cmd.Stdin = cmdStdin
			cmd.Stdout = cmdStdout
			cmd.Stderr = cmdStderr

			if err := cmd.Start(); err != nil {
				_, _ = fmt.Fprintf(s.stream.stderr, "failed to start %s: %v\n", cmd.Args[0], err)
				if currentPipeWriter != nil {
					_ = currentPipeWriter.Close()
				}
				continue
			}

			wg.Add(1)
			go func(c *exec.Cmd, pw io.Closer) {
				defer wg.Done()
				_ = c.Wait()
				if pw != nil {
					_ = pw.Close()
				}
				if closer, ok := c.Stdin.(io.ReadCloser); ok && c.Stdin != s.stream.stdin {
					_ = closer.Close()
				}
			}(cmd, currentPipeWriter)
		} else {
			if _, err := fmt.Fprintf(s.stream.stdout, "%s: command not found\n", command); err != nil {
				if currentPipeWriter != nil {
					_ = currentPipeWriter.Close()
				}
				return fmt.Errorf("write command output: %w", err)
			}
		}
	}

	wg.Wait()
	return nil
}
