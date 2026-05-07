package shell

import (
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
		"declare":  {s.cmdDeclare, false},
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

func (s *Shell) cmdJobs(_ []string, cmdIO shellStream) error {
	s.showJobs(cmdIO, false)
	return nil
}

func (s *Shell) cmdComplete(args []string, cmdIO shellStream) error {
	if len(args) < 1 {
		_, _ = fmt.Fprintln(cmdIO.stderr, "complete: missing argument")
		return nil
	}
	switch args[0] {
	case "-C":
		if len(args) != 3 {
			_, _ = fmt.Fprintln(cmdIO.stderr, "complete: -C option requires exactly 2 arguments")
			return nil
		}
		path := args[1]
		command := args[2]
		s.completers[command] = path
	case "-r":
		if len(args) != 2 {
			_, _ = fmt.Fprintln(cmdIO.stderr, "complete: -r option requires exactly 1 argument")
			return nil
		}
		command := args[1]
		delete(s.completers, command)
	case "-p":
		if len(args) != 2 {
			_, _ = fmt.Fprintln(cmdIO.stderr, "complete: -p option requires exactly 1 argument")
			return nil
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
		_, _ = fmt.Fprintln(cmdIO.stderr, "declare: missing argument")
		return nil
	}
	switch args[0] {
	case "-p":
		if len(args) != 2 {
			_, _ = fmt.Fprintln(cmdIO.stderr, "declare: -p option requires exactly 1 argument")
			return nil
		}
		name := args[1]
		value, found := s.variables[name]
		if found {
			_, _ = fmt.Fprintf(cmdIO.stdout, "declare -- %s=\"%s\"\n", name, value)
		} else {
			_, _ = fmt.Fprintf(cmdIO.stderr, "declare: %s: not found\n", name)
		}
	default:
		parts := strings.SplitN(args[0], "=", 2)
		if len(parts) != 2 {
			_, _ = fmt.Fprintf(cmdIO.stderr, "declare: invalid argument: %s\n", args[0])
			return nil
		}
		name, value := parts[0], parts[1]
		if name == "" || !isIdentifierStart(name[0]) {
			_, _ = fmt.Fprintf(cmdIO.stderr, "declare: `%s=%s': not a valid identifier\n", name, value)
			return nil
		}
		for i := 1; i < len(name); i++ {
			if !isIdentifierPart(name[i]) {
				_, _ = fmt.Fprintf(cmdIO.stderr, "declare: `%s=%s': not a valid identifier\n", name, value)
				return nil
			}
		}
		s.variables[name] = value
	}
	return nil
}
