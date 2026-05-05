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

type builtinCommand struct {
	fn          func(args []string, cmdIO shellStream) error
	canRunAsync bool
}

func (s *Shell) getCommandFuncMap() map[string]builtinCommand {
	return map[string]builtinCommand{
		"exit":     {s.cmdExit, false},
		"echo":     {s.cmdEcho, true},
		"type":     {s.cmdType, true},
		"pwd":      {s.cmdPwd, true},
		"cd":       {s.cmdCd, false},
		"history":  {s.cmdHistory, true},
		"jobs":     {s.cmdJobs, true},
		"complete": {s.cmdComplete, true},
		"declare":  {s.cmdDeclare, true},
	}
}

func (s *Shell) cmdExit(_ []string, _ shellStream) error {
	_ = s.writeHistoryToFile(s.env.histfile, s.stream, os.O_APPEND, s.history.startLine)
	return io.EOF
}

func (s *Shell) cmdEcho(args []string, cmdIO shellStream) error {
	_, err := fmt.Fprintln(cmdIO.stdout, strings.Join(args, " "))
	return err
}

func (s *Shell) cmdType(args []string, cmdIO shellStream) error {
	for _, arg := range args {
		if _, ok := s.builtinCommandMap[arg]; ok {
			if _, err := fmt.Fprintf(cmdIO.stdout, "%s is a shell builtin\n", arg); err != nil {
				return err
			}
		} else if path, found := s.findExecutable(arg); found {
			if _, err := fmt.Fprintf(cmdIO.stdout, "%s is %s\n", arg, path); err != nil {
				return err
			}
		} else {
			if _, err := fmt.Fprintf(cmdIO.stderr, "%s: not found\n", arg); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Shell) cmdPwd(_ []string, cmdIO shellStream) error {
	_, err := fmt.Fprintln(cmdIO.stdout, s.workingDir)
	return err
}

func (s *Shell) cmdCd(args []string, cmdIO shellStream) error {
	if len(args) == 0 {
		s.workingDir = os.Getenv("HOME")
		return nil
	}
	if len(args) > 1 {
		_, err := fmt.Fprintln(cmdIO.stderr, "cd: too many arguments")
		return err
	}

	newPath := args[0]
	if filepath.IsAbs(newPath) {
		return s.checkDirectory(newPath, cmdIO.stderr)
	}

	if strings.HasPrefix(newPath, "~") {
		s.workingDir = os.Getenv("HOME")
		newPath = newPath[1:]
	}
	absPath := filepath.Join(s.workingDir, newPath)
	return s.checkDirectory(absPath, cmdIO.stderr)
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

func (s *Shell) cmdHistory(args []string, cmdIO shellStream) error {
	if len(args) > 1 && args[0] == "-r" {
		return s.readHistoryFromFile(args[1], cmdIO)
	}

	if len(args) > 0 && args[0] == "-w" {
		return s.writeHistoryToFile(args[1], cmdIO, os.O_TRUNC, 0)
	}

	if len(args) > 0 && args[0] == "-a" {
		err := s.writeHistoryToFile(args[1], cmdIO, os.O_APPEND, s.history.appendLine)
		s.history.appendLine = len(s.history.lines)
		return err
	}

	start := 1

	if len(args) > 0 {
		n, err := strconv.Atoi(args[0])
		if err != nil {
			if _, err := fmt.Fprintf(cmdIO.stderr, "history: %s: numeric argument required\n", args[0]); err != nil {
				return err
			}
			return nil
		}

		start = max(len(s.history.lines)-n+1, 1)
	}

	for i, cmd := range s.history.lines[start-1:] {
		if _, err := fmt.Fprintf(cmdIO.stdout, "%d %s\n", i+start, cmd); err != nil {
			return err
		}
	}

	return nil
}

func (s *Shell) readHistoryFromFile(filepath string, cmdIO shellStream) error {
	//nolint:gosec // A shell's intended behavior is to open files specified by the user
	file, err := os.Open(filepath)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("open history file: %w", err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			if _, err := fmt.Fprintf(cmdIO.stderr, "close history file error: %v\n", err); err != nil {
				return
			}
		}
	}()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		s.history.lines = append(s.history.lines, line)
	}
	if err := scanner.Err(); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			_, printfErr := fmt.Fprintf(cmdIO.stderr, "history: %s: No such file or directory\n", filepath)
			return printfErr
		}
	}
	return nil
}

func (s *Shell) writeHistoryToFile(filepath string, cmdIO shellStream, flag int, index int) error {
	//nolint:gosec // A shell's intended behavior is to open files specified by the user
	file, err := os.OpenFile(filepath, os.O_CREATE|os.O_WRONLY|flag, 0o644)
	if err != nil {
		return fmt.Errorf("open history file: %w", err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			if _, err := fmt.Fprintf(cmdIO.stderr, "close history file error: %v\n", err); err != nil {
				return
			}
		}
	}()

	writer := bufio.NewWriter(file)
	for _, cmd := range s.history.lines[index:] {
		if _, err := writer.WriteString(cmd + "\n"); err != nil {
			return fmt.Errorf("write history to file: %w", err)
		}
	}
	if err := writer.Flush(); err != nil {
		return fmt.Errorf("flush history to file: %w", err)
	}
	return nil
}

func (s *Shell) cmdJobs(_ []string, cmdIO shellStream) error {
	s.showJobs(cmdIO, false)
	return nil
}

func (s *Shell) showJobs(cmdIO shellStream, onlyDone bool) {
	s.jobs.mutex.Lock()
	defer s.jobs.mutex.Unlock()
	for i, job := range s.jobs.jobs {
		if onlyDone && job.status != "Done" {
			continue
		}
		if job.status == "Done" {
			job.hasShowed = true
		}
		indicator := " "
		if i == len(s.jobs.jobs)-1 {
			indicator = "+"
		}
		if i == len(s.jobs.jobs)-2 {
			indicator = "-"
		}
		_, _ = fmt.Fprintf(
			cmdIO.stdout,
			"[%d]%s  %-24s %s\n",
			job.id,
			indicator,
			job.status,
			job.command,
		)
	}
	newJobs := make([]*job, 0)
	for _, job := range s.jobs.jobs {
		if !job.hasShowed {
			newJobs = append(newJobs, job)
		}
	}
	s.jobs.jobs = newJobs
}

func (s *Shell) cmdComplete(args []string, cmdIO shellStream) error {
	if len(args) < 1 {
		_, _ = fmt.Fprintln(cmdIO.stderr, "complete: missing argument")
		return nil
	}
	switch args[0] {
	case "-C":
		if len(args) != 3 {
			return fmt.Errorf("complete: -C option requires exactly 2 arguments")
		}
		path := args[1]
		command := args[2]
		s.completers[command] = path
	case "-r":
		if len(args) != 2 {
			return fmt.Errorf("complete: -r option requires exactly 1 argument")
		}
		command := args[1]
		delete(s.completers, command)
	case "-p":
		if len(args) != 2 {
			return fmt.Errorf("complete: -p option requires exactly 1 argument")
		}
		command := args[1]
		path, found := s.completers[command]
		if found {
			_, _ = fmt.Fprintf(cmdIO.stdout, "complete -C '%s' %s\n", path, command)
		} else {
			_, _ = fmt.Fprintf(cmdIO.stderr, "complete: %s: no completion specification\n", command)
		}
	}
	return nil
}

func (s *Shell) cmdDeclare(args []string, cmdIO shellStream) error {
	if len(args) < 1 {
		return fmt.Errorf("declare: missing argument")
	}
	switch args[0] {
	case "-p":
		if len(args) != 2 {
			return fmt.Errorf("declare: -p option requires exactly 1 argument")
		}
		name := args[1]
		value, found := s.declares[name]
		if found {
			_, _ = fmt.Fprintf(cmdIO.stdout, "declare -- %s=\"%s\"\n", name, value)
		} else {
			_, _ = fmt.Fprintf(cmdIO.stderr, "declare: %s: not found\n", name)
		}
	default:
		parts := strings.SplitN(args[0], "=", 2)
		if len(parts) != 2 {
			return fmt.Errorf("declare: invalid argument: %s", args[0])
		}
		s.declares[parts[0]] = parts[1]
	}
	return nil
}
