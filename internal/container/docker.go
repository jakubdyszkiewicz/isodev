package container

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"

	"github.com/jakubdyszkiewicz/isodev/internal/config"
	"github.com/jakubdyszkiewicz/isodev/internal/shell"
)

// inspection is the subset of `docker inspect` output that isodev checks.
type inspection struct {
	ID     string `json:"Id"`
	Config struct {
		Labels     map[string]string
		WorkingDir string
	}
	State  struct{ Running bool }
	Mounts []struct {
		Type, Source, Destination string
	}
	NetworkSettings struct{ Networks map[string]json.RawMessage }
}

func exists(args ...string) bool { _, err := shell.Run("docker", args...); return err == nil }

func inspect(id string) (inspection, error) {
	var arr []inspection
	s, err := shell.Run("docker", "inspect", id)
	if err != nil {
		return inspection{}, err
	}
	err = json.Unmarshal([]byte(s), &arr)
	if err != nil || len(arr) != 1 {
		return inspection{}, fmt.Errorf("invalid docker inspection: %w", err)
	}
	return arr[0], nil
}

func buildArgs(cfg config.Config) []string {
	return []string{"build", "--tag", cfg.Image, "--file", cfg.Build.Dockerfile, cfg.Build.Context}
}

// buildImage builds the configured image, if any, streaming output to stderr.
func buildImage(cfg config.Config) error {
	if cfg.Build == nil {
		return nil
	}
	if err := cfg.ValidateBuild(); err != nil {
		return err
	}
	cmd := exec.Command("docker", buildArgs(cfg)...)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	cmd.Stdout, cmd.Stderr = os.Stderr, os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("build image %s: %w", cfg.Image, err)
	}
	return nil
}

func bindArgument(source, target string) string {
	return mountArgument(config.Mount{Type: config.MountBind, Source: source, Target: target})
}

func mountArgument(m config.Mount) string {
	fields := []string{"type=" + m.Type, "source=" + m.Source, "target=" + m.Target}
	if m.ReadOnly {
		fields = append(fields, "readonly")
	}
	var b bytes.Buffer
	w := csv.NewWriter(&b)
	_ = w.Write(fields)
	w.Flush()
	return strings.TrimSuffix(b.String(), "\n")
}

func overlaps(a, b string) bool {
	return a == b || strings.HasPrefix(a, b+"/") || strings.HasPrefix(b, a+"/")
}

// runArgs returns the `docker run` arguments that create r's container.
func runArgs(r Record, cfg config.Config) ([]string, error) {
	args := append([]string{"run", "-d", "--init", "--name", r.Container, "--hostname", r.Container}, managedLabels(r.Cwd)...)
	args = append(args, "--network", r.Network)
	if cfg.Privileged {
		args = append(args, "--privileged")
	}
	keys := make([]string, 0, len(cfg.Environment))
	for k := range cfg.Environment {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		args = append(args, "--env", k+"="+cfg.Environment[k])
	}
	for _, m := range cfg.Volumes {
		if m.Target == r.WorkspaceFolder || strings.HasPrefix(r.WorkspaceFolder, m.Target+"/") {
			return nil, fmt.Errorf("mount target %s overlaps workspace %s", m.Target, r.WorkspaceFolder)
		}
		if r.GitCommonDir != "" && overlaps(m.Target, r.GitCommonDir) {
			return nil, fmt.Errorf("mount target %s overlaps the Git common directory %s", m.Target, r.GitCommonDir)
		}
		if m.Type == config.MountVolume {
			m.Source = volumeSource(r, m)
		}
		args = append(args, "--mount", mountArgument(m))
	}
	seen := map[string]bool{}
	for _, name := range privateVolumes(r, cfg) {
		if seen[name] {
			return nil, fmt.Errorf("two private volumes would be named %s; change one of their targets", name)
		}
		seen[name] = true
	}
	if r.GitCommonDir != "" {
		if r.WorkspaceFolder != r.Cwd {
			return nil, fmt.Errorf("mounting the Git common directory requires the workspace at its host path")
		}
		args = append(args, "--mount", bindArgument(r.GitCommonDir, r.GitCommonDir))
		// Other worktrees aren't mounted, so automatic garbage collection in
		// the container would prune their registrations.
		if _, set := cfg.Environment["GIT_CONFIG_COUNT"]; !set {
			args = append(args, "--env", "GIT_CONFIG_COUNT=1", "--env", "GIT_CONFIG_KEY_0=gc.worktreePruneExpire", "--env", "GIT_CONFIG_VALUE_0=never")
		}
	}
	args = append(args, "--mount", bindArgument(r.Cwd, r.WorkspaceFolder), "--workdir", r.WorkspaceFolder, cfg.Image)
	return append(args, cfg.Command...), nil
}

func hookArgs(r Record) []string {
	probe := "0"
	if r.Probe {
		probe = "1"
	}
	// The script is read completely before it runs, with stdin from /dev/null,
	// so commands in it that read stdin can't consume the rest of the script.
	const run = `f=$(mktemp) && cat >"$f" && sh "$f" </dev/null; status=$?; rm -f "$f"; exit $status`
	return []string{"exec", "-i", "--workdir", r.WorkspaceFolder, "--env", "ISODEV_PROBE=" + probe, r.Container, "sh", "-c", run}
}

// runHooks runs hooks in order and stops at the first failure.
func runHooks(r Record, name string, hooks []config.Script) error {
	for i, hook := range hooks {
		if err := runHook(r, fmt.Sprintf("lifecycle.%s.scripts[%d]", name, i), hook); err != nil {
			return err
		}
	}
	return nil
}

// runHook streams a host-side hook script to sh in the container.
func runHook(r Record, name string, hook config.Script) error {
	f, err := os.Open(hook.File)
	if err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	defer f.Close()
	cmd := exec.Command("docker", hookArgs(r)...)
	cmd.Stdin = f
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s (%s) failed: %w", name, hook.File, err)
	}
	return nil
}
