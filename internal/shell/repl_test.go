package shell

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestPrintAPrompt(t *testing.T) {
	t.Parallel()
	testTemplate(
		t,
		"",
		"$ ",
		os.Getenv("PATH"),
	)
}

func TestHandleInvalidCommands(t *testing.T) {
	t.Parallel()
	testTemplate(
		t,
		"xyz\n",
		"$ xyz: command not found\n$ ",
		os.Getenv("PATH"),
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
		os.Getenv("PATH"),
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
		os.Getenv("PATH"),
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
		os.Getenv("PATH"),
	)
}

func TestImplementType(t *testing.T) {
	t.Parallel()
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
		os.Getenv("PATH"),
	)
}

func TestLocateExecutableFiles(t *testing.T) {
	t.Parallel()
	testTemplate(
		t,
		strings.Join([]string{
			"type ls\n",
			"type basename\n",
			"type invalid_command\n",
		}, ""),
		strings.Join([]string{
			"$ ",
			"ls is /usr/bin/ls\n",
			"$ ",
			"basename is /usr/bin/basename\n",
			"$ ",
			"invalid_command: not found\n",
			"$ ",
		}, ""),
		"/usr/bin:/usr/local/bin:"+os.Getenv("PATH"),
	)
}

func TestRunAProgram(t *testing.T) {
	t.Parallel()
	testTemplate(
		t,
		"basename /hello/world/golang\n",
		strings.Join([]string{
			"$ ",
			"golang\n",
			"$ ",
		}, ""),
		os.Getenv("PATH"),
	)
}

func testTemplate(t *testing.T, input string, expectedOutput string, sysPath string) {
	var out bytes.Buffer
	err := Repl(strings.NewReader(input), &out, sysPath)
	if err != nil {
		t.Fatalf("Repl() error = %v, want nil", err)
	}

	if got, want := out.String(), expectedOutput; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
}
