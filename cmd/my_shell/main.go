package main

import (
	"fmt"
	"os"

	"github.com/codecrafters-io/shell-starter-go/internal/shell"
)

func main() {
	err := shell.Repl(os.Stdin, os.Stdout, os.Getenv("PATH"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "shell error: %v\n", err)
		os.Exit(1)
	}
}
