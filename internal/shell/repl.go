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
	commandFuncMap map[string]func(args []string) error
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

	s.commandFuncMap = map[string]func([]string) error{
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
		AutoComplete:    &customCompleter{shell: s},
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

func (s *Shell) execute(input string) error {
	command, args := parseCommand(input)

	if command == "" {
		return nil
	}

	originalStdOut := s.stdOut
	originalStdErr := s.stdErr
	defer func() {
		s.stdOut = originalStdOut
		s.stdErr = originalStdErr
	}()

	args, toBeClosed, err := s.handleRedirect(args)
	if err != nil {
		_, _ = fmt.Fprintln(s.stdErr, err)
		return nil
	}
	if toBeClosed != nil {
		defer func() {
			_ = toBeClosed.Close()
		}()
	}

	if commandFunc, ok := s.commandFuncMap[command]; ok {
		return commandFunc(args)
	}

	if path, found := s.findExecutable(command); found {
		//nolint:gosec // Executing dynamic user input is the intended behavior of a shell
		cmd := exec.CommandContext(context.Background(), path, args...)
		cmd.Args[0] = command
		cmd.Dir = s.workingDir
		cmd.Stdout = s.stdOut
		cmd.Stderr = s.stdErr
		_ = cmd.Run()
		return nil
	}
	if _, err := fmt.Fprintf(s.stdOut, "%s: command not found\n", input); err != nil {
		return fmt.Errorf("write command output: %w", err)
	}

	return nil
}

type customCompleter struct {
	shell *Shell
}

// Do implements readline.AutoCompleter to provide tab completion for built-in commands.
func (c *customCompleter) Do(line []rune, pos int) (newLine [][]rune, length int) {
	lineStr := string(line[:pos])
	var matches [][]rune

	for builtin := range c.shell.commandFuncMap {
		if strings.HasPrefix(builtin, lineStr) {
			completion := builtin[len(lineStr):] + " "
			matches = append(matches, []rune(completion))
		}
	}

	if len(matches) == 0 {
		if _, err := fmt.Fprint(c.shell.stdOut, "\x07"); err != nil {
			return nil, len(lineStr)
		}
		return nil, len(lineStr)
	}

	return matches, len(lineStr)
}
