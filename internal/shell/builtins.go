package shell

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func (s *Shell) getCommandFuncMap() map[string]func([]string, io.Reader, io.Writer, io.Writer) error {
	return map[string]func([]string, io.Reader, io.Writer, io.Writer) error{
		"exit":    s.cmdExit,
		"echo":    s.cmdEcho,
		"type":    s.cmdType,
		"pwd":     s.cmdPwd,
		"cd":      s.cmdCd,
		"history": s.cmdHistory,
	}
}

func (s *Shell) cmdExit(_ []string, _ io.Reader, _ io.Writer, _ io.Writer) error {
	return io.EOF
}

func (s *Shell) cmdEcho(args []string, _ io.Reader, stdout io.Writer, _ io.Writer) error {
	_, err := fmt.Fprintln(stdout, strings.Join(args, " "))
	return err
}

func (s *Shell) cmdType(args []string, _ io.Reader, stdout io.Writer, stderr io.Writer) error {
	for _, arg := range args {
		if _, ok := s.commandFuncMap[arg]; ok {
			if _, err := fmt.Fprintf(stdout, "%s is a shell builtin\n", arg); err != nil {
				return err
			}
		} else if path, found := s.findExecutable(arg); found {
			if _, err := fmt.Fprintf(stdout, "%s is %s\n", arg, path); err != nil {
				return err
			}
		} else {
			if _, err := fmt.Fprintf(stderr, "%s: not found\n", arg); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Shell) cmdPwd(_ []string, _ io.Reader, stdout io.Writer, _ io.Writer) error {
	_, err := fmt.Fprintln(stdout, s.workingDir)
	return err
}

func (s *Shell) cmdCd(args []string, _ io.Reader, _ io.Writer, stderr io.Writer) error {
	if len(args) == 0 {
		s.workingDir = os.Getenv("HOME")
		return nil
	}
	if len(args) > 1 {
		_, err := fmt.Fprintln(stderr, "cd: too many arguments")
		return err
	}

	newPath := args[0]
	if filepath.IsAbs(newPath) {
		return s.checkDirectory(newPath, stderr)
	}

	if strings.HasPrefix(newPath, "~") {
		s.workingDir = os.Getenv("HOME")
		newPath = newPath[1:]
	}
	absPath := filepath.Join(s.workingDir, newPath)
	return s.checkDirectory(absPath, stderr)
}

func (s *Shell) checkDirectory(path string, stderr io.Writer) error {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		_, printfErr := fmt.Fprintf(stderr, "cd: %s: No such file or directory\n", path)
		return printfErr
	}
	if err != nil {
		return fmt.Errorf("check directory: %w", err)
	}
	if !info.IsDir() {
		_, printfErr := fmt.Fprintf(stderr, "cd: %s: Not a directory\n", path)
		return printfErr
	}
	s.workingDir = path
	return nil
}

//nolint:gocognit // Will be refactored in a future exercise
func (s *Shell) cmdHistory(args []string, _ io.Reader, stdout io.Writer, stderr io.Writer) error {
	if len(args) > 1 && args[0] == "-r" {
		historyFilePath := args[1]
		//nolint:gosec // A shell's intended behavior is to open files specified by the user
		file, err := os.Open(historyFilePath)
		if err != nil {
			return fmt.Errorf("open history file: %w", err)
		}
		defer func() {
			if err := file.Close(); err != nil {
				if _, err := fmt.Fprintf(stderr, "close history file error: %v\n", err); err != nil {
					return
				}
			}
		}()

		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := scanner.Text()
			s.history = append(s.history, line)
		}
		if err := scanner.Err(); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				_, printfErr := fmt.Fprintf(stderr, "history: %s: No such file or directory\n", args[1])
				return printfErr
			}
		}
	}

	start := 1

	if len(args) > 0 {
		n, err := strconv.Atoi(args[0])
		if err != nil {
			if _, err := fmt.Fprintf(stderr, "history: %s: numeric argument required\n", args[0]); err != nil {
				return err
			}
			return nil
		}

		start = max(len(s.history)-n+1, 1)
	}

	for i, cmd := range s.history[start-1:] {
		if _, err := fmt.Fprintf(stdout, "%d %s\n", i+start, cmd); err != nil {
			return err
		}
	}

	return nil
}
