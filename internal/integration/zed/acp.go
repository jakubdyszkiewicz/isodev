// Package zed integrates isodev with Zed worktree tasks and ACP agent servers.
package zed

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/jakubdyszkiewicz/isodev/internal/container"
	"github.com/jakubdyszkiewicz/isodev/internal/fsutil"
)

type acpWriter struct {
	mu sync.Mutex
	w  io.Writer
}

func (w *acpWriter) send(b []byte) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	_, err := w.w.Write(append(b, '\n'))
	return err
}

func acpError(id json.RawMessage, message string) []byte {
	b, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": id, "error": map[string]any{"code": -32602, "message": message}})
	return b
}

// filterClientACP restricts a connection to one workspace and advertises no host file or terminal tools.
func filterClientACP(line []byte, workspace string) ([]byte, []byte, error) {
	var msg map[string]json.RawMessage
	if err := json.Unmarshal(line, &msg); err != nil {
		return nil, nil, fmt.Errorf("invalid ACP message: %w", err)
	}
	var method string
	_ = json.Unmarshal(msg["method"], &method)
	if method == "initialize" {
		var params map[string]json.RawMessage
		if err := json.Unmarshal(msg["params"], &params); err != nil || params == nil {
			return nil, nil, fmt.Errorf("invalid ACP initialize parameters")
		}
		var caps map[string]json.RawMessage
		if len(params["clientCapabilities"]) > 0 {
			if err := json.Unmarshal(params["clientCapabilities"], &caps); err != nil {
				return nil, nil, err
			}
		}
		if caps == nil {
			caps = make(map[string]json.RawMessage)
		}
		caps["terminal"] = json.RawMessage(`false`)
		caps["fs"] = json.RawMessage(`{"readTextFile":false,"writeTextFile":false}`)
		params["clientCapabilities"], _ = json.Marshal(caps)
		msg["params"], _ = json.Marshal(params)
		b, err := json.Marshal(msg)
		return b, nil, err
	}
	if method == "session/new" || method == "session/load" || method == "session/resume" {
		var params struct {
			Cwd string `json:"cwd"`
		}
		if err := json.Unmarshal(msg["params"], &params); err != nil || params.Cwd == "" {
			return nil, acpError(msg["id"], "isodev requires a workspace cwd"), nil
		}
		cwd, err := fsutil.Real(params.Cwd)
		if err != nil {
			return nil, acpError(msg["id"], "isodev cannot resolve session workspace: "+err.Error()), nil
		}
		rel, err := filepath.Rel(workspace, cwd)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return nil, acpError(msg["id"], fmt.Sprintf("session workspace %s is outside isodev workspace %s; open it in its own Zed window", cwd, workspace)), nil
		}
	}
	return line, nil, nil
}

func scanACP(r io.Reader, handle func([]byte) error) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 4096), 64*1024*1024)
	for scanner.Scan() {
		if err := handle(scanner.Bytes()); err != nil {
			return err
		}
	}
	return scanner.Err()
}

// ACPCommand returns the ACP agent command to run in the container.
func ACPCommand(agent string) ([]string, error) {
	switch agent {
	case "opencode", "opencode-acp":
		return []string{"opencode", "acp"}, nil
	case "codex":
		return []string{"codex-acp"}, nil
	default:
		return nil, fmt.Errorf("unsupported ACP agent %q: use opencode or codex", agent)
	}
}

// Bridge connects Zed on stdio to an ACP agent in the running container of the
// workspace containing path. Sessions outside that workspace are rejected and
// host terminal and filesystem capabilities are disabled in both directions.
func Bridge(path, agentName string) error {
	command, err := ACPCommand(agentName)
	if err != nil {
		return err
	}
	if path == "" {
		return fmt.Errorf("workspace must not be empty")
	}
	r, err := container.FindForPath(path)
	if err != nil {
		return err
	}
	if r.WorkspaceFolder != r.Cwd {
		return fmt.Errorf("workspace container must use the same host and container path; prepare it with isodev int zed setup")
	}
	toAgentR, toAgentW := io.Pipe()
	fromAgentR, fromAgentW := io.Pipe()
	agent := &acpWriter{w: toAgentW}
	client := &acpWriter{w: os.Stdout}
	go func() {
		err := scanACP(os.Stdin, func(line []byte) error {
			msg, reject, err := filterClientACP(line, r.Cwd)
			if err != nil {
				return err
			}
			if reject != nil {
				return client.send(reject)
			}
			return agent.send(msg)
		})
		_ = toAgentW.CloseWithError(err)
	}()
	go func() {
		err := scanACP(fromAgentR, func(line []byte) error {
			var msg struct {
				Method string          `json:"method"`
				ID     json.RawMessage `json:"id"`
			}
			if err := json.Unmarshal(line, &msg); err != nil {
				return fmt.Errorf("invalid agent ACP message: %w", err)
			}
			if strings.HasPrefix(msg.Method, "terminal/") || strings.HasPrefix(msg.Method, "fs/") {
				if len(msg.ID) > 0 {
					return agent.send(acpError(msg.ID, "isodev does not allow host terminal or filesystem requests"))
				}
				return nil
			}
			return client.send(line)
		})
		_ = fromAgentR.CloseWithError(err)
	}()
	err = container.LaunchIO(r.Container, command, false, toAgentR, fromAgentW)
	_ = toAgentR.Close()
	_ = fromAgentW.Close()
	_ = fromAgentR.Close()
	return err
}
