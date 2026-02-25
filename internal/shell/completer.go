package shell

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

type customCompleter struct {
	shell    *Shell
	tabCount int
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

	if len(matches) == 0 || (len(matches) > 1 && c.tabCount == 0) {
		if _, err := fmt.Fprint(c.shell.stdOut, "\x07"); err != nil {
			return nil, len(lineStr)
		}
		if len(matches) > 1 {
			c.tabCount = 1
		} else {
			c.tabCount = 0
		}
		return nil, len(lineStr)
	}

	c.tabCount = 0

	slices.SortFunc(matches, slices.Compare)
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
