package shell

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
)

const prompt = "$ "

// Shell represents the core state of the interactive shell.
type Shell struct {
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
func NewShell(out io.Writer, opts ...Option) *Shell {
	wd, err := os.Getwd()
	if err != nil {
		wd = "/"
	}

	s := &Shell{
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
func (s *Shell) Repl(in io.Reader) error {
	scanner := bufio.NewScanner(in)
	for {
		if err := s.runOnce(scanner); err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return fmt.Errorf("shell session ended with error: %w", err)
		}
	}
}

func (s *Shell) runOnce(scanner *bufio.Scanner) error {
	if _, err := fmt.Fprint(s.stdOut, prompt); err != nil {
		return fmt.Errorf("write prompt: %w", err)
	}

	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return fmt.Errorf("read command: %w", err)
		}
		return io.EOF
	}

	input := scanner.Text()
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
	//nolint:gosec // Printing dynamic user input is the intended behavior of a shell
	if _, err := fmt.Fprintf(s.stdOut, "%s: command not found\n", input); err != nil {
		return fmt.Errorf("write command output: %w", err)
	}

	return nil
}

func (s *Shell) findExecutable(command string) (string, bool) {
	paths := filepath.SplitList(s.sysPath)
	for _, dir := range paths {
		fullPath := filepath.Join(dir, command)
		if _, err := exec.LookPath(fullPath); err == nil {
			return fullPath, true
		}
	}
	return "", false
}

//nolint:gocognit // Will be refactored in a future exercise
func (s *Shell) handleRedirect(args []string) ([]string, *os.File, error) {
	for i, arg := range args {
		if (arg == ">" || arg == "1>") && i < len(args)-1 {
			filename := args[i+1]
			//nolint:gosec // Opening files based on user input is the intended behavior of a shell
			file, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
			if err != nil {
				return nil, nil, fmt.Errorf("%s: %s", filename, err.Error())
			}
			s.stdOut = file
			return append(args[:i], args[i+2:]...), file, nil
		}
		if arg == "2>" && i < len(args)-1 {
			filename := args[i+1]
			//nolint:gosec // Opening files based on user input is the intended behavior of a shell
			file, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
			if err != nil {
				return nil, nil, fmt.Errorf("%s: %s", filename, err.Error())
			}
			s.stdErr = file
			return append(args[:i], args[i+2:]...), file, nil
		}
		if (arg == ">>" || arg == "1>>") && i < len(args)-1 {
			filename := args[i+1]
			//nolint:gosec // Opening files based on user input is the intended behavior of a shell
			file, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o644)
			if err != nil {
				return nil, nil, fmt.Errorf("%s: %s", filename, err.Error())
			}
			s.stdOut = file
			return append(args[:i], args[i+2:]...), file, nil
		}
		if arg == "2>>" && i < len(args)-1 {
			filename := args[i+1]
			//nolint:gosec // Opening files based on user input is the intended behavior of a shell
			file, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o644)
			if err != nil {
				return nil, nil, fmt.Errorf("%s: %s", filename, err.Error())
			}
			s.stdErr = file
			return append(args[:i], args[i+2:]...), file, nil
		}
	}
	return args, nil, nil
}
