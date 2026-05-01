package shell

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func handleQuotesAndEscapes(input string) []string {
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

	return fields
}

func splitPipeline(input []string) [][]string {
	var pipeline [][]string
	currentCommand := []string{}
	for _, arg := range input {
		if arg == "|" {
			pipeline = append(pipeline, currentCommand)
			currentCommand = []string{}
		} else {
			currentCommand = append(currentCommand, arg)
		}
	}
	if len(currentCommand) > 0 {
		pipeline = append(pipeline, currentCommand)
	}
	return pipeline
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
	paths := filepath.SplitList(s.env.path)
	for _, dir := range paths {
		fullPath := filepath.Join(dir, command)
		if _, err := exec.LookPath(fullPath); err == nil {
			return fullPath, true
		}
	}
	return "", false
}

func handleRedirect(args []string, currStdout *io.Writer, currStderr *io.Writer) ([]string, io.Closer, error) {
	redirectMap := map[string]struct {
		flag   int
		writer *io.Writer
	}{
		">":   {os.O_TRUNC, currStdout},
		"1>":  {os.O_TRUNC, currStdout},
		"2>":  {os.O_TRUNC, currStderr},
		">>":  {os.O_APPEND, currStdout},
		"1>>": {os.O_APPEND, currStdout},
		"2>>": {os.O_APPEND, currStderr},
	}
	for i, arg := range args {
		if redirect, ok := redirectMap[arg]; ok && i < len(args)-1 {
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
