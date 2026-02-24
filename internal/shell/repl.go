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
	"strings"
)

const prompt = "$ "

// Shell represents the core state of the interactive shell.
type Shell struct {
	out            io.Writer
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
		out:        out,
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
	if _, err := fmt.Fprint(s.out, prompt); err != nil {
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

	if commandFunc, ok := s.commandFuncMap[command]; ok {
		return commandFunc(args)
	}

	if path, found := s.findExecutable(command); found {
		//nolint:gosec // Executing dynamic user input is the intended behavior of a shell
		cmd := exec.CommandContext(context.Background(), path, args...)
		cmd.Args[0] = command
		cmd.Dir = s.workingDir
		cmd.Stdout = s.out
		cmd.Stderr = s.out
		_ = cmd.Run()
		return nil
	}
	//nolint:gosec // Printing dynamic user input is the intended behavior of a shell
	if _, err := fmt.Fprintf(s.out, "%s: command not found\n", input); err != nil {
		return fmt.Errorf("write command output: %w", err)
	}

	return nil
}

func parseCommand(input string) (command string, args []string) {
	fields := []string{}
	inSingleQuotes := false
	inDoubleQuotes := false
	curField := ""
	for _, r := range input {
		if r == '\'' && !inDoubleQuotes {
			inSingleQuotes = !inSingleQuotes
			continue
		}
		if r == '"' && !inSingleQuotes {
			inDoubleQuotes = !inDoubleQuotes
			continue
		}
		if r == ' ' && !inSingleQuotes && !inDoubleQuotes {
			if curField != "" {
				fields = append(fields, curField)
				curField = ""
			}
			continue
		}
		curField += string(r)
	}

	if curField != "" {
		fields = append(fields, curField)
	}

	if len(fields) == 0 {
		return "", nil
	}
	return fields[0], fields[1:]
}

func (s *Shell) cmdExit(_ []string) error {
	return io.EOF
}

func (s *Shell) cmdEcho(args []string) error {
	_, err := fmt.Fprintln(s.out, strings.Join(args, " "))
	return err
}

func (s *Shell) cmdType(args []string) error {
	for _, arg := range args {
		if _, ok := s.commandFuncMap[arg]; ok {
			if _, err := fmt.Fprintf(s.out, "%s is a shell builtin\n", arg); err != nil {
				return err
			}
		} else if path, found := s.findExecutable(arg); found {
			//nolint:gosec // Printing dynamic user input is the intended behavior of a shell
			if _, err := fmt.Fprintf(s.out, "%s is %s\n", arg, path); err != nil {
				return err
			}
		} else {
			if _, err := fmt.Fprintf(s.out, "%s: not found\n", arg); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Shell) cmdPwd(_ []string) error {
	//nolint:gosec // Printing working directory is the intended behavior of a shell
	_, err := fmt.Fprintln(s.out, s.workingDir)
	return err
}

func (s *Shell) cmdCd(args []string) error {
	if len(args) == 0 {
		s.workingDir = os.Getenv("HOME")
		return nil
	}
	if len(args) > 1 {
		_, err := fmt.Fprintln(s.out, "cd: too many arguments")
		return err
	}

	newPath := args[0]
	if filepath.IsAbs(newPath) {
		return s.checkDirectory(newPath)
	}

	if strings.HasPrefix(newPath, "~") {
		s.workingDir = os.Getenv("HOME")
		newPath = newPath[1:]
	}
	absPath := filepath.Join(s.workingDir, newPath)
	return s.checkDirectory(absPath)
}

func (s *Shell) checkDirectory(path string) error {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		_, printfErr := fmt.Fprintf(s.out, "cd: %s: No such file or directory\n", path)
		return printfErr
	}
	if err != nil {
		return fmt.Errorf("check directory: %w", err)
	}
	if !info.IsDir() {
		_, printfErr := fmt.Fprintf(s.out, "cd: %s: Not a directory\n", path)
		return printfErr
	}
	s.workingDir = path
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
