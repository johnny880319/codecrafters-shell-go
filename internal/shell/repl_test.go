package shell

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
	"testing/iotest"
)

func TestRunOnceUnknownCommand(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer

	err := RunOnce(strings.NewReader("hello\n"), &out)
	if err != nil {
		t.Fatalf("RunOnce() error = %v, want nil", err)
	}

	if got, want := out.String(), "$ hello: command not found\n"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
}

func TestRunOnceEOF(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer

	err := RunOnce(strings.NewReader(""), &out)
	if !errors.Is(err, io.EOF) {
		t.Fatalf("RunOnce() error = %v, want io.EOF", err)
	}

	if got, want := out.String(), "$ "; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
}

func TestRunOnceReadError(t *testing.T) {
	t.Parallel()

	readErr := errors.New("boom")
	var out bytes.Buffer

	err := RunOnce(iotest.ErrReader(readErr), &out)
	if err == nil {
		t.Fatal("RunOnce() error = nil, want non-nil")
	}

	if !strings.Contains(err.Error(), "read command") {
		t.Fatalf("error = %q, want to contain %q", err.Error(), "read command")
	}

	if got, want := out.String(), "$ "; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
}
