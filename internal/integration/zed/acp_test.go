package zed

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestACPCommand(t *testing.T) {
	for _, tc := range []struct{ agent, want string }{{"opencode", "opencode acp"}, {"codex", "codex-acp"}} {
		got, err := ACPCommand(tc.agent)
		if err != nil || strings.Join(got, " ") != tc.want {
			t.Fatalf("%s: %v, %v", tc.agent, got, err)
		}
	}
	if _, err := ACPCommand("unknown"); err == nil {
		t.Fatal("expected unsupported agent error")
	}
}

func TestACPClientFiltering(t *testing.T) {
	root := t.TempDir()
	child := filepath.Join(root, "child")
	if err := os.Mkdir(child, 0700); err != nil {
		t.Fatal(err)
	}
	init := []byte(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":1,"clientCapabilities":{"terminal":true,"fs":{"readTextFile":true,"writeTextFile":true},"other":true}}}`)
	msg, reject, err := filterClientACP(init, root)
	if err != nil || reject != nil {
		t.Fatalf("initialize: %s %v", reject, err)
	}
	var data struct {
		Params struct {
			Caps map[string]any `json:"clientCapabilities"`
		} `json:"params"`
	}
	if err := json.Unmarshal(msg, &data); err != nil {
		t.Fatal(err)
	}
	if data.Params.Caps["terminal"] != false || data.Params.Caps["other"] != true {
		t.Fatalf("capabilities: %s", msg)
	}
	fs := data.Params.Caps["fs"].(map[string]any)
	if fs["readTextFile"] != false || fs["writeTextFile"] != false {
		t.Fatalf("filesystem enabled: %s", msg)
	}
	for _, tc := range []struct {
		path     string
		accepted bool
	}{{root, true}, {child, true}, {t.TempDir(), false}, {root + "-other", false}, {"", false}} {
		payload, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 2, "method": "session/new", "params": map[string]string{"cwd": tc.path}})
		_, rejection, err := filterClientACP(payload, root)
		if err != nil || (rejection == nil) != tc.accepted {
			t.Fatalf("path %q: rejection %s err %v", tc.path, rejection, err)
		}
	}
	if _, _, err := filterClientACP([]byte(`{`), root); err == nil || !strings.Contains(err.Error(), "invalid ACP") {
		t.Fatalf("invalid JSON: %v", err)
	}
}
