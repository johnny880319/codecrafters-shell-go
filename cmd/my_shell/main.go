// Entry point for the shell application.
package main

import (
	"fmt"
	"os"

	"github.com/codecrafters-io/shell-starter-go/internal/shell"
)

func main() {
	myShell, err := shell.NewShell(os.Stdin, os.Stdout)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to initialize shell: %v\n", err)
		os.Exit(1)
	}
	err = myShell.Repl()
	if err != nil {
		fmt.Fprintf(os.Stderr, "shell error: %v\n", err)
		os.Exit(1)
	}
}
