package shell

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"

	"github.com/chzyer/readline"
)

const prompt = "$ "

var readlineInitMutex sync.Mutex

// Shell represents the core state of the interactive shell.
type Shell struct {
	stdIn          io.Reader
	stdOut         io.Writer
	stdErr         io.Writer
	sysPath        string
	workingDir     string
	commandFuncMap map[string]func(args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) error
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
		stdIn:      in,
		stdOut:     out,
		stdErr:     out,
		sysPath:    os.Getenv("PATH"),
		workingDir: wd,
	}

	for _, opt := range opts {
		opt(s)
	}

	s.commandFuncMap = map[string]func([]string, io.Reader, io.Writer, io.Writer) error{
		"exit": s.cmdExit,
		"echo": s.cmdEcho,
		"type": s.cmdType,
		"pwd":  s.cmdPwd,
		"cd":   s.cmdCd,
	}

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
		Stdin:           io.NopCloser(s.stdIn),
		Stdout:          s.stdOut,
		Stderr:          s.stdErr,
	})
	readlineInitMutex.Unlock()

	if err != nil {
		return fmt.Errorf("failed to initialize readline: %w", err)
	}
	defer func() {
		_ = rl.Close()
	}()

	for {
		if _, err := fmt.Fprint(s.stdOut, prompt); err != nil {
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
	pipelineStr := strings.Split(input, "|")

	var externalCmds []*exec.Cmd
	var redirectClosers []*os.File

	var wg sync.WaitGroup

	prevReader := s.stdIn
	for i, cmdStr := range pipelineStr {
		command, args := parseCommand(cmdStr)

		if command == "" {
			continue
		}

		cmdStdin := prevReader
		cmdStdout := s.stdOut
		cmdStderr := s.stdErr

		var currentPipeWriter io.Closer
		if i < len(pipelineStr)-1 {
			pr, pw := io.Pipe()
			cmdStdout = pw
			prevReader = pr
			currentPipeWriter = pw
		}

		var err error
		var closer *os.File
		args, closer, err = handleRedirect(args, &cmdStdout, &cmdStderr)
		if err != nil {
			_, _ = fmt.Fprintln(s.stdErr, err)
			continue
		}
		redirectClosers = append(redirectClosers, closer)

		if commandFunc, ok := s.commandFuncMap[command]; ok {
			if command == "cd" || command == "exit" {
				_ = commandFunc(args, cmdStdin, cmdStdout, cmdStderr)
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

			externalCmds = append(externalCmds, cmd)
			wg.Add(1)
			go func(c *exec.Cmd, pw io.Closer) {
				defer wg.Done()
				_ = c.Wait()
				if pw != nil {
					_ = pw.Close()
				}
			}(cmd, currentPipeWriter)
		} else {
			if _, err := fmt.Fprintf(s.stdOut, "%s: command not found\n", command); err != nil {
				return fmt.Errorf("write command output: %w", err)
			}
		}
	}

	for _, cmd := range externalCmds {
		if err := cmd.Start(); err != nil {
			_, _ = fmt.Fprintf(s.stdErr, "failed to start %s: %v\n", cmd.Args[0], err)
		}
		wg.Wait()

		for _, c := range redirectClosers {
			if c != nil {
				_ = c.Close()
			}
		}
	}

	return nil
}
