package shell

import (
	"fmt"
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
			fields = append(fields, curField)
			curField = ""
			continue
		}
		str, escaped, inSingleQuotes, inDoubleQuotes = parseCharacter(r, escaped, inSingleQuotes, inDoubleQuotes)
		curField += str
	}

	fields = append(fields, curField)

	return fields
}

func splitPipeline(input []string) ([][]string, error) {
	var pipeline [][]string
	currentCommand := []string{}
	for _, arg := range input {
		switch arg {
		case "":
			continue
		case "|":
			pipeline = append(pipeline, currentCommand)
			currentCommand = []string{}
		default:
			currentCommand = append(currentCommand, arg)
		}
	}
	if len(currentCommand) == 0 {
		return nil, fmt.Errorf("syntax error: unexpected token `|'")
	}
	pipeline = append(pipeline, currentCommand)
	return pipeline, nil
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

type redirect struct {
	fd         int
	appendFlag int
	file       string // currently only supports file path.
}

func parseRedirect(args []string) ([]string, []redirect, error) {
	redirectMap := map[string]struct {
		fd         int
		appendFlag int
	}{
		">":   {1, os.O_TRUNC},
		"1>":  {1, os.O_TRUNC},
		"2>":  {2, os.O_TRUNC},
		">>":  {1, os.O_APPEND},
		"1>>": {1, os.O_APPEND},
		"2>>": {2, os.O_APPEND},
	}
	for i, arg := range args {
		r, ok := redirectMap[arg]
		if !ok {
			continue
		}
		if i+1 >= len(args) {
			return nil, nil, fmt.Errorf("syntax error near unexpected token `%s'", arg)
		}

		return append(args[:i], args[i+2:]...), []redirect{{fd: r.fd, appendFlag: r.appendFlag, file: args[i+1]}}, nil
	}
	return args, nil, nil
}
