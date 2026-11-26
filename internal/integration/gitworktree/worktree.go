// Package gitworktree detects linked Git worktrees, and replaces a worktree's
// link with an independent repository and restores it.
package gitworktree

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/jakubdyszkiewicz/isodev/internal/fsutil"
	"github.com/jakubdyszkiewicz/isodev/internal/paths"
	"github.com/jakubdyszkiewicz/isodev/internal/shell"
)

// ArchiveDir is created in the workspace by Restore. Each restore saves the
// worktree's independent repository in a new directory in it.
const ArchiveDir = ".isodev-git-archive"

func git(cwd string, args ...string) (string, error) {
	return shell.Run("git", append([]string{"-C", cwd}, args...)...)
}

// Marker records how to restore a replaced worktree. Replace writes it to
// Isodev's state directory on the host, not into the worktree, so a container
// that can write the worktree can't change what Restore does on the host.
type Marker struct {
	Version   int    `json:"version"`
	Workspace string `json:"workspace"`
	Source    string `json:"sourceCheckout"`
	Gitdir    string `json:"originalGitdir"`
	Pointer   string `json:"originalGitPointer"`
	Head      string `json:"initialCommit"`
	Branch    string `json:"branch"`
	Remote    string `json:"remote"`
}

const markerVersion = 2

// markerPath is where Replace records how to restore the worktree at cwd.
func markerPath(cwd string) string {
	sum := sha256.Sum256([]byte(cwd))
	return filepath.Join(paths.StateDir(), "worktrees", hex.EncodeToString(sum[:8])+".json")
}

// readMarker returns the marker of the replaced worktree at cwd. The error
// satisfies os.IsNotExist when Isodev didn't replace it.
func readMarker(cwd string) (Marker, error) {
	var m Marker
	if err := fsutil.ReadJSON(markerPath(cwd), &m); err != nil {
		return m, err
	}
	if m.Version != markerVersion || m.Workspace != cwd {
		return m, fmt.Errorf("invalid replacement record %s", markerPath(cwd))
	}
	return m, nil
}

// registered reports whether the worktree registration gitdir, such as
// COMMON/worktrees/NAME, points at dot. The registration's gitdir file can
// hold a path relative to the registration.
func registered(gitdir, dot string) error {
	b, err := os.ReadFile(filepath.Join(gitdir, "gitdir"))
	if err != nil {
		return err
	}
	p := strings.TrimSpace(string(b))
	if !filepath.IsAbs(p) {
		p = filepath.Join(gitdir, p)
	}
	if filepath.Clean(p) != dot {
		return fmt.Errorf("worktree registration %s points to %s, not %s", gitdir, p, dot)
	}
	return nil
}

// RemoteName is the source repository's remote for the worktree registered
// as gitdir.
func RemoteName(gitdir string) string { return "isodev-" + filepath.Base(gitdir) }

// checkRemote reports whether source already has the worktree's remote, and
// fails if a remote with that name points anywhere else.
func checkRemote(source, name, cwd string) (bool, error) {
	url, err := git(source, "remote", "get-url", name)
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 2 {
			return false, nil
		}
		return false, err
	}
	if url != cwd {
		return false, fmt.Errorf("remote %s in %s already points to %s", name, source, url)
	}
	return true, nil
}

// ensureRemote points source's remote name at the replaced worktree at cwd.
func ensureRemote(source, name, cwd string) error {
	has, err := checkRemote(source, name, cwd)
	if err != nil {
		return err
	}
	args := []string{"remote", "add", "--", name, cwd}
	if has {
		args = []string{"remote", "set-url", "--", name, cwd}
	}
	if _, err = git(source, args...); err != nil {
		return fmt.Errorf("add remote %s: %w", name, err)
	}
	return nil
}

// exclude adds the archive directory to a Git directory's info/exclude, so
// the archived metadata is never committed.
func exclude(gitdir string) error {
	path := filepath.Join(gitdir, "info", "exclude")
	b, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	line := "/" + ArchiveDir + "/"
	for _, l := range strings.Split(string(b), "\n") {
		if strings.TrimSpace(l) == line {
			return nil
		}
	}
	if err = os.MkdirAll(filepath.Dir(path), 0755); err != nil { //nolint:gosec // Git's own permissions for .git/info.
		return err
	}
	if len(b) > 0 && !strings.HasSuffix(string(b), "\n") {
		line = "\n" + line
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644) //nolint:gosec // Git's own permissions for info/exclude.
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(f, line)
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	return err
}

// Kind is what Detect finds at a workspace's root.
type Kind int

const (
	// NotWorktree is a directory that isn't the root of a linked worktree,
	// such as a main checkout, a submodule or a non-Git directory.
	NotWorktree Kind = iota
	// Linked is a linked worktree whose .git file points into the main
	// repository's Git directory.
	Linked
	// Replaced is a worktree whose link Replace replaced.
	Replaced
)

// State describes a workspace's Git worktree.
type State struct {
	Kind Kind
	// CommonDir is the main repository's Git directory, Git's "common
	// directory", for a linked or replaced worktree.
	CommonDir string
}

// Detect reports whether path is the root of a linked worktree, or of one
// whose link was replaced. It reads Git's files directly and doesn't run Git.
func Detect(path string) (State, error) {
	if real, err := fsutil.Real(path); err == nil {
		path = real
	}
	dot := filepath.Join(path, ".git")
	st, err := os.Lstat(dot)
	if os.IsNotExist(err) {
		return State{}, nil
	}
	if err != nil {
		return State{}, err
	}
	if st.IsDir() {
		m, err := readMarker(path)
		if os.IsNotExist(err) {
			return State{}, nil
		} else if err != nil {
			return State{}, err
		}
		// The original registration is COMMON/worktrees/NAME.
		return State{Kind: Replaced, CommonDir: filepath.Dir(filepath.Dir(m.Gitdir))}, nil
	}
	if !st.Mode().IsRegular() {
		return State{}, nil
	}
	b, err := os.ReadFile(dot)
	if err != nil {
		return State{}, err
	}
	gitdir, ok := strings.CutPrefix(strings.TrimSpace(string(b)), "gitdir:")
	if !ok {
		return State{}, nil
	}
	gitdir = strings.TrimSpace(gitdir)
	if !filepath.IsAbs(gitdir) {
		gitdir = filepath.Join(path, gitdir)
	}
	// A linked worktree's Git directory names the common directory; a
	// submodule's doesn't.
	c, err := os.ReadFile(filepath.Join(gitdir, "commondir"))
	if os.IsNotExist(err) {
		return State{}, nil
	}
	if err != nil {
		return State{}, err
	}
	common := strings.TrimSpace(string(c))
	if !filepath.IsAbs(common) {
		common = filepath.Join(gitdir, common)
	}
	common, err = fsutil.Real(common)
	if err != nil {
		return State{}, err
	}
	return State{Kind: Linked, CommonDir: common}, nil
}

// SourceCheckout returns the checkout to replace a worktree from: the main
// worktree that contains the common directory, or the common directory itself
// for a bare repository.
func SourceCheckout(common string) string {
	if filepath.Base(common) == ".git" {
		return filepath.Dir(common)
	}
	return common
}

func optionalGitConfig(cwd, key string) (string, error) {
	value, err := git(cwd, "config", "--get", key)
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
			return "", nil
		}
		return "", err
	}
	return value, nil
}

// Replace replaces the worktree's .git pointer with independent Git metadata,
// keeping its files, branch and commit. The worktree must belong to source and
// have no tracked changes. allowDetached permits a detached HEAD, as created by
// Zed. Cached origin refs are copied from source without network access.
func Replace(path, source string, allowDetached bool) (map[string]any, error) {
	cwd, e := fsutil.Real(path)
	if e != nil {
		return nil, e
	}
	source, e = fsutil.Real(source)
	if e != nil {
		return nil, e
	}
	if cwd == source {
		return nil, fmt.Errorf("refusing to swap source checkout")
	}
	common, e := git(source, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if e != nil {
		return nil, e
	}
	common, e = fsutil.Real(common)
	if e != nil {
		return nil, e
	}
	dot := filepath.Join(cwd, ".git")
	st, e := os.Lstat(dot)
	if e != nil {
		return nil, e
	}
	if st.IsDir() {
		m, e := readMarker(cwd)
		if os.IsNotExist(e) {
			return nil, fmt.Errorf("%s has its own .git directory, and Isodev has no record of replacing it", cwd)
		} else if e != nil {
			return nil, e
		}
		parent, e := fsutil.Real(filepath.Join(m.Gitdir, "../.."))
		if e != nil || parent != common {
			return nil, fmt.Errorf("checkout belongs to another project")
		}
		// Repair the remote if it was removed while the worktree was replaced.
		if e = ensureRemote(source, m.Remote, cwd); e != nil {
			return nil, e
		}
		return map[string]any{"alreadyReplaced": true, "path": cwd, "remote": m.Remote}, nil
	}
	if !st.Mode().IsRegular() {
		return nil, fmt.Errorf("expected worktree pointer")
	}
	original, e := git(cwd, "rev-parse", "--absolute-git-dir")
	if e != nil {
		return nil, e
	}
	original, e = fsutil.Real(original)
	if e != nil {
		return nil, e
	}
	actual, e := git(cwd, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if e != nil {
		return nil, e
	}
	actual, e = fsutil.Real(actual)
	if e != nil || actual != common || original == common {
		return nil, fmt.Errorf("unexpected worktree repository")
	}
	// Restore checks this too; checking it now avoids a replacement that can't
	// be restored.
	if e = registered(original, dot); e != nil {
		return nil, e
	}
	head, e := git(cwd, "rev-parse", "HEAD")
	if e != nil {
		return nil, e
	}
	branch, e := git(cwd, "symbolic-ref", "--quiet", "--short", "HEAD")
	if e != nil {
		var exitErr *exec.ExitError
		if !allowDetached || !errors.As(e, &exitErr) || exitErr.ExitCode() != 1 {
			return nil, fmt.Errorf("worktree must be on a branch (detached HEAD is not supported): %w", e)
		}
		branch = ""
	}
	for _, a := range [][]string{{"diff", "--quiet"}, {"diff", "--cached", "--quiet"}} {
		if _, e = git(cwd, a...); e != nil {
			return nil, fmt.Errorf("worktree has tracked changes: %w", e)
		}
	}
	remote := RemoteName(original)
	if _, e = checkRemote(source, remote, cwd); e != nil {
		return nil, e
	}
	staging, e := os.MkdirTemp(filepath.Dir(cwd), ".isodev-git-swap-")
	if e != nil {
		return nil, e
	}
	keepStaging := false
	defer func() {
		if !keepStaging {
			_ = os.RemoveAll(staging)
		}
	}()
	clone := filepath.Join(staging, "clone")
	if _, e = shell.Run("git", "clone", "--no-local", "--no-checkout", "--", source, clone); e != nil {
		return nil, e
	}
	// A detached worktree can point at a commit not referenced by any source branch.
	if _, e = git(clone, "fetch", "--no-tags", "--no-write-fetch-head", "--", source, head); e != nil {
		return nil, e
	}
	steps := [][]string{}
	if branch == "" {
		steps = append(steps, []string{"update-ref", "--no-deref", "HEAD", head})
	} else {
		steps = append(steps, []string{"update-ref", "refs/heads/" + branch, head}, []string{"symbolic-ref", "HEAD", "refs/heads/" + branch})
	}
	steps = append(steps, []string{"read-tree", head})
	for _, a := range steps {
		if _, e = git(clone, a...); e != nil {
			return nil, e
		}
	}
	remotes, e := git(source, "remote")
	if e != nil {
		return nil, e
	}
	has := false
	for _, r := range strings.Split(remotes, "\n") {
		if r == "origin" {
			has = true
		}
	}
	if has {
		url, e := git(source, "remote", "get-url", "origin")
		if e != nil {
			return nil, e
		}
		// A local clone treats the source's local branches as origin/* refs.
		// Remove those synthetic refs before copying the source's real cached
		// origin refs, without contacting the network.
		if _, e = git(clone, "remote", "remove", "origin"); e != nil {
			return nil, e
		}
		if _, e = git(clone, "remote", "add", "origin", url); e != nil {
			return nil, e
		}
		if _, e = git(clone, "fetch", "--no-tags", "--no-write-fetch-head", "--", source, "+refs/remotes/origin/*:refs/remotes/origin/*"); e != nil {
			return nil, e
		}
		// Preserve origin/HEAD as a symbolic ref when the source has one.
		originHead, symErr := git(source, "symbolic-ref", "--quiet", "refs/remotes/origin/HEAD")
		if symErr == nil {
			if _, e = git(clone, "symbolic-ref", "refs/remotes/origin/HEAD", originHead); e != nil {
				return nil, e
			}
		}
		remote := ""
		if branch != "" {
			remote, e = optionalGitConfig(cwd, "branch."+branch+".remote")
		}
		if e != nil {
			return nil, e
		}
		if remote == "origin" {
			merge, e := optionalGitConfig(cwd, "branch."+branch+".merge")
			if e != nil {
				return nil, e
			}
			if merge != "" {
				if _, e = git(clone, "config", "--local", "branch."+branch+".remote", remote); e != nil {
					return nil, e
				}
				if _, e = git(clone, "config", "--local", "branch."+branch+".merge", merge); e != nil {
					return nil, e
				}
			}
		}
	} else if _, e = git(clone, "remote", "remove", "origin"); e != nil {
		return nil, e
	}
	if _, e = os.Stat(filepath.Join(original, "paseo")); e == nil {
		if e = copyDir(filepath.Join(original, "paseo"), filepath.Join(clone, ".git", "paseo")); e != nil {
			return nil, e
		}
	} else if !os.IsNotExist(e) {
		return nil, e
	}
	pointer, e := os.ReadFile(dot)
	if e != nil {
		return nil, e
	}
	// Files the main repository ignores locally stay ignored.
	if b, err := os.ReadFile(filepath.Join(common, "info", "exclude")); err == nil {
		if err = os.WriteFile(filepath.Join(clone, ".git", "info", "exclude"), b, 0644); err != nil { //nolint:gosec // Git's own permissions for info/exclude.
			return nil, err
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	if e = exclude(filepath.Join(clone, ".git")); e != nil {
		return nil, e
	}
	m := Marker{markerVersion, cwd, source, original, string(pointer), head, branch, remote}
	if e = os.MkdirAll(filepath.Dir(markerPath(cwd)), 0700); e != nil {
		return nil, e
	}
	if e = fsutil.WriteJSONAtomic(markerPath(cwd), m); e != nil {
		return nil, e
	}
	saved := filepath.Join(staging, "original.git")
	if e = os.Rename(dot, saved); e != nil {
		_ = os.Remove(markerPath(cwd))
		return nil, e
	}
	if e = os.Rename(filepath.Join(clone, ".git"), dot); e != nil {
		if rollbackErr := os.Rename(saved, dot); rollbackErr != nil {
			keepStaging = true
			return nil, fmt.Errorf("replace worktree pointer: %w; the original pointer remains at %s: %v", e, saved, rollbackErr)
		}
		_ = os.Remove(markerPath(cwd))
		return nil, e
	}
	if e = ensureRemote(source, remote, cwd); e != nil {
		return nil, fmt.Errorf("%w; the worktree is already replaced", e)
	}
	return map[string]any{"replaced": true, "path": cwd, "branch": branch, "head": head, "remote": remote}, nil
}

// Restore restores the original link of the worktree that Replace replaced,
// and moves its independent repository to a new directory in ArchiveDir.
//
// It first fetches the worktree's branches into refs/remotes/REMOTE/* of the
// source repository, and its HEAD into refs/remotes/REMOTE/HEAD, so no commit
// is lost when the worktree is deleted. If the worktree is still on its
// original branch, or still detached, and its commits extend the original
// commit, the restored worktree is fast-forwarded to them and keeps its
// staged changes. The remote's configuration is then removed; its refs stay.
//
// Everything Restore decides comes from the host: the marker in Isodev's state
// directory and the worktree's registration in the source repository. The
// worktree's own repository, which a container can change, is only read
// through Git's transport.
func Restore(path string) (map[string]any, error) {
	cwd, e := fsutil.Real(path)
	if e != nil {
		return nil, e
	}
	dot := filepath.Join(cwd, ".git")
	m, e := readMarker(cwd)
	if os.IsNotExist(e) {
		if st, err := os.Lstat(dot); err == nil && st.Mode().IsRegular() {
			return map[string]any{"alreadyRestored": true, "path": cwd}, nil
		}
		return nil, fmt.Errorf("Isodev has no record of replacing %s", cwd) //nolint:staticcheck // Isodev is a name.
	} else if e != nil {
		return nil, e
	}
	common, e := git(m.Source, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if e != nil {
		return nil, e
	}
	regs, e := fsutil.Real(filepath.Join(common, "worktrees"))
	if e != nil {
		return nil, e
	}
	original, e := fsutil.Real(m.Gitdir)
	if e != nil {
		return nil, e
	}
	if filepath.Dir(original) != regs {
		return nil, fmt.Errorf("invalid worktree registration %s", m.Gitdir)
	}
	if e = registered(original, dot); e != nil {
		return nil, e
	}
	if st, err := os.Lstat(dot); err == nil && st.Mode().IsRegular() {
		// A previous restore was interrupted after restoring the pointer.
		_ = os.Remove(markerPath(cwd))
		return map[string]any{"alreadyRestored": true, "path": cwd}, nil
	}
	remote := RemoteName(original)
	head, branch, e := remoteHead(m.Source, cwd)
	if e != nil {
		return nil, fmt.Errorf("read the worktree's repository: %w", e)
	}
	// Fetch before changing anything, so a failure leaves the worktree as it was.
	if _, e = git(m.Source, "fetch", "--quiet", "--no-tags", "--", cwd, "+refs/heads/*:refs/remotes/"+remote+"/*", "+HEAD:refs/remotes/"+remote+"/HEAD"); e != nil {
		return nil, fmt.Errorf("fetch the worktree's commits into %s: %w", remote, e)
	}
	if e = exclude(common); e != nil {
		return nil, e
	}
	archive, e := newArchive(cwd)
	if e != nil {
		return nil, e
	}
	if _, err := os.Lstat(dot); err == nil {
		if e = os.Rename(dot, archive); e != nil {
			return nil, e
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	f, e := os.OpenFile(dot, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644) //nolint:gosec // Git's own permissions for a .git file.
	createdPointer := e == nil
	if e == nil {
		_, e = f.WriteString(m.Pointer)
		if closeErr := f.Close(); e == nil {
			e = closeErr
		}
	}
	if e != nil {
		if createdPointer {
			_ = os.Remove(dot)
		}
		if rollbackErr := os.Rename(archive, dot); rollbackErr != nil {
			return nil, fmt.Errorf("restore worktree pointer: %w; independent Git metadata remains at %s: %v", e, archive, rollbackErr)
		}
		return nil, e
	}
	if e = os.Remove(markerPath(cwd)); e != nil && !os.IsNotExist(e) {
		return nil, e
	}
	var warnings []string
	// The fetched refs stay; without the remote, nothing points at the
	// archive, which disappears with the worktree.
	if _, err := git(m.Source, "config", "--remove-section", "remote."+remote); err != nil {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) || exitErr.ExitCode() != 128 {
			warnings = append(warnings, fmt.Sprintf("could not remove remote %s: %v", remote, err))
		}
	}
	result := map[string]any{"restored": true, "path": cwd, "savedGit": archive, "remote": remote, "head": head, "fastForwarded": false}
	finish := func() (map[string]any, error) {
		if len(warnings) > 0 {
			result["warning"] = strings.Join(warnings, "; ")
		}
		return result, nil
	}
	kept := remote + "/HEAD"
	if branch != "" {
		kept = remote + "/" + branch
	}
	if head == m.Head {
		copyIndex(cwd, archive, original)
		return finish()
	}
	if branch != m.Branch {
		warnings = append(warnings, fmt.Sprintf("the worktree is no longer on its original branch; its commits are in %s", kept))
		return finish()
	}
	if _, err := git(cwd, "merge-base", "--is-ancestor", m.Head, head); err != nil {
		warnings = append(warnings, fmt.Sprintf("the worktree's commits don't extend the original commit; they are in %s", kept))
		return finish()
	}
	ref := []string{"update-ref", "refs/heads/" + m.Branch, head, m.Head}
	if m.Branch == "" {
		ref = []string{"update-ref", "--no-deref", "HEAD", head, m.Head}
	}
	if _, err := git(cwd, ref...); err != nil {
		warnings = append(warnings, fmt.Sprintf("could not fast-forward the worktree: %v; its commits are in %s", err, kept))
		return finish()
	}
	result["fastForwarded"] = true
	copyIndex(cwd, archive, original)
	return finish()
}

// remoteHead returns the commit and the branch, empty if detached, of the
// repository at dir, as seen through Git's transport from source.
func remoteHead(source, dir string) (string, string, error) {
	out, err := git(source, "ls-remote", "--symref", "--", dir, "HEAD")
	if err != nil {
		return "", "", err
	}
	head, branch := "", ""
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		switch {
		case len(fields) == 3 && fields[0] == "ref:" && fields[2] == "HEAD":
			branch = strings.TrimPrefix(fields[1], "refs/heads/")
		case len(fields) == 2 && fields[1] == "HEAD":
			head = fields[0]
		}
	}
	if !validCommit(head) {
		return "", "", fmt.Errorf("no valid HEAD in %s", dir)
	}
	if branch != "" && strings.HasPrefix(branch, "refs/") {
		branch = "" // HEAD points outside refs/heads, so treat it as detached.
	}
	return head, branch, nil
}

func validCommit(s string) bool {
	if len(s) != 40 && len(s) != 64 {
		return false
	}
	for _, c := range s {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

// newArchive returns a new directory path in the workspace's ArchiveDir.
func newArchive(cwd string) (string, error) {
	dir := filepath.Join(cwd, ArchiveDir)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}
	base := filepath.Join(dir, time.Now().UTC().Format("20060102T150405Z"))
	for i := 0; ; i++ {
		p := base
		if i > 0 {
			p = fmt.Sprintf("%s-%d", base, i)
		}
		if _, err := os.Lstat(p); os.IsNotExist(err) {
			return p, nil
		} else if err != nil {
			return "", err
		}
	}
}

// copyIndex gives the restored worktree the independent index, which matches
// its files and HEAD and holds any staged changes. If Git can't read the
// copy, the index is rebuilt from HEAD instead.
func copyIndex(cwd, archive, original string) {
	b, err := os.ReadFile(filepath.Join(archive, "index"))
	if err == nil {
		err = os.WriteFile(filepath.Join(original, "index"), b, 0644) //nolint:gosec // Git's own permissions for the index.
	}
	if err == nil {
		_, err = git(cwd, "update-index", "-q", "--refresh")
		if err != nil {
			var exitErr *exec.ExitError
			if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
				err = nil // Exit status 1 only reports modified files.
			}
		}
	}
	if err != nil {
		_, _ = git(cwd, "read-tree", "HEAD")
	}
}

// copyDir copies a tree of regular files and directories, keeping permissions.
// It carries Paseo's per-worktree metadata into the independent Git directory.
func copyDir(src, dst string) error {
	return filepath.WalkDir(src, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		info, err := d.Info()
		if err != nil {
			return err
		}
		if d.IsDir() {
			return os.MkdirAll(target, info.Mode().Perm())
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("unsupported file %s", p)
		}
		b, err := os.ReadFile(p) //nolint:gosec // Paseo's metadata in a Git directory Isodev just validated.
		if err != nil {
			return err
		}
		return os.WriteFile(target, b, info.Mode().Perm())
	})
}
