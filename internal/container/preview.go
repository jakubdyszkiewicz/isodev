package container

import (
	"fmt"
	"io"
	"os"

	"github.com/jakubdyszkiewicz/isodev/internal/config"
	"github.com/jakubdyszkiewicz/isodev/internal/fsutil"
	"github.com/jakubdyszkiewicz/isodev/internal/shell"
)

// Previews describe what the corresponding operation would do for --dry-run.

// PreviewProvision describes Provision for the workspace at path.
func PreviewProvision(w io.Writer, path string, opts Options) error {
	cwd, err := fsutil.Real(path)
	if err != nil {
		return err
	}
	fmt.Fprintln(w, "# If the container exists, validate it and start it if stopped:")
	shell.Print(w, "docker", "start", "<existing-container-id>")
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if err := cfg.ValidateBuild(); err != nil {
		return err
	}
	fmt.Fprintln(w, "# Otherwise, build the image if configured, and create the network and container:")
	return previewCreate(w, plan(cwd, opts), cfg, true)
}

// PreviewRecreate describes Recreate for the workspace at path.
func PreviewRecreate(w io.Writer, path string, opts Options) error {
	cwd, err := fsutil.Real(path)
	if err != nil {
		return err
	}
	existing, err := Lookup(cwd)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	found := err == nil
	if found {
		opts.Probe = existing.Probe
		match, err := recreationMatch(existing)
		if err != nil {
			return err
		}
		opts.MatchWorkspacePath = opts.MatchWorkspacePath || match
	}
	r := plan(cwd, opts)
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if err := cfg.ValidateBuild(); err != nil {
		return err
	}
	if _, err := runArgs(r, cfg); err != nil {
		return err
	}
	if cfg.Build != nil {
		fmt.Fprintln(w, "# Build the replacement image before removing the existing container:")
		shell.Print(w, append([]string{"docker"}, buildArgs(cfg)...)...)
	}
	if found {
		previewRemove(w, existing, opts.RenewVolumes)
	} else {
		fmt.Fprintln(w, "# No existing workspace container; create a new one")
	}
	return previewCreate(w, r, cfg, false)
}

func previewCreate(w io.Writer, r Record, cfg config.Config, includeBuild bool) error {
	if includeBuild && cfg.Build != nil {
		shell.Print(w, append([]string{"docker"}, buildArgs(cfg)...)...)
	}
	for _, name := range privateVolumes(r, cfg) {
		fmt.Fprintln(w, "# Create the private volume if it doesn't exist:")
		shell.Print(w, append(append([]string{"docker", "volume", "create"}, managedLabels(r.Cwd)...), name)...)
	}
	shell.Print(w, append(append([]string{"docker", "network", "create"}, managedLabels(r.Cwd)...), r.Network)...)
	args, err := runArgs(r, cfg)
	if err != nil {
		return err
	}
	shell.Print(w, append([]string{"docker"}, args...)...)
	previewHooks(w, r, "on_start", cfg.Lifecycle.OnStart.Scripts)
	previewHooks(w, r, "on_create", cfg.Lifecycle.OnCreate.Scripts)
	fmt.Fprintln(w, "# Inspect the resulting container and write its workspace record; on failure remove newly created resources")
	return nil
}

func previewHooks(w io.Writer, r Record, name string, hooks []config.Script) {
	for i, hook := range hooks {
		fmt.Fprintf(w, "# Run lifecycle.%s.scripts[%d] from %s\n", name, i, hook.File)
		shell.Print(w, append([]string{"docker"}, hookArgs(r)...)...)
	}
	if len(hooks) > 0 {
		fmt.Fprintln(w, "# Script contents are supplied on stdin; a failing script stops the rest")
	}
}

// PreviewDestroy describes Destroy for the workspace at path.
func PreviewDestroy(w io.Writer, path string) error {
	r, err := Lookup(path)
	if os.IsNotExist(err) {
		fmt.Fprintf(w, "# No isodev container found for %s\n", path)
		return nil
	}
	if err != nil {
		return err
	}
	previewRemove(w, r, true)
	return nil
}

func previewRemove(w io.Writer, r Record, volumes bool) {
	fmt.Fprintln(w, "# Remove each resource if present:")
	shell.Print(w, "docker", "stop", r.Container)
	shell.Print(w, "docker", "rm", "--volumes", r.Container)
	shell.Print(w, "docker", "network", "rm", r.Network)
	if volumes {
		fmt.Fprintln(w, "# Remove the workspace's private volumes:")
		shell.Print(w, append([]string{"docker"}, workspaceVolumeArgs(r.Cwd)...)...)
	} else {
		fmt.Fprintln(w, "# Keep the workspace's private volumes for the new container")
	}
	fmt.Fprintln(w, "# Delete workspace record")
}

// PreviewStop describes Stop for the workspace at path.
func PreviewStop(w io.Writer, path string) error {
	r, err := Lookup(path)
	if err != nil {
		return fmt.Errorf("find container for workspace %s: %w", path, err)
	}
	shell.Print(w, "docker", "stop", r.Container)
	return nil
}

// PreviewLaunch describes Launch in container.
func PreviewLaunch(w io.Writer, container string, args []string, forwardPaseoEnv bool) error {
	a, err := execArgs(container, args, forwardPaseoEnv, "<temporary-pid-file>")
	if err != nil {
		return err
	}
	shell.Print(w, append([]string{"docker"}, a...)...)
	fmt.Fprintln(w, "# Remove the temporary PID file after exit; terminate the process group on interrupt")
	return nil
}

// PreviewEnter describes Enter for r.
func PreviewEnter(w io.Writer, r Record) {
	shell.Print(w, append([]string{"docker"}, enterArgs(r.Container)...)...)
}

// PreviewList describes List.
func PreviewList(w io.Writer, all bool) {
	shell.Print(w, append([]string{"docker"}, listArgs(all)...)...)
}
