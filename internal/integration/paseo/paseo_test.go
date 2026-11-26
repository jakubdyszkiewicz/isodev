package paseo

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jakubdyszkiewicz/isodev/internal/fsutil"
	"github.com/jakubdyszkiewicz/isodev/internal/testutil"
)

func TestInstallOpenCodeACPProvider(t *testing.T) {
	home := testutil.Home(t)
	path := filepath.Join(home, ".paseo", "config.json")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	initial, err := json.Marshal(map[string]any{"version": 1, "agents": map[string]any{"providers": map[string]any{"isodev-opencode-acp": map[string]any{"command": []string{filepath.Join(home, ".local/bin/isodev"), "integration", "paseo", "exec", "opencode", "acp"}}}}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, initial, 0600); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := InstallProvider("opencode-acp"); err != nil {
			t.Fatal(err)
		}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var config struct {
		Agents struct {
			Providers map[string]struct {
				Extends string   `json:"extends"`
				Label   string   `json:"label"`
				Command []string `json:"command"`
				Options struct {
					ClientCapabilities struct {
						Terminal bool            `json:"terminal"`
						FS       map[string]bool `json:"fs"`
					} `json:"clientCapabilities"`
				} `json:"options"`
				PaseoTools map[string]bool `json:"paseoTools"`
			} `json:"providers"`
		} `json:"agents"`
	}
	if err := json.Unmarshal(data, &config); err != nil {
		t.Fatal(err)
	}
	p := config.Agents.Providers["isodev-opencode-acp"]
	if p.Extends != "acp" || p.Label != "OpenCode ACP (Isodev)" || strings.Join(p.Command, " ") != "isodev int paseo exec opencode acp" || p.Options.ClientCapabilities.Terminal || p.Options.ClientCapabilities.FS["readTextFile"] || p.Options.ClientCapabilities.FS["writeTextFile"] || p.PaseoTools["enabled"] {
		t.Fatalf("unexpected provider: %+v", p)
	}
}

func TestInstallProviderReplacesExistingCommand(t *testing.T) {
	home := testutil.Home(t)
	path := filepath.Join(home, ".paseo", "config.json")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	config := `{"agents":{"providers":{"isodev-codex":{"command":["/usr/local/bin/isodev","integration","paseo","exec","codex"]}}}}`
	if err := os.WriteFile(path, []byte(config), 0600); err != nil {
		t.Fatal(err)
	}
	if err := InstallProvider("codex"); err != nil {
		t.Fatal(err)
	}
	var data struct {
		Agents struct {
			Providers map[string]struct {
				Command []string `json:"command"`
			} `json:"providers"`
		} `json:"agents"`
	}
	if err := fsutil.ReadJSON(path, &data); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(data.Agents.Providers["isodev-codex"].Command, " "); got != "isodev int paseo exec codex" {
		t.Fatalf("unexpected command: %s", got)
	}
}

func TestInstallClaudeProvider(t *testing.T) {
	home := testutil.Home(t)
	path := filepath.Join(home, ".paseo", "config.json")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"agents":{"providers":{}}}`), 0600); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := InstallProvider("claude"); err != nil {
			t.Fatal(err)
		}
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var cfg struct {
		Agents struct {
			Providers map[string]struct {
				Extends    string          `json:"extends"`
				Command    []string        `json:"command"`
				PaseoTools map[string]bool `json:"paseoTools"`
			} `json:"providers"`
		} `json:"agents"`
	}
	if err := json.Unmarshal(b, &cfg); err != nil {
		t.Fatal(err)
	}
	p := cfg.Agents.Providers["isodev-claude"]
	if p.Extends != "claude" || strings.Join(p.Command, " ") != "isodev int paseo exec claude" || p.PaseoTools["enabled"] {
		t.Fatalf("unexpected provider: %+v", p)
	}
}

func TestInstallProviderRejectsForeignEntry(t *testing.T) {
	home := testutil.Home(t)
	path := filepath.Join(home, ".paseo", "config.json")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	config := `{"agents":{"providers":{"isodev-codex":{"command":["/usr/bin/other","exec","codex"]}}}}`
	if err := os.WriteFile(path, []byte(config), 0600); err != nil {
		t.Fatal(err)
	}
	if err := InstallProvider("codex"); err == nil || !strings.Contains(err.Error(), "another installation") {
		t.Fatalf("expected ownership error, got %v", err)
	}
	if err := InstallProvider("unknown"); err == nil {
		t.Fatal("expected unsupported provider error")
	}
}
