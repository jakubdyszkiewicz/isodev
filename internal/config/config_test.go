package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/jakubdyszkiewicz/isodev/internal/paths"
	"github.com/jakubdyszkiewicz/isodev/internal/testutil"
)

func TestLoad(t *testing.T) {
	dir := testutil.Config(t, `image: test:latest
privileged: true
command: [sleep, infinity]
environment:
  HELLO: world
volumes:
  - type: bind
    source: ~/credentials
    target: /run/secrets/credentials/
    read_only: true
lifecycle:
  on_create:
    scripts:
      - file: ./create.sh
      - file: ~/second.sh
  on_exec:
    forward_env_vars: [TOKEN]
git_worktree:
  mode: replace
`)
	if err := os.WriteFile(filepath.Join(paths.Home, "credentials"), []byte("private"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, script := range []string{filepath.Join(dir, "create.sh"), filepath.Join(paths.Home, "second.sh")} {
		if err := os.WriteFile(script, []byte("true\n"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	want := []Mount{{Type: "bind", Source: filepath.Join(paths.Home, "credentials"), Target: "/run/secrets/credentials", ReadOnly: true}}
	if cfg.GitWorktree.Mode != GitWorktreeReplace {
		t.Fatalf("git_worktree.mode = %q", cfg.GitWorktree.Mode)
	}
	if cfg.Image != "test:latest" || !cfg.Privileged || cfg.Environment["HELLO"] != "world" || !reflect.DeepEqual(cfg.Volumes, want) {
		t.Fatalf("unexpected configuration: %+v", cfg)
	}
	wantScripts := []Script{{File: filepath.Join(dir, "create.sh")}, {File: filepath.Join(paths.Home, "second.sh")}}
	if !reflect.DeepEqual(cfg.Lifecycle.OnCreate.Scripts, wantScripts) || cfg.Lifecycle.OnStart.Scripts != nil || !reflect.DeepEqual(cfg.Lifecycle.OnExec.ForwardEnvVars, []string{"TOKEN"}) {
		t.Fatalf("unexpected hooks: %+v", cfg.Lifecycle)
	}
}

func TestLoadDefaults(t *testing.T) {
	for _, content := range []string{"", "# comments only\n"} {
		testutil.Config(t, content)
		cfg, err := Load()
		if err != nil {
			t.Fatalf("%q: %v", content, err)
		}
		if cfg.Image != DefaultImage || !reflect.DeepEqual(cfg.Command, []string{"sleep", "infinity"}) {
			t.Fatalf("%q: unexpected defaults: %+v", content, cfg)
		}
	}
	testutil.Home(t)
	if cfg, err := Load(); err != nil || cfg.Image != DefaultImage {
		t.Fatalf("missing file: %+v, %v", cfg, err)
	}
}

func TestLoadRejectsInvalidInput(t *testing.T) {
	dir := testutil.Config(t, "")
	path := filepath.Join(dir, "config.yaml")
	for _, content := range []string{
		"unknown_field: true\n",
		"volumes:\n  - type: bind\n    source: ~/missing\n    target: /secret\n",
		"volumes:\n  - type: bind\n    source: ~/\n    target: relative\n",
		"lifecycle:\n  on_create:\n    scripts:\n      - file: ./missing\n",
		"lifecycle:\n  on_start:\n    scripts:\n      - {}\n",
		"lifecycle:\n  on_start:\n    scripts:\n      - file: ./x\n        run: true\n",
		"command: []\n",
		"environment:\n  A=B: c\n",
		"lifecycle:\n  on_exec:\n    forward_env_vars: ['']\n",
		"image: a\n---\nimage: b\n",
		"git_worktree:\n  mode: link\n",
		"volumes:\n  - type: volume\n    source: ../escape\n    target: /data\n",
		"volumes:\n  - type: volume\n    target: /data\n  - type: volume\n    target: /data/\n",
		"volumes:\n  - type: tmpfs\n    target: /data\n",
	} {
		if err := os.WriteFile(path, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := Load(); err == nil {
			t.Errorf("accepted invalid config: %s", content)
		}
	}
}

func TestBuildPathsAndValidation(t *testing.T) {
	dir := testutil.Config(t, "image: example:latest\nbuild:\n  context: ./image\n  dockerfile: Containerfile\n")
	imageDir := filepath.Join(dir, "image")
	if err := os.Mkdir(imageDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(imageDir, "Containerfile"), []byte("FROM ubuntu:24.04\n"), 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Build.Context != imageDir || cfg.Build.Dockerfile != filepath.Join(imageDir, "Containerfile") {
		t.Fatalf("unexpected paths: %+v", cfg.Build)
	}
	if err := cfg.ValidateBuild(); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ISODEV_IMAGE", "override:latest")
	if cfg, err = Load(); err != nil || cfg.Image != "override:latest" {
		t.Fatalf("image override not used: %q, %v", cfg.Image, err)
	}
	t.Setenv("ISODEV_IMAGE", "")
	for _, content := range []string{"build:\n  context: ./image\n", "image: test:latest\nbuild:\n  context: ./missing\n", "image: test:latest\nbuild:\n  context: ./image\n  dockerfile: missing\n", "image: test:latest\nbuild: {}\n"} {
		if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
		cfg, err := Load()
		if err == nil {
			err = cfg.ValidateBuild()
		}
		if err == nil {
			t.Errorf("accepted invalid build: %s", content)
		}
	}
}

func TestLoadRejectsLegacyLifecycle(t *testing.T) {
	for content, want := range map[string]string{
		"lifecycle:\n  on_start:\n    file: start.sh\n":    "lifecycle.on_start lists its scripts under scripts",
		"lifecycle:\n  on_create:\n    - file: start.sh\n": "lifecycle.on_create lists its scripts under scripts",
		"on_exec:\n  forward_env_vars: [TOKEN]\n":          "on_exec belongs under lifecycle",
	} {
		dir := testutil.Config(t, content)
		if err := os.WriteFile(filepath.Join(dir, "start.sh"), []byte("true\n"), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := Load(); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%q: expected %q, got %v", content, want, err)
		}
	}
}

func TestLoadVolumes(t *testing.T) {
	testutil.Config(t, "volumes:\n  - type: volume\n    target: /var/lib/docker/\n  - type: volume\n    source: go-build\n    target: /root/.cache/go-build\n")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	want := []Mount{{Type: MountVolume, Target: "/var/lib/docker"}, {Type: MountVolume, Source: "go-build", Target: "/root/.cache/go-build"}}
	if !reflect.DeepEqual(cfg.Volumes, want) {
		t.Fatalf("volumes = %+v", cfg.Volumes)
	}
}
