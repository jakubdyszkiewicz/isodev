package container

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/jakubdyszkiewicz/isodev/internal/config"
	"github.com/jakubdyszkiewicz/isodev/internal/fsutil"
	"github.com/jakubdyszkiewicz/isodev/internal/paths"
	"github.com/jakubdyszkiewicz/isodev/internal/shell"
	"github.com/jakubdyszkiewicz/isodev/internal/testutil"
)

func TestRunArgs(t *testing.T) {
	testutil.Config(t, `image: test:latest
privileged: true
environment:
  HELLO: world
volumes:
  - type: bind
    source: ~/credentials
    target: /run/secrets/credentials
    read_only: true
`)
	if err := os.WriteFile(filepath.Join(paths.Home, "credentials"), []byte("private"), 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	r := Record{Cwd: "/tmp/ws", WorkspaceFolder: "/workspaces/ws", Container: "worker", Network: "net"}
	args, err := runArgs(r, cfg)
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(args, " ")
	for _, expected := range []string{"--privileged", "--env HELLO=world", "--label isodev.managed=true", "--label isodev.workspace=/tmp/ws", "source=" + filepath.Join(paths.Home, "credentials"), "target=/run/secrets/credentials,readonly", "test:latest sleep infinity"} {
		if !strings.Contains(joined, expected) {
			t.Errorf("missing %q in %q", expected, joined)
		}
	}
	cfg.Volumes = append(cfg.Volumes, config.Mount{Type: config.MountBind, Source: "/x", Target: "/workspaces"})
	if _, err := runArgs(r, cfg); err == nil {
		t.Fatal("expected mount overlapping the workspace to fail")
	}
}

func TestVolumeMounts(t *testing.T) {
	testutil.Home(t)
	r := Record{Cwd: "/tmp/example", WorkspaceFolder: "/workspaces/example", Container: "isodev-abc", Network: "isodev-abc-net"}
	cfg := config.Config{Image: "test", Volumes: []config.Mount{
		{Type: config.MountVolume, Target: "/var/lib/docker"},
		{Type: config.MountVolume, Source: "shared-cache", Target: "/root/.cache", ReadOnly: true},
		{Type: config.MountBind, Source: "/host", Target: "/host"},
	}}
	args, err := runArgs(r, cfg)
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(args, " ")
	for _, want := range []string{
		"type=volume,source=isodev-abc-var-lib-docker,target=/var/lib/docker",
		"type=volume,source=shared-cache,target=/root/.cache,readonly",
		"type=bind,source=/host,target=/host",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("missing %q in %q", want, joined)
		}
	}
	if got := privateVolumes(r, cfg); len(got) != 1 || got[0] != "isodev-abc-var-lib-docker" {
		t.Fatalf("private volumes = %v", got)
	}
	cfg.Volumes = append(cfg.Volumes, config.Mount{Type: config.MountVolume, Target: "/var/lib-docker"})
	if _, err := runArgs(r, cfg); err == nil || !strings.Contains(err.Error(), "two private volumes") {
		t.Fatalf("expected a name collision, got %v", err)
	}
}

func TestBuildRespectsConfiguredBuilder(t *testing.T) {
	dir := t.TempDir()
	dockerfile := filepath.Join(dir, "Dockerfile")
	if err := os.WriteFile(dockerfile, []byte("FROM scratch\n"), 0600); err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(dir, "docker")
	if err := os.WriteFile(script, []byte("#!/bin/sh\n[ \"$BUILDX_BUILDER\" = desktop-linux ]\n"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("BUILDX_BUILDER", "desktop-linux")
	if err := buildImage(config.Config{Image: "test:latest", Build: &config.Build{Context: dir, Dockerfile: dockerfile}}); err != nil {
		t.Fatal(err)
	}
	if _, err := shell.Run("docker", "version"); err != nil {
		t.Fatal(err)
	}
}

func TestForwardExecEnvironment(t *testing.T) {
	testutil.Config(t, "lifecycle:\n  on_exec:\n    forward_env_vars: [EXAMPLE_TOKEN, TEST_EMPTY, NOT_SET, PASEO_AGENT_ID]\n")
	t.Setenv("EXAMPLE_TOKEN", "secret-value")
	t.Setenv("TEST_EMPTY", "")
	t.Setenv("PASEO_AGENT_ID", "agent")
	t.Setenv("PASEO_AGENT_CWD", "/workspace")
	t.Setenv("NOT_SET", "")
	os.Unsetenv("NOT_SET")
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		paseo bool
		want  []string
	}{
		{false, []string{"--env", "EXAMPLE_TOKEN", "--env", "TEST_EMPTY", "--env", "PASEO_AGENT_ID"}},
		{true, []string{"--env", "EXAMPLE_TOKEN", "--env", "TEST_EMPTY", "--env", "PASEO_AGENT_ID", "--env", "PASEO_AGENT_CWD"}},
	} {
		if got := forwardedEnvArgs(cfg.Lifecycle.OnExec.ForwardEnvVars, tc.paseo); !reflect.DeepEqual(got, tc.want) {
			t.Fatalf("paseo=%v got %v want %v", tc.paseo, got, tc.want)
		}
	}
}

func TestProvisionAcceptsOrdinaryWorkspace(t *testing.T) {
	testutil.Config(t, "image: ubuntu:24.04\n")
	workspace := t.TempDir()
	bin := t.TempDir()
	docker := filepath.Join(bin, "docker")
	if err := os.WriteFile(docker, []byte("#!/bin/sh\nif [ \"$1\" = network ] && [ \"$2\" = create ]; then echo reached-docker >&2; exit 1; fi\nexit 1\n"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	_, err := Provision(workspace, Options{})
	if err == nil || !strings.Contains(err.Error(), "reached-docker") {
		t.Fatalf("expected provisioning to reach Docker for ordinary directory, got %v", err)
	}
}

func writeRecord(t *testing.T, r Record) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(recordPath(r.Cwd)), 0700); err != nil {
		t.Fatal(err)
	}
	if err := fsutil.WriteJSON(recordPath(r.Cwd), r); err != nil {
		t.Fatal(err)
	}
}

func TestFindForPathFindsNearestAncestor(t *testing.T) {
	testutil.Home(t)
	root, err := fsutil.Real(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	nested := filepath.Join(root, "nested")
	child := filepath.Join(nested, "child")
	if err := os.MkdirAll(child, 0700); err != nil {
		t.Fatal(err)
	}
	for _, r := range []Record{{Cwd: root, Container: "outer"}, {Cwd: nested, Container: "inner"}} {
		writeRecord(t, r)
	}
	for _, tc := range []struct{ path, want string }{{root, "outer"}, {child, "inner"}} {
		r, err := FindForPath(tc.path)
		if err != nil || r.Container != tc.want {
			t.Fatalf("lookup %s: got %q, %v", tc.path, r.Container, err)
		}
	}
	if _, err := FindForPath(t.TempDir()); err == nil {
		t.Fatal("expected missing container error")
	}
	if _, err := Lookup(child); !os.IsNotExist(err) {
		t.Fatalf("Lookup of a nested path: %v", err)
	}
}
