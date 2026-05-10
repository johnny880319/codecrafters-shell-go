package shell

import (
	"fmt"
	"os"
	"strings"
)

type commandLine struct {
	pipeline     []commandSegment
	isBackground bool
}

type commandSegment struct {
	command   string
	args      []string
	redirects []redirect
}

type redirect struct {
	fd         int
	appendFlag int
	file       string // currently only supports file path.
}

func (s *Shell) parseInput(input string) (commandLine, error) {
	cl := commandLine{}
	input = strings.TrimSpace(input)
	if len(input) > 0 && input[len(input)-1] == '&' {
		cl.isBackground = true
		input = strings.TrimSpace(input[:len(input)-1])
	}
	inputTokens := handleQuotesAndEscapes(input)
	splitedInput, err := s.expandVariables(inputTokens)
	if err != nil {
		return commandLine{}, err
	}
	pipelineStr, err := splitPipeline(splitedInput)
	if err != nil {
		return cl, err
	}

	for _, cmdStr := range pipelineStr {
		if len(cmdStr) == 0 {
			return cl, fmt.Errorf("empty command")
		}
		command, args := cmdStr[0], cmdStr[1:]

		args, redirects, err := parseRedirect(args)
		if err != nil {
			return cl, err
		}
		cl.pipeline = append(cl.pipeline, commandSegment{
			command:   command,
			args:      args,
			redirects: redirects,
		})
	}
	return cl, nil
}

type inputToken struct {
	value          string
	quotedBySingle bool
}

func handleQuotesAndEscapes(input string) []inputToken {
	fields := []inputToken{}
	var str string
	escaped := false
	inSingleQuotes := false
	inDoubleQuotes := false
	quotedBySingle := false
	curField := ""
	for _, r := range input {
		if r == ' ' && !inSingleQuotes && !inDoubleQuotes && !escaped {
			fields = append(fields, inputToken{value: curField, quotedBySingle: quotedBySingle})
			curField = ""
			quotedBySingle = false
			continue
		}
		str, escaped, inSingleQuotes, inDoubleQuotes = parseCharacter(r, escaped, inSingleQuotes, inDoubleQuotes)
		quotedBySingle = quotedBySingle || inSingleQuotes
		curField += str
	}

	fields = append(fields, inputToken{value: curField, quotedBySingle: quotedBySingle})

	return fields
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

func (s *Shell) expandVariables(tokens []inputToken) ([]string, error) {
	expanded := make([]string, len(tokens))
	for i, token := range tokens {
		if token.quotedBySingle {
			expanded[i] = token.value
			continue
		}
		replacedArg, err := s.expandTokenVariables(token.value)
		if err != nil {
			return nil, err
		}
		expanded[i] = replacedArg
	}
	return expanded, nil
}

func (s *Shell) expandTokenVariables(token string) (string, error) {
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
		replacedValue, nextCursor, err := s.expandVariableAt(token, cursor)
		if err != nil {
			return "", err
		}

		replacedArg += replacedValue
		cursor = nextCursor
		if cursor >= len(token) {
			break
		}
	}
	return replacedArg, nil
}

func (s *Shell) expandVariableAt(token string, cursor int) (string, int, error) {
	var replaced string
	var nextCursor int
	if token[cursor] == '{' {
		cursor++
		end := strings.Index(token[cursor:], "}")
		if end == -1 {
			return "", 0, fmt.Errorf("syntax error: missing `}'")
		}
		replaced = token[cursor : cursor+end]
		nextCursor = cursor + end + 1
	} else {
		end := cursor
		if !isIdentifierStart(token[cursor]) {
			return "", 0, fmt.Errorf("syntax error: invalid variable name")
		}
		end++

		for end < len(token) && isIdentifierPart(token[end]) {
			end++
		}
		replaced = token[cursor:end]
		nextCursor = end
	}

	if value, ok := s.variables[replaced]; ok {
		return value, nextCursor, nil
	} else {
		return "", nextCursor, nil
	}
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
