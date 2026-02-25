package shell

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

//nolint:gocognit // Will be refactored in a future exercise
func parseCommand(input string) (command string, args []string) {
	fields := []string{}
	escaped := false
	inSingleQuotes := false
	inDoubleQuotes := false
	curField := ""
	for _, r := range input {
		if escaped && (inDoubleQuotes && !strings.ContainsRune("\"\\$`\n", r)) {
			curField += "\\"
			curField += string(r)
			escaped = false
			continue
		}
		if escaped {
			curField += string(r)
			escaped = false
			continue
		}
		if r == '\\' && !inSingleQuotes {
			escaped = true
			continue
		}
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
