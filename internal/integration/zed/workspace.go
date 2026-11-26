package zed

import (
	"fmt"
	"io"
	"os"

	"github.com/jakubdyszkiewicz/isodev/internal/container"
	"github.com/jakubdyszkiewicz/isodev/internal/workspace"
)

const taskLabel = "Prepare isodev worktree"

func setupOptions(source string) workspace.Options {
	return workspace.Options{Integration: "Zed", Source: source, MatchWorkspacePath: true, Warn: os.Stderr}
}

// Setup prepares a Zed worktree, which may have a detached HEAD, and its
// container, following git_worktree.mode.
func Setup(path, source string) (workspace.Result, error) {
	result, err := workspace.Up(path, setupOptions(source))
	if err != nil && result.Worktree != nil {
		err = fmt.Errorf("%w; recover with isodev int zed teardown --workspace %q", err, path)
	}
	return result, err
}

// PreviewSetup describes Setup for --dry-run.
func PreviewSetup(w io.Writer, path, source string) error {
	return workspace.PreviewUp(w, path, setupOptions(source))
}

// Teardown removes the container and restores the worktree if it was replaced.
func Teardown(path string) (map[string]any, error) {
	return workspace.Down(path)
}

// PreviewACP describes Bridge for --dry-run.
func PreviewACP(w io.Writer, path, agent string) error {
	command, err := ACPCommand(agent)
	if err != nil {
		return err
	}
	r, err := container.FindForPath(path)
	if err != nil {
		return err
	}
	fmt.Fprintf(w, "# Disable ACP host terminal and file capabilities and reject sessions outside %s\n", r.Cwd)
	return container.PreviewLaunch(w, r.Container, command, false)
}
