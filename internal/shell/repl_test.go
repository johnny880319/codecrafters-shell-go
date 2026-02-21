package shell

import (
	"bytes"
	"strings"
	"testing"
)

func TestPrintAPrompt(t *testing.T) {
	t.Parallel()
	testTemplate(
		t,
		"",
		"$ ",
	)
}

func TestHandleInvalidCommands(t *testing.T) {
	t.Parallel()
	testTemplate(
		t,
		"xyz\n",
		"$ xyz: command not found\n$ ",
	)
}

func TestImplementARepl(t *testing.T) {
	t.Parallel()
	testTemplate(
		t,
		strings.Join([]string{
			"invalid_command_1\n",
			"invalid_command_2\n",
			"invalid_command_3\n",
		}, ""),
		strings.Join([]string{
			"$ ",
			"invalid_command_1: command not found\n",
			"$ ",
			"invalid_command_2: command not found\n",
			"$ ",
			"invalid_command_3: command not found\n",
			"$ ",
		}, ""),
	)
}

func TestImplementExit(t *testing.T) {
	t.Parallel()
	testTemplate(
		t,
		strings.Join([]string{
			"invalid_command_1\n",
			"exit\n",
		}, ""),
		strings.Join([]string{
			"$ ",
			"invalid_command_1: command not found\n",
			"$ ",
		}, ""),
	)
}

func TestImplementEcho(t *testing.T) {
	t.Parallel()
	testTemplate(
		t,
		strings.Join([]string{
			"echo hello world\n",
			"echo pineapple strawberry\n",
		}, ""),
		strings.Join([]string{
			"$ ",
			"hello world\n",
			"$ ",
			"pineapple strawberry\n",
			"$ ",
		}, ""),
	)
}

func TestImplementType(t *testing.T) {
	t.Parallel()
	// $ type echo
	// echo is a shell builtin
	// $ type exit
	// exit is a shell builtin
	// $ type type
	// type is a shell builtin
	// $ type invalid_command
	// invalid_command: not found
	// $
	testTemplate(
		t,
		strings.Join([]string{
			"type echo\n",
			"type exit\n",
			"type type\n",
			"type invalid_command\n",
		}, ""),
		strings.Join([]string{
			"$ ",
			"echo is a shell builtin\n",
			"$ ",
			"exit is a shell builtin\n",
			"$ ",
			"type is a shell builtin\n",
			"$ ",
			"invalid_command: not found\n",
			"$ ",
		}, ""),
	)
}

func testTemplate(t *testing.T, input string, expectedOutput string) {
	var out bytes.Buffer
	err := Repl(strings.NewReader(input), &out)
	if err != nil {
		t.Fatalf("Repl() error = %v, want nil", err)
	}

	if got, want := out.String(), expectedOutput; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
}
