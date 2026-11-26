package container

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jakubdyszkiewicz/isodev/internal/config"
	"github.com/jakubdyszkiewicz/isodev/internal/fsutil"
	"github.com/jakubdyszkiewicz/isodev/internal/shell"
)

// Options controls how a workspace container is created.
type Options struct {
	// Probe marks the Paseo provider probe workspace, which has its own record.
	Probe bool
	// MatchWorkspacePath mounts the workspace at its host path instead of
	// /workspaces/<name>.
	MatchWorkspacePath bool
	// GitCommonDir, when set, is mounted read-write at its host path so a
	// linked Git worktree works in the container. It requires
	// MatchWorkspacePath.
	GitCommonDir string
	// RenewVolumes makes Recreate remove the old container's private volumes
	// instead of reusing them.
	RenewVolumes bool
}

func workspaceFolder(r Record) string {
	if r.Probe {
		return "/workspaces/probe"
	}
	if r.MatchWorkspacePath {
		return r.Cwd
	}
	return filepath.Join("/workspaces", filepath.Base(r.Cwd))
}

// plan returns the record a new container for cwd would get.
func plan(cwd string, opts Options) Record {
	name := Name(cwd)
	r := Record{Version: 1, Cwd: cwd, Probe: opts.Probe, MatchWorkspacePath: opts.MatchWorkspacePath, GitCommonDir: opts.GitCommonDir, Container: name, Network: name + "-net"}
	r.WorkspaceFolder = workspaceFolder(r)
	return r
}

// Provision returns the workspace's container, starting it if it is stopped
// or creating it if it does not exist.
func Provision(cwd string, opts Options) (Record, error) {
	return provision(cwd, opts, nil)
}

// provision creates the container with prepared, a configuration whose image
// was already built, or loads and builds the configuration when it is nil.
func provision(cwd string, opts Options, prepared *config.Config) (Record, error) {
	cwd, err := fsutil.Real(cwd)
	if err != nil {
		return Record{}, err
	}
	info, err := os.Stat(cwd)
	if err != nil {
		return Record{}, err
	}
	if !info.IsDir() {
		return Record{}, fmt.Errorf("workspace is not a directory: %s", cwd)
	}
	path := recordPath(cwd)
	if opts.Probe {
		path = probeRecordPath()
	}
	if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return Record{}, err
	}
	var previous Record
	readErr := fsutil.ReadJSON(path, &previous)
	if !opts.Probe && os.IsNotExist(readErr) {
		var probeRecord Record
		if e := fsutil.ReadJSON(probeRecordPath(), &probeRecord); e == nil && probeRecord.Probe && probeRecord.Cwd == cwd {
			previous, readErr = probeRecord, nil
			opts.Probe = true
			path = probeRecordPath()
		} else if e != nil && !os.IsNotExist(e) {
			return Record{}, e
		}
	}
	if readErr != nil && !os.IsNotExist(readErr) {
		return Record{}, readErr
	}
	if readErr == nil && opts.Probe && previous.Cwd != cwd {
		if resolved, e := fsutil.Real(previous.Cwd); e == nil && resolved == cwd {
			cwd = previous.Cwd
		}
	}
	if readErr == nil && (previous.Cwd != cwd || previous.Container == "") {
		return Record{}, fmt.Errorf("invalid workspace mapping")
	}
	if readErr == nil && exists("container", "inspect", previous.Container) {
		return startExisting(previous, cwd, opts)
	}
	cfg := prepared
	if cfg == nil {
		loaded, err := config.Load()
		if err != nil {
			return Record{}, err
		}
		cfg = &loaded
	}
	r := plan(cwd, opts)
	args, err := runArgs(r, *cfg)
	if err != nil {
		return Record{}, err
	}
	if prepared == nil {
		if err = buildImage(*cfg); err != nil {
			return Record{}, err
		}
	}
	return create(r, args, *cfg, path)
}

// create creates the volumes, network and container of r, runs its scripts
// and writes its record to path. On failure it removes only the resources it
// created, so a container, network or volume that existed before is kept.
func create(r Record, args []string, cfg config.Config, path string) (Record, error) {
	created := Record{Cwd: r.Cwd}
	var createdVolumes []string
	fail := func(err error) (Record, error) {
		_ = destroyRecord(created)
		_ = removeVolumes(createdVolumes)
		return Record{}, err
	}
	createdVolumes, err := ensureVolumes(r, cfg)
	if err != nil {
		return fail(err)
	}
	newNetwork, err := ensureNetwork(r)
	if err != nil {
		return fail(err)
	}
	if newNetwork {
		created.Network = r.Network
	}
	if exists("container", "inspect", r.Container) {
		return fail(fmt.Errorf("container %s already exists without an Isodev record; remove it with: docker rm -f %s", r.Container, r.Container))
	}
	containerID, err := shell.Run("docker", args...)
	if err != nil {
		// Docker can create the container and then fail to start it, for
		// example when the command doesn't exist.
		if exists("container", "inspect", r.Container) {
			created.Container = r.Container
		}
		return fail(err)
	}
	created.Container = r.Container
	containerID = strings.TrimSpace(containerID)
	if containerID == "" {
		return fail(fmt.Errorf("docker returned no container ID"))
	}
	i, err := inspect(containerID)
	if err != nil {
		return fail(err)
	}
	if err = checkCreated(r, i); err != nil {
		return fail(err)
	}
	r.ContainerID = i.ID
	if err = runHooks(r, "on_start", cfg.Lifecycle.OnStart.Scripts); err != nil {
		return fail(err)
	}
	if err = runHooks(r, "on_create", cfg.Lifecycle.OnCreate.Scripts); err != nil {
		return fail(err)
	}
	if err = fsutil.WriteJSONAtomic(path, r); err != nil {
		return fail(err)
	}
	return r, nil
}

// startExisting validates an existing container and starts it if stopped.
func startExisting(previous Record, cwd string, opts Options) (Record, error) {
	i, err := inspect(previous.Container)
	if err != nil {
		return Record{}, err
	}
	if opts.MatchWorkspacePath && i.Config.WorkingDir != cwd {
		return Record{}, fmt.Errorf("existing isodev container uses a different workspace path; remove it before Paseo setup")
	}
	if i.Config.Labels[labelManaged] != "true" || i.Config.Labels[labelWorkspace] != cwd {
		return Record{}, fmt.Errorf("isodev container identity mismatch")
	}
	if previous.WorkspaceFolder == "" {
		previous.WorkspaceFolder = i.Config.WorkingDir
	}
	if !i.State.Running {
		if err := startStopped(previous); err != nil {
			return Record{}, err
		}
	}
	return previous, nil
}

// ensureNetwork creates the workspace network, or reuses an existing one that
// belongs to the same workspace. It reports whether it created the network.
func ensureNetwork(r Record) (bool, error) {
	if !exists("network", "inspect", r.Network) {
		_, err := shell.Run("docker", append(append([]string{"network", "create"}, managedLabels(r.Cwd)...), r.Network)...)
		return err == nil, err
	}
	out, err := shell.Run("docker", "network", "inspect", r.Network)
	if err != nil {
		return false, err
	}
	var networks []struct{ Labels map[string]string }
	if err = json.Unmarshal([]byte(out), &networks); err != nil {
		return false, err
	}
	if len(networks) != 1 || networks[0].Labels[labelManaged] != "true" || networks[0].Labels[labelWorkspace] != r.Cwd {
		return false, fmt.Errorf("existing network %s belongs to another workspace", r.Network)
	}
	return false, nil
}

func checkCreated(r Record, i inspection) error {
	workspace := false
	for _, mount := range i.Mounts {
		if mount.Type == "bind" && mount.Destination == r.WorkspaceFolder && mount.Source == r.Cwd {
			workspace = true
		}
	}
	if !workspace || len(i.NetworkSettings.Networks) != 1 || i.NetworkSettings.Networks[r.Network] == nil {
		return fmt.Errorf("container %s has an unexpected workspace or network", r.Container)
	}
	return nil
}

// Check validates what creating the container of the workspace at cwd with
// opts needs, without changing anything: the configuration, its mounts and a
// reachable Docker daemon.
func Check(cwd string, opts Options) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if err = cfg.ValidateBuild(); err != nil {
		return err
	}
	if _, err = runArgs(plan(cwd, opts), cfg); err != nil {
		return err
	}
	if _, err = shell.Run("docker", "version", "--format", "{{.Server.Version}}"); err != nil {
		return fmt.Errorf("docker is not available: %w", err)
	}
	return nil
}

// Exists reports whether the workspace at path has a container.
func Exists(path string) bool {
	return State(path) != Missing
}

// Container states reported by State.
const (
	Missing = "missing"
	Stopped = "stopped"
	Running = "running"
)

// State reports whether the workspace at path has a container, and whether it
// is running.
func State(path string) string {
	r, err := Lookup(path)
	if err != nil {
		return Missing
	}
	i, err := inspect(r.Container)
	switch {
	case err != nil:
		return Missing
	case i.State.Running:
		return Running
	default:
		return Stopped
	}
}

// Recreate replaces the workspace's container using the current configuration
// and opts. The new container keeps the old one's workspace path if that was
// the host path. The image is built before the old container is removed.
func Recreate(path string, opts Options) (Record, error) {
	existing, err := Lookup(path)
	if os.IsNotExist(err) {
		return Provision(path, opts)
	}
	if err != nil {
		return Record{}, err
	}
	cfg, err := config.Load()
	if err != nil {
		return Record{}, err
	}
	match, err := recreationMatch(existing)
	if err != nil {
		return Record{}, err
	}
	opts.Probe = existing.Probe
	opts.MatchWorkspacePath = opts.MatchWorkspacePath || match
	if _, err := runArgs(plan(existing.Cwd, opts), cfg); err != nil {
		return Record{}, err
	}
	if err := buildImage(cfg); err != nil {
		return Record{}, err
	}
	if _, err := destroy(path, opts.RenewVolumes); err != nil {
		return Record{}, err
	}
	return provision(existing.Cwd, opts, &cfg)
}

// recreationMatch reports whether a recreated container keeps the host path.
func recreationMatch(existing Record) (bool, error) {
	match := existing.MatchWorkspacePath || (!existing.Probe && existing.WorkspaceFolder == existing.Cwd)
	if !existing.Probe && existing.WorkspaceFolder == "" && exists("container", "inspect", existing.Container) {
		info, err := inspect(existing.Container)
		if err != nil {
			return false, err
		}
		match = match || info.Config.WorkingDir == existing.Cwd
	}
	return match, nil
}

// startStopped starts a container and reruns its on_start hook, stopping the
// container again if the hook fails.
func startStopped(r Record) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if _, err := shell.Run("docker", "start", r.Container); err != nil {
		return err
	}
	if err := runHooks(r, "on_start", cfg.Lifecycle.OnStart.Scripts); err != nil {
		if _, stopErr := shell.Run("docker", "stop", r.Container); stopErr != nil {
			return fmt.Errorf("%w; also failed to stop container: %v", err, stopErr)
		}
		return err
	}
	return nil
}

// Stop stops the workspace's container, keeping its filesystem.
func Stop(path string) (Record, error) {
	r, err := Lookup(path)
	if err != nil {
		return Record{}, err
	}
	if _, err := shell.Run("docker", "stop", r.Container); err != nil {
		return Record{}, err
	}
	return r, nil
}

// destroyRecord removes the container, with its anonymous volumes, and the
// network of r. Private volumes are named, so they are kept.
func destroyRecord(r Record) error {
	for _, kind := range []string{"container", "network"} {
		name := r.Container
		if kind == "network" {
			name = r.Network
		}
		if name == "" || !exists(kind, "inspect", name) {
			continue
		}
		if kind == "container" {
			if _, err := shell.Run("docker", "stop", name); err != nil {
				return err
			}
			if _, err := shell.Run("docker", "rm", "--volumes", name); err != nil {
				return err
			}
		} else if _, err := shell.Run("docker", "network", "rm", name); err != nil {
			return err
		}
	}
	return nil
}

// Destroy removes the workspace's container, network, private volumes and
// record. It reports false if the workspace had no
// container.
func Destroy(cwd string) (bool, error) {
	return destroy(cwd, true)
}

// destroy is Destroy, keeping private volumes unless volumes is set.
func destroy(cwd string, volumes bool) (bool, error) {
	r, err := Lookup(cwd)
	if os.IsNotExist(err) {
		if volumes {
			if real, e := fsutil.RealOrAbs(cwd); e == nil {
				return false, removeWorkspaceVolumes(real)
			}
		}
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if err = destroyRecord(r); err != nil {
		return false, err
	}
	if volumes {
		if err = removeWorkspaceVolumes(r.Cwd); err != nil {
			return false, err
		}
	}
	path := recordPath(r.Cwd)
	if r.Probe {
		path = probeRecordPath()
	}
	if err = os.Remove(path); err != nil {
		return false, err
	}
	return true, nil
}
