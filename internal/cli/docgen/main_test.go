package main

import (
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/jakubdyszkiewicz/isodev/internal/cli"
	"github.com/jakubdyszkiewicz/isodev/internal/gendoc"
)

// TestReferenceUpToDate fails when the committed command reference no longer
// matches the command definitions. Run `go generate ./...` to update it.
func TestReferenceUpToDate(t *testing.T) {
	const path = "../../../docs/content/docs/commands.md"
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	begin, end := gendoc.Markers(docSource)
	text := string(data)
	b, e := strings.Index(text, begin), strings.Index(text, end)
	if b < 0 || e < b {
		t.Fatalf("%s has no generated reference section", path)
	}
	if got, want := text[b+len(begin):e], "\n\n"+reference(); got != want {
		t.Errorf("%s is out of date; run `go generate ./...`", path)
	}
}

// TestEveryCommandDocumented requires a description for every visible command
// and flag, so the reference has no gaps.
func TestEveryCommandDocumented(t *testing.T) {
	var check func(cmd *cobra.Command)
	check = func(cmd *cobra.Command) {
		if cmd.Short == "" {
			t.Errorf("%s has no Short description", cmd.CommandPath())
		}
		cmd.LocalFlags().VisitAll(func(f *pflag.Flag) {
			if !f.Hidden && f.Usage == "" {
				t.Errorf("%s --%s has no usage text", cmd.CommandPath(), f.Name)
			}
		})
		for _, c := range cmd.Commands() {
			if c.IsAvailableCommand() {
				check(c)
			}
		}
	}
	check(cli.NewRootCommand())
}
