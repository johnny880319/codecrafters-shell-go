package shell

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strings"
)

const prompt = "$ "

// Repl starts a read-eval-print loop that reads commands from in, executes them, and writes output to out.
func Repl(in io.Reader, out io.Writer) error {
	scanner := bufio.NewScanner(in)
	for {
		if err := runOnce(scanner, out); err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
	}
}

func runOnce(scanner *bufio.Scanner, out io.Writer) error {
	if _, err := fmt.Fprint(out, prompt); err != nil {
		return fmt.Errorf("write prompt: %w", err)
	}

	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return fmt.Errorf("read command: %w", err)
		}
		return io.EOF
	}

	input := scanner.Text()
	command, args := parseCommand(input)
	if commandFunc, ok := commandFuncMap[command]; ok {
		return commandFunc(args, out)
	}
	//nolint:gosec // This is plain terminal output, not HTML/JS rendering.
	if _, err := fmt.Fprintf(out, "%s: command not found\n", input); err != nil {
		return fmt.Errorf("write command output: %w", err)
	}

	return nil
}

func parseCommand(input string) (command string, args []string) {
	fields := strings.Fields(input)
	if len(fields) == 0 {
		return "", nil
	}
	return fields[0], fields[1:]
}

var commandFuncMap map[string]func(args []string, out io.Writer) error

func cmdExit(_ []string, _ io.Writer) error {
	return io.EOF
}

func cmdEcho(args []string, out io.Writer) error {
	//nolint:gosec // This is plain terminal output, not HTML/JS rendering.
	if _, err := fmt.Fprintln(out, strings.Join(args, " ")); err != nil {
		return err
	}
	return nil
}

func cmdType(args []string, out io.Writer) error {
	for _, arg := range args {
		if _, ok := commandFuncMap[arg]; ok {
			//nolint:gosec // This is plain terminal output, not HTML/JS rendering.
			if _, err := fmt.Fprintf(out, "%s is a shell builtin\n", arg); err != nil {
				return err
			}
		} else {
			//nolint:gosec // This is plain terminal output, not HTML/JS rendering.
			if _, err := fmt.Fprintf(out, "%s: not found\n", arg); err != nil {
				return err
			}
		}
	}
	return nil
}

func init() {
	commandFuncMap = map[string]func(args []string, out io.Writer) error{
		"exit": cmdExit,
		"echo": cmdEcho,
		"type": cmdType,
	}
}
