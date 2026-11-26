// Package testutil isolates tests from the user's isodev configuration and state.
package testutil

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jakubdyszkiewicz/isodev/internal/paths"
)

// Home points isodev at a new temporary home directory for the test, so
// configuration, state and data are all isolated.
func Home(t testing.TB) string {
	t.Helper()
	old := paths.Home
	paths.Home = t.TempDir()
	t.Cleanup(func() { paths.Home = old })
	t.Setenv("ISODEV_IMAGE", "")
	return paths.Home
}

// Config isolates the home directory, writes config.yaml and returns the
// configuration directory.
func Config(t testing.TB, content string) string {
	t.Helper()
	Home(t)
	dir := paths.ConfigDir()
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	return dir
}

// RequireDocker skips Docker integration tests unless they are enabled.
func RequireDocker(t testing.TB) {
	t.Helper()
	if os.Getenv("ISODEV_DOCKER_TEST") != "1" {
		t.Skip("set ISODEV_DOCKER_TEST=1 to run Docker integration")
	}
}
