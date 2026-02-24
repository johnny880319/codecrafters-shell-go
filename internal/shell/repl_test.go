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
		WithSysPath("/usr/bin:/usr/local/bin:"+os.Getenv("PATH")),
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
	)
}

func TestThePwdBuiltin(t *testing.T) {
	t.Parallel()
	testTemplate(
		t,
		"pwd\n",
		strings.Join([]string{
			"$ ",
			"/usr/local/bin\n",
			"$ ",
		}, ""),
		WithWorkingDir("/usr/local/bin"),
	)
}

func TestTheCdBuiltinAbsolutePaths(t *testing.T) {
	t.Parallel()
	testTemplate(
		t,
		strings.Join([]string{
			"cd /usr/local/bin\n",
			"pwd\n",
			"cd /does_not_exist\n",
		}, ""),
		strings.Join([]string{
			"$ ",
			"$ ",
			"/usr/local/bin\n",
			"$ ",
			"cd: /does_not_exist: No such file or directory\n",
			"$ ",
		}, ""),
	)
}

func TestTheCdBuiltinRelativePaths(t *testing.T) {
	t.Parallel()
	testTemplate(
		t,
		strings.Join([]string{
			"cd /usr\n",
			"pwd\n",
			"cd ./local/bin\n",
			"pwd\n",
			"cd ../../\n",
			"pwd\n",
		}, ""),
		strings.Join([]string{
			"$ ",
			"$ ",
			"/usr\n",
			"$ ",
			"$ ",
			"/usr/local/bin\n",
			"$ ",
			"$ ",
			"/usr\n",
			"$ ",
		}, ""),
	)
}

func TestTheCdBuiltinHomeDirectory(t *testing.T) {
	t.Parallel()
	testTemplate(
		t,
		strings.Join([]string{
			"cd /usr/local/bin\n",
			"pwd\n",
			"cd ~\n",
			"pwd\n",
		}, ""),
		strings.Join([]string{
			"$ ",
			"$ ",
			"/usr/local/bin\n",
			"$ ",
			"$ ",
			os.Getenv("HOME") + "\n",
			"$ ",
		}, ""),
	)
}

func TestSingleQuotes(t *testing.T) {
	t.Parallel()
	testTemplate(
		t,
		strings.Join([]string{
			"echo 'shell hello'\n",
			"echo 'world     test'\n",
		}, ""),
		strings.Join([]string{
			"$ ",
			"shell hello\n",
			"$ ",
			"world     test\n",
			"$ ",
		}, ""),
	)
}

func TestDoubleQuotes(t *testing.T) {
	t.Parallel()
	testTemplate(
		t,
		strings.Join([]string{
			"echo \"quz  hello\"  \"bar\"\n",
			"echo \"bar\"  \"shell's\"  \"foo\"\n",
		}, ""),
		strings.Join([]string{
			"$ ",
			"quz  hello bar\n",
			"$ ",
			"bar shell's foo\n",
			"$ ",
		}, ""),
	)
}

func TestBackslashOutsideQuotes(t *testing.T) {
	t.Parallel()
	testTemplate(
		t,
		strings.Join([]string{
			"echo multiple\\ \\ \\ \\ spaces\n",
			"echo \\'\\\"literal quotes\\\"\\'\n",
			"echo ignore\\_backslash\n",
		}, ""),
		strings.Join([]string{
			"$ ",
			"multiple    spaces\n",
			"$ ",
			"'\"literal quotes\"'\n",
			"$ ",
			"ignore_backslash\n",
			"$ ",
		}, ""),
	)
}

func testTemplate(t *testing.T, input string, expectedOutput string, opts ...Option) {
	var out bytes.Buffer
	myShell := NewShell(&out, opts...)
	err := myShell.Repl(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Repl() error = %v, want nil", err)
	}

	if got, want := out.String(), expectedOutput; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
}
