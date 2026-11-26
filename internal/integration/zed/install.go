package zed

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/tailscale/hujson"
)

// ConfigPath returns the path of a global Zed configuration file on this machine.
func ConfigPath(name string) (string, error) {
	return configPathForOS(runtime.GOOS, name)
}

func configPathForOS(goos, name string) (string, error) {
	if goos == "darwin" {
		dir, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(dir, ".config", "zed", name), nil
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "zed", name), nil
}

// update modifies JSON with comments without discarding other user settings.
func update(path, initial string, update func(*hujson.Value) error) error {
	// Write through a symlink, such as one into a dotfiles repository, instead
	// of replacing it.
	if target, err := filepath.EvalSymlinks(path); err == nil {
		path = target
	}
	data, err := os.ReadFile(path)
	exists := err == nil
	if os.IsNotExist(err) {
		data = []byte(initial)
	} else if err != nil {
		return err
	}
	v, err := hujson.Parse(data)
	if err != nil {
		return fmt.Errorf("parse %s: %w", path, err)
	}
	if err = update(&v); err != nil {
		return err
	}
	v.Format()
	out := v.Pack()
	if string(out) == string(data) {
		return nil
	}
	if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	if exists {
		backup := fmt.Sprintf("%s.before-isodev-%d", path, time.Now().UnixNano())
		if err = os.WriteFile(backup, data, 0600); err != nil {
			return err
		}
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".isodev-zed-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	mode := os.FileMode(0600)
	if exists {
		if info, e := os.Stat(path); e == nil {
			mode = info.Mode().Perm()
		}
	}
	if err = f.Chmod(mode); err == nil {
		_, err = f.Write(out)
	}
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}
func patch(v *hujson.Value, path string, value any) error {
	op := "add"
	if v.Find(path) != nil {
		op = "replace"
	}
	p, err := json.Marshal([]any{map[string]any{"op": op, "path": path, "value": value}})
	if err != nil {
		return err
	}
	return v.Patch(p)
}

// InstallProvider adds or updates an isodev ACP agent server in the Zed
// settings file at path, preserving comments and backing up the file.
func InstallProvider(path, agent, exe string) error {
	if _, err := ACPCommand(agent); err != nil {
		return err
	}
	name := "OpenCode (isodev)"
	args := []string{"int", "zed", "acp"}
	if agent == "codex" {
		name = "Codex (isodev)"
		args = append(args, "--agent", "codex")
	}
	return update(path, "{}", func(v *hujson.Value) error {
		if v.Value.Kind() != '{' {
			return fmt.Errorf("%s must be a JSON object", path)
		}
		servers := v.Find("/agent_servers")
		if servers == nil {
			if err := patch(v, "/agent_servers", map[string]any{}); err != nil {
				return err
			}
			servers = v.Find("/agent_servers")
		}
		if servers.Value.Kind() != '{' {
			return fmt.Errorf("agent_servers must be an object")
		}
		pointer := "/agent_servers/" + strings.ReplaceAll(strings.ReplaceAll(name, "~", "~0"), "/", "~1")
		if existing := v.Find(pointer); existing != nil {
			var old struct {
				Command string   `json:"command"`
				Args    []string `json:"args"`
				Type    string   `json:"type"`
			}
			standard := existing.Clone()
			standard.Standardize()
			if json.Unmarshal(standard.Pack(), &old) != nil || old.Type != "custom" || filepath.Base(old.Command) != "isodev" || len(old.Args) < 3 || (old.Args[0] != "int" && old.Args[0] != "integration") || old.Args[1] != "zed" || old.Args[2] != "acp" {
				return fmt.Errorf("agent %q already exists and is not an isodev provider", name)
			}
			if err := patch(v, pointer+"/command", exe); err != nil {
				return err
			}
			return patch(v, pointer+"/args", args)
		}
		return patch(v, pointer, map[string]any{"type": "custom", "command": exe, "args": args})
	})
}

// InstallTask adds or updates the manual worktree setup task in the Zed tasks
// file at path, preserving comments and backing up the file.
func InstallTask(path string) error {
	return update(path, "[]", func(v *hujson.Value) error {
		if v.Value.Kind() != '[' {
			return fmt.Errorf("%s must be a JSON array", path)
		}
		copy := v.Clone()
		copy.Standardize()
		var tasks []map[string]json.RawMessage
		if err := json.Unmarshal(copy.Pack(), &tasks); err != nil {
			return err
		}
		index := -1
		for i, task := range tasks {
			var label, command string
			_ = json.Unmarshal(task["label"], &label)
			if label != taskLabel {
				continue
			}
			_ = json.Unmarshal(task["command"], &command)
			if filepath.Base(command) != "isodev" {
				return fmt.Errorf("task %q already belongs to another command", label)
			}
			if index >= 0 {
				return fmt.Errorf("multiple isodev task entries found")
			}
			index = i
		}
		target := "/-"
		if index >= 0 {
			target = fmt.Sprintf("/%d", index)
		}
		return patch(v, target, map[string]any{"label": taskLabel, "command": "isodev", "args": []string{"int", "zed", "setup", "--workspace", "$ZED_WORKTREE_ROOT", "--source", "$ZED_MAIN_GIT_WORKTREE"}, "reevaluate_context": true, "reveal": "no_focus", "hide": "on_success"})
	})
}
