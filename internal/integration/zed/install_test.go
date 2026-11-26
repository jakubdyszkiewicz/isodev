package zed

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tailscale/hujson"
)

func TestMacConfigPath(t *testing.T) {
	t.Setenv("HOME", "/Users/example")
	t.Setenv("XDG_CONFIG_HOME", "/tmp/other-config")
	for _, name := range []string{"tasks.json", "settings.json"} {
		got, err := configPathForOS("darwin", name)
		want := filepath.Join("/Users/example/.config/zed", name)
		if err != nil || got != want {
			t.Fatalf("got %q, %v; want %q", got, err, want)
		}
	}
}

func TestProviderInstallPreservesCommentsAndEntries(t *testing.T) {
	p := filepath.Join(t.TempDir(), "settings.json")
	original := `{ // keep this comment
 "font_size": 17,
 "agent_servers": {"existing": {"type": "custom", "command": "other"},},
}`
	if err := os.WriteFile(p, []byte(original), 0644); err != nil {
		t.Fatal(err)
	}
	for _, agent := range []string{"codex", "opencode-acp", "codex"} {
		if err := InstallProvider(p, agent, "/tmp/isodev"); err != nil {
			t.Fatal(err)
		}
	}
	data, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "keep this comment") {
		t.Fatalf("comment lost: %s", data)
	}
	standard, err := hujson.Standardize(data)
	if err != nil {
		t.Fatal(err)
	}
	var cfg struct {
		FontSize int `json:"font_size"`
		Agents   map[string]struct {
			Command string   `json:"command"`
			Args    []string `json:"args"`
		} `json:"agent_servers"`
	}
	if err := json.Unmarshal(standard, &cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.FontSize != 17 || len(cfg.Agents) != 3 || cfg.Agents["existing"].Command != "other" || strings.Join(cfg.Agents["Codex (isodev)"].Args, " ") != "int zed acp --agent codex" {
		t.Fatalf("unexpected settings: %s", data)
	}
	backups, _ := filepath.Glob(p + ".before-isodev-*")
	if len(backups) < 1 {
		t.Fatal("missing backup")
	}
}
func TestTaskInstallPreservesAndIsIdempotent(t *testing.T) {
	p := filepath.Join(t.TempDir(), "tasks.json")
	if err := os.WriteFile(p, []byte("[// keep\n{\"label\":\"other\",\"command\":\"true\"},\n{\"label\":\"Prepare isodev worktree\",\"command\":\"/tmp/isodev\",\"hooks\":[\"create_worktree\"]},\n]"), 0600); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := InstallTask(p); err != nil {
			t.Fatal(err)
		}
	}
	data, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "// keep") || strings.Count(string(data), "Prepare isodev worktree") != 1 || strings.Contains(string(data), "create_worktree") || !strings.Contains(string(data), "reevaluate_context") {
		t.Fatalf("unexpected tasks: %s", data)
	}
	standard, err := hujson.Standardize(data)
	if err != nil {
		t.Fatal(err)
	}
	var tasks []struct {
		Label   string `json:"label"`
		Command string `json:"command"`
	}
	if err := json.Unmarshal(standard, &tasks); err != nil {
		t.Fatal(err)
	}
	if tasks[1].Command != "isodev" {
		t.Fatalf("task command = %q, want isodev", tasks[1].Command)
	}
	if err := os.WriteFile(p, []byte(`[{"label":"Prepare isodev worktree","command":"other"}]`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := InstallTask(p); err == nil {
		t.Fatal("expected conflict")
	}
}
