// Package container manages one Docker container per workspace: creation,
// lifecycle hooks, lookup, command execution and removal.
package container

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jakubdyszkiewicz/isodev/internal/fsutil"
	"github.com/jakubdyszkiewicz/isodev/internal/paths"
	"github.com/jakubdyszkiewicz/isodev/internal/shell"
)

// Record is the persisted state of a workspace container.
type Record struct {
	Version            int    `json:"version"`
	Cwd                string `json:"cwd"`
	WorkspaceFolder    string `json:"workspaceFolder,omitempty"`
	MatchWorkspacePath bool   `json:"matchWorkspacePath,omitempty"`
	GitCommonDir       string `json:"gitCommonDir,omitempty"`
	Probe              bool   `json:"probe"`
	Container          string `json:"container"`
	Network            string `json:"network"`
	ContainerID        string `json:"containerId,omitempty"`
}

// Info describes a container in `isodev ls` and `isodev ps` output.
type Info struct {
	Workspace string `json:"workspace"`
	Name      string `json:"container"`
	ID        string `json:"id"`
	Image     string `json:"image"`
	State     string `json:"state"`
	Status    string `json:"status"`
}

// Docker labels identify isodev resources and the workspace they belong to.
const (
	labelManaged   = "isodev.managed"
	labelWorkspace = "isodev.workspace"
)

func managedLabels(cwd string) []string {
	return []string{"--label", labelManaged + "=true", "--label", labelWorkspace + "=" + cwd}
}

// Name returns the container name for a workspace. The network is Name+"-net".
func Name(cwd string) string {
	h := sha256.Sum256([]byte(cwd))
	return fmt.Sprintf("isodev-%x", h[:8])
}

// ProbeDir is the workspace directory used for Paseo's provider probe.
func ProbeDir() string { return filepath.Join(paths.StateDir(), "probe") }

func recordPath(cwd string) string {
	return filepath.Join(paths.StateDir(), "workspaces", Name(cwd)+".json")
}

func probeRecordPath() string { return filepath.Join(paths.StateDir(), "probe.json") }

// Lookup returns the record for exactly this workspace. The error satisfies
// os.IsNotExist when the workspace has no container.
func Lookup(cwd string) (Record, error) {
	cwd, err := fsutil.RealOrAbs(cwd)
	if err != nil {
		return Record{}, err
	}
	var r Record
	if err = fsutil.ReadJSON(recordPath(cwd), &r); os.IsNotExist(err) {
		var probe Record
		if probeErr := fsutil.ReadJSON(probeRecordPath(), &probe); probeErr == nil && probe.Probe && probe.Cwd == cwd {
			r, err = probe, nil
		} else if probeErr != nil && !os.IsNotExist(probeErr) {
			return r, probeErr
		}
	}
	if err != nil {
		return r, err
	}
	if r.Cwd != cwd || r.Container == "" {
		return r, fmt.Errorf("invalid workspace mapping")
	}
	return r, nil
}

// FindForPath returns the container of the nearest workspace containing path.
func FindForPath(path string) (Record, error) {
	path, err := fsutil.Real(path)
	if err != nil {
		return Record{}, err
	}
	start := path
	for {
		r, err := Lookup(path)
		if err == nil {
			return r, nil
		}
		if !os.IsNotExist(err) {
			return Record{}, err
		}
		parent := filepath.Dir(path)
		if parent == path {
			return Record{}, fmt.Errorf("no Isodev container for %s or its parent directories", start)
		}
		path = parent
	}
}

func listArgs(all bool) []string {
	args := []string{"ps", "--filter", "label=" + labelManaged + "=true"}
	if all {
		args = append(args, "--all")
	}
	return append(args, "--format", `{{json .}}\t{{json (.Label "`+labelWorkspace+`")}}`)
}

// List returns isodev containers; all includes stopped ones.
func List(all bool) ([]Info, error) {
	out, err := shell.Run("docker", listArgs(all)...)
	if err != nil {
		return nil, err
	}
	items := []Info{}
	if out == "" {
		return items, nil
	}
	for _, line := range strings.Split(out, "\n") {
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid Docker container list output")
		}
		var detail struct{ ID, Names, Image, State, Status string }
		var workspace string
		if err := json.Unmarshal([]byte(parts[0]), &detail); err != nil {
			return nil, fmt.Errorf("invalid Docker container list: %w", err)
		}
		if err := json.Unmarshal([]byte(parts[1]), &workspace); err != nil {
			return nil, fmt.Errorf("invalid workspace label: %w", err)
		}
		items = append(items, Info{Workspace: workspace, Name: detail.Names, ID: detail.ID, Image: detail.Image, State: detail.State, Status: detail.Status})
	}
	return items, nil
}
