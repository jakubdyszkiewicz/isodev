package gitworktree

import (
	"fmt"
	"io"

	"github.com/jakubdyszkiewicz/isodev/internal/shell"
)

// PreviewReplace describes Replace for --dry-run.
func PreviewReplace(w io.Writer, path, source string, allowDetached bool) {
	fmt.Fprintln(w, "# Validate the worktree, branch and tracked changes; skip if already replaced")
	shell.Print(w, "git", "-C", path, "rev-parse", "HEAD")
	shell.Print(w, "git", "-C", path, "symbolic-ref", "--short", "HEAD")
	shell.Print(w, "git", "-C", path, "diff", "--quiet")
	shell.Print(w, "git", "-C", path, "diff", "--cached", "--quiet")
	shell.Print(w, "git", "clone", "--no-local", "--no-checkout", "--", source, "<temporary-dir>/clone")
	shell.Print(w, "git", "-C", "<temporary-dir>/clone", "fetch", "--no-tags", "--no-write-fetch-head", "--", source, "<worktree-head>")
	if allowDetached {
		fmt.Fprintln(w, "# If HEAD is detached, update it directly; otherwise preserve the current branch")
		shell.Print(w, "git", "-C", "<temporary-dir>/clone", "update-ref", "--no-deref", "HEAD", "<worktree-head>")
		fmt.Fprintln(w, "# If on a branch instead:")
	}
	for _, a := range [][]string{{"update-ref", "refs/heads/<current-branch>", "<worktree-head>"}, {"symbolic-ref", "HEAD", "refs/heads/<current-branch>"}, {"read-tree", "<worktree-head>"}} {
		shell.Print(w, append([]string{"git", "-C", "<temporary-dir>/clone"}, a...)...)
	}
	fmt.Fprintln(w, "# Remove clone-generated origin refs; if source has origin, configure its URL and copy its cached tracking refs locally")
	shell.Print(w, "git", "-C", "<temporary-dir>/clone", "remote", "remove", "origin")
	shell.Print(w, "git", "-C", "<temporary-dir>/clone", "remote", "add", "origin", "<source-origin-url>")
	shell.Print(w, "git", "-C", "<temporary-dir>/clone", "fetch", "--no-tags", "--no-write-fetch-head", "--", source, "+refs/remotes/origin/*:refs/remotes/origin/*")
	fmt.Fprintln(w, "# Restore origin/HEAD and current-branch upstream if present; copy Paseo metadata, write marker and replace the .git pointer")
	fmt.Fprintln(w, "# Add the worktree as a remote of the source checkout, or point the existing one back at it")
	shell.Print(w, "git", "-C", source, "remote", "add", "isodev-<worktree-name>", path)
}

// PreviewRestore describes Restore for --dry-run.
func PreviewRestore(w io.Writer, path string) {
	fmt.Fprintf(w, "# Validate the original worktree registration for %s\n", path)
	fmt.Fprintln(w, "# Fetch the worktree's commits into the source checkout's remote")
	shell.Print(w, "git", "-C", "<source-checkout>", "fetch", "--no-tags", "isodev-<worktree-name>")
	fmt.Fprintf(w, "# Save independent Git metadata in %s/%s/git and restore the .git pointer\n", path, ArchiveDir)
	fmt.Fprintln(w, "# If still on the original branch and the commits extend it, fast-forward and keep the index")
	shell.Print(w, "git", "-C", path, "update-ref", "refs/heads/<branch>", "<worktree-head>", "<original-head>")
	shell.Print(w, "git", "-C", "<source-checkout>", "remote", "set-url", "isodev-<worktree-name>", path+"/"+ArchiveDir+"/git")
}
