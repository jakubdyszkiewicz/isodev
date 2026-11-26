package gitworktree

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jakubdyszkiewicz/isodev/internal/paths"
	"github.com/jakubdyszkiewicz/isodev/internal/shell"
)

// TestMain keeps replacement records out of the real state directory.
func TestMain(m *testing.M) {
	home, err := os.MkdirTemp("", "isodev-gitworktree-test-")
	if err != nil {
		panic(err)
	}
	paths.Home = home
	code := m.Run()
	_ = os.RemoveAll(home)
	os.Exit(code)
}

func TestUnlinkAndRestore(t *testing.T) {
	root := t.TempDir()
	repo := filepath.Join(root, "repo")
	work := filepath.Join(root, "work")
	call := func(cmd string, a ...string) {
		t.Helper()
		if _, e := shell.Run(cmd, a...); e != nil {
			t.Fatal(e)
		}
	}
	call("git", "init", "-q", repo)
	call("git", "-C", repo, "-c", "user.name=Test", "-c", "user.email=t@example.com", "commit", "--allow-empty", "-qm", "initial")
	call("git", "-C", repo, "worktree", "add", "-qb", "worker", work)
	result, e := Replace(work, repo, false)
	if e != nil {
		t.Fatal(e)
	}
	if result["branch"] != "worker" {
		t.Fatalf("expected inferred branch, got %v", result["branch"])
	}
	real, _ := filepath.EvalSymlinks(work)
	if _, err := os.Stat(markerPath(real)); err != nil {
		t.Fatalf("marker missing from the state directory: %v", err)
	}
	if _, err := os.Stat(filepath.Join(work, ".git", "isodev-git-worktree-unlink.json")); err == nil {
		t.Fatal("marker written into the worktree")
	}
	if _, e := Replace(work, repo, false); e != nil {
		t.Fatal(e)
	}
	if _, e := git(work, "-c", "user.name=Test", "-c", "user.email=t@example.com", "commit", "--allow-empty", "-qm", "private"); e != nil {
		t.Fatal(e)
	}
	a, _ := git(repo, "rev-parse", "HEAD")
	b, _ := git(work, "rev-parse", "HEAD")
	if a == b {
		t.Fatal("private commit leaked")
	}
	restored, e := Restore(work)
	if e != nil {
		t.Fatal(e)
	}
	saved, _ := restored["savedGit"].(string)
	if filepath.Dir(saved) != filepath.Join(real, ArchiveDir) {
		t.Fatalf("unexpected archive path: %v", saved)
	}
	if _, err := os.Stat(filepath.Join(saved, "HEAD")); err != nil {
		t.Fatalf("archive missing the repository: %v", err)
	}
	if _, err := os.Stat(markerPath(real)); !os.IsNotExist(err) {
		t.Fatalf("marker not removed: %v", err)
	}
	call("git", "-C", repo, "worktree", "remove", "--force", work)
}

func TestRepeatedReplaceAndRestore(t *testing.T) {
	repo, work, call := newWorktree(t)
	var archives []string
	for i := 0; i < 2; i++ {
		if _, err := Replace(work, repo, false); err != nil {
			t.Fatalf("replace %d: %v", i, err)
		}
		call("-C", work, "commit", "-q", "--allow-empty", "-m", "agent")
		restored, err := Restore(work)
		if err != nil {
			t.Fatalf("restore %d: %v", i, err)
		}
		if restored["fastForwarded"] != true {
			t.Fatalf("restore %d: %v", i, restored)
		}
		archives = append(archives, restored["savedGit"].(string))
	}
	if archives[0] == archives[1] {
		t.Fatalf("archives reused: %v", archives)
	}
	if got := call("-C", repo, "rev-list", "--count", "worker"); got != "3" {
		t.Fatalf("worker has %s commits, expected 3", got)
	}
}

func TestDetachedWorktreeRejected(t *testing.T) {
	root := t.TempDir()
	repo, work := filepath.Join(root, "repo"), filepath.Join(root, "work")
	for _, args := range [][]string{{"init", "-q", repo}, {"-C", repo, "-c", "user.name=Test", "-c", "user.email=t@example.com", "commit", "--allow-empty", "-qm", "initial"}, {"-C", repo, "worktree", "add", "--detach", work}} {
		if _, err := shell.Run("git", args...); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := Replace(work, repo, false); err == nil || !strings.Contains(err.Error(), "detached HEAD") {
		t.Fatalf("expected detached HEAD error, got %v", err)
	}
	info, err := os.Stat(filepath.Join(work, ".git"))
	if err != nil || !info.Mode().IsRegular() {
		t.Fatalf("worktree pointer changed: %v", err)
	}
}

func TestDetachedReplacePreservesUnreferencedHead(t *testing.T) {
	root := t.TempDir()
	repo, work := filepath.Join(root, "repo"), filepath.Join(root, "work")
	for _, args := range [][]string{{"init", "-q", repo}, {"-C", repo, "-c", "user.name=Test", "-c", "user.email=t@example.com", "commit", "--allow-empty", "-qm", "initial"}, {"-C", repo, "worktree", "add", "--detach", work}, {"-C", work, "-c", "user.name=Test", "-c", "user.email=t@example.com", "commit", "--allow-empty", "-qm", "detached"}} {
		if _, err := shell.Run("git", args...); err != nil {
			t.Fatal(err)
		}
	}
	head, err := git(work, "rev-parse", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = Replace(work, repo, true); err != nil {
		t.Fatal(err)
	}
	got, err := git(work, "rev-parse", "HEAD")
	if err != nil || got != head {
		t.Fatalf("HEAD = %s, %v; expected %s", got, err, head)
	}
	if _, err := git(work, "symbolic-ref", "--quiet", "HEAD"); err == nil {
		t.Fatal("expected detached HEAD")
	}
	if _, err := git(work, "-c", "user.name=Test", "-c", "user.email=t@example.com", "commit", "--allow-empty", "-qm", "private"); err != nil {
		t.Fatal(err)
	}
	private, err := git(work, "rev-parse", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	restored, err := Restore(work)
	if err != nil {
		t.Fatal(err)
	}
	got, err = git(work, "rev-parse", "HEAD")
	if err != nil || got != private || restored["fastForwarded"] != true {
		t.Fatalf("restored HEAD = %s, %v, %v; expected fast-forward to %s", got, err, restored, private)
	}
	if _, err := git(work, "symbolic-ref", "--quiet", "HEAD"); err == nil {
		t.Fatal("expected restored HEAD to stay detached")
	}
	// The commit stays referenced in the main repository after the worktree is gone.
	if _, err := shell.Run("git", "-C", repo, "worktree", "remove", "--force", work); err != nil {
		t.Fatal(err)
	}
	if got, err := git(repo, "rev-parse", "refs/remotes/isodev-work/HEAD"); err != nil || got != private {
		t.Fatalf("isodev-work/HEAD = %s, %v; expected %s", got, err, private)
	}
}

func TestDirtyWorktreeRejected(t *testing.T) {
	root := t.TempDir()
	repo := filepath.Join(root, "repo")
	work := filepath.Join(root, "work")
	for _, a := range [][]string{{"init", "-q", repo}, {"-C", repo, "-c", "user.name=Test", "-c", "user.email=t@example.com", "commit", "--allow-empty", "-qm", "initial"}, {"-C", repo, "worktree", "add", "-qb", "worker", work}} {
		if _, e := shell.Run("git", a...); e != nil {
			t.Fatal(e)
		}
	}
	if e := os.WriteFile(filepath.Join(work, "new"), []byte("x"), 0600); e != nil {
		t.Fatal(e)
	}
	if _, e := git(work, "add", "new"); e != nil {
		t.Fatal(e)
	}
	if _, e := Replace(work, repo, false); e == nil {
		t.Fatal("accepted staged changes")
	}
}

func TestReplacePreservesRemoteTrackingRefs(t *testing.T) {
	root := t.TempDir()
	repo, work := filepath.Join(root, "repo"), filepath.Join(root, "work")
	call := func(args ...string) string {
		t.Helper()
		out, err := shell.Run("git", args...)
		if err != nil {
			t.Fatal(err)
		}
		return out
	}
	call("init", "-q", "-b", "main", repo)
	call("-C", repo, "-c", "user.name=Test", "-c", "user.email=t@example.com", "commit", "--allow-empty", "-qm", "old")
	old := call("-C", repo, "rev-parse", "HEAD")
	call("-C", repo, "remote", "add", "origin", "https://example.invalid/example.git")
	call("-C", repo, "branch", "local-only")
	call("-C", repo, "worktree", "add", "-qb", "feature/test", work)
	call("-C", work, "-c", "user.name=Test", "-c", "user.email=t@example.com", "commit", "--allow-empty", "-qm", "new")
	head := call("-C", work, "rev-parse", "HEAD")
	call("-C", repo, "update-ref", "refs/remotes/origin/main", head)
	call("-C", repo, "symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/main")
	call("-C", repo, "config", "branch.feature/test.remote", "origin")
	call("-C", repo, "config", "branch.feature/test.merge", "refs/heads/main")
	if _, err := Replace(work, repo, false); err != nil {
		t.Fatal(err)
	}
	if got := call("-C", work, "rev-parse", "HEAD"); got != head {
		t.Fatalf("worktree moved: %s", got)
	}
	if got := call("-C", work, "rev-parse", "origin/main"); got != head || got == old {
		t.Fatalf("incorrect origin/main: %s", got)
	}
	if got := call("-C", work, "symbolic-ref", "refs/remotes/origin/HEAD"); got != "refs/remotes/origin/main" {
		t.Fatalf("origin HEAD: %s", got)
	}
	if _, err := shell.Run("git", "-C", work, "show-ref", "--verify", "--quiet", "refs/remotes/origin/local-only"); err == nil {
		t.Fatal("local branch incorrectly became a remote-tracking branch")
	}
	if got := call("-C", work, "remote", "get-url", "origin"); got != "https://example.invalid/example.git" {
		t.Fatalf("origin URL: %s", got)
	}
	if got := call("-C", work, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{upstream}"); got != "origin/main" {
		t.Fatalf("upstream: %s", got)
	}
}

func TestReplaceOriginWithoutTrackingRefs(t *testing.T) {
	root := t.TempDir()
	repo, work := filepath.Join(root, "repo"), filepath.Join(root, "work")
	for _, args := range [][]string{{"init", "-q", repo}, {"-C", repo, "-c", "user.name=Test", "-c", "user.email=t@example.com", "commit", "--allow-empty", "-qm", "init"}, {"-C", repo, "remote", "add", "origin", "https://example.invalid/repo"}, {"-C", repo, "worktree", "add", "-qb", "worker", work}} {
		if _, err := shell.Run("git", args...); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := Replace(work, repo, false); err != nil {
		t.Fatal(err)
	}
	refs, err := git(work, "for-each-ref", "--format=%(refname)", "refs/remotes/origin")
	if err != nil || refs != "" {
		t.Fatalf("unexpected remote refs %q: %v", refs, err)
	}
}

// newWorktree creates a repository with a committed file and a worktree on
// branch worker, and returns the repository, the worktree and a git helper.
func newWorktree(t *testing.T) (string, string, func(args ...string) string) {
	t.Helper()
	root := t.TempDir()
	repo, work := filepath.Join(root, "repo"), filepath.Join(root, "work")
	call := func(args ...string) string {
		t.Helper()
		out, err := shell.Run("git", append([]string{"-c", "user.name=Test", "-c", "user.email=t@example.com"}, args...)...)
		if err != nil {
			t.Fatal(err)
		}
		return out
	}
	call("init", "-q", "-b", "main", repo)
	if err := os.WriteFile(filepath.Join(repo, "file"), []byte("one\n"), 0644); err != nil {
		t.Fatal(err)
	}
	call("-C", repo, "add", "file")
	call("-C", repo, "commit", "-qm", "initial")
	call("-C", repo, "worktree", "add", "-qb", "worker", work)
	return repo, work, call
}

func TestReplaceAddsRemoteAndRestoreFastForwards(t *testing.T) {
	repo, work, call := newWorktree(t)
	result, err := Replace(work, repo, false)
	if err != nil {
		t.Fatal(err)
	}
	if result["remote"] != "isodev-work" {
		t.Fatalf("remote = %v", result["remote"])
	}
	real, _ := filepath.EvalSymlinks(work)
	if got := call("-C", repo, "remote", "get-url", "isodev-work"); got != real {
		t.Fatalf("remote URL = %s, expected %s", got, real)
	}
	if err := os.WriteFile(filepath.Join(work, "file"), []byte("two\n"), 0644); err != nil {
		t.Fatal(err)
	}
	call("-C", work, "commit", "-qam", "agent")
	agent := call("-C", work, "rev-parse", "HEAD")
	call("-C", work, "branch", "extra")
	// The host can fetch the agent's commits while the worktree is independent.
	call("-C", repo, "fetch", "-q", "isodev-work")
	if got := call("-C", repo, "rev-parse", "isodev-work/worker"); got != agent {
		t.Fatalf("fetched worker = %s, expected %s", got, agent)
	}
	if err := os.WriteFile(filepath.Join(work, "staged"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	call("-C", work, "add", "staged")
	restored, err := Restore(work)
	if err != nil {
		t.Fatal(err)
	}
	if restored["fastForwarded"] != true || restored["warning"] != nil {
		t.Fatalf("restore result: %v", restored)
	}
	if got := call("-C", repo, "rev-parse", "worker"); got != agent {
		t.Fatalf("worker = %s, expected %s", got, agent)
	}
	if got := call("-C", work, "status", "--porcelain"); got != "A  staged" {
		t.Fatalf("status after restore = %q", got)
	}
	// The remote's configuration is gone, so nothing points at the archive,
	// but the fetched branches stay.
	if _, err := shell.Run("git", "-C", repo, "remote", "get-url", "isodev-work"); err == nil {
		t.Fatal("remote kept after restore")
	}
	if got := call("-C", repo, "rev-parse", "isodev-work/extra"); got != agent {
		t.Fatalf("extra branch = %s", got)
	}
	// A second replace adds the remote again.
	call("-C", work, "reset", "-q")
	if _, err := Replace(work, repo, false); err != nil {
		t.Fatal(err)
	}
	if got := call("-C", repo, "remote", "get-url", "isodev-work"); got != real {
		t.Fatalf("remote after second replace = %s", got)
	}
}

func TestRestoreKeepsBranchWhenNotFastForward(t *testing.T) {
	for name, change := range map[string][]string{
		"rewritten": {"commit", "-q", "--amend", "--allow-empty", "-m", "rewritten"},
		"switched":  {"switch", "-q", "-c", "other"},
	} {
		t.Run(name, func(t *testing.T) {
			repo, work, call := newWorktree(t)
			initial := call("-C", repo, "rev-parse", "worker")
			if _, err := Replace(work, repo, false); err != nil {
				t.Fatal(err)
			}
			call(append([]string{"-C", work}, change...)...)
			call("-C", work, "commit", "-q", "--allow-empty", "-m", "agent")
			agent := call("-C", work, "rev-parse", "HEAD")
			restored, err := Restore(work)
			if err != nil {
				t.Fatal(err)
			}
			if restored["fastForwarded"] != false || restored["warning"] == nil {
				t.Fatalf("restore result: %v", restored)
			}
			if got := call("-C", repo, "rev-parse", "worker"); got != initial {
				t.Fatalf("worker moved to %s", got)
			}
			if _, err := shell.Run("git", "-C", repo, "cat-file", "-e", agent+"^{commit}"); err != nil {
				t.Fatalf("agent commit not fetched: %v", err)
			}
		})
	}
}

func TestReplaceRejectsForeignRemote(t *testing.T) {
	repo, work, call := newWorktree(t)
	call("-C", repo, "remote", "add", "isodev-work", "https://example.invalid/other.git")
	if _, err := Replace(work, repo, false); err == nil || !strings.Contains(err.Error(), "already points to") {
		t.Fatalf("expected remote conflict, got %v", err)
	}
	if info, err := os.Lstat(filepath.Join(work, ".git")); err != nil || !info.Mode().IsRegular() {
		t.Fatalf("worktree pointer changed: %v", err)
	}
}

func TestArchiveIsExcluded(t *testing.T) {
	repo, work, call := newWorktree(t)
	if _, err := Replace(work, repo, false); err != nil {
		t.Fatal(err)
	}
	if _, err := Restore(work); err != nil {
		t.Fatal(err)
	}
	if got := call("-C", work, "status", "--porcelain", "--untracked-files=all"); got != "" {
		t.Fatalf("archive visible after restore: %q", got)
	}
	if err := os.WriteFile(filepath.Join(work, ArchiveDir, "marker-check"), nil, 0644); err != nil {
		t.Fatal(err)
	}
	if got := call("-C", work, "status", "--porcelain", "--untracked-files=all"); got != "" {
		t.Fatalf("archive visible after restore: %q", got)
	}
}

func TestDetect(t *testing.T) {
	repo, work, call := newWorktree(t)
	call("-c", "protocol.file.allow=always", "-C", repo, "submodule", "add", "-q", repo, "sub")
	cases := map[string]struct {
		path string
		want Kind
	}{
		"main checkout": {repo, NotWorktree},
		"worktree":      {work, Linked},
		"submodule":     {filepath.Join(repo, "sub"), NotWorktree},
		"not Git":       {t.TempDir(), NotWorktree},
	}
	for name, c := range cases {
		st, err := Detect(c.path)
		if err != nil || st.Kind != c.want {
			t.Errorf("%s: Detect = %+v, %v; want kind %d", name, st, err, c.want)
		}
	}
	st, _ := Detect(work)
	common, _ := filepath.EvalSymlinks(filepath.Join(repo, ".git"))
	if st.CommonDir != common || SourceCheckout(st.CommonDir) != filepath.Dir(common) {
		t.Fatalf("common dir = %s, source = %s; want %s", st.CommonDir, SourceCheckout(st.CommonDir), common)
	}
	if _, err := Replace(work, repo, false); err != nil {
		t.Fatal(err)
	}
	if st, err := Detect(work); err != nil || st.Kind != Replaced {
		t.Fatalf("after replace: %+v, %v", st, err)
	}
}

func TestRestoreIgnoresContainerChanges(t *testing.T) {
	repo, work, call := newWorktree(t)
	if _, err := Replace(work, repo, false); err != nil {
		t.Fatal(err)
	}
	pwned := filepath.Join(t.TempDir(), "pwned")
	// What a container could write into the worktree's repository: a forged
	// marker, configuration that runs commands, and a commit on main.
	forged := `{"version":2,"workspace":"` + work + `","sourceCheckout":"/","branch":"main","remote":"--upload-pack=touch ` + pwned + `"}`
	if err := os.WriteFile(filepath.Join(work, ".git", "isodev-git-worktree-unlink.json"), []byte(forged), 0644); err != nil {
		t.Fatal(err)
	}
	mainBefore := call("-C", repo, "rev-parse", "main")
	call("-C", work, "commit", "-q", "--allow-empty", "-m", "agent")
	// Running Git in the worktree after this would run these; Restore must not.
	for _, kv := range [][]string{{"core.fsmonitor", "touch " + pwned}, {"uploadpack.packObjectsHook", "touch " + pwned}, {"core.hooksPath", "/nonexistent"}} {
		call("-C", work, "config", kv[0], kv[1])
	}
	if _, err := Restore(work); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(pwned); err == nil {
		t.Fatal("a command from the container's configuration ran on the host")
	}
	if got := call("-C", repo, "rev-parse", "main"); got != mainBefore {
		t.Fatalf("main moved to %s", got)
	}
}

func TestReplaceRelativeRegistration(t *testing.T) {
	repo, work, call := newWorktree(t)
	real, _ := filepath.EvalSymlinks(work)
	// What git worktree add --relative-paths writes (Git 2.48 and later).
	reg := filepath.Join(repo, ".git", "worktrees", "work")
	if err := os.WriteFile(filepath.Join(reg, "gitdir"), []byte("../../../../work/.git\n"), 0644); err != nil {
		t.Fatal(err)
	}
	pointer := "gitdir: ../repo/.git/worktrees/work\n"
	if err := os.WriteFile(filepath.Join(real, ".git"), []byte(pointer), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := Replace(work, repo, false); err != nil {
		t.Fatal(err)
	}
	if _, err := Restore(work); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(filepath.Join(real, ".git")); string(b) != pointer {
		t.Fatalf(".git after restore = %q, expected %q", b, pointer)
	}
	call("-C", work, "status", "--short")
}

func TestReplaceKeepsLocalExcludes(t *testing.T) {
	repo, work, call := newWorktree(t)
	if err := os.WriteFile(filepath.Join(repo, ".git", "info", "exclude"), []byte(".env\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(work, ".env"), []byte("SECRET=1"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Replace(work, repo, false); err != nil {
		t.Fatal(err)
	}
	if got := call("-C", work, "status", "--porcelain"); got != "" {
		t.Fatalf("locally excluded file visible after replace: %q", got)
	}
}
