package shell

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func parseCommand(input string) (command string, args []string) {
	fields := []string{}
	var str string
	escaped := false
	inSingleQuotes := false
	inDoubleQuotes := false
	curField := ""
	for _, r := range input {
		if r == ' ' && !inSingleQuotes && !inDoubleQuotes && !escaped {
			if curField != "" {
				fields = append(fields, curField)
				curField = ""
			}
			continue
		}
		str, escaped, inSingleQuotes, inDoubleQuotes = parseCharacter(r, escaped, inSingleQuotes, inDoubleQuotes)
		curField += str
	}

	if curField != "" {
		fields = append(fields, curField)
	}

	if len(fields) == 0 {
		return "", nil
	}
	return fields[0], fields[1:]
}

func parseCharacter(r rune, escaped, inSingleQuotes, inDoubleQuotes bool) (string, bool, bool, bool) {
	if escaped && (inDoubleQuotes && !strings.ContainsRune("\"\\$`\n", r)) {
		return "\\" + string(r), false, inSingleQuotes, inDoubleQuotes
	}
	if escaped {
		return string(r), false, inSingleQuotes, inDoubleQuotes
	}
	if r == '\\' && !inSingleQuotes {
		return "", true, inSingleQuotes, inDoubleQuotes
	}
	if r == '\'' && !inDoubleQuotes {
		return "", escaped, !inSingleQuotes, inDoubleQuotes
	}
	if r == '"' && !inSingleQuotes {
		return "", escaped, inSingleQuotes, !inDoubleQuotes
	}
	return string(r), escaped, inSingleQuotes, inDoubleQuotes
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

func (s *Shell) handleRedirect(args []string) ([]string, *os.File, error) {
	redirect_map := map[string]struct {
		flag   int
		writer *io.Writer
	}{
		">":   {os.O_TRUNC, &s.stdOut},
		"1>":  {os.O_TRUNC, &s.stdOut},
		"2>":  {os.O_TRUNC, &s.stdErr},
		">>":  {os.O_APPEND, &s.stdOut},
		"1>>": {os.O_APPEND, &s.stdOut},
		"2>>": {os.O_APPEND, &s.stdErr},
	}
	for i, arg := range args {
		if redirect, ok := redirect_map[arg]; ok && i < len(args)-1 {
			filename := args[i+1]
			//nolint:gosec // Opening files based on user input is the intended behavior of a shell
			file, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|redirect.flag, 0o644)
			if err != nil {
				return nil, nil, fmt.Errorf("%s: %s", filename, err.Error())
			}
			*redirect.writer = file
			return append(args[:i], args[i+2:]...), file, nil
		}
	}
	return args, nil, nil
}
