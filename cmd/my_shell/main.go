package main

import (
	"fmt"
	"os"

	"github.com/codecrafters-io/shell-starter-go/internal/shell"
)

func main() {
	myShell := shell.NewShell(os.Stdout)
	err := myShell.Repl(os.Stdin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "shell error: %v\n", err)
		os.Exit(1)
	}
}
