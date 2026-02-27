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
	stdin              io.Reader
	stdout             io.Writer
	stderr             io.Writer
	sysPath            string
	workingDir         string
	commandFuncMap     map[string]func(args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) error
	history            []string
	historyStartIndex  int
	historyAppendIndex int
	sysHistFile        string
}

// Option defines a functional parameter for configuring a Shell instance.
type Option func(*Shell)

// WithSysPath overrides the default system PATH used by the Shell.
func WithSysPath(p string) Option {
	return func(s *Shell) { s.sysPath = p }
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
		stdin:       in,
		stdout:      out,
		stderr:      out,
		sysPath:     os.Getenv("PATH"),
		workingDir:  wd,
		sysHistFile: os.Getenv("HISTFILE"),
	}

	for _, opt := range opts {
		opt(s)
	}

	s.commandFuncMap = s.getCommandFuncMap()
	if err := s.readHistoryFromFile(s.sysHistFile, s.stderr); err != nil {
		_, _ = fmt.Fprintf(s.stderr, "read history from file error: %v\n", err)
	}
	s.historyStartIndex = len(s.history)
	s.historyAppendIndex = len(s.history)

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
		Stdin:           io.NopCloser(s.stdin),
		Stdout:          s.stdout,
		Stderr:          s.stderr,
	})
	readlineInitMutex.Unlock()

	if err != nil {
		return fmt.Errorf("failed to initialize readline: %w", err)
	}
	defer func() {
		_ = rl.Close()
	}()

	for {
		if _, err := fmt.Fprint(s.stdout, prompt); err != nil {
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
	s.history = append(s.history, input)
	splitedInput := handleQuotesAndEscapes(input)
	pipelineStr := splitPipeline(splitedInput)
	var wg sync.WaitGroup

	prevReader := s.stdin
	for i, cmdStr := range pipelineStr {
		command, args := cmdStr[0], cmdStr[1:]

		if command == "" {
			continue
		}

		cmdStdin := prevReader
		cmdStdout := s.stdout
		cmdStderr := s.stderr

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
			_, _ = fmt.Fprintln(s.stderr, err)
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
				if closer, ok := in.(io.ReadCloser); ok && in != s.stdin {
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
				_, _ = fmt.Fprintf(s.stderr, "failed to start %s: %v\n", cmd.Args[0], err)
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
				if closer, ok := c.Stdin.(io.ReadCloser); ok && c.Stdin != s.stdin {
					_ = closer.Close()
				}
			}(cmd, currentPipeWriter)
		} else {
			if _, err := fmt.Fprintf(s.stdout, "%s: command not found\n", command); err != nil {
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
