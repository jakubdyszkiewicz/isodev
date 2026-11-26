// Package workspace creates and removes workspace containers, applying
// git_worktree.mode to workspaces that are linked Git worktrees.
package workspace

import (
	"fmt"
	"io"

	"github.com/jakubdyszkiewicz/isodev/internal/config"
	"github.com/jakubdyszkiewicz/isodev/internal/container"
	"github.com/jakubdyszkiewicz/isodev/internal/fsutil"
	"github.com/jakubdyszkiewicz/isodev/internal/integration/gitworktree"
	"github.com/jakubdyszkiewicz/isodev/internal/paths"
)

// Options controls Up.
type Options struct {
	// Integration names the integration calling Up, such as "Paseo". In a
	// worktree, an integration requires git_worktree.mode to be set, where the
	// CLI only warns.
	Integration string
	// Source is the main checkout to replace a worktree from. When empty, it
	// is found from the worktree.
	Source string
	// MatchWorkspacePath mounts the workspace at its host path.
	MatchWorkspacePath bool
	// Recreate replaces an existing container.
	Recreate bool
	// RenewVolumes makes Recreate empty the container's private volumes.
	RenewVolumes bool
	// Warn receives warnings, such as an unset git_worktree.mode.
	Warn io.Writer
}

// Result is the outcome of Up.
type Result struct {
	// Worktree is set when Up replaced the worktree's link.
	Worktree map[string]any `json:"worktree,omitempty"`
	// Action is what Up did to the container: Created, Started, Running (it
	// was already running) or Recreated.
	Action string `json:"action"`
	// Restored is set when Up restored a replaced worktree, because its mode
	// is no longer replace.
	Restored  map[string]any   `json:"restored,omitempty"`
	Container container.Record `json:"container"`
}

// mode returns the git_worktree mode for a linked worktree at cwd.
func mode(cfg config.Config, cwd string, o Options) (string, error) {
	if m := cfg.GitWorktree.Mode; m != "" {
		return m, nil
	}
	if o.Integration != "" {
		return "", fmt.Errorf("%s is a Git worktree: set git_worktree.mode in %s to %s, %s or %s before %s setup",
			cwd, paths.ConfigFile(), config.GitWorktreeReplace, config.GitWorktreeMountCommonDir, config.GitWorktreeNone, o.Integration)
	}
	if o.Warn != nil {
		fmt.Fprintf(o.Warn, "WARNING: %s is a Git worktree, and Git doesn't work in the container without the main repository. Set git_worktree.mode in %s to %s or %s, or to %s to silence this warning.\n",
			cwd, paths.ConfigFile(), config.GitWorktreeReplace, config.GitWorktreeMountCommonDir, config.GitWorktreeNone)
	}
	return config.GitWorktreeNone, nil
}

// steps is how Up handles a worktree before creating its container.
type steps struct {
	opts container.Options
	// restore restores a replaced worktree whose mode is no longer replace.
	restore bool
	// source is the checkout to replace the worktree from, if it is replaced.
	source string
}

// plan works out how a worktree at cwd is handled.
func plan(cwd string, o Options) (steps, error) {
	s := steps{opts: container.Options{MatchWorkspacePath: o.MatchWorkspacePath, RenewVolumes: o.RenewVolumes}}
	st, err := gitworktree.Detect(cwd)
	if err != nil || st.Kind == gitworktree.NotWorktree {
		return s, err
	}
	cfg, err := config.Load()
	if err != nil {
		return s, err
	}
	creating := o.Recreate || !container.Exists(cwd)
	var m string
	if st.Kind == gitworktree.Replaced {
		// A replaced worktree is restored only when a container is created
		// and another mode is set.
		m = cfg.GitWorktree.Mode
		if !creating || m == "" || m == config.GitWorktreeReplace {
			return s, nil
		}
		s.restore = true
	} else if m, err = mode(cfg, cwd, o); err != nil {
		return s, err
	}
	// The mode applies when a container is created.
	if !creating {
		return s, nil
	}
	opts := &s.opts
	switch m {
	case config.GitWorktreeMountCommonDir:
		opts.MatchWorkspacePath = true
		opts.GitCommonDir = st.CommonDir
	case config.GitWorktreeReplace:
		s.source = o.Source
		if s.source == "" {
			s.source = gitworktree.SourceCheckout(st.CommonDir)
		}
	}
	return s, nil
}

// Up creates the workspace's container, or starts it if it exists. In a
// linked worktree it first applies git_worktree.mode.
func Up(path string, o Options) (Result, error) {
	cwd, err := fsutil.Real(path)
	if err != nil {
		return Result{}, err
	}
	var res Result
	err = Lock(cwd, func() error {
		var err error
		res, err = up(cwd, o)
		return err
	})
	return res, err
}

func up(cwd string, o Options) (Result, error) {
	s, err := plan(cwd, o)
	if err != nil {
		return Result{}, err
	}
	var res Result
	switch state := container.State(cwd); {
	case o.Recreate && state != container.Missing:
		res.Action = "Recreated"
	case state == container.Running:
		res.Action = "Running"
	case state == container.Stopped:
		res.Action = "Started"
	default:
		res.Action = "Created"
	}
	// Check what can be checked before changing the worktree, so a bad
	// configuration or a stopped Docker daemon doesn't leave it replaced.
	if s.restore || s.source != "" {
		if err = container.Check(cwd, s.opts); err != nil {
			return Result{}, err
		}
	}
	if s.restore {
		if res.Restored, err = gitworktree.Restore(cwd); err != nil {
			return Result{}, fmt.Errorf("restore Git worktree: %w", err)
		}
	}
	if s.source != "" {
		if res.Worktree, err = gitworktree.Replace(cwd, s.source, true); err != nil {
			return res, fmt.Errorf("replace Git worktree: %w", err)
		}
	}
	if o.Recreate {
		res.Container, err = container.Recreate(cwd, s.opts)
	} else {
		res.Container, err = container.Provision(cwd, s.opts)
	}
	if err != nil && res.Worktree != nil {
		err = fmt.Errorf("%w; the worktree's link is already replaced", err)
	}
	return res, err
}

// PreviewUp describes Up for --dry-run.
func PreviewUp(w io.Writer, path string, o Options) error {
	cwd, err := fsutil.Real(path)
	if err != nil {
		return err
	}
	s, err := plan(cwd, o)
	if err != nil {
		return err
	}
	if s.restore {
		fmt.Fprintln(w, "# Restore the replaced Git worktree, since git_worktree.mode is no longer replace")
		gitworktree.PreviewRestore(w, cwd)
	}
	if s.source != "" {
		fmt.Fprintln(w, "# Replace the Git worktree's link (git_worktree.mode: replace)")
		gitworktree.PreviewReplace(w, cwd, s.source, true)
	}
	if s.opts.GitCommonDir != "" {
		fmt.Fprintf(w, "# Mount the Git common directory %s at its host path (git_worktree.mode: mount-common-dir)\n", s.opts.GitCommonDir)
	}
	if o.Recreate {
		return container.PreviewRecreate(w, cwd, s.opts)
	}
	return container.PreviewProvision(w, cwd, s.opts)
}

// Down removes the workspace's container and, if the workspace is a worktree
// whose link was replaced, restores it. It restores the worktree even when
// there is no container.
func Down(path string) (map[string]any, error) {
	cwd, err := fsutil.RealOrAbs(path)
	if err != nil {
		return nil, err
	}
	var res map[string]any
	err = Lock(cwd, func() error {
		var err error
		res, err = down(cwd)
		return err
	})
	return res, err
}

// down restores the worktree before removing the container, so a failed
// restore leaves both as they were.
func down(cwd string) (map[string]any, error) {
	res := map[string]any{"removed": false, "container": ""}
	st, err := gitworktree.Detect(cwd)
	if err != nil {
		return nil, err
	}
	if st.Kind == gitworktree.Replaced {
		wt, err := gitworktree.Restore(cwd)
		if err != nil {
			return nil, fmt.Errorf("restore Git worktree: %w", err)
		}
		res["worktree"] = wt
	}
	if r, err := container.Lookup(cwd); err == nil {
		res["container"] = r.Container
	}
	removed, err := container.Destroy(cwd)
	if err != nil {
		return res, err
	}
	res["removed"] = removed
	return res, nil
}

// PreviewDown describes Down for --dry-run.
func PreviewDown(w io.Writer, path string) error {
	cwd, err := fsutil.RealOrAbs(path)
	if err != nil {
		return err
	}
	if err := container.PreviewDestroy(w, cwd); err != nil {
		return err
	}
	if st, err := gitworktree.Detect(cwd); err != nil {
		return err
	} else if st.Kind == gitworktree.Replaced {
		fmt.Fprintln(w, "# Restore the replaced Git worktree")
		gitworktree.PreviewRestore(w, cwd)
	}
	return nil
}
