package shell

import (
	"bufio"
	"errors"
	"fmt"
	"io"
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

	command := scanner.Text()
	//nolint:gosec // This is plain terminal output, not HTML/JS rendering.
	if _, err := fmt.Fprintf(out, "%s: command not found\n", command); err != nil {
		return fmt.Errorf("write command output: %w", err)
	}

	return nil
}
