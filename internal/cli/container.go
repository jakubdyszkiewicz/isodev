package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/jakubdyszkiewicz/isodev/internal/config"
	"github.com/jakubdyszkiewicz/isodev/internal/container"
	"github.com/jakubdyszkiewicz/isodev/internal/workspace"
)

func newConfigCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "config", Short: "Inspect the configuration"}
	cmd.AddCommand(&cobra.Command{Use: "show", Short: "Show the effective configuration", Long: configShowLong, Args: cobra.NoArgs, RunE: action(func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		if jsonOutput(cmd) {
			return json.NewEncoder(cmd.OutOrStdout()).Encode(cfg)
		}
		enc := yaml.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent(2)
		return enc.Encode(cfg)
	}, nil)})
	return cmd
}

// workspaceFlag returns the --workspace flag, which defaults to ".".
func workspaceFlag(cmd *cobra.Command) (string, error) {
	return stringFlag(cmd, "workspace", "workspace must not be empty")
}

func addWorkspaceFlag(cmd *cobra.Command, usage string) {
	cmd.Flags().String("workspace", ".", usage)
}

// lookupWorkspace returns the container of the workspace selected by --workspace.
func lookupWorkspace(cmd *cobra.Command) (container.Record, error) {
	path, err := workspaceFlag(cmd)
	if err != nil {
		return container.Record{}, err
	}
	r, err := container.Lookup(path)
	if err != nil {
		return r, lookupError(path, err)
	}
	return r, nil
}

// lookupError explains a failed container lookup for the workspace at path.
func lookupError(path string, err error) error {
	if !os.IsNotExist(err) {
		return err
	}
	if _, statErr := os.Stat(path); statErr != nil {
		return fmt.Errorf("workspace %s does not exist", path)
	}
	return fmt.Errorf("no Isodev container for workspace %s; create one with: isodev up --workspace %s", path, path)
}

// upOptions reads the flags of up.
func upOptions(cmd *cobra.Command) (workspace.Options, error) {
	recreate, _ := cmd.Flags().GetBool("recreate")
	renew, _ := cmd.Flags().GetBool("renew-volumes")
	if renew && !recreate {
		return workspace.Options{}, fmt.Errorf("--renew-volumes requires --recreate")
	}
	return workspace.Options{Recreate: recreate, RenewVolumes: renew, Warn: cmd.ErrOrStderr()}, nil
}

func newContainerCommands() []*cobra.Command {
	up := &cobra.Command{
		Use: "up", Aliases: []string{"start"}, Short: "Create or start the workspace's container (alias: start)", Args: cobra.NoArgs,
		Long: upLong,
		RunE: action(func(cmd *cobra.Command, args []string) error {
			path, err := workspaceFlag(cmd)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "Preparing container for %s...\n", path)
			o, err := upOptions(cmd)
			if err != nil {
				return err
			}
			res, err := workspace.Up(path, o)
			if err != nil {
				if res.Worktree != nil {
					err = fmt.Errorf("%w; restore it with: isodev down --workspace %s", err, path)
				}
				return err
			}
			r := res.Container
			if jsonFlag, _ := cmd.Flags().GetBool("json"); jsonFlag {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(r)
			}
			return output(cmd, r, func() { printUp(cmd, res) })
		}, func(cmd *cobra.Command, args []string) error {
			path, err := workspaceFlag(cmd)
			if err != nil {
				return err
			}
			o, err := upOptions(cmd)
			if err != nil {
				return err
			}
			return workspace.PreviewUp(cmd.OutOrStdout(), path, o)
		}),
	}
	addWorkspaceFlag(up, "workspace directory")
	up.Flags().Bool("json", false, "output the container record as JSON")
	_ = up.Flags().MarkHidden("json")
	up.Flags().Bool("recreate", false, "replace the container and its private filesystem")
	up.Flags().Bool("renew-volumes", false, "with --recreate, empty the container's private volumes")

	down := &cobra.Command{
		Use: "down", Aliases: []string{"rm"}, Short: "Remove the workspace's container (alias: rm)", Args: cobra.NoArgs,
		Long: downLong,
		RunE: action(func(cmd *cobra.Command, args []string) error {
			path, err := workspaceFlag(cmd)
			if err != nil {
				return err
			}
			result, err := workspace.Down(path)
			if err != nil {
				return err
			}
			return output(cmd, result, func() { printDown(cmd, path, result) })
		}, func(cmd *cobra.Command, args []string) error {
			path, err := workspaceFlag(cmd)
			if err != nil {
				return err
			}
			return workspace.PreviewDown(cmd.OutOrStdout(), path)
		}),
	}
	addWorkspaceFlag(down, "workspace directory")

	stop := &cobra.Command{Use: "stop", Short: "Stop the workspace's container", Long: stopLong, Args: cobra.NoArgs, RunE: action(func(cmd *cobra.Command, args []string) error {
		path, err := workspaceFlag(cmd)
		if err != nil {
			return err
		}
		r, err := container.Stop(path)
		if err != nil {
			return lookupError(path, err)
		}
		return output(cmd, map[string]any{"workspace": r.Cwd, "container": r.Container, "stopped": true}, func() {
			printStatus(cmd.OutOrStdout(), "Container", r.Container, "Stopped")
		})
	}, func(cmd *cobra.Command, args []string) error {
		path, err := workspaceFlag(cmd)
		if err != nil {
			return err
		}
		return container.PreviewStop(cmd.OutOrStdout(), path)
	})}
	addWorkspaceFlag(stop, "workspace directory")

	execCmd := &cobra.Command{Use: "exec COMMAND [ARG...]", Short: "Run a command in the workspace's container", Long: execLong, Example: execExample, Args: cobra.MinimumNArgs(1), Annotations: map[string]string{streamsOutput: "true"}, RunE: action(func(cmd *cobra.Command, args []string) error {
		r, err := lookupWorkspace(cmd)
		if err != nil {
			return err
		}
		return container.Launch(r.Container, args, false)
	}, func(cmd *cobra.Command, args []string) error {
		r, err := lookupWorkspace(cmd)
		if err != nil {
			return err
		}
		return container.PreviewLaunch(cmd.OutOrStdout(), r.Container, args, false)
	})}
	addWorkspaceFlag(execCmd, "workspace directory (before COMMAND)")
	execCmd.Flags().SetInterspersed(false)

	enter := &cobra.Command{Use: "enter", Short: "Open a shell in the workspace's container", Long: enterLong, Args: cobra.NoArgs, Annotations: map[string]string{streamsOutput: "true"}, RunE: action(func(cmd *cobra.Command, args []string) error {
		r, err := lookupWorkspace(cmd)
		if err != nil {
			return err
		}
		return container.Enter(r)
	}, func(cmd *cobra.Command, args []string) error {
		r, err := lookupWorkspace(cmd)
		if err != nil {
			return err
		}
		container.PreviewEnter(cmd.OutOrStdout(), r)
		return nil
	})}
	addWorkspaceFlag(enter, "workspace directory")

	commands := []*cobra.Command{up, stop, down, execCmd, enter}
	for _, list := range []struct {
		name, short, long string
		all               bool
	}{
		{"ls", "List all Isodev containers", lsLong, true}, {"ps", "List running Isodev containers", psLong, false},
	} {
		all := list.all
		commands = append(commands, &cobra.Command{Use: list.name, Short: list.short, Long: list.long, Args: cobra.NoArgs, RunE: action(func(cmd *cobra.Command, args []string) error {
			containers, err := container.List(all)
			if err != nil {
				return err
			}
			if jsonOutput(cmd) {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(containers)
			}
			w := tabwriter.NewWriter(cmd.OutOrStdout(), 10, 1, 3, ' ', 0)
			fmt.Fprintln(w, "WORKSPACE\tSTATUS\tCONTAINER")
			for _, c := range containers {
				fmt.Fprintf(w, "%s\t%s\t%s\n", c.Workspace, c.Status, c.Name)
			}
			return w.Flush()
		}, func(cmd *cobra.Command, args []string) error {
			container.PreviewList(cmd.OutOrStdout(), all)
			return nil
		})})
	}
	return commands
}
