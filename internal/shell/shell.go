// Package shell runs external commands and formats them for previews.
package shell

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

// Run executes a command and returns its trimmed standard output. On failure,
// the error includes the command's standard error. Git never prompts.
func Run(name string, args ...string) (string, error) {
	c := exec.Command(name, args...)
	c.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	b, err := c.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			if stderr := strings.TrimSpace(string(exitErr.Stderr)); stderr != "" {
				return "", commandError{name + ": " + stderr, err}
			}
		}
		return "", commandError{name + ": " + err.Error(), err}
	}
	return strings.TrimSpace(string(b)), nil
}

// commandError reports a failed command by its standard error, such as
// "docker: Error response from daemon: ...", and unwraps to the exit error.
type commandError struct {
	msg string
	err error
}

func (e commandError) Error() string { return e.msg }
func (e commandError) Unwrap() error { return e.err }

// Print writes a command line, quoting arguments for a POSIX shell.
func Print(w io.Writer, args ...string) {
	for i, a := range args {
		if i > 0 {
			fmt.Fprint(w, " ")
		}
		if a != "" && !strings.ContainsAny(a, " \t\n'\"$`\\;|&<>(){}*?[]!#") {
			fmt.Fprint(w, a)
		} else {
			fmt.Fprintf(w, "'%s'", strings.ReplaceAll(a, "'", "'\\''"))
		}
	}
	fmt.Fprintln(w)
}
