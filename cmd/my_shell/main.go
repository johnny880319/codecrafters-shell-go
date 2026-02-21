package main

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/codecrafters-io/shell-starter-go/internal/shell"
)

func main() {
	err := shell.RunOnce(os.Stdin, os.Stdout)
	if err != nil && !errors.Is(err, io.EOF) {
		fmt.Fprintf(os.Stderr, "shell error: %v\n", err)
		os.Exit(1)
	}
}
