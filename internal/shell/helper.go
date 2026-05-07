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

func (s *Shell) replaceVariables(splitedInput []string) ([]string, error) {
	for i, token := range splitedInput {
		replacedArg, err := s.replaceToken(token)
		if err != nil {
			return nil, err
		}
		splitedInput[i] = replacedArg
	}
	return splitedInput, nil
}

func (s *Shell) replaceToken(token string) (string, error) {
	replacedArg := ""
	cursor := 0
	for {
		idx := strings.Index(token[cursor:], "$")
		if idx == -1 {
			replacedArg += token[cursor:]
			break
		}
		replacedArg += token[cursor : cursor+idx]
		cursor += idx + 1
		if cursor >= len(token) {
			break
		}
		newCursor, end, err := computeReplaceRange(token, cursor)
		if err != nil {
			return "", err
		}
		cursor = newCursor

		varName := token[cursor:end]
		var varvalue string
		if value, ok := s.declares[varName]; ok {
			varvalue = value
		} else {
			varvalue = ""
		}
		replacedArg += varvalue
		cursor = end + 1
		if cursor >= len(token) {
			break
		}
	}
	return replacedArg, nil
}

func computeReplaceRange(token string, cursor int) (int, int, error) {
	if token[cursor] == '{' {
		cursor++
		end := strings.Index(token[cursor:], "}")
		if end == -1 {
			return 0, 0, fmt.Errorf("syntax error: missing `}'")
		}
		return cursor, cursor + end, nil
	}

	end := cursor
	if !isIdentifierStart(token[cursor]) {
		return 0, 0, fmt.Errorf("syntax error: invalid variable name")
	}
	end++

	for end < len(token) && isIdentifierPart(token[end]) {
		end++
	}
	return cursor, end, nil
}

func isIdentifierStart(ch byte) bool {
	return ch == '_' || (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z')
}

func isIdentifierPart(ch byte) bool {
	return isIdentifierStart(ch) || (ch >= '0' && ch <= '9')
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
