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
	if err := s.writeHistoryToFile(s.env.histfile, os.O_APPEND, s.history.startLine); err != nil {
		_, _ = fmt.Fprintf(s.stream.stderr, "failed to save history: %v\n", err)
	}
	return io.EOF
}

func (s *Shell) cmdEcho(args []string, cmdIO shellStream) error {
	_, _ = fmt.Fprintln(cmdIO.stdout, strings.Join(args, " "))
	return nil
}

func (s *Shell) cmdType(args []string, cmdIO shellStream) error {
	for _, arg := range args {
		if _, ok := s.builtinCommandMap[arg]; ok {
			_, _ = fmt.Fprintf(cmdIO.stdout, "%s is a shell builtin\n", arg)
		} else if path, found := s.findExecutable(arg); found {
			_, _ = fmt.Fprintf(cmdIO.stdout, "%s is %s\n", arg, path)
		} else {
			_, _ = fmt.Fprintf(cmdIO.stderr, "%s: not found\n", arg)
		}
	}
	return nil
}

func (s *Shell) cmdPwd(_ []string, cmdIO shellStream) error {
	_, _ = fmt.Fprintln(cmdIO.stdout, s.workingDir)
	return nil
}

func (s *Shell) cmdCd(args []string, cmdIO shellStream) error {
	if len(args) == 0 {
		s.workingDir = os.Getenv("HOME")
		return nil
	}
	if len(args) > 1 {
		_, _ = fmt.Fprintln(cmdIO.stderr, "cd: too many arguments")
		return nil
	}

	newPath := args[0]
	if filepath.IsAbs(newPath) {
		if err := s.checkDirectory(newPath); err != nil {
			_, _ = fmt.Fprintln(cmdIO.stderr, err)
		}
		return nil
	}

	if strings.HasPrefix(newPath, "~") {
		s.workingDir = os.Getenv("HOME")
		newPath = newPath[1:]
	}
	absPath := filepath.Join(s.workingDir, newPath)
	if err := s.checkDirectory(absPath); err != nil {
		_, _ = fmt.Fprintln(cmdIO.stderr, err)
	}
	return nil
}

func (s *Shell) cmdHistory(args []string, cmdIO shellStream) error {
	if len(args) > 1 && args[0] == "-r" {
		if err := s.readHistoryFromFile(args[1]); err != nil {
			_, _ = fmt.Fprintln(cmdIO.stderr, err)
		}
		return nil
	}

	if len(args) > 0 && args[0] == "-w" {
		if err := s.writeHistoryToFile(args[1], os.O_TRUNC, 0); err != nil {
			_, _ = fmt.Fprintln(cmdIO.stderr, err)
		}
		return nil
	}

	if len(args) > 0 && args[0] == "-a" {
		if err := s.writeHistoryToFile(args[1], os.O_APPEND, s.history.appendLine); err != nil {
			_, _ = fmt.Fprintln(cmdIO.stderr, err)
		}
		s.history.appendLine = len(s.history.lines)
		return nil
	}

	start := 1

	if len(args) > 0 {
		n, err := strconv.Atoi(args[0])
		if err != nil {
			_, _ = fmt.Fprintf(cmdIO.stderr, "history: %s: numeric argument required\n", args[0])
			//nolint:nilerr // User input error was reported to stderr; keep the shell running.
			return nil
		}
		start = max(len(s.history.lines)-n+1, 1)
	}

	for i, cmd := range s.history.lines[start-1:] {
		_, _ = fmt.Fprintf(cmdIO.stdout, "%d %s\n", i+start, cmd)
	}

	return nil
}

func (s *Shell) cmdJobs(_ []string, cmdIO shellStream) error {
	return s.showJobs(cmdIO, false)
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
			_, err := fmt.Fprintf(cmdIO.stderr, "complete: %s: no completion specification\n", command)
			if err != nil {
				return fmt.Errorf("fail to write to stderr: %w", err)
			}
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
