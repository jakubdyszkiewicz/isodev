package cli

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/jakubdyszkiewicz/isodev/internal/integration/gitworktree"
	"github.com/jakubdyszkiewicz/isodev/internal/integration/paseo"
	"github.com/jakubdyszkiewicz/isodev/internal/integration/zed"
	"github.com/jakubdyszkiewicz/isodev/internal/workspace"
)

var streams = map[string]string{streamsOutput: "true"}

func newIntegrationCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "integration", Aliases: []string{"int"}, Short: "Integrations with Git worktrees, Paseo and Zed (alias: int)", Long: integrationLong}
	cmd.AddCommand(newWorktreeCommand(), newPaseoCommand(), newZedCommand())
	return cmd
}

func newWorktreeCommand() *cobra.Command {
	replace := &cobra.Command{
		Use: "replace", Short: "Replace a worktree's link with an independent repository", Args: cobra.NoArgs,
		Long:    replaceLong,
		Example: "  isodev int git-worktree replace --worktree-path /path/to/worktree",
		RunE: action(func(cmd *cobra.Command, args []string) error {
			path, source, err := replaceFlags(cmd)
			if err != nil {
				return err
			}
			result, err := gitworktree.Replace(path, source, false)
			if err != nil {
				return err
			}
			return output(cmd, result, func() { printReplaced(cmd, path, result) })
		}, func(cmd *cobra.Command, args []string) error {
			path, source, err := replaceFlags(cmd)
			if err != nil {
				return err
			}
			gitworktree.PreviewReplace(cmd.OutOrStdout(), path, source, false)
			return nil
		}),
	}
	replace.Flags().String("worktree-path", "", "path to the worktree (required)")
	replace.Flags().String("source-path", "", "main checkout the worktree belongs to (default: found from the worktree)")

	restore := &cobra.Command{
		Use: "restore", Short: "Restore a replaced worktree's link", Args: cobra.NoArgs,
		Long:    restoreLong,
		Example: "  isodev int git-worktree restore --worktree-path /path/to/worktree",
		RunE: action(func(cmd *cobra.Command, args []string) error {
			path, err := stringFlag(cmd, "worktree-path", "restore requires --worktree-path")
			if err != nil {
				return err
			}
			result, err := gitworktree.Restore(path)
			if err != nil {
				return err
			}
			return output(cmd, result, func() { printRestored(cmd, path, result) })
		}, func(cmd *cobra.Command, args []string) error {
			path, err := stringFlag(cmd, "worktree-path", "restore requires --worktree-path")
			if err != nil {
				return err
			}
			gitworktree.PreviewRestore(cmd.OutOrStdout(), path)
			return nil
		}),
	}
	restore.Flags().String("worktree-path", "", "path to the worktree (required)")

	cmd := &cobra.Command{Use: "git-worktree", Short: "Replace a worktree's link with an independent repository, and restore it", Long: gitWorktreeLong}
	cmd.AddCommand(replace, restore)
	return cmd
}

// printStatus writes a status line in the form Docker Compose uses, such as
// "Container isodev-1a2b  Started".
func printStatus(w io.Writer, kind, name, status string) {
	fmt.Fprintf(w, "%s %s  %s\n", kind, name, status)
}

// warn writes a warning to stderr, prefixed as Docker does.
func warn(cmd *cobra.Command, format string, args ...any) {
	fmt.Fprintf(cmd.ErrOrStderr(), "WARNING: "+format+"\n", args...)
}

// printReplaced reports a replaced worktree link.
func printReplaced(cmd *cobra.Command, path string, result map[string]any) {
	w := cmd.OutOrStdout()
	printStatus(w, "Worktree", path, "Replaced")
	if remote, ok := result["remote"].(string); ok && remote != "" {
		fmt.Fprintf(w, "Fetch its commits in the main checkout with: git fetch %s\n", remote)
	}
}

// printRestored reports a restored worktree link and what happened to its commits.
func printRestored(cmd *cobra.Command, path string, result map[string]any) {
	printStatus(cmd.OutOrStdout(), "Worktree", path, "Restored")
	if result["fastForwarded"] == true {
		head, _ := result["head"].(string)
		if len(head) > 12 {
			head = head[:12]
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Fast-forwarded to %s\n", head)
	}
	if warning, ok := result["warning"].(string); ok {
		warn(cmd, "%s", warning)
	}
}

// printUp reports what up or a setup did to the worktree and the container.
func printUp(cmd *cobra.Command, res workspace.Result) {
	if res.Restored != nil {
		printRestored(cmd, res.Container.Cwd, res.Restored)
	}
	if res.Worktree != nil {
		printReplaced(cmd, res.Container.Cwd, res.Worktree)
	}
	printStatus(cmd.OutOrStdout(), "Container", res.Container.Container, res.Action)
}

// printDown reports what down or a teardown removed and restored.
func printDown(cmd *cobra.Command, path string, result map[string]any) {
	name, _ := result["container"].(string)
	wt, restored := result["worktree"].(map[string]any)
	switch {
	case result["removed"] == true:
		printStatus(cmd.OutOrStdout(), "Container", name, "Removed")
	case !restored:
		warn(cmd, "No Isodev container for %s", path)
	}
	if restored {
		printRestored(cmd, path, wt)
	}
}

func replaceFlags(cmd *cobra.Command) (string, string, error) {
	path, err := stringFlag(cmd, "worktree-path", "replace requires --worktree-path")
	if err != nil {
		return "", "", err
	}
	if source, _ := cmd.Flags().GetString("source-path"); source != "" {
		return path, source, nil
	}
	st, err := gitworktree.Detect(path)
	if err != nil {
		return "", "", err
	}
	if st.Kind != gitworktree.Linked {
		return "", "", fmt.Errorf("%s is not a linked Git worktree; pass --source-path", path)
	}
	return path, gitworktree.SourceCheckout(st.CommonDir), nil
}

func newPaseoCommand() *cobra.Command {
	setup := &cobra.Command{Use: "setup", Short: "Start a Paseo worktree's container", Long: paseoSetupLong, Args: cobra.NoArgs, RunE: action(func(cmd *cobra.Command, args []string) error {
		path, source, err := paseo.SetupPaths()
		if err != nil {
			return err
		}
		result, err := paseo.Setup(path, source)
		if err != nil {
			return err
		}
		return output(cmd, result, func() { printUp(cmd, result) })
	}, func(cmd *cobra.Command, args []string) error {
		path, source, err := paseo.SetupPaths()
		if err != nil {
			return err
		}
		return paseo.PreviewSetup(cmd.OutOrStdout(), path, source)
	})}

	teardown := &cobra.Command{Use: "teardown", Short: "Remove a Paseo worktree's container and restore the worktree", Long: paseoTeardownLong, Args: cobra.NoArgs, RunE: action(func(cmd *cobra.Command, args []string) error {
		path, err := paseo.TeardownPath()
		if err != nil {
			return err
		}
		result, err := workspace.Down(path)
		if err != nil {
			return err
		}
		return output(cmd, result, func() { printDown(cmd, path, result) })
	}, func(cmd *cobra.Command, args []string) error {
		path, err := paseo.TeardownPath()
		if err != nil {
			return err
		}
		return workspace.PreviewDown(cmd.OutOrStdout(), path)
	})}

	execCmd := &cobra.Command{Use: "exec COMMAND [ARG...]", Short: "Run an agent in a Paseo workspace's container", Long: paseoExecLong, Args: cobra.MinimumNArgs(1), Annotations: streams, RunE: action(func(cmd *cobra.Command, args []string) error {
		return paseo.Exec(args)
	}, func(cmd *cobra.Command, args []string) error {
		return paseo.PreviewExec(cmd.OutOrStdout(), args)
	})}
	execCmd.Flags().SetInterspersed(false)

	install := &cobra.Command{Use: "install-provider", Short: "Install a Paseo agent provider", Long: paseoInstallLong, Example: paseoInstallExample, Args: cobra.NoArgs, RunE: action(func(cmd *cobra.Command, args []string) error {
		t, _ := cmd.Flags().GetString("type")
		if err := paseo.InstallProvider(t); err != nil {
			return err
		}
		return output(cmd, map[string]any{"installed": true, "provider": t}, func() {
			printStatus(cmd.OutOrStdout(), "Paseo provider", "isodev-"+t, "Installed")
			fmt.Fprintln(cmd.OutOrStdout(), "Apply it with: paseo reload")
		})
	}, func(cmd *cobra.Command, args []string) error {
		t, _ := cmd.Flags().GetString("type")
		return paseo.PreviewInstallProvider(cmd.OutOrStdout(), t)
	})}
	install.Flags().String("type", "", "provider type (codex, claude, or opencode-acp)")

	enter := &cobra.Command{Use: "enter", Short: "Open a shell in the current workspace's container", Long: paseoEnterLong, Args: cobra.NoArgs, Annotations: streams, RunE: action(func(cmd *cobra.Command, args []string) error {
		return paseo.Enter()
	}, func(cmd *cobra.Command, args []string) error {
		return paseo.PreviewEnter(cmd.OutOrStdout())
	})}

	cmd := &cobra.Command{Use: "paseo", Short: "Paseo worktree hooks and agent providers", Long: paseoLong}
	cmd.AddCommand(setup, teardown, execCmd, install, enter)
	return cmd
}

// zedPaths reads the worktree, and the source checkout if needed, from flags
// or the variables Zed sets for tasks.
func zedPaths(cmd *cobra.Command, needsSource bool) (string, string, error) {
	path, _ := cmd.Flags().GetString("workspace")
	if path == "" {
		path = os.Getenv("ZED_WORKTREE_ROOT")
	}
	if path == "" {
		return "", "", fmt.Errorf("zed %s requires --workspace or ZED_WORKTREE_ROOT", cmd.Name())
	}
	if !needsSource {
		return path, "", nil
	}
	source, _ := cmd.Flags().GetString("source")
	if source == "" {
		source = os.Getenv("ZED_MAIN_GIT_WORKTREE")
	}
	if source == "" {
		return "", "", fmt.Errorf("zed setup requires --source or ZED_MAIN_GIT_WORKTREE")
	}
	return path, source, nil
}

// zedConfigFlag returns --path, defaulting to the global Zed file name.
func zedConfigFlag(cmd *cobra.Command, name string) (string, error) {
	if path, _ := cmd.Flags().GetString("path"); path != "" {
		return path, nil
	}
	return zed.ConfigPath(name)
}

func newZedCommand() *cobra.Command {
	setup := &cobra.Command{Use: "setup", Short: "Prepare a Zed worktree and its container", Long: zedSetupLong, Args: cobra.NoArgs, RunE: action(func(cmd *cobra.Command, args []string) error {
		path, source, err := zedPaths(cmd, true)
		if err != nil {
			return err
		}
		result, err := zed.Setup(path, source)
		if err != nil {
			return err
		}
		return output(cmd, result, func() { printUp(cmd, result) })
	}, func(cmd *cobra.Command, args []string) error {
		path, source, err := zedPaths(cmd, true)
		if err != nil {
			return err
		}
		return zed.PreviewSetup(cmd.OutOrStdout(), path, source)
	})}
	setup.Flags().String("workspace", "", "worktree path (or ZED_WORKTREE_ROOT)")
	setup.Flags().String("source", "", "main checkout path (or ZED_MAIN_GIT_WORKTREE)")

	teardown := &cobra.Command{Use: "teardown", Short: "Remove a Zed worktree's container and restore the worktree", Long: zedTeardownLong, Args: cobra.NoArgs, RunE: action(func(cmd *cobra.Command, args []string) error {
		path, _, err := zedPaths(cmd, false)
		if err != nil {
			return err
		}
		result, err := zed.Teardown(path)
		if err != nil {
			return err
		}
		return output(cmd, result, func() { printDown(cmd, path, result) })
	}, func(cmd *cobra.Command, args []string) error {
		path, _, err := zedPaths(cmd, false)
		if err != nil {
			return err
		}
		return workspace.PreviewDown(cmd.OutOrStdout(), path)
	})}
	teardown.Flags().String("workspace", "", "worktree path (or ZED_WORKTREE_ROOT)")

	acp := &cobra.Command{Use: "acp", Short: "Run an ACP agent in the workspace's container", Long: zedACPLong, Args: cobra.NoArgs, Annotations: streams, RunE: action(func(cmd *cobra.Command, args []string) error {
		path, err := workspaceFlag(cmd)
		if err != nil {
			return err
		}
		agent, _ := cmd.Flags().GetString("agent")
		return zed.Bridge(path, agent)
	}, func(cmd *cobra.Command, args []string) error {
		path, _ := cmd.Flags().GetString("workspace")
		agent, _ := cmd.Flags().GetString("agent")
		return zed.PreviewACP(cmd.OutOrStdout(), path, agent)
	})}
	addWorkspaceFlag(acp, "workspace directory containing the running container")
	acp.Flags().String("agent", "opencode", "ACP agent (opencode or codex)")

	installProvider := &cobra.Command{Use: "install-provider", Short: "Install a Zed ACP agent", Long: zedInstallProviderLong, Args: cobra.NoArgs, RunE: action(func(cmd *cobra.Command, args []string) error {
		agent, _ := cmd.Flags().GetString("type")
		if _, err := zed.ACPCommand(agent); err != nil {
			return err
		}
		path, err := zedConfigFlag(cmd, "settings.json")
		if err != nil {
			return err
		}
		exe, err := os.Executable()
		if err != nil {
			return err
		}
		if err = zed.InstallProvider(path, agent, exe); err != nil {
			return err
		}
		return output(cmd, map[string]any{"installed": true, "provider": agent, "path": path}, func() {
			printStatus(cmd.OutOrStdout(), "Zed provider", agent, "Installed in "+path)
		})
	}, func(cmd *cobra.Command, args []string) error {
		agent, _ := cmd.Flags().GetString("type")
		if _, err := zed.ACPCommand(agent); err != nil {
			return err
		}
		path, err := zedConfigFlag(cmd, "settings.json")
		if err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "# Back up and update %s with the isodev Zed provider\n", path)
		return nil
	})}
	installProvider.Flags().String("type", "", "provider type (opencode-acp or codex)")
	installProvider.Flags().String("path", "", "Zed settings file (default: this machine's global settings)")

	installTask := &cobra.Command{Use: "install-task", Short: "Install the Zed worktree setup task", Long: zedInstallTaskLong, Args: cobra.NoArgs, RunE: action(func(cmd *cobra.Command, args []string) error {
		path, err := zedConfigFlag(cmd, "tasks.json")
		if err != nil {
			return err
		}
		if err := zed.InstallTask(path); err != nil {
			return err
		}
		return output(cmd, map[string]any{"installed": true, "path": path}, func() {
			printStatus(cmd.OutOrStdout(), "Zed task", "worktree setup", "Installed in "+path)
		})
	}, func(cmd *cobra.Command, args []string) error {
		path, err := zedConfigFlag(cmd, "tasks.json")
		if err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "# Back up and update %s with the isodev Zed task\n", path)
		return nil
	})}
	installTask.Flags().String("path", "", "Zed tasks file (default: this machine's global tasks)")

	cmd := &cobra.Command{Use: "zed", Short: "Zed worktree task and ACP agents", Long: zedLong}
	cmd.AddCommand(setup, teardown, acp, installProvider, installTask)
	return cmd
}
