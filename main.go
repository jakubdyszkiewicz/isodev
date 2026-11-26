// Command isodev gives each Git worktree its own Docker container, for running
// development environments and coding agents side by side.
package main

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"runtime/debug"

	"github.com/jakubdyszkiewicz/isodev/internal/cli"
)

// Set by GoReleaser at build time.
var (
	version = ""
	commit  = ""
	date    = ""
)

func main() {
	if err := cli.Execute(versionString()); err != nil {
		var code cli.ExitCode
		if errors.As(err, &code) {
			os.Exit(int(code))
		}
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}

// pseudoVersion matches versions Go derives for untagged commits, such as
// v0.0.0-20261126092456-90813e4b18f4.
var pseudoVersion = regexp.MustCompile(`\d{14}-[0-9a-f]{12}`)

// versionString describes the build. Release builds use the values GoReleaser
// sets. `go install ...@vX.Y.Z` builds fall back to the module version, and
// local builds to the Git commit that Go embeds.
func versionString() string {
	v, c, modified := version, commit, false
	if info, ok := debug.ReadBuildInfo(); ok {
		if mv := info.Main.Version; v == "" && mv != "" && mv != "(devel)" && !pseudoVersion.MatchString(mv) {
			v = mv
		}
		for _, s := range info.Settings {
			switch {
			case s.Key == "vcs.revision" && c == "":
				c = s.Value
			case s.Key == "vcs.modified":
				modified = s.Value == "true"
			}
		}
	}
	if v == "" {
		v = "dev"
	}
	if len(c) > 12 {
		c = c[:12]
	}
	if c != "" && modified && commit == "" {
		c += "-dirty"
	}
	switch {
	case c != "" && date != "":
		return fmt.Sprintf("%s (commit %s, built %s)", v, c, date)
	case c != "":
		return fmt.Sprintf("%s (commit %s)", v, c)
	}
	return v
}
