package container

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jakubdyszkiewicz/isodev/internal/config"
	"github.com/jakubdyszkiewicz/isodev/internal/shell"
	"github.com/jakubdyszkiewicz/isodev/internal/testutil"
)

func TestDockerLifecycleIntegration(t *testing.T) {
	testutil.RequireDocker(t)
	dir := testutil.Config(t, `image: ubuntu:24.04
command: [sleep, infinity]
environment:
  TEST_VALUE: works
volumes:
  - type: bind
    source: ./secret
    target: /run/isodev-secret
    read_only: true
lifecycle:
  on_start:
    scripts:
      - file: ./start.sh
  on_create:
    scripts:
      - file: ./create.sh
`)
	for name, data := range map[string]string{
		"secret":    "private",
		"start.sh":  "set -eu\n[ \"$TEST_VALUE\" = works ]\n[ \"$(cat /run/isodev-secret)\" = private ]\necho start:$ISODEV_PROBE >> events\n",
		"create.sh": "set -eu\necho create:$ISODEV_PROBE >> events\n",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}
	workspace := t.TempDir()
	r, err := Provision(workspace, Options{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = destroyRecord(r) })
	if r.WorkspaceFolder != filepath.Join("/workspaces", filepath.Base(workspace)) {
		t.Fatalf("unexpected record: %+v", r)
	}
	hostname, err := shell.Run("docker", "exec", r.Container, "hostname")
	if err != nil || hostname != r.Container {
		t.Fatalf("hostname %q does not match container %q: %v", hostname, r.Container, err)
	}
	readEvents := func() string {
		t.Helper()
		b, err := os.ReadFile(filepath.Join(workspace, "events"))
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	if got := readEvents(); got != "start:0\ncreate:0\n" {
		t.Fatalf("initial hooks: %q", got)
	}
	if _, err := Provision(workspace, Options{}); err != nil {
		t.Fatal(err)
	}
	if got := readEvents(); got != "start:0\ncreate:0\n" {
		t.Fatalf("running hooks repeated: %q", got)
	}
	if _, err := shell.Run("docker", "stop", r.Container); err != nil {
		t.Fatal(err)
	}
	if _, err := Provision(workspace, Options{}); err != nil {
		t.Fatal(err)
	}
	if got := readEvents(); got != "start:0\ncreate:0\nstart:0\n" {
		t.Fatalf("restart hooks: %q", got)
	}
	if _, err := shell.Run("docker", "stop", r.Container); err != nil {
		t.Fatal(err)
	}
	if _, err := Provision(workspace, Options{}); err != nil {
		t.Fatal(err)
	}
	if got := readEvents(); got != "start:0\ncreate:0\nstart:0\nstart:0\n" {
		t.Fatalf("provision restart hooks: %q", got)
	}
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	probeHook := r
	probeHook.Probe = true
	if err := runHooks(probeHook, "on_create", cfg.Lifecycle.OnCreate.Scripts); err != nil {
		t.Fatal(err)
	}
	if got := readEvents(); !strings.HasSuffix(got, "create:1\n") {
		t.Fatalf("probe environment: %q", got)
	}
	if _, err := shell.Run("docker", "stop", r.Container); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "start.sh"), []byte("exit 3\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Provision(workspace, Options{}); err == nil {
		t.Fatal("expected restart hook failure")
	}
	inspection, err := inspect(r.Container)
	if err != nil || inspection.State.Running {
		t.Fatalf("failed restart left container running: %+v %v", inspection.State, err)
	}
	if _, err := Destroy(workspace); err != nil {
		t.Fatal(err)
	}
	if exists("container", "inspect", r.Container) || exists("network", "inspect", r.Network) {
		t.Fatal("resources were not removed")
	}

	if err := os.WriteFile(filepath.Join(dir, "start.sh"), []byte("echo start:$ISODEV_PROBE >> events\n"), 0600); err != nil {
		t.Fatal(err)
	}
	{
		probeDir := t.TempDir()
		probe, err := Provision(probeDir, Options{Probe: true})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = destroyRecord(probe) })
		b, err := os.ReadFile(filepath.Join(probeDir, "events"))
		if err != nil || string(b) != "start:1\ncreate:1\n" {
			t.Fatalf("probe hooks %q: %v", b, err)
		}
		recreated, err := Recreate(probeDir, Options{})
		if err != nil {
			t.Fatal(err)
		}
		if !recreated.Probe || recreated.ContainerID == probe.ContainerID || recreated.WorkspaceFolder != "/workspaces/probe" {
			t.Fatalf("probe recreation: %+v", recreated)
		}
		probe = recreated
		if _, err := Destroy(probeDir); err != nil {
			t.Fatal(err)
		}
	}

	if err := os.WriteFile(filepath.Join(dir, "start.sh"), []byte("exit 3\n"), 0600); err != nil {
		t.Fatal(err)
	}
	failureDir := t.TempDir()
	if _, err := Provision(failureDir, Options{}); err == nil || !strings.Contains(err.Error(), "on_start") {
		t.Fatalf("expected hook failure, got %v", err)
	}
	if exists("container", "inspect", Name(failureDir)) || exists("network", "inspect", Name(failureDir)+"-net") {
		t.Fatal("failed provisioning left resources")
	}
	if _, err := os.Stat(recordPath(failureDir)); !os.IsNotExist(err) {
		t.Fatalf("failed provisioning left record: %v", err)
	}
}

func TestDockerBuildIntegration(t *testing.T) {
	testutil.RequireDocker(t)
	dir := testutil.Config(t, "")
	image := "isodev-build-test-" + strings.TrimPrefix(Name(dir), "isodev-") + ":latest"
	t.Cleanup(func() { _, _ = shell.Run("docker", "image", "rm", image) })
	imageDir := filepath.Join(dir, "image")
	if err := os.Mkdir(imageDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte("image: "+image+"\nbuild:\n  context: ./image\n"), 0600); err != nil {
		t.Fatal(err)
	}
	dockerfile := filepath.Join(imageDir, "Dockerfile")
	writeBuild := func(value string) {
		t.Helper()
		if err := os.WriteFile(dockerfile, []byte("FROM ubuntu:24.04\nRUN echo "+value+" > /image-marker\n"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	writeBuild("one")
	workA := t.TempDir()
	a, err := Provision(workA, Options{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = destroyRecord(a) })
	checkMarker := func(r Record, expected string) {
		t.Helper()
		got, err := shell.Run("docker", "exec", r.Container, "cat", "/image-marker")
		if err != nil || got != expected {
			t.Fatalf("marker %q: %q %v", expected, got, err)
		}
	}
	checkMarker(a, "one")
	if err := os.Remove(dockerfile); err != nil {
		t.Fatal(err)
	}
	if _, err := shell.Run("docker", "stop", a.Container); err != nil {
		t.Fatal(err)
	}
	if _, err := Provision(workA, Options{}); err != nil {
		t.Fatal(err)
	}
	checkMarker(a, "one")
	writeBuild("two")
	workB := t.TempDir()
	b, err := Provision(workB, Options{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = destroyRecord(b) })
	checkMarker(b, "two")
	if err := os.WriteFile(dockerfile, []byte("INVALID\n"), 0600); err != nil {
		t.Fatal(err)
	}
	workC := t.TempDir()
	if _, err := Provision(workC, Options{}); err == nil {
		t.Fatal("expected build failure")
	}
	if exists("network", "inspect", Name(workC)+"-net") || exists("container", "inspect", Name(workC)) {
		t.Fatal("build failure left resources")
	}
	if _, err := Recreate(workA, Options{}); err == nil {
		t.Fatal("expected recreation build failure")
	}
	checkMarker(a, "one")
	if !exists("network", "inspect", a.Network) {
		t.Fatal("failed build removed the existing network")
	}
}

func TestScriptListsIntegration(t *testing.T) {
	testutil.RequireDocker(t)
	dir := testutil.Config(t, `image: ubuntu:24.04
lifecycle:
  on_start:
    scripts:
      - file: a.sh
      - file: b.sh
  on_create:
    scripts:
      - file: c.sh
      - file: fail.sh
      - file: d.sh
`)
	for name, script := range map[string]string{
		"a.sh":    "echo start-a >> events\n",
		"b.sh":    "echo start-b >> events\n",
		"c.sh":    "echo create-c >> events\n",
		"fail.sh": "echo create-fail >> events; exit 7\n",
		"d.sh":    "echo create-d >> events\n",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(script), 0600); err != nil {
			t.Fatal(err)
		}
	}
	workspace := t.TempDir()
	_, err := Provision(workspace, Options{})
	if err == nil || !strings.Contains(err.Error(), "lifecycle.on_create.scripts[1]") || !strings.Contains(err.Error(), "fail.sh") {
		t.Fatalf("expected lifecycle.on_create.scripts[1] to fail, got %v", err)
	}
	events, err := os.ReadFile(filepath.Join(workspace, "events"))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(events), "start-a\nstart-b\ncreate-c\ncreate-fail\n"; got != want {
		t.Fatalf("hooks ran as %q, want %q", got, want)
	}
	if exists("container", "inspect", Name(workspace)) || exists("network", "inspect", Name(workspace)+"-net") {
		t.Fatal("failed hook left the container or network")
	}
}

func TestVolumesIntegration(t *testing.T) {
	testutil.RequireDocker(t)
	shared := "isodev-test-shared-" + filepath.Base(t.TempDir())
	testutil.Config(t, `image: ubuntu:24.04
volumes:
  - type: volume
    target: /data
  - type: volume
    source: `+shared+`
    target: /shared
`)
	t.Cleanup(func() { _, _ = shell.Run("docker", "volume", "rm", "-f", shared) })
	workspace := t.TempDir()
	r, err := Provision(workspace, Options{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = Destroy(workspace) })
	private := r.Container + "-data"
	sh := func(script string) (string, error) {
		return shell.Run("docker", "exec", r.Container, "sh", "-c", script)
	}
	if _, err := sh("echo private > /data/f && echo shared > /shared/f"); err != nil {
		t.Fatal(err)
	}
	if labels, err := shell.Run("docker", "volume", "inspect", "--format", `{{index .Labels "isodev.workspace"}}`, private); err != nil || labels != r.Cwd {
		t.Fatalf("private volume label = %q, %v", labels, err)
	}
	if r, err = Recreate(workspace, Options{}); err != nil {
		t.Fatal(err)
	}
	if got, err := sh("cat /data/f"); err != nil || got != "private" {
		t.Fatalf("private volume not kept by recreate: %q, %v", got, err)
	}
	if r, err = Recreate(workspace, Options{RenewVolumes: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := sh("test ! -e /data/f"); err != nil {
		t.Fatal("private volume not emptied by --renew-volumes")
	}
	if got, err := sh("cat /shared/f"); err != nil || got != "shared" {
		t.Fatalf("shared volume changed: %q, %v", got, err)
	}
	if _, err := Destroy(workspace); err != nil {
		t.Fatal(err)
	}
	if _, err := shell.Run("docker", "volume", "inspect", private); err == nil {
		t.Fatal("private volume not removed by down")
	}
	if _, err := shell.Run("docker", "volume", "inspect", shared); err != nil {
		t.Fatalf("shared volume removed by down: %v", err)
	}
}

func TestFailedRunLeavesNoContainer(t *testing.T) {
	testutil.RequireDocker(t)
	// Docker creates the container, then fails to start it: a file can't be
	// mounted over the image's /etc directory.
	dir := testutil.Config(t, "image: ubuntu:24.04\nvolumes:\n  - type: bind\n    source: file\n    target: /etc\n")
	if err := os.WriteFile(filepath.Join(dir, "file"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	workspace := t.TempDir()
	t.Cleanup(func() { _, _ = shell.Run("docker", "rm", "-f", Name(workspace)) })
	if _, err := Provision(workspace, Options{}); err == nil {
		t.Fatal("expected docker run to fail")
	}
	real, _ := filepath.EvalSymlinks(workspace)
	if exists("container", "inspect", Name(real)) {
		t.Fatal("failed docker run left the container behind")
	}
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte("image: ubuntu:24.04\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Provision(workspace, Options{}); err != nil {
		t.Fatalf("up after fixing the configuration: %v", err)
	}
	if _, err := Destroy(workspace); err != nil {
		t.Fatal(err)
	}
}

func TestScriptsCantReadThemselves(t *testing.T) {
	testutil.RequireDocker(t)
	dir := testutil.Config(t, "image: ubuntu:24.04\nlifecycle:\n  on_create:\n    scripts:\n      - file: stdin.sh\n")
	// cat reads stdin; it must not consume the rest of the script.
	if err := os.WriteFile(filepath.Join(dir, "stdin.sh"), []byte("cat >/dev/null\necho ran > events\n"), 0600); err != nil {
		t.Fatal(err)
	}
	workspace := t.TempDir()
	if _, err := Provision(workspace, Options{}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = Destroy(workspace) })
	if b, err := os.ReadFile(filepath.Join(workspace, "events")); err != nil || string(b) != "ran\n" {
		t.Fatalf("script stopped early: %q, %v", b, err)
	}
}

func TestDestroyDeletedWorkspace(t *testing.T) {
	testutil.RequireDocker(t)
	testutil.Config(t, "image: ubuntu:24.04\nvolumes:\n  - type: volume\n    target: /data\n")
	workspace := filepath.Join(t.TempDir(), "gone")
	if err := os.Mkdir(workspace, 0700); err != nil {
		t.Fatal(err)
	}
	r, err := Provision(workspace, Options{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = shell.Run("docker", "rm", "-f", r.Container) })
	if err := os.Remove(workspace); err != nil {
		t.Fatal(err)
	}
	if removed, err := Destroy(workspace); err != nil || !removed {
		t.Fatalf("down after deleting the workspace: %v, %v", removed, err)
	}
	if exists("container", "inspect", r.Container) || exists("volume", "inspect", r.Container+"-data") {
		t.Fatal("resources remain")
	}
}
