// Package config loads and validates the global isodev configuration in
// ~/.config/isodev/config.yaml.
package config

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/jakubdyszkiewicz/isodev/internal/paths"
)

//go:generate go run ./schemagen -schema ../../docs/static/config.schema.json -docs ../../docs/content/docs/configuration.md

// Defaults for unset fields. The generated schema and reference read them from here.
const (
	DefaultImage      = "ubuntu:24.04"
	DefaultDockerfile = "Dockerfile"
)

// DefaultCommand is the container command used when command is not set.
func DefaultCommand() []string { return []string{"sleep", "infinity"} }

// The doc comments on the fields below are the user-facing reference for
// config.yaml: `go generate ./...` turns them into the documentation site's
// configuration page and JSON Schema under docs/. Keep them accurate and written for users.

// Config describes how isodev creates and runs workspace containers.
type Config struct {
	// Image for new containers. The `ISODEV_IMAGE` environment variable overrides it. Required when `build` is set, because it names the built image.
	Image string `yaml:"image" json:"image"`
	// Build the image before creating each new container, with `docker build --tag IMAGE --file DOCKERFILE CONTEXT`. Docker reuses cached layers and build output goes to stderr. A failed build prevents creation. Existing containers are not rebuilt.
	Build *Build `yaml:"build" json:"build"`
	// Command arguments passed to the image. The container must keep running so that lifecycle scripts and agent commands can run in it. Containers run with `--init`.
	Command []string `yaml:"command" json:"command"`
	// Run containers with `--privileged`, for example for Docker-in-Docker. Privileged containers have broad access to the host.
	Privileged bool `yaml:"privileged" json:"privileged"`
	// Fixed environment variables set when a container is created. Values are strings; variable interpolation is not supported.
	Environment map[string]string `yaml:"environment" json:"environment"`
	// Mounts added to each new container, in the Compose long form: host directories and files, and Docker volumes. Isodev adds the workspace mount itself. Mounted sources are visible to the container even when read-only.
	Volumes []Mount `yaml:"volumes" json:"volumes"`
	// What Isodev does when it creates or starts a container, and when it runs a command in one.
	Lifecycle Lifecycle `yaml:"lifecycle" json:"lifecycle"`
	// How Isodev handles a workspace that is a linked Git worktree. A worktree's `.git` file points into the main repository's Git directory, which the container doesn't have unless Isodev provides it.
	GitWorktree GitWorktree `yaml:"git_worktree" json:"git_worktree"`
}

// Git worktree modes.
const (
	GitWorktreeNone           = "none"
	GitWorktreeMountCommonDir = "mount-common-dir"
	GitWorktreeReplace        = "replace"
)

// GitWorktree configures how linked Git worktrees work in containers.
type GitWorktree struct {
	// `none` mounts only the worktree, so Git doesn't work in the container. `mount-common-dir` also mounts the main repository's Git directory, Git's common directory, read-write at its host path, and the workspace at its host path. `replace` replaces the worktree's link with an independent repository before the container is created, and `isodev down` restores it. The mode applies when a container is created, so switch modes with `isodev up --recreate`; a replaced worktree is then restored first if the new mode isn't `replace`. When unset in a worktree, `isodev up` warns and uses `none`, and Paseo and Zed setup fail.
	Mode string `yaml:"mode" json:"mode" jsonschema:"enum=none,enum=mount-common-dir,enum=replace"`
}

// Build configures building the image.
type Build struct {
	// Build context directory, which must exist. It is sent to Docker, so keep credentials and hook scripts outside it.
	Context string `yaml:"context" json:"context" jsonschema:"required"`
	// Dockerfile path, relative to the context.
	Dockerfile string `yaml:"dockerfile" json:"dockerfile"`
}

// Mount types.
const (
	MountBind   = "bind"
	MountVolume = "volume"
)

// Mount is a host path or Docker volume mounted into each new container.
type Mount struct {
	// `bind` mounts a host directory or file. `volume` mounts a Docker volume, which lives on Docker's own disk, so it works for data that needs a real Linux filesystem, such as an inner Docker daemon's `/var/lib/docker`, on macOS too.
	Type string `yaml:"type" json:"type" jsonschema:"required,enum=bind,enum=volume"`
	// For `bind`, the host path, which must exist; `~/` is the host user's home, and relative paths are relative to `~/.config/isodev`. For `volume`, the name of a Docker volume shared by all containers and kept when they are removed. Without a source, a `volume` is private to the container: it is kept by `isodev up --recreate`, emptied by `up --recreate --renew-volumes`, and removed by `isodev down`.
	Source string `yaml:"source" json:"source"`
	// Absolute path in the container. It must not overlap the workspace.
	Target string `yaml:"target" json:"target" jsonschema:"required"`
	// Mount read-only.
	ReadOnly bool `yaml:"read_only" json:"read_only"`
}

// Lifecycle configures the container lifecycle.
type Lifecycle struct {
	// Runs once in each new container, after `on_start`. If a script fails, the rest are skipped and the new container and its network are removed. Changes apply only to new containers.
	OnCreate Stage `yaml:"on_create" json:"on_create"`
	// Runs in each new container before `on_create`, and again whenever Isodev starts a stopped container with `up`, `start`, or Paseo or Zed setup, using the current configuration. If a script fails, the rest are skipped: a new container and its network are removed, and a started container is stopped again. Starting a container with `docker start` skips it.
	OnStart Stage `yaml:"on_start" json:"on_start"`
	// Applied each time Isodev runs a command in a container: `isodev exec`, and Paseo and Zed agent launches. Changes apply without recreating the container.
	OnExec Exec `yaml:"on_exec" json:"on_exec"`
}

// Stage is a lifecycle stage that runs scripts.
type Stage struct {
	// Scripts that run in order, each streamed to `sh` in the container with the workspace as the working directory.
	Scripts []Script `yaml:"scripts" json:"scripts"`
}

// Script is a host-side shell script streamed to `sh` in the container, with
// the workspace as the working directory.
type Script struct {
	// Script path. `~/` is the host user's home, and relative paths are relative to `~/.config/isodev`. The script runs with `sh`, with standard input from `/dev/null`; its shebang is ignored. `ISODEV_PROBE` is `1` in the Paseo probe container and `0` otherwise. Output goes to stderr.
	File string `yaml:"file" json:"file" jsonschema:"required"`
}

// Exec configures commands that isodev runs in containers.
type Exec struct {
	// Names of environment variables copied from the invoking process into each command. Values are read on every invocation and passed by name, so they don't appear in process arguments. Unset variables are omitted; empty values are forwarded. Forward secrets only to containers and commands you trust.
	ForwardEnvVars []string `yaml:"forward_env_vars" json:"forward_env_vars"`
}

// Load reads the configuration file if present, applies defaults and the
// ISODEV_IMAGE override, resolves paths and validates the result.
func Load() (Config, error) {
	cfg := Config{Command: DefaultCommand()}
	path := paths.ConfigFile()
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return cfg, err
	}
	if err == nil {
		if err := checkLegacy(data); err != nil {
			return cfg, fmt.Errorf("read %s: %w", path, err)
		}
		dec := yaml.NewDecoder(bytes.NewReader(data))
		dec.KnownFields(true)
		if err := dec.Decode(&cfg); err != nil && err != io.EOF {
			return cfg, fmt.Errorf("read %s: %w", path, err)
		}
		var extra any
		if err := dec.Decode(&extra); err != io.EOF {
			if err == nil {
				err = fmt.Errorf("multiple YAML documents are not supported")
			}
			return cfg, fmt.Errorf("read %s: %w", path, err)
		}
	}
	if image := os.Getenv("ISODEV_IMAGE"); image != "" {
		cfg.Image = image
	}
	if cfg.Image == "" && cfg.Build != nil {
		return cfg, fmt.Errorf("image is required when build is configured")
	}
	if cfg.Image == "" {
		cfg.Image = DefaultImage
	}
	if len(cfg.Command) == 0 || cfg.Command[0] == "" {
		return cfg, fmt.Errorf("command must not be empty")
	}
	for k := range cfg.Environment {
		if k == "" || strings.ContainsAny(k, "=\x00") {
			return cfg, fmt.Errorf("invalid environment variable name %q", k)
		}
	}
	for _, name := range cfg.Lifecycle.OnExec.ForwardEnvVars {
		if name == "" || strings.ContainsAny(name, "=\x00") {
			return cfg, fmt.Errorf("invalid lifecycle.on_exec environment variable name %q", name)
		}
	}
	switch cfg.GitWorktree.Mode {
	case "", GitWorktreeNone, GitWorktreeMountCommonDir, GitWorktreeReplace:
	default:
		return cfg, fmt.Errorf("git_worktree.mode: unknown mode %q; use %s, %s or %s", cfg.GitWorktree.Mode, GitWorktreeNone, GitWorktreeMountCommonDir, GitWorktreeReplace)
	}
	base := filepath.Dir(path)
	if cfg.Build != nil {
		cfg.Build.Context, err = expandPath(cfg.Build.Context, base)
		if err != nil {
			return cfg, fmt.Errorf("build context: %w", err)
		}
		if cfg.Build.Dockerfile == "" {
			cfg.Build.Dockerfile = DefaultDockerfile
		}
		cfg.Build.Dockerfile, err = expandPath(cfg.Build.Dockerfile, cfg.Build.Context)
		if err != nil {
			return cfg, fmt.Errorf("build dockerfile: %w", err)
		}
	}
	for i := range cfg.Volumes {
		m := &cfg.Volumes[i]
		if m.Type != MountBind && m.Type != MountVolume {
			return cfg, fmt.Errorf("volumes[%d]: unsupported type %q; use %s or %s", i, m.Type, MountBind, MountVolume)
		}
		if !filepath.IsAbs(m.Target) || filepath.Clean(m.Target) == "/" {
			return cfg, fmt.Errorf("volumes[%d]: target must be an absolute non-root path", i)
		}
		m.Target = filepath.Clean(m.Target)
		for j := range cfg.Volumes[:i] {
			if filepath.Clean(cfg.Volumes[j].Target) == m.Target {
				return cfg, fmt.Errorf("volumes[%d]: target %s is already mounted by volumes[%d]", i, m.Target, j)
			}
		}
		if m.Type == MountVolume {
			if m.Source != "" && !volumeName.MatchString(m.Source) {
				return cfg, fmt.Errorf("volumes[%d]: invalid volume name %q; volume names use letters, digits, _, . and -", i, m.Source)
			}
			continue
		}
		m.Source, err = expandPath(m.Source, base)
		if err != nil {
			return cfg, fmt.Errorf("volumes[%d]: %w", i, err)
		}
		if _, err = os.Stat(m.Source); err != nil {
			return cfg, fmt.Errorf("volumes[%d] source %s: %w", i, m.Source, err)
		}
	}
	for _, list := range []struct {
		name    string
		scripts []Script
	}{{"on_start", cfg.Lifecycle.OnStart.Scripts}, {"on_create", cfg.Lifecycle.OnCreate.Scripts}} {
		name, scripts := list.name, list.scripts
		for i := range scripts {
			script := &scripts[i]
			if script.File == "" {
				return cfg, fmt.Errorf("lifecycle.%s.scripts[%d]: file is required", name, i)
			}
			script.File, err = expandPath(script.File, base)
			if err != nil {
				return cfg, fmt.Errorf("lifecycle.%s.scripts[%d]: %w", name, i, err)
			}
			if _, err = os.ReadFile(script.File); err != nil {
				return cfg, fmt.Errorf("lifecycle.%s.scripts[%d] %s: %w", name, i, script.File, err)
			}
		}
	}
	return cfg, nil
}

// volumeName matches the Docker volume names that volumes[].source accepts.
var volumeName = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]*$`)

// checkLegacy rejects earlier forms of settings, with an error that shows the
// current form: docker_storage, a single script or a list of scripts directly
// under on_create or on_start, and a top-level on_exec.
func checkLegacy(data []byte) error {
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil || len(doc.Content) == 0 {
		return nil // The decoder reports syntax errors.
	}
	root := doc.Content[0]
	if storage := mappingValue(root, "docker_storage"); storage != nil {
		return fmt.Errorf("line %d: docker_storage was replaced by a private volume; remove it and add:\n  volumes:\n    - type: volume\n      target: /var/lib/docker", storage.Line)
	}
	if exec := mappingValue(root, "on_exec"); exec != nil {
		return fmt.Errorf("line %d: on_exec belongs under lifecycle; write it as:\n  lifecycle:\n    on_exec:\n      forward_env_vars: [NAME]", exec.Line)
	}
	lifecycle := mappingValue(root, "lifecycle")
	for _, name := range []string{"on_start", "on_create"} {
		stage := mappingValue(lifecycle, name)
		if stage == nil || (stage.Kind == yaml.MappingNode && mappingValue(stage, "file") == nil) {
			continue
		}
		if stage.Kind == yaml.SequenceNode || stage.Kind == yaml.MappingNode {
			return fmt.Errorf("line %d: lifecycle.%s lists its scripts under scripts; write it as:\n  %s:\n    scripts:\n      - file: SCRIPT", stage.Line, name, name)
		}
	}
	return nil
}

func mappingValue(node *yaml.Node, key string) *yaml.Node {
	if node == nil || node.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			return node.Content[i+1]
		}
	}
	return nil
}

// ValidateBuild checks that the configured build context and Dockerfile exist.
func (c Config) ValidateBuild() error {
	if c.Build == nil {
		return nil
	}
	info, err := os.Stat(c.Build.Context)
	if err != nil {
		return fmt.Errorf("build context %s: %w", c.Build.Context, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("build context %s is not a directory", c.Build.Context)
	}
	info, err = os.Stat(c.Build.Dockerfile)
	if err != nil {
		return fmt.Errorf("build dockerfile %s: %w", c.Build.Dockerfile, err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("build dockerfile %s is not a regular file", c.Build.Dockerfile)
	}
	return nil
}

// expandPath resolves ~/ against the user's home and relative paths against base.
func expandPath(path, base string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("path cannot be empty")
	}
	switch {
	case path == "~":
		path = paths.Home
	case strings.HasPrefix(path, "~/"):
		path = filepath.Join(paths.Home, path[2:])
	case strings.HasPrefix(path, "~"):
		return "", fmt.Errorf("only ~/ paths are supported: %s", path)
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(base, path)
	}
	return filepath.Clean(path), nil
}
