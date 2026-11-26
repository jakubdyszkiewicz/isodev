// Package paths locates isodev's configuration, state and data directories.
package paths

import (
	"os"
	"path/filepath"
)

// Home is the host user's home directory. Tests replace it with a temporary
// directory to isolate configuration and state.
var Home, _ = os.UserHomeDir()

// ConfigDir holds config.yaml and, by convention, hook scripts and image files.
func ConfigDir() string { return filepath.Join(Home, ".config", "isodev") }

// ConfigFile is the optional global configuration file.
func ConfigFile() string { return filepath.Join(ConfigDir(), "config.yaml") }

// StateDir holds workspace records, locks and the Paseo probe workspace.
func StateDir() string { return filepath.Join(Home, ".local", "state", "isodev") }
