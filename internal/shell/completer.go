package shell

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type customCompleter struct {
	shell *Shell
}

// Do implements readline.AutoCompleter to provide tab completion for built-in commands.
func (c *customCompleter) Do(line []rune, pos int) (newLine [][]rune, length int) {
	lineStr := string(line[:pos])
	var matches [][]rune

	for builtin := range c.shell.commandFuncMap {
		if strings.HasPrefix(builtin, lineStr) {
			completion := builtin[len(lineStr):] + " "
			matches = append(matches, []rune(completion))
		}
	}

	for _, cmd := range c.findPrefixExecutables(lineStr) {
		completion := cmd[len(lineStr):] + " "
		matches = append(matches, []rune(completion))
	}

	if len(matches) == 0 {
		if _, err := fmt.Fprint(c.shell.stdOut, "\x07"); err != nil {
			return nil, len(lineStr)
		}
		return nil, len(lineStr)
	}

	return matches, len(lineStr)
}

func (c *customCompleter) findPrefixExecutables(prefix string) []string {
	matches := make(map[string]struct{})
	paths := filepath.SplitList(c.shell.sysPath)
	for _, dir := range paths {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			if !strings.HasPrefix(entry.Name(), prefix) {
				continue
			}
			info, err := entry.Info()
			if err != nil {
				continue
			}
			if info.Mode()&0o111 == 0 {
				continue
			}
			matches[entry.Name()] = struct{}{}
		}
	}
	result := make([]string, 0, len(matches))
	for match := range matches {
		result = append(result, match)
	}
	return result
}
