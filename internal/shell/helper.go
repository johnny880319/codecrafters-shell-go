package shell

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
)

func isIdentifierStart(ch byte) bool {
	return ch == '_' || (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z')
}

func isIdentifierPart(ch byte) bool {
	return isIdentifierStart(ch) || (ch >= '0' && ch <= '9')
}

func (s *Shell) findExecutable(command string) (string, bool) {
	paths := filepath.SplitList(s.env.path)
	for _, dir := range paths {
		fullPath := filepath.Join(dir, command)
		if _, err := exec.LookPath(fullPath); err == nil {
			return fullPath, true
		}
	}
	return "", false
}

func (s *Shell) checkDirectory(path string, stderr io.Writer) error {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		_, printfErr := fmt.Fprintf(stderr, "cd: %s: No such file or directory\n", path)
		return printfErr
	}
	if err != nil {
		return fmt.Errorf("check directory: %w", err)
	}
	if !info.IsDir() {
		_, printfErr := fmt.Fprintf(stderr, "cd: %s: Not a directory\n", path)
		return printfErr
	}
	s.workingDir = path
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

func (s *Shell) showJobs(cmdIO shellStream, onlyDone bool) {
	s.jobs.mutex.Lock()
	defer s.jobs.mutex.Unlock()
	for i, job := range s.jobs.jobs {
		if onlyDone && job.status != "Done" {
			continue
		}
		if job.status == "Done" {
			job.hasShowed = true
		}
		indicator := " "
		if i == len(s.jobs.jobs)-1 {
			indicator = "+"
		}
		if i == len(s.jobs.jobs)-2 {
			indicator = "-"
		}
		_, _ = fmt.Fprintf(
			cmdIO.stdout,
			"[%d]%s  %-24s %s\n",
			job.id,
			indicator,
			job.status,
			job.command,
		)
	}
	newJobs := make([]*shellJob, 0)
	for _, job := range s.jobs.jobs {
		if !job.hasShowed {
			newJobs = append(newJobs, job)
		}
	}
	s.jobs.jobs = newJobs
}

func (s *Shell) readHistoryFromFile(filepath string, cmdIO shellStream) error {
	//nolint:gosec // A shell's intended behavior is to open files specified by the user
	file, err := os.Open(filepath)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("open history file: %w", err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			if _, err := fmt.Fprintf(cmdIO.stderr, "close history file error: %v\n", err); err != nil {
				return
			}
		}
	}()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		s.history.lines = append(s.history.lines, line)
	}
	if err := scanner.Err(); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			_, printfErr := fmt.Fprintf(cmdIO.stderr, "history: %s: No such file or directory\n", filepath)
			return printfErr
		}
	}
	return nil
}

func (s *Shell) writeHistoryToFile(filepath string, cmdIO shellStream, flag int, index int) error {
	// If HISTFILE is not set, the shell should not save the history to any file, but it should also not return an error.
	if filepath == "" {
		return nil
	}
	//nolint:gosec // A shell's intended behavior is to open files specified by the user
	file, err := os.OpenFile(filepath, os.O_CREATE|os.O_WRONLY|flag, 0o644)
	if err != nil {
		return fmt.Errorf("open history file: %w", err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			if _, err := fmt.Fprintf(cmdIO.stderr, "close history file error: %v\n", err); err != nil {
				return
			}
		}
	}()

	writer := bufio.NewWriter(file)
	for _, cmd := range s.history.lines[index:] {
		if _, err := writer.WriteString(cmd + "\n"); err != nil {
			return fmt.Errorf("write history to file: %w", err)
		}
	}
	if err := writer.Flush(); err != nil {
		return fmt.Errorf("flush history to file: %w", err)
	}
	return nil
}
