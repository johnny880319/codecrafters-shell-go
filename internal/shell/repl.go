package shell

import (
	"bufio"
	"fmt"
	"io"
)

const prompt = "$ "

// RunOnce reads one command and writes a single shell response.
// This is a small, testable unit that main() can call.
func RunOnce(in io.Reader, out io.Writer, errOut io.Writer) error {
	if _, err := fmt.Fprint(out, prompt); err != nil {
		return fmt.Errorf("write prompt: %w", err)
	}

	scanner := bufio.NewScanner(in)
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			if _, writeErr := fmt.Fprintf(errOut, "read command: %v\n", err); writeErr != nil {
				return fmt.Errorf("write read error: %w", writeErr)
			}

			return fmt.Errorf("read command: %w", err)
		}

		return io.EOF
	}

	command := scanner.Text()
	if _, err := fmt.Fprintf(out, "%s: command not found\n", command); err != nil {
		return fmt.Errorf("write command output: %w", err)
	}

	return nil
}
