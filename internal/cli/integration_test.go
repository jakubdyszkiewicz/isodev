package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jakubdyszkiewicz/isodev/internal/container"
	"github.com/jakubdyszkiewicz/isodev/internal/shell"
	"github.com/jakubdyszkiewicz/isodev/internal/testutil"
)

func dockerExists(args ...string) bool { _, err := shell.Run("docker", args...); return err == nil }

func gitIn(dir string, args ...string) (string, error) {
	return shell.Run("git", append([]string{"-C", dir}, args...)...)
}

func TestZedSetupTeardownDocker(t *testing.T) {
	testutil.RequireDocker(t)
	testutil.Config(t, "image: ubuntu:24.04\ncommand: [sleep, infinity]\ngit_worktree:\n  mode: replace\n")
	root := t.TempDir()
	repo, work := filepath.Join(root, "repo"), filepath.Join(root, "work")
	for _, args := range [][]string{{"init", "-q", repo}, {"-C", repo, "-c", "user.name=Test", "-c", "user.email=t@example.com", "commit", "--allow-empty", "-qm", "initial"}, {"-C", repo, "worktree", "add", "--detach", work}} {
		if _, err := shell.Run("git", args...); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() { _, _ = container.Destroy(work) })
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		go func() {
			cmd := NewRootCommand()
			cmd.SetArgs([]string{"int", "zed", "setup", "--workspace", work, "--source", repo})
			results <- cmd.Execute()
		}()
	}
	for i := 0; i < 2; i++ {
		if err := <-results; err != nil {
			t.Fatal(err)
		}
	}
	r, err := container.Lookup(work)
	if err != nil {
		t.Fatal(err)
	}
	if r.WorkspaceFolder != work {
		t.Fatalf("workspace in container = %s, expected %s", r.WorkspaceFolder, work)
	}
	if _, err := shell.Run("docker", "exec", r.Container, "test", "-d", filepath.Join(work, ".git")); err != nil {
		t.Fatal(err)
	}
	cmd := NewRootCommand()
	cmd.SetArgs([]string{"int", "zed", "teardown", "--workspace", work})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(filepath.Join(work, ".git"))
	if err != nil || !info.Mode().IsRegular() {
		t.Fatalf("worktree pointer not restored: %v", err)
	}
	if dockerExists("container", "inspect", r.Container) {
		t.Fatal("container still exists")
	}
}

func TestCLIOutputIntegration(t *testing.T) {
	testutil.RequireDocker(t)
	testutil.Config(t, "image: ubuntu:24.04\n")
	workspace := filepath.Join(t.TempDir(), "work, space")
	if err := os.Mkdir(workspace, 0700); err != nil {
		t.Fatal(err)
	}
	runCLI := func(args ...string) string {
		t.Helper()
		var out bytes.Buffer
		cmd := NewRootCommand()
		cmd.SetOut(&out)
		cmd.SetErr(io.Discard)
		cmd.SetArgs(args)
		if err := cmd.Execute(); err != nil {
			t.Fatalf("%v: %v", args, err)
		}
		return out.String()
	}
	output := runCLI("up", "--workspace", workspace, "-o", "json")
	var rec container.Record
	if err := json.Unmarshal([]byte(output), &rec); err != nil || rec.Container == "" {
		t.Fatalf("invalid up JSON %q: %v", output, err)
	}
	t.Cleanup(func() { _, _ = container.Destroy(rec.Cwd) })
	output = runCLI("ls", "-o", "json")
	var list []map[string]any
	if err := json.Unmarshal([]byte(output), &list); err != nil {
		t.Fatalf("invalid list JSON: %s: %v", output, err)
	}
	found := false
	for _, item := range list {
		if item["workspace"] == workspace && item["container"] == rec.Container {
			found = true
		}
	}
	if !found {
		t.Fatalf("workspace missing from list: %s", output)
	}
	output = runCLI("stop", "--workspace", workspace, "-o", "json")
	var stopped map[string]any
	if err := json.Unmarshal([]byte(output), &stopped); err != nil || stopped["workspace"] != workspace || stopped["stopped"] != true {
		t.Fatalf("invalid stop output: %s: %v", output, err)
	}
	output = runCLI("ps", "-o", "json")
	if strings.Contains(output, rec.Container) {
		t.Fatalf("stopped container appears in ps: %s", output)
	}
	output = runCLI("ls")
	if !strings.Contains(output, workspace) || !strings.Contains(output, rec.Container) {
		t.Fatalf("workspace missing from text list: %s", output)
	}
	output = runCLI("start", "--workspace", workspace, "-o", "json")
	var restarted container.Record
	if err := json.Unmarshal([]byte(output), &restarted); err != nil || restarted.ContainerID != rec.ContainerID {
		t.Fatalf("start did not reuse container: %s: %v", output, err)
	}
	runCLI("exec", "--workspace", workspace, "sh", "-c", "touch from-exec")
	if _, err := os.Stat(filepath.Join(workspace, "from-exec")); err != nil {
		t.Fatalf("exec did not run in workspace: %v", err)
	}
	output = runCLI("--dry-run", "enter", "--workspace", workspace)
	if !strings.Contains(output, rec.Container) {
		t.Fatalf("enter did not select workspace container: %s", output)
	}
	output = runCLI("rm", "--workspace", workspace, "-o", "json")
	var removed map[string]any
	if err := json.Unmarshal([]byte(output), &removed); err != nil || removed["removed"] != true {
		t.Fatalf("invalid removal JSON: %s: %v", output, err)
	}
	output = runCLI("up", "--workspace", workspace)
	if output != "Container "+rec.Container+"  Created\n" {
		t.Fatalf("invalid text output: %s", output)
	}
	output = runCLI("up", "--workspace", workspace)
	if output != "Container "+rec.Container+"  Running\n" {
		t.Fatalf("invalid text output: %s", output)
	}
	output = runCLI("rm", "--workspace", workspace)
	if output != "Container "+rec.Container+"  Removed\n" {
		t.Fatalf("invalid removal output: %s", output)
	}
}

func TestPaseoSetupWithoutBranch(t *testing.T) {
	testutil.RequireDocker(t)
	testutil.Config(t, "image: ubuntu:24.04\ngit_worktree:\n  mode: replace\n")
	root := t.TempDir()
	repo, work := filepath.Join(root, "repo"), filepath.Join(root, "work")
	for _, args := range [][]string{{"init", "-q", repo}, {"-C", repo, "-c", "user.name=Test", "-c", "user.email=t@example.com", "commit", "--allow-empty", "-qm", "initial"}, {"-C", repo, "worktree", "add", "-qb", "feature/check", work}} {
		if _, err := shell.Run("git", args...); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PASEO_WORKTREE_PATH", work)
	t.Setenv("PASEO_SOURCE_CHECKOUT_PATH", repo)
	t.Setenv("PASEO_BRANCH_NAME", "")
	cmd := NewRootCommand()
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"int", "paseo", "setup"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	r, err := container.Lookup(work)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = container.Destroy(r.Cwd) })
	before := r.ContainerID
	recreated, err := container.Recreate(work, container.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if recreated.ContainerID == before || recreated.WorkspaceFolder != work || !recreated.MatchWorkspacePath {
		t.Fatalf("Paseo workspace path not preserved: %+v", recreated)
	}
	branch, err := gitIn(work, "branch", "--show-current")
	if err != nil || branch != "feature/check" {
		t.Fatalf("branch %q: %v", branch, err)
	}
	cmd = NewRootCommand()
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"integration", "paseo", "teardown"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(filepath.Join(work, ".git"))
	if err != nil || !info.Mode().IsRegular() {
		t.Fatalf("worktree link not restored: %v", err)
	}
}

func TestDownByWorkspace(t *testing.T) {
	testutil.RequireDocker(t)
	testutil.Config(t, "image: ubuntu:24.04\n")
	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "keep"), []byte("workspace data"), 0600); err != nil {
		t.Fatal(err)
	}
	r, err := container.Provision(workspace, container.Options{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = container.Destroy(r.Cwd) })
	var stderr bytes.Buffer
	cli := func(args ...string) string {
		t.Helper()
		var out bytes.Buffer
		stderr.Reset()
		cmd := NewRootCommand()
		cmd.SetOut(&out)
		cmd.SetErr(&stderr)
		cmd.SetArgs(args)
		if err := cmd.Execute(); err != nil {
			t.Fatal(err)
		}
		return out.String()
	}
	preview := cli("--dry-run", "down", "--workspace", workspace)
	if !strings.Contains(preview, "docker rm --volumes "+r.Container) {
		t.Fatalf("missing removal preview: %s", preview)
	}
	if !dockerExists("container", "inspect", r.Container) {
		t.Fatal("dry run removed container")
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(cli("down", "--workspace", workspace, "-o", "json")), &result); err != nil || result["removed"] != true {
		t.Fatalf("down result: %v %v", result, err)
	}
	if dockerExists("container", "inspect", r.Container) || dockerExists("network", "inspect", r.Network) {
		t.Fatal("resources remain")
	}
	if b, err := os.ReadFile(filepath.Join(workspace, "keep")); err != nil || string(b) != "workspace data" {
		t.Fatalf("workspace changed: %q %v", b, err)
	}
	if output := cli("down", "--workspace", workspace); output != "" || !strings.Contains(stderr.String(), "WARNING: No Isodev container for") {
		t.Fatalf("unexpected output: %q, stderr %q", output, stderr.String())
	}
}

func TestRecreateWorkspace(t *testing.T) {
	testutil.RequireDocker(t)
	dir := testutil.Config(t, "")
	script := filepath.Join(dir, "create.sh")
	if err := os.WriteFile(script, []byte("printf '%s\\n' \"$TEST_VERSION\" >> created\n"), 0600); err != nil {
		t.Fatal(err)
	}
	writeConfig := func(value string) {
		t.Helper()
		data := "image: ubuntu:24.04\nenvironment:\n  TEST_VERSION: " + value + "\nlifecycle:\n  on_create:\n    scripts:\n      - file: ./create.sh\n"
		if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}
	writeConfig("one")
	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "keep"), []byte("preserved"), 0600); err != nil {
		t.Fatal(err)
	}
	call := func(args ...string) string {
		t.Helper()
		var out bytes.Buffer
		cmd := NewRootCommand()
		cmd.SetOut(&out)
		cmd.SetErr(io.Discard)
		cmd.SetArgs(args)
		if err := cmd.Execute(); err != nil {
			t.Fatalf("%v: %v", args, err)
		}
		return out.String()
	}
	var first container.Record
	if err := json.Unmarshal([]byte(call("up", "--recreate", "--workspace", workspace, "-o", "json")), &first); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = container.Destroy(first.Cwd) })
	if _, err := shell.Run("docker", "exec", first.Container, "touch", "/root/private-data"); err != nil {
		t.Fatal(err)
	}
	writeConfig("two")
	dry := call("--dry-run", "start", "--recreate", "--workspace", workspace)
	if !strings.Contains(dry, "docker rm --volumes "+first.Container) || !strings.Contains(dry, "docker run") {
		t.Fatalf("unexpected preview: %s", dry)
	}
	if !dockerExists("container", "inspect", first.Container) {
		t.Fatal("dry run removed the container")
	}
	var second container.Record
	if err := json.Unmarshal([]byte(call("start", "--recreate", "--workspace", workspace, "-o", "json")), &second); err != nil {
		t.Fatal(err)
	}
	if second.ContainerID == first.ContainerID {
		t.Fatal("container was not recreated")
	}
	if _, err := shell.Run("docker", "exec", second.Container, "test", "-e", "/root/private-data"); err == nil {
		t.Fatal("private container data survived recreation")
	}
	if data, err := os.ReadFile(filepath.Join(workspace, "created")); err != nil || string(data) != "one\ntwo\n" {
		t.Fatalf("hooks not rerun with new config: %q %v", data, err)
	}
	if data, err := os.ReadFile(filepath.Join(workspace, "keep")); err != nil || string(data) != "preserved" {
		t.Fatalf("workspace changed: %q %v", data, err)
	}
}

// newWorktree creates a repository with a worktree on branch worker and
// returns their real paths.
func newWorktree(t *testing.T) (string, string) {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	repo, work := filepath.Join(root, "repo"), filepath.Join(root, "work")
	for _, args := range [][]string{{"init", "-q", repo}, {"-C", repo, "-c", "user.name=Test", "-c", "user.email=t@example.com", "commit", "--allow-empty", "-qm", "initial"}, {"-C", repo, "worktree", "add", "-qb", "worker", work}} {
		if _, err := shell.Run("git", args...); err != nil {
			t.Fatal(err)
		}
	}
	return repo, work
}

func TestIntegrationsRequireWorktreeMode(t *testing.T) {
	testutil.Config(t, "image: ubuntu:24.04\n")
	repo, work := newWorktree(t)
	t.Setenv("PASEO_WORKTREE_PATH", work)
	t.Setenv("PASEO_SOURCE_CHECKOUT_PATH", repo)
	for _, args := range [][]string{{"int", "paseo", "setup"}, {"int", "zed", "setup", "--workspace", work, "--source", repo}} {
		cmd := NewRootCommand()
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		cmd.SetArgs(args)
		if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "set git_worktree.mode") {
			t.Fatalf("%v: expected a git_worktree.mode error, got %v", args, err)
		}
	}
	if info, err := os.Lstat(filepath.Join(work, ".git")); err != nil || !info.Mode().IsRegular() {
		t.Fatalf("worktree changed: %v", err)
	}
}

func TestGitWorktreeModesDocker(t *testing.T) {
	testutil.RequireDocker(t)
	dir := testutil.Config(t, "")
	if err := os.MkdirAll(filepath.Join(dir, "image"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "image", "Dockerfile"), []byte("FROM alpine:3.20\nRUN apk add --no-cache git\n"), 0600); err != nil {
		t.Fatal(err)
	}
	config := func(mode string) {
		content := "image: isodev-test-git:latest\nbuild:\n  context: image\n"
		if mode != "" {
			content += "git_worktree:\n  mode: " + mode + "\n"
		}
		if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	run := func(args ...string) (string, string, error) {
		var out, errOut bytes.Buffer
		cmd := NewRootCommand()
		cmd.SetOut(&out)
		cmd.SetErr(&errOut)
		cmd.SetArgs(args)
		err := cmd.Execute()
		return out.String(), errOut.String(), err
	}
	commitIn := func(t *testing.T, r container.Record, message string) {
		t.Helper()
		if out, err := shell.Run("docker", "exec", "-w", r.WorkspaceFolder, r.Container, "git", "-c", "safe.directory=*", "-c", "user.name=Agent", "-c", "user.email=a@example.com", "commit", "--allow-empty", "-qm", message); err != nil {
			t.Fatalf("commit in container: %v: %s", err, out)
		}
	}
	up := func(t *testing.T, work string) (container.Record, string) {
		t.Helper()
		out, stderr, err := run("up", "--workspace", work, "-o", "json")
		if err != nil {
			t.Fatalf("up: %v: %s", err, stderr)
		}
		var r container.Record
		if err := json.Unmarshal([]byte(out), &r); err != nil {
			t.Fatalf("up JSON %q: %v", out, err)
		}
		t.Cleanup(func() { _, _ = container.Destroy(work) })
		return r, stderr
	}

	t.Run("unset", func(t *testing.T) {
		config("")
		_, work := newWorktree(t)
		_, stderr := up(t, work)
		if !strings.Contains(stderr, "is a Git worktree") || !strings.Contains(stderr, "git_worktree.mode") {
			t.Fatalf("missing warning: %s", stderr)
		}
		config("none")
		if _, stderr, err := run("up", "--workspace", work); err != nil || strings.Contains(stderr, "warning") {
			t.Fatalf("explicit none: %v: %s", err, stderr)
		}
	})

	t.Run("mount-common-dir", func(t *testing.T) {
		config("mount-common-dir")
		repo, work := newWorktree(t)
		r, _ := up(t, work)
		common := filepath.Join(repo, ".git")
		if r.WorkspaceFolder != work || r.GitCommonDir != common {
			t.Fatalf("record: %+v", r)
		}
		commitIn(t, r, "in container")
		if got, err := gitIn(repo, "log", "-1", "--format=%s", "worker"); err != nil || got != "in container" {
			t.Fatalf("commit not in main repository: %q, %v", got, err)
		}
		if env, err := shell.Run("docker", "exec", r.Container, "git", "config", "--get", "gc.worktreePruneExpire"); err != nil || env != "never" {
			t.Fatalf("gc.worktreePruneExpire = %q, %v", env, err)
		}
		if _, _, err := run("down", "--workspace", work); err != nil {
			t.Fatal(err)
		}
		if info, err := os.Lstat(filepath.Join(work, ".git")); err != nil || !info.Mode().IsRegular() {
			t.Fatalf("worktree changed: %v", err)
		}
	})

	t.Run("switch with recreate", func(t *testing.T) {
		config("mount-common-dir")
		repo, work := newWorktree(t)
		up(t, work)
		recreate := func(t *testing.T) container.Record {
			t.Helper()
			out, stderr, err := run("up", "--recreate", "--workspace", work, "-o", "json")
			if err != nil {
				t.Fatalf("up --recreate: %v: %s", err, stderr)
			}
			var r container.Record
			if err := json.Unmarshal([]byte(out), &r); err != nil {
				t.Fatalf("up JSON %q: %v", out, err)
			}
			return r
		}
		config("replace")
		// Without --recreate, the existing container and worktree stay as they are.
		if _, stderr, err := run("up", "--workspace", work); err != nil {
			t.Fatalf("up: %v: %s", err, stderr)
		}
		if info, err := os.Lstat(filepath.Join(work, ".git")); err != nil || !info.Mode().IsRegular() {
			t.Fatalf("worktree replaced without --recreate: %v", err)
		}
		r := recreate(t)
		if info, err := os.Lstat(filepath.Join(work, ".git")); err != nil || !info.IsDir() || r.GitCommonDir != "" {
			t.Fatalf("not replaced by --recreate: %v, %+v", err, r)
		}
		commitIn(t, r, "while replaced")
		config("mount-common-dir")
		r = recreate(t)
		if info, err := os.Lstat(filepath.Join(work, ".git")); err != nil || !info.Mode().IsRegular() || r.GitCommonDir != filepath.Join(repo, ".git") {
			t.Fatalf("not restored by --recreate: %v, %+v", err, r)
		}
		if got, err := gitIn(repo, "log", "-1", "--format=%s", "worker"); err != nil || got != "while replaced" {
			t.Fatalf("worker not fast-forwarded: %q, %v", got, err)
		}
		commitIn(t, r, "mounted again")
		if got, err := gitIn(repo, "log", "-1", "--format=%s", "worker"); err != nil || got != "mounted again" {
			t.Fatalf("commit not in main repository: %q, %v", got, err)
		}
	})

	t.Run("replace", func(t *testing.T) {
		config("replace")
		repo, work := newWorktree(t)
		r, _ := up(t, work)
		if info, err := os.Lstat(filepath.Join(work, ".git")); err != nil || !info.IsDir() {
			t.Fatalf("worktree not replaced: %v", err)
		}
		commitIn(t, r, "agent")
		// Changing the mode doesn't affect a replaced worktree.
		config("none")
		out, _, err := run("down", "--workspace", work, "-o", "json")
		if err != nil {
			t.Fatal(err)
		}
		var res struct {
			Removed  bool
			Worktree map[string]any
		}
		if err := json.Unmarshal([]byte(out), &res); err != nil || !res.Removed || res.Worktree["fastForwarded"] != true {
			t.Fatalf("down output %s: %v", out, err)
		}
		if got, err := gitIn(repo, "log", "-1", "--format=%s", "worker"); err != nil || got != "agent" {
			t.Fatalf("worker not fast-forwarded: %q, %v", got, err)
		}
		if info, err := os.Lstat(filepath.Join(work, ".git")); err != nil || !info.Mode().IsRegular() {
			t.Fatalf("worktree not restored: %v", err)
		}
	})
}
