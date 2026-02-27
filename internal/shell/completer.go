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
	matches := c.getMatchStrings(lineStr)

	if len(matches) == 0 {
		c.tabCount = 0
		if _, err := fmt.Fprint(c.shell.stream.stdout, "\x07"); err != nil {
			return nil, len(lineStr)
		}
		return nil, len(lineStr)
	}

	if len(matches) == 1 {
		c.tabCount = 0
		return [][]rune{[]rune(matches[0] + " ")}, len(lineStr)
	}

	if longest := longestCommonPrefix(matches); longest != "" && longest != lineStr {
		c.tabCount = 0
		return [][]rune{[]rune(longest)}, len(lineStr)
	}

	if c.tabCount == 0 {
		if _, err := fmt.Fprint(c.shell.stream.stdout, "\x07"); err != nil {
			return nil, len(lineStr)
		}
		c.tabCount = 1
		return nil, len(lineStr)
	}

	c.tabCount = 0

	if _, err := fmt.Fprintln(c.shell.stream.stdout); err != nil {
		return nil, len(lineStr)
	}
	for i, match := range matches {
		if _, err := fmt.Fprint(c.shell.stream.stdout, string(line[:pos])+match); err != nil {
			return nil, len(lineStr)
		}
		if i < len(matches)-1 {
			if _, err := fmt.Fprint(c.shell.stream.stdout, "  "); err != nil {
				return nil, len(lineStr)
			}
		}
	}

	if _, err := fmt.Fprint(c.shell.stream.stdout, "\n"+prompt+string(line[:pos])); err != nil {
		return nil, len(lineStr)
	}

	return nil, len(lineStr)
}

func longestCommonPrefix(strs []string) string {
	if len(strs) == 0 {
		return ""
	}
	prefix := strs[0]
	for _, s := range strs[1:] {
		for !strings.HasPrefix(s, prefix) {
			prefix = prefix[:len(prefix)-1]
			if prefix == "" {
				return ""
			}
		}
	}
	return prefix
}

func (c *customCompleter) getMatchStrings(lineStr string) []string {
	var matches []string
	splitedStr := strings.Split(lineStr, " ")
	if len(splitedStr) == 1 {
		for builtin := range c.shell.commandFuncMap {
			if strings.HasPrefix(builtin, lineStr) {
				completion := builtin[len(lineStr):]
				matches = append(matches, completion)
			}
		}

		for _, cmd := range c.findPrefixExecutables(lineStr) {
			completion := cmd[len(lineStr):]
			matches = append(matches, completion)
		}
	} else {
		lastPart := splitedStr[len(splitedStr)-1]
		for _, cmd := range c.findPrefixPaths(lastPart) {
			completion := cmd[len(lastPart):]
			matches = append(matches, completion)
		}
	}
	slices.Sort(matches)
	return slices.Compact(matches)
}

func (c *customCompleter) findPrefixExecutables(prefix string) []string {
	matches := make(map[string]struct{})
	paths := filepath.SplitList(c.shell.env.path)
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

func (c *customCompleter) findPrefixPaths(prefix string) []string {
	absPathPrefix := c.shell.workingDir
	if strings.HasPrefix(prefix, "/") {
		absPathPrefix = prefix
	} else {
		if strings.HasPrefix(prefix, "~") {
			absPathPrefix = os.Getenv("HOME")
			prefix = prefix[1:]
		}
		absPathPrefix = filepath.Join(absPathPrefix, prefix)
	}

	dir := filepath.Dir(absPathPrefix)
	basePrefix := filepath.Base(absPathPrefix)

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	var matches []string
	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), basePrefix) {
			continue
		}
		matches = append(matches, filepath.Join(dir, entry.Name()))
	}
	return matches
}
