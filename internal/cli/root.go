// Package cli defines the isodev command-line interface.
package cli

import (
	"encoding/json"
	"fmt"
	"os/exec"

	"github.com/spf13/cobra"
)

// ExitCode is the non-zero exit status of a command that Isodev ran in a
// container, such as with isodev exec. Isodev exits with the same status and
// prints nothing more, as docker exec does.
type ExitCode int

func (c ExitCode) Error() string { return fmt.Sprintf("exit status %d", int(c)) }

// usageError is an invalid command line. Like Docker, it points to the help.
type usageError struct {
	command string
	err     error
}

func (e usageError) Error() string { return fmt.Sprintf("%v\nSee '%s --help'.", e.err, e.command) }

func (e usageError) Unwrap() error { return e.err }

// Execute runs the isodev command line. version is shown by --version.
func Execute(version string) error {
	root := NewRootCommand()
	root.Version = version
	cmd, err := root.ExecuteC()
	// The command run in the container already reported its failure.
	if exitErr, ok := err.(*exec.ExitError); ok && cmd.Annotations[streamsOutput] == "true" {
		if code := exitErr.ExitCode(); code > 0 {
			return ExitCode(code)
		}
		return ExitCode(1)
	}
	return err
}

//go:generate go run ./docgen -docs ../../docs/content/docs/commands.md

func init() {
	// Keep commands in the order they are added, in help and in the reference.
	cobra.EnableCommandSorting = false
}

// NewRootCommand builds the isodev command tree.
func NewRootCommand() *cobra.Command {
	root := &cobra.Command{Use: "isodev", Short: "Run each Git worktree in its own Docker container", Long: rootLong, SilenceUsage: true, SilenceErrors: true, Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error { return cmd.Help() }}
	root.CompletionOptions.DisableDefaultCmd = true
	root.SetVersionTemplate("isodev {{.Version}}\n")
	root.PersistentFlags().Bool("dry-run", false, "preview actions without making changes")
	root.PersistentFlags().StringP("output", "o", "text", "output format (text or json)")
	root.PersistentPreRunE = validateGlobalFlags
	root.AddCommand(newContainerCommands()...)
	root.AddCommand(newConfigCommand(), newIntegrationCommand())
	root.SetFlagErrorFunc(func(cmd *cobra.Command, err error) error {
		return usageError{cmd.CommandPath(), err}
	})
	markUsageErrors(root)
	return root
}

// markUsageErrors makes argument errors of cmd and its subcommands usage
// errors.
func markUsageErrors(cmd *cobra.Command) {
	if validate := cmd.Args; validate != nil {
		cmd.Args = func(c *cobra.Command, args []string) error {
			if err := validate(c, args); err != nil {
				return usageError{c.CommandPath(), err}
			}
			return nil
		}
	}
	for _, sub := range cmd.Commands() {
		markUsageErrors(sub)
	}
}

func validateGlobalFlags(cmd *cobra.Command, args []string) error {
	format, _ := cmd.Flags().GetString("output")
	if format != "text" && format != "json" {
		return usageError{cmd.CommandPath(), fmt.Errorf("unsupported output format %q: use text or json", format)}
	}
	if format == "json" && dryRun(cmd) {
		return usageError{cmd.CommandPath(), fmt.Errorf("dry-run only supports text output")}
	}
	if format == "json" && cmd.Annotations[streamsOutput] == "true" {
		return usageError{cmd.CommandPath(), fmt.Errorf("%s streams command output and does not support -o json", cmd.CommandPath())}
	}
	return nil
}

// streamsOutput marks commands whose output is the command's own stream.
const streamsOutput = "isodev/streams-output"

type runFunc func(cmd *cobra.Command, args []string) error

// action returns a RunE that calls preview instead of run under --dry-run.
// A nil preview means the command does not support dry runs.
func action(run, preview runFunc) runFunc {
	return func(cmd *cobra.Command, args []string) error {
		if !dryRun(cmd) {
			return run(cmd, args)
		}
		if preview == nil {
			return fmt.Errorf("dry run is not supported for %s", cmd.CommandPath())
		}
		return preview(cmd, args)
	}
}

func dryRun(cmd *cobra.Command) bool {
	dry, _ := cmd.Flags().GetBool("dry-run")
	return dry
}

func jsonOutput(cmd *cobra.Command) bool {
	format, _ := cmd.Flags().GetString("output")
	return format == "json"
}

// output writes v as JSON with -o json, or calls text otherwise.
func output(cmd *cobra.Command, v any, text func()) error {
	if jsonOutput(cmd) {
		return json.NewEncoder(cmd.OutOrStdout()).Encode(v)
	}
	text()
	return nil
}

// stringFlag returns a flag value that must not be empty.
func stringFlag(cmd *cobra.Command, name, message string) (string, error) {
	v, _ := cmd.Flags().GetString(name)
	if v == "" {
		return "", fmt.Errorf("%s", message)
	}
	return v, nil
}
