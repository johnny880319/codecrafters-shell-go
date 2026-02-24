package shell

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

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
