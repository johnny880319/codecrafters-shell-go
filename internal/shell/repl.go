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

// Guards readline.NewEx during parallel test execution.
var readlineInitMutex sync.Mutex

// Shell represents the core state of the interactive shell.
type Shell struct {
	workingDir        string
	builtinCommandMap map[string]builtinCommand
	stream            shellStream
	env               shellEnv
	history           shellHistory
	jobs              shellJobs
	completers        map[string]string
	variables         map[string]string
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

type shellJobs struct {
	jobs  []*shellJob
	mutex sync.Mutex
}

type shellJob struct {
	id        int
	pid       int
	command   string
	status    string
	hasShowed bool
}

// NewShell creates a new Shell instance with default OS environment variables.
func NewShell(in io.Reader, out io.Writer) (*Shell, error) {
	wd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("get working directory: %w", err)
	}

	s := &Shell{
		stream:     shellStream{stdin: in, stdout: out, stderr: out},
		env:        shellEnv{path: os.Getenv("PATH"), histfile: os.Getenv("HISTFILE")},
		workingDir: wd,
		completers: make(map[string]string),
		variables:  make(map[string]string),
	}

	s.builtinCommandMap = s.getCommandFuncMap()
	if err := s.readHistoryFromFile(s.env.histfile, s.stream); err != nil {
		_, err = fmt.Fprintf(s.stream.stderr, "read history from file error: %v\n", err)
		return nil, err
	}
	s.history.startLine = len(s.history.lines)
	s.history.appendLine = len(s.history.lines)

	return s, nil
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
	cmdline, err := s.parseInput(input)
	if err != nil {
		return err
	}

	var wg sync.WaitGroup

	var currentJob *shellJob
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
		path, found := s.findExecutable(pl.command)
		if !found {
			_, _ = fmt.Fprintf(s.stream.stdout, "%s: command not found\n", pl.command)
			closeAll(closers)
			continue
		}
		currentJob, err = s.startExternalCommand(path, pl, cmdStream, closers, &wg, cmdline.isBackground)
		if err != nil {
			return err
		}
	}

	if cmdline.isBackground {
		_, _ = fmt.Fprintf(s.stream.stdout, "[%d] %d\n", currentJob.id, currentJob.pid)
	} else {
		wg.Wait()
	}
	s.showJobs(s.stream, true)
	return nil
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
	path string,
	segment commandSegment,
	cmdStream shellStream,
	closers []io.Closer,
	wg *sync.WaitGroup,
	isBackground bool,
) (*shellJob, error) {
	//nolint:gosec // Executing dynamic user input is the intended behavior of a shell
	cmd := exec.CommandContext(context.Background(), path, segment.args...)
	cmd.Args[0] = segment.command
	cmd.Dir = s.workingDir
	cmd.Stdin = cmdStream.stdin
	cmd.Stdout = cmdStream.stdout
	cmd.Stderr = cmdStream.stderr

	if err := cmd.Start(); err != nil {
		closeAll(closers)
		return nil, fmt.Errorf("failed to start command: %w", err)
	}

	var currentJob *shellJob
	if isBackground {
		s.jobs.mutex.Lock()
		jobID := 1
		for {
			idExists := false
			for _, job := range s.jobs.jobs {
				if job.id == jobID {
					idExists = true
					break
				}
			}
			if !idExists {
				break
			}
			jobID++
		}

		currentJob = &shellJob{
			id:        jobID,
			pid:       cmd.Process.Pid,
			command:   segment.command + " " + strings.Join(segment.args, " "),
			status:    "Running",
			hasShowed: false,
		}
		s.jobs.jobs = append(s.jobs.jobs, currentJob)
		s.jobs.mutex.Unlock()
	}

	wg.Add(1)
	go func(c *exec.Cmd, closers []io.Closer) {
		defer closeAll(closers)
		defer wg.Done()
		_ = c.Wait()
		if isBackground {
			s.jobs.mutex.Lock()
			currentJob.status = "Done"
			s.jobs.mutex.Unlock()
		}
	}(cmd, closers)

	return currentJob, nil
}

func closeAll(closers []io.Closer) {
	for _, closer := range closers {
		_ = closer.Close()
	}
}
