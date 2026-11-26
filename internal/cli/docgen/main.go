// Command docgen generates the command reference on the documentation site
// from the cobra command tree. Run it with `go generate ./...`.
package main

import (
	"flag"
	"fmt"
	"log"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/jakubdyszkiewicz/isodev/internal/cli"
	"github.com/jakubdyszkiewicz/isodev/internal/gendoc"
)

const docSource = "the command definitions and help texts in internal/cli"

func main() {
	docsPath := flag.String("docs", "", "Markdown file whose generated reference section is replaced")
	flag.Parse()
	if *docsPath == "" {
		log.Fatal("usage: docgen -docs FILE")
	}
	if err := gendoc.Splice(*docsPath, docSource, reference()); err != nil {
		log.Fatal(err)
	}
}

// reference renders every visible command, starting with the root command and
// its global flags.
func reference() string {
	root := cli.NewRootCommand()
	// Add the --help and --version flags that cobra creates when it runs.
	root.Version = "dev"
	root.InitDefaultHelpFlag()
	root.InitDefaultVersionFlag()
	var b strings.Builder
	writeCommand(&b, root)
	return b.String()
}

func writeCommand(b *strings.Builder, cmd *cobra.Command) {
	level := "##"
	if depth(cmd) > 1 {
		level = "###"
	}
	fmt.Fprintf(b, "%s `%s`\n\n", level, cmd.CommandPath())
	description := cmd.Long
	if description == "" {
		description = cmd.Short
	}
	fmt.Fprintf(b, "%s\n\n", strings.TrimSpace(description))
	if cmd.Runnable() {
		fmt.Fprintf(b, "```text\n%s\n```\n\n", cmd.UseLine())
	}
	if len(cmd.Aliases) > 0 {
		aliases := make([]string, len(cmd.Aliases))
		for i, a := range cmd.Aliases {
			aliases[i] = "`" + a + "`"
		}
		fmt.Fprintf(b, "**Aliases:** %s\n\n", strings.Join(aliases, ", "))
	}
	flags := cmd.LocalNonPersistentFlags()
	if !cmd.HasParent() {
		flags = cmd.Flags() // The root command documents the global flags.
	}
	if lines := flagLines(flags); len(lines) > 0 {
		title := "Flags"
		if !cmd.HasParent() {
			title = "Global flags"
		}
		fmt.Fprintf(b, "**%s:**\n\n%s\n\n", title, strings.Join(lines, "\n"))
	}
	if cmd.Example != "" {
		fmt.Fprintf(b, "**Examples:**\n\n```sh\n%s\n```\n\n", dedent(cmd.Example))
	}
	var children []*cobra.Command
	for _, c := range cmd.Commands() {
		if c.IsAvailableCommand() {
			children = append(children, c)
		}
	}
	if len(children) > 0 && cmd.HasParent() {
		fmt.Fprintf(b, "**Commands:**\n\n")
		for _, c := range children {
			fmt.Fprintf(b, "- [`%s`](#%s): %s\n", c.Name(), anchor(c), c.Short)
		}
		b.WriteString("\n")
	}
	for _, c := range children {
		writeCommand(b, c)
	}
}

func depth(cmd *cobra.Command) int {
	n := 0
	for c := cmd; c.HasParent(); c = c.Parent() {
		n++
	}
	return n
}

// anchor is the heading ID Hugo generates for a command's heading.
func anchor(cmd *cobra.Command) string {
	return strings.ReplaceAll(cmd.CommandPath(), " ", "-")
}

func flagLines(flags *pflag.FlagSet) []string {
	var lines []string
	flags.VisitAll(func(f *pflag.Flag) {
		if f.Hidden {
			return
		}
		name := "--" + f.Name
		if f.Shorthand != "" {
			name = "-" + f.Shorthand + ", " + name
		}
		if t := f.Value.Type(); t != "bool" {
			name += " " + t
		}
		line := fmt.Sprintf("- `%s`: %s", name, f.Usage)
		if d := f.DefValue; d != "" && d != "false" && d != "[]" {
			line += fmt.Sprintf(" (default `%s`)", d)
		}
		lines = append(lines, line)
	})
	return lines
}

// dedent removes the indentation cobra examples conventionally use.
func dedent(s string) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	for i, l := range lines {
		lines[i] = strings.TrimPrefix(l, "  ")
	}
	return strings.Join(lines, "\n")
}
