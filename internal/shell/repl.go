// Package shell implements a simple interactive shell.
package shell

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"

	"github.com/chzyer/readline"
)

const prompt = "$ "

var readlineInitMutex sync.Mutex

// Shell represents the core state of the interactive shell.
type Shell struct {
	workingDir        string
	builtinCommandMap map[string]builtinCommand
	stream            shellStream
	env               shellEnv
	history           shellHistory
	jobs              map[int]job
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
		jobs:       make(map[int]job),
	}

	for _, opt := range opts {
		opt(s)
	}

	s.builtinCommandMap = s.getCommandFuncMap()
	if err := s.readHistoryFromFile(s.env.histfile, s.stream); err != nil {
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

func (s *Shell) execute(input string) error {
	s.history.lines = append(s.history.lines, input)
	cmdline, err := parseInput(input)
	if err != nil {
		return err
	}

	var wg sync.WaitGroup

	pid := 0
	prevReader := s.stream.stdin
	for i, pl := range cmdline.pipeline {
		var (
			cmdStream shellStream
			closers   []io.Closer
			err       error
		)
		cmdStream, closers, prevReader, err = s.handleStream(prevReader, i, cmdline.pipeline)
		if err != nil {
			return err
		}

		if bc, ok := s.builtinCommandMap[pl.command]; ok {
			err := startBuiltinCommand(bc, pl, cmdStream, closers, &wg)
			if err != nil {
				return err
			}
			continue
		}
		pid, err = s.startExternalCommand(pl, cmdStream, closers, &wg)
		if err != nil {
			return err
		}
	}

	if cmdline.isBackground {
		_, _ = fmt.Fprintf(s.stream.stdout, "[%d] %d\n", s.jobs[pid].id, pid)
	} else {
		wg.Wait()
	}
	return nil
}

func (s *Shell) handleStream(
	prevReader io.Reader,
	i int,
	pipeline []commandSegment,
) (shellStream, []io.Closer, io.Reader, error) {
	var closers []io.Closer
	cmdStream := shellStream{
		stdin:  prevReader,
		stdout: s.stream.stdout,
		stderr: s.stream.stderr,
	}

	if i > 0 {
		closer, ok := prevReader.(io.Closer)
		if !ok {
			return shellStream{}, nil, nil, fmt.Errorf("previous reader is not closable")
		}
		closers = append(closers, closer)
	}
	if i < len(pipeline)-1 {
		pr, pw := io.Pipe()
		cmdStream.stdout = pw
		prevReader = pr // used for the next round of loop
		closers = append(closers, pw)
	}

	for _, redirect := range pipeline[i].redirects {
		//nolint:gosec // Opening files based on user input is the intended behavior of a shell
		file, err := os.OpenFile(redirect.file, os.O_CREATE|os.O_WRONLY|redirect.appendFlag, 0o644)
		if err != nil {
			closeAll(closers)
			return shellStream{}, nil, nil, fmt.Errorf("%s: %s", redirect.file, err.Error())
		}
		closers = append(closers, file)
		switch redirect.fd {
		case 1:
			cmdStream.stdout = file
		case 2:
			cmdStream.stderr = file
		default:
			closeAll(closers)
			return shellStream{}, nil, nil, fmt.Errorf("unsupported redirect fd: %d", redirect.fd)
		}
	}
	return cmdStream, closers, prevReader, nil
}

func startBuiltinCommand(
	bc builtinCommand,
	segment commandSegment,
	cmdStream shellStream,
	closers []io.Closer,
	wg *sync.WaitGroup,
) error {
	if !bc.canRunAsync {
		err := bc.fn(segment.args, cmdStream)
		closeAll(closers)
		return err
	}

	wg.Add(1)
	go func(args []string, cmdStream shellStream, closers []io.Closer) {
		defer closeAll(closers)
		defer wg.Done()
		_ = bc.fn(args, cmdStream)
	}(segment.args, cmdStream, closers)

	return nil
}

func (s *Shell) startExternalCommand(
	segment commandSegment,
	cmdStream shellStream,
	closers []io.Closer,
	wg *sync.WaitGroup,
) (int, error) {
	path, found := s.findExecutable(segment.command)
	if !found {
		_, _ = fmt.Fprintf(s.stream.stdout, "%s: command not found\n", segment.command)
		closeAll(closers)
		return 0, nil
	}
	//nolint:gosec // Executing dynamic user input is the intended behavior of a shell
	cmd := exec.CommandContext(context.Background(), path, segment.args...)
	cmd.Args[0] = segment.command
	cmd.Dir = s.workingDir
	cmd.Stdin = cmdStream.stdin
	cmd.Stdout = cmdStream.stdout
	cmd.Stderr = cmdStream.stderr

	if err := cmd.Start(); err != nil {
		closeAll(closers)
		return 0, fmt.Errorf("failed to start command: %w", err)
	}

	pid := cmd.Process.Pid
	s.jobs[pid] = job{
		id:      len(s.jobs) + 1,
		pid:     pid,
		command: segment.command + " " + strings.Join(segment.args, " "),
		status:  "Running",
	}

	wg.Add(1)
	go func(c *exec.Cmd, closers []io.Closer) {
		defer closeAll(closers)
		defer wg.Done()
		_ = c.Wait()
	}(cmd, closers)

	return pid, nil
}

func closeAll(closers []io.Closer) {
	for _, closer := range closers {
		_ = closer.Close()
	}
}

type commandLine struct {
	pipeline     []commandSegment
	isBackground bool
}

type commandSegment struct {
	command   string
	args      []string
	redirects []redirect
}

func parseInput(input string) (commandLine, error) {
	cl := commandLine{}
	input = strings.TrimSpace(input)
	if input[len(input)-1] == '&' {
		cl.isBackground = true
		input = strings.TrimSpace(input[:len(input)-1])
	}
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
