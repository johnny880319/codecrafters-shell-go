package shell

import "strings"

//nolint:gocognit // Will be refactored in a future exercise
func parseCommand(input string) (command string, args []string) {
	fields := []string{}
	escaped := false
	inSingleQuotes := false
	inDoubleQuotes := false
	curField := ""
	for _, r := range input {
		if escaped && (inSingleQuotes || (inDoubleQuotes && !strings.ContainsRune("\"\\$`\n", r))) {
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
		if r == '\\' {
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
