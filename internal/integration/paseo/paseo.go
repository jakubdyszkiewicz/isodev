// Package paseo integrates isodev with Paseo worktree hooks and agent providers.
package paseo

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jakubdyszkiewicz/isodev/internal/container"
	"github.com/jakubdyszkiewicz/isodev/internal/fsutil"
	"github.com/jakubdyszkiewicz/isodev/internal/paths"
	"github.com/jakubdyszkiewicz/isodev/internal/workspace"
)

// SetupPaths reads the worktree and source checkout from Paseo's hook environment.
func SetupPaths() (string, string, error) {
	path := os.Getenv("PASEO_WORKTREE_PATH")
	source := os.Getenv("PASEO_SOURCE_CHECKOUT_PATH")
	if source == "" {
		source = os.Getenv("PASEO_ROOT_PATH")
	}
	if path == "" || source == "" {
		return "", "", fmt.Errorf("paseo setup requires PASEO_WORKTREE_PATH and PASEO_SOURCE_CHECKOUT_PATH")
	}
	return path, source, nil
}

// TeardownPath reads the worktree from Paseo's hook environment.
func TeardownPath() (string, error) {
	path := os.Getenv("PASEO_WORKTREE_PATH")
	if path == "" {
		return "", fmt.Errorf("paseo teardown requires PASEO_WORKTREE_PATH")
	}
	return path, nil
}

// setupTimeout is how long an agent waits for Paseo's setup hook to create its
// workspace's container, which can include building the image.
const setupTimeout = 15 * time.Minute

// agentContainer returns the running container for the agent's workspace.
// Paseo may launch the agent before the setup hook finishes, so the record is
// awaited. A container that is stopped or gone, for example after isodev stop
// or a reboot, is started or created again as setup would. Without a
// workspace, Paseo is probing the provider and a separate probe container is
// used. Progress goes to stderr; stdout belongs to the agent.
func agentContainer() (container.Record, error) {
	cwd := os.Getenv("PASEO_AGENT_CWD")
	if cwd == "" {
		p := container.ProbeDir()
		if err := os.MkdirAll(p, 0700); err != nil {
			return container.Record{}, err
		}
		var r container.Record
		err := workspace.Lock(p, func() error {
			var err error
			r, err = container.Provision(p, container.Options{Probe: true})
			return err
		})
		return r, err
	}
	deadline := time.Now().Add(setupTimeout)
	waiting := false
	for {
		r, err := container.Lookup(cwd)
		if err == nil {
			if container.State(cwd) == container.Running {
				return r, nil
			}
			res, err := workspace.Up(cwd, setupOptions(""))
			return res.Container, err
		}
		if !os.IsNotExist(err) {
			return r, err
		}
		if time.Now().After(deadline) {
			return r, fmt.Errorf("no Isodev container for %s after %s; check that the project's paseo.json runs isodev int paseo setup", cwd, setupTimeout)
		}
		if !waiting {
			fmt.Fprintf(os.Stderr, "Waiting for Paseo's setup hook to create the Isodev container for %s\n", cwd)
			waiting = true
		}
		time.Sleep(500 * time.Millisecond)
	}
}

// Exec runs an agent command in the container of the Paseo agent's workspace.
func Exec(args []string) error {
	if len(args) == 0 || args[0] == "" {
		return fmt.Errorf("exec requires a command")
	}
	r, err := agentContainer()
	if err != nil {
		return err
	}
	return container.Launch(r.Container, args, true)
}

// PreviewExec describes Exec for --dry-run.
func PreviewExec(w io.Writer, args []string) error {
	cwd := os.Getenv("PASEO_AGENT_CWD")
	if cwd == "" {
		fmt.Fprintln(w, "# Provision the probe container if needed")
		return container.PreviewLaunch(w, "<probe-container-id>", args, true)
	}
	cwd, err := fsutil.Real(cwd)
	if err != nil {
		return err
	}
	fmt.Fprintln(w, "# Look up or wait for the workspace container")
	r, err := container.Lookup(cwd)
	if err != nil {
		fmt.Fprintln(w, "# Container record not yet available")
		return container.PreviewLaunch(w, "<workspace-container-id>", args, true)
	}
	return container.PreviewLaunch(w, r.Container, args, true)
}

// currentContainer returns the container of the workspace containing the
// current directory, as in a Paseo workspace terminal.
func currentContainer() (container.Record, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return container.Record{}, err
	}
	return container.FindForPath(cwd)
}

// Enter opens a shell in the current workspace's container.
func Enter() error {
	r, err := currentContainer()
	if err != nil {
		return err
	}
	return container.Enter(r)
}

// PreviewEnter describes Enter for --dry-run.
func PreviewEnter(w io.Writer) error {
	r, err := currentContainer()
	if err != nil {
		return err
	}
	container.PreviewEnter(w, r)
	return nil
}

// Setup is the Paseo worktree setup hook.
func Setup(path, source string) (workspace.Result, error) {
	return workspace.Up(path, setupOptions(source))
}

func setupOptions(source string) workspace.Options {
	return workspace.Options{Integration: "Paseo", Source: source, MatchWorkspacePath: true, Warn: os.Stderr}
}

// PreviewSetup describes Setup for --dry-run.
func PreviewSetup(w io.Writer, path, source string) error {
	return workspace.PreviewUp(w, path, setupOptions(source))
}

// providerTypes are the provider types supported by InstallProvider.
var providerTypes = []string{"codex", "claude", "opencode-acp"}

func checkProviderType(providerType string) error {
	for _, t := range providerTypes {
		if t == providerType {
			return nil
		}
	}
	return fmt.Errorf("paseo install-provider requires --type=codex, --type=claude, or --type=opencode-acp")
}

func configPath() string { return filepath.Join(paths.Home, ".paseo", "config.json") }

// providerCommand is the command Paseo runs for the provider.
func providerCommand(providerType string) []string {
	switch providerType {
	case "claude":
		return []string{"isodev", "int", "paseo", "exec", "claude"}
	case "opencode-acp":
		return []string{"isodev", "int", "paseo", "exec", "opencode", "acp"}
	default:
		return []string{"isodev", "int", "paseo", "exec", "codex"}
	}
}

func providerConfig(providerType string, command []string) map[string]any {
	noTools := map[string]bool{"enabled": false}
	switch providerType {
	case "codex":
		return map[string]any{"extends": "codex", "label": "Codex (Isodev)", "command": command, "options": map[string]string{"approval_policy": "never", "sandbox_mode": "danger-full-access"}, "paseoTools": noTools}
	case "claude":
		return map[string]any{"extends": "claude", "label": "Claude (Isodev)", "command": command, "paseoTools": noTools}
	default:
		return map[string]any{"extends": "acp", "label": "OpenCode ACP (Isodev)", "command": command, "options": map[string]any{"clientCapabilities": map[string]any{"terminal": false, "fs": map[string]bool{"readTextFile": false, "writeTextFile": false}}}, "paseoTools": noTools}
	}
}

// checkOwnership rejects replacing a provider entry that another installation
// owns. isodev's own entries may use either "int" or "integration".
func checkOwnership(id string, existing any, exe string, command []string) error {
	old, ok := existing.(map[string]any)
	if !ok {
		return nil
	}
	foreign := fmt.Errorf("provider %s already belongs to another installation", id)
	a, ok := old["command"].([]any)
	if !ok || len(a) == 0 {
		return foreign
	}
	first, ok := a[0].(string)
	if !ok || (first != exe && filepath.Base(first) != "isodev") {
		return foreign
	}
	parts := make([]string, 0, len(a)-1)
	for _, arg := range a[1:] {
		value, ok := arg.(string)
		if !ok {
			return foreign
		}
		parts = append(parts, value)
	}
	if len(parts) > 0 && parts[0] == "integration" {
		parts[0] = "int"
	}
	if strings.Join(parts, " ") != strings.Join(command[1:], " ") {
		return foreign
	}
	return nil
}

// InstallProvider adds or updates an isodev provider in ~/.paseo/config.json,
// backing up the previous file.
func InstallProvider(providerType string) error {
	if err := checkProviderType(providerType); err != nil {
		return err
	}
	p := configPath()
	// Write through a symlink, such as one into a dotfiles repository, instead
	// of replacing it.
	if target, err := filepath.EvalSymlinks(p); err == nil {
		p = target
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return err
	}
	var cfg map[string]any
	if err = json.Unmarshal(b, &cfg); err != nil {
		return err
	}
	agents, ok := cfg["agents"].(map[string]any)
	if !ok {
		agents = map[string]any{}
		cfg["agents"] = agents
	}
	providers, ok := agents["providers"].(map[string]any)
	if !ok {
		providers = map[string]any{}
		agents["providers"] = providers
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	id := "isodev-" + providerType
	command := providerCommand(providerType)
	if err := checkOwnership(id, providers[id], exe, command); err != nil {
		return err
	}
	providers[id] = providerConfig(providerType, command)
	if err = os.WriteFile(fmt.Sprintf("%s.before-isodev-provider-%d", p, time.Now().UnixMilli()), b, 0600); err != nil {
		return err
	}
	tmp := p + ".isodev-provider-tmp"
	if err = fsutil.WriteJSON(tmp, cfg); err != nil {
		return err
	}
	return os.Rename(tmp, p)
}

// PreviewInstallProvider describes InstallProvider for --dry-run.
func PreviewInstallProvider(w io.Writer, providerType string) error {
	if err := checkProviderType(providerType); err != nil {
		return err
	}
	fmt.Fprintf(w, "# Back up and update %s with the isodev-%s provider\n", configPath(), providerType)
	return nil
}
