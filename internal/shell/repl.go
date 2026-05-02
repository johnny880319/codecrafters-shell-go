// Package shell implements a simple interactive shell.
package shell

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"

	"github.com/chzyer/readline"
)

const prompt = "$ "

var readlineInitMutex sync.Mutex

// Shell represents the core state of the interactive shell.
type Shell struct {
	workingDir     string
	commandFuncMap map[string]func(args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) error
	stream         shellStream
	env            shellEnv
	history        shellHistory
}

type shellStream struct {
	stdin  io.Reader
	stdout io.Writer
	stderr io.Writer
}

type shellEnv struct {
	path     string
	histfile string
}

type shellHistory struct {
	lines      []string
	startLine  int
	appendLine int
}

// Option defines a functional parameter for configuring a Shell instance.
type Option func(*Shell)

// WithSysPath overrides the default system PATH used by the Shell.
func WithSysPath(p string) Option {
	return func(s *Shell) { s.env.path = p }
}

// WithWorkingDir overrides the default initial working directory for the Shell.
func WithWorkingDir(d string) Option {
	return func(s *Shell) { s.workingDir = d }
}

// NewShell creates a new Shell instance with default OS environment variables.
func NewShell(in io.Reader, out io.Writer, opts ...Option) *Shell {
	wd, err := os.Getwd()
	if err != nil {
		wd = "/"
	}

	s := &Shell{
		stream:     shellStream{stdin: in, stdout: out, stderr: out},
		env:        shellEnv{path: os.Getenv("PATH"), histfile: os.Getenv("HISTFILE")},
		workingDir: wd,
	}

	for _, opt := range opts {
		opt(s)
	}

	s.commandFuncMap = s.getCommandFuncMap()
	if err := s.readHistoryFromFile(s.env.histfile, s.stream.stderr); err != nil {
		_, _ = fmt.Fprintf(s.stream.stderr, "read history from file error: %v\n", err)
	}
	s.history.startLine = len(s.history.lines)
	s.history.appendLine = len(s.history.lines)

	return s
}

// Repl starts a read-eval-print loop that reads commands from in, executes them, and writes output to out.
func (s *Shell) Repl() error {
	readlineInitMutex.Lock()
	rl, err := readline.NewEx(&readline.Config{
		Prompt:          prompt,
		AutoComplete:    &customCompleter{shell: s, tabCount: 0},
		InterruptPrompt: "^C",
		EOFPrompt:       "exit",
		Stdin:           io.NopCloser(s.stream.stdin),
		Stdout:          s.stream.stdout,
		Stderr:          s.stream.stderr,
	})
	readlineInitMutex.Unlock()

	if err != nil {
		return fmt.Errorf("failed to initialize readline: %w", err)
	}
	defer func() {
		_ = rl.Close()
	}()

	for {
		if _, err := fmt.Fprint(s.stream.stdout, prompt); err != nil {
			return fmt.Errorf("write prompt: %w", err)
		}
		input, err := rl.Readline()
		if err != nil {
			if errors.Is(err, readline.ErrInterrupt) {
				continue
			}
			if errors.Is(err, io.EOF) {
				return nil
			}
			return fmt.Errorf("read line error: %w", err)
		}

		if err := s.execute(input); err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
		}
	}
}

type ioFlow struct {
	stdin  io.Reader
	stdout io.Writer
	stderr io.Writer
}

//nolint:gocognit // Will be refactored in a future exercise
func (s *Shell) execute(input string) error {
	s.history.lines = append(s.history.lines, input)
	cmdline, err := parseInput(input)
	if err != nil {
		return err
	}

	var wg sync.WaitGroup

	prevReader := s.stream.stdin
	for i, pl := range cmdline.pipeline {
		var closers []io.Closer
		cmdIO := ioFlow{
			stdin:  prevReader,
			stdout: s.stream.stdout,
			stderr: s.stream.stderr,
		}

		if i > 0 {
			closer, ok := prevReader.(io.Closer)
			if !ok {
				return fmt.Errorf("previous reader is not closable")
			}
			closers = append(closers, closer)
		}
		if i < len(cmdline.pipeline)-1 {
			pr, pw := io.Pipe()
			cmdIO.stdout = pw
			prevReader = pr // used for the next round of loop
			closers = append(closers, pw)
		}

		for _, redirect := range pl.redirects {
			//nolint:gosec // Opening files based on user input is the intended behavior of a shell
			file, err := os.OpenFile(redirect.file, os.O_CREATE|os.O_WRONLY|redirect.appendFlag, 0o644)
			if err != nil {
				closeAll(closers)
				return fmt.Errorf("%s: %s", redirect.file, err.Error())
			}
			closers = append(closers, file)
			switch redirect.fd {
			case 1:
				cmdIO.stdout = file
			case 2:
				cmdIO.stderr = file
			default:
				closeAll(closers)
				return fmt.Errorf("unsupported redirect fd: %d", redirect.fd)
			}
		}

		if commandFunc, ok := s.commandFuncMap[pl.command]; ok {
			if pl.command == "cd" || pl.command == "exit" {
				err = commandFunc(pl.args, cmdIO.stdin, cmdIO.stdout, cmdIO.stderr)
				closeAll(closers)
				if errors.Is(err, io.EOF) {
					return io.EOF
				}
				continue
			}
			wg.Add(1)
			go func(args []string, cmdIO ioFlow, closers []io.Closer) {
				defer closeAll(closers)
				defer wg.Done()
				_ = commandFunc(args, cmdIO.stdin, cmdIO.stdout, cmdIO.stderr)
			}(pl.args, cmdIO, closers)

			continue
		}

		path, found := s.findExecutable(pl.command)
		if !found {
			_, _ = fmt.Fprintf(s.stream.stdout, "%s: command not found\n", pl.command)
			closeAll(closers)
			return nil
		}
		//nolint:gosec // Executing dynamic user input is the intended behavior of a shell
		cmd := exec.CommandContext(context.Background(), path, pl.args...)
		cmd.Args[0] = pl.command
		cmd.Dir = s.workingDir
		cmd.Stdin = cmdIO.stdin
		cmd.Stdout = cmdIO.stdout
		cmd.Stderr = cmdIO.stderr

		if err := cmd.Start(); err != nil {
			closeAll(closers)
			return fmt.Errorf("failed to start command: %w", err)
		}

		wg.Add(1)
		go func(c *exec.Cmd, closers []io.Closer) {
			defer closeAll(closers)
			defer wg.Done()
			_ = c.Wait()
		}(cmd, closers)
	}

	wg.Wait()
	return nil
}

func closeAll(closers []io.Closer) {
	for _, closer := range closers {
		_ = closer.Close()
	}
}

type commandLine struct {
	pipeline []commandSegment
}

type commandSegment struct {
	command   string
	args      []string
	redirects []redirect
}

func parseInput(input string) (commandLine, error) {
	cl := commandLine{}
	splitedInput := handleQuotesAndEscapes(input)
	pipelineStr, err := splitPipeline(splitedInput)
	if err != nil {
		return cl, err
	}

	for _, cmdStr := range pipelineStr {
		if len(cmdStr) == 0 {
			return cl, fmt.Errorf("empty command")
		}
		command, args := cmdStr[0], cmdStr[1:]

		args, redirects, err := parseRedirect(args)
		if err != nil {
			return cl, err
		}
		cl.pipeline = append(cl.pipeline, commandSegment{
			command:   command,
			args:      args,
			redirects: redirects,
		})
	}
	return cl, nil
}
