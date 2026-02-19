package main

import (
	"errors"
	"io"
	"os"

	"github.com/codecrafters-io/shell-starter-go/internal/shell"
)

func main() {
	err := shell.RunOnce(os.Stdin, os.Stdout, os.Stderr)
	if err != nil && !errors.Is(err, io.EOF) {
		os.Exit(1)
	}
}
