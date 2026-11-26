package cli

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/jakubdyszkiewicz/isodev/internal/config"
	"github.com/jakubdyszkiewicz/isodev/internal/container"
	"github.com/jakubdyszkiewicz/isodev/internal/fsutil"
	"github.com/jakubdyszkiewicz/isodev/internal/paths"
	"github.com/jakubdyszkiewicz/isodev/internal/testutil"
)

// writeRecord stores a workspace record as if the container had been created.
func writeRecord(t *testing.T, r container.Record) {
	t.Helper()
	path := filepath.Join(paths.StateDir(), "workspaces", container.Name(r.Cwd)+".json")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := fsutil.WriteJSON(path, r); err != nil {
		t.Fatal(err)
	}
}

func TestDryRunExecForwardsArguments(t *testing.T) {
	testutil.Home(t)
	workspace := t.TempDir()
	writeRecord(t, container.Record{Cwd: workspace, Container: "worker"})
	var out bytes.Buffer
	cmd := NewRootCommand()
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--dry-run", "exec", "--workspace", workspace, "printf", "%s", "hello world", "--version"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "worker setsid") || !strings.Contains(out.String(), "printf %s 'hello world' --version") {
		t.Fatalf("unexpected preview: %s", out.String())
	}
}

func TestDryRunShowsImageBuild(t *testing.T) {
	dir := testutil.Config(t, "image: example:latest\nbuild:\n  context: ./image\n")
	imageDir := filepath.Join(dir, "image")
	if err := os.Mkdir(imageDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(imageDir, "Dockerfile"), []byte("FROM ubuntu:24.04\n"), 0600); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	cmd := NewRootCommand()
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--dry-run", "up", "--workspace", t.TempDir()})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "docker build --tag example:latest --file ") {
		t.Fatalf("missing build preview: %s", out.String())
	}
}

func TestDryRunRejectsEmptyWorkdir(t *testing.T) {
	cmd := NewRootCommand()
	cmd.SetArgs([]string{"--dry-run", "up", "--workspace", ""})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected empty workdir error")
	}
}

func TestDryRunUpDefaultsToCurrentDirectory(t *testing.T) {
	testutil.Config(t, "image: ubuntu:24.04\n")
	var out bytes.Buffer
	cmd := NewRootCommand()
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--dry-run", "up"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "docker run") {
		t.Fatalf("unexpected preview: %s", out.String())
	}
}

func TestDryRunPaseoSetupWithoutBranch(t *testing.T) {
	testutil.Config(t, "image: ubuntu:24.04\ngit_worktree:\n  mode: replace\n")
	repo, work := newWorktree(t)
	t.Setenv("PASEO_WORKTREE_PATH", work)
	t.Setenv("PASEO_SOURCE_CHECKOUT_PATH", repo)
	t.Setenv("PASEO_BRANCH_NAME", "")
	var out bytes.Buffer
	cmd := NewRootCommand()
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--dry-run", "int", "paseo", "setup"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "<current-branch>") {
		t.Fatalf("missing inferred branch: %s", out.String())
	}
}

func TestIntegrationAliasesDryRun(t *testing.T) {
	for _, args := range [][]string{
		{"--dry-run", "integration", "git-worktree", "replace", "--worktree-path", "/worktree", "--source-path", "/source"},
		{"--dry-run", "int", "git-worktree", "replace", "--worktree-path", "/worktree", "--source-path", "/source"},
	} {
		var out bytes.Buffer
		cmd := NewRootCommand()
		cmd.SetOut(&out)
		cmd.SetArgs(args)
		if err := cmd.Execute(); err != nil {
			t.Fatalf("%v: %v", args, err)
		}
		if !strings.Contains(out.String(), "git clone") {
			t.Fatalf("%v: %s", args, out.String())
		}
	}
}

func TestIntegrationShownAndOldPathsRejected(t *testing.T) {
	var out bytes.Buffer
	cmd := NewRootCommand()
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--help"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "integration") {
		t.Fatalf("missing integration: %s", out.String())
	}
	if strings.Contains(out.String(), "  git-worktree ") || strings.Contains(out.String(), "  paseo ") {
		t.Fatalf("top-level integration commands visible: %s", out.String())
	}
}

func TestOldCommandPathsRejected(t *testing.T) {
	for _, name := range []string{"paseo", "git-worktree"} {
		cmd := NewRootCommand()
		cmd.SetArgs([]string{name, "unknown"})
		if err := cmd.Execute(); err == nil {
			t.Errorf("old command %s should fail", name)
		}
	}
}

func TestDownEmptyWorkdirRejected(t *testing.T) {
	for _, args := range [][]string{{"down", "--workspace", ""}, {"--dry-run", "down", "--workspace", ""}} {
		cmd := NewRootCommand()
		cmd.SetArgs(args)
		if err := cmd.Execute(); err == nil {
			t.Errorf("expected error for %v", args)
		}
	}
}

func TestDryRunRecreateBuildsBeforeRemoval(t *testing.T) {
	dir := testutil.Config(t, "image: example:latest\nbuild:\n  context: ./image\n")
	imageDir := filepath.Join(dir, "image")
	if err := os.Mkdir(imageDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(imageDir, "Dockerfile"), []byte("FROM ubuntu:24.04\n"), 0600); err != nil {
		t.Fatal(err)
	}
	workspace := t.TempDir()
	writeRecord(t, container.Record{Cwd: workspace, Container: "existing", Network: "existing-net", WorkspaceFolder: workspace, MatchWorkspacePath: true})
	var out bytes.Buffer
	cmd := NewRootCommand()
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--dry-run", "up", "--recreate", "--workspace", workspace})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	build, remove, create := strings.Index(text, "docker build"), strings.Index(text, "docker stop existing"), strings.Index(text, "docker run")
	if build < 0 || remove < build || create < remove || !strings.Contains(text, "--workdir "+workspace) {
		t.Fatalf("unexpected recreate preview: %s", text)
	}
	if _, err := container.Lookup(workspace); err != nil {
		t.Fatalf("preview removed record: %v", err)
	}
}

func TestConfigShow(t *testing.T) {
	dir := testutil.Config(t, "privileged: true\nenvironment:\n  HELLO: world\n")
	path := filepath.Join(dir, "config.yaml")
	for _, tc := range []struct {
		name    string
		args    []string
		missing bool
		image   string
	}{
		{name: "defaults", args: []string{"config", "show"}, missing: true},
		{name: "partial JSON and image override", args: []string{"config", "show", "-o", "json"}, image: "custom:latest"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.missing {
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
			} else if err := os.WriteFile(path, []byte("privileged: true\nenvironment:\n  HELLO: world\n"), 0600); err != nil {
				t.Fatal(err)
			}
			t.Setenv("ISODEV_IMAGE", tc.image)
			cmd := NewRootCommand()
			var output bytes.Buffer
			cmd.SetOut(&output)
			cmd.SetArgs(tc.args)
			if err := cmd.Execute(); err != nil {
				t.Fatal(err)
			}
			var got config.Config
			if err := yaml.Unmarshal(output.Bytes(), &got); err != nil {
				t.Fatal(err)
			}
			wantImage := config.DefaultImage
			if tc.image != "" {
				wantImage = tc.image
			}
			if got.Image != wantImage || !reflect.DeepEqual(got.Command, []string{"sleep", "infinity"}) || got.Privileged != !tc.missing {
				t.Fatalf("unexpected configuration: %+v", got)
			}
			if !tc.missing && got.Environment["HELLO"] != "world" {
				t.Fatalf("missing environment: %+v", got)
			}
		})
	}
}

func TestUsageErrors(t *testing.T) {
	for _, args := range [][]string{{"up", "--bogus"}, {"ls", "extra"}, {"-o", "yaml", "ls"}} {
		cmd := NewRootCommand()
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		cmd.SetArgs(args)
		err := cmd.Execute()
		var usage usageError
		if !errors.As(err, &usage) || !strings.HasSuffix(err.Error(), "--help'.") {
			t.Errorf("%v: expected a usage error pointing to --help, got %v", args, err)
		}
	}
}
