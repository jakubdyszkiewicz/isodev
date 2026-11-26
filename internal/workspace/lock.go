package workspace

import (
	"os"
	"path/filepath"
	"syscall"

	"github.com/jakubdyszkiewicz/isodev/internal/container"
	"github.com/jakubdyszkiewicz/isodev/internal/fsutil"
	"github.com/jakubdyszkiewicz/isodev/internal/paths"
)

// Lock runs fn while holding the lock of the workspace at path, so that only
// one Isodev process at a time creates, replaces, restores or removes its
// container and worktree. Paseo and Zed can run setup more than once, or launch
// an agent while setup runs.
func Lock(path string, fn func() error) error {
	cwd, err := fsutil.RealOrAbs(path)
	if err != nil {
		return err
	}
	dir := filepath.Join(paths.StateDir(), "locks")
	if err = os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	f, err := os.OpenFile(filepath.Join(dir, container.Name(cwd)+".lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	if err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		return err
	}
	defer func() { _ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN) }()
	return fn()
}
