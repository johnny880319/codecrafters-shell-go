package shell

import (
	"bufio"
	"fmt"
	"io"
)

const prompt = "$ "

// RunOnce reads a single command from in, executes it, and writes the output to out.
func RunOnce(in io.Reader, out io.Writer) error {
	if _, err := fmt.Fprint(out, prompt); err != nil {
		return fmt.Errorf("write prompt: %w", err)
	}

	scanner := bufio.NewScanner(in)
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
