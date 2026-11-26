---
title: Commands
weight: 4
---

Every command and flag, generated from the same definitions as `isodev --help`. For step-by-step setups, see [Getting started](../getting-started) and the [integration guides](../integrations).

<!-- BEGIN GENERATED REFERENCE: edit the command definitions and help texts in internal/cli and run `go generate ./...` -->

## `isodev`

Isodev gives each Git worktree its own Docker container, so you can run development environments and coding agents side by side on the same repository. Workspace files stay on the host; everything else in the container is private to it.

Workspace commands act on the current directory unless `--workspace` selects another one.

With `-o json`, lists print a JSON array that includes each `workspace`, and other commands print a JSON object. Commands that stream output, such as `exec` and `enter`, don't support JSON.

`--dry-run` previews the external commands and file changes a command would make, without running them. Values that are only known during execution are shown as placeholders. Put it before the subcommand, for example `isodev --dry-run up`.

Documentation: https://jakubdyszkiewicz.github.io/isodev/docs/

```text
isodev [flags]
```

**Global flags:**

- `--dry-run`: preview actions without making changes
- `-h, --help`: help for isodev
- `-o, --output string`: output format (text or json) (default `text`)
- `-v, --version`: version for isodev

## `isodev up`

Create the workspace's container, or start it if it is stopped. The workspace is mounted at `/workspaces/<directory-name>`; the Paseo and Zed integrations mount it at its host path instead.

Creating a container builds the configured image, then runs the `on_start` and `on_create` scripts. Starting a stopped container reruns the `on_start` scripts. A container that is already running is left as it is.

With `--recreate`, replace the container with one that uses the current configuration. Workspace files and private volumes stay, but the rest of the container's private filesystem is removed; add `--renew-volumes` to empty the private volumes too. The image is built before the old container is removed, so a failed build leaves it in place; if the new container's scripts fail, the old container cannot be recovered. Without an existing container, `--recreate` creates one.

If the workspace is a linked Git worktree, a new container follows `git_worktree.mode`: `mount-common-dir` also mounts the main repository's Git directory and the workspace at their host paths, and `replace` replaces the worktree's link first. A replaced worktree is restored first if the mode is set to something else. If the mode isn't set, `up` warns on stderr and Git won't work in the container.

```text
isodev up [flags]
```

**Aliases:** `start`

**Flags:**

- `--recreate`: replace the container and its private filesystem
- `--renew-volumes`: with --recreate, empty the container's private volumes
- `--workspace string`: workspace directory (default `.`)

## `isodev stop`

Stop the workspace's container and keep its private filesystem. `isodev up` starts it again and reruns the `on_start` scripts.

```text
isodev stop [flags]
```

**Flags:**

- `--workspace string`: workspace directory (default `.`)

## `isodev down`

Stop and remove the workspace's container, its network, its private filesystem and its private volumes. Workspace files and shared volumes stay.

If the workspace is a Git worktree whose link was replaced, restore it, as `isodev int git-worktree restore` does. This happens even if the workspace has no container.

```text
isodev down [flags]
```

**Aliases:** `rm`

**Flags:**

- `--workspace string`: workspace directory (default `.`)

## `isodev exec`

Run a command in the workspace's container, attached to this terminal's input and output. Put `--workspace` before the command; everything after the command is passed to it unchanged. Isodev exits with the command's exit status, as `docker exec` does.

Environment variables listed in `lifecycle.on_exec.forward_env_vars` are forwarded. The command runs in its own process group, so interrupting `isodev` also stops its child processes.

```text
isodev exec COMMAND [ARG...] [flags]
```

**Flags:**

- `--workspace string`: workspace directory (before COMMAND) (default `.`)

**Examples:**

```sh
isodev exec go test ./...
isodev exec --workspace ~/src/other sh -c 'echo $HOME'
```

## `isodev enter`

Open an interactive shell in the workspace's container: Fish if it is installed, otherwise Bash or `sh`.

```text
isodev enter [flags]
```

**Flags:**

- `--workspace string`: workspace directory (default `.`)

## `isodev ls`

List every Isodev container, running or stopped, with its workspace, status and name.

```text
isodev ls
```

## `isodev ps`

List running Isodev containers with their workspace, status and name.

```text
isodev ps
```

## `isodev config`

Inspect the configuration

**Commands:**

- [`show`](#isodev-config-show): Show the effective configuration

### `isodev config show`

Print the configuration from `~/.config/isodev/config.yaml` after applying defaults, resolving paths and applying the `ISODEV_IMAGE` override. The output includes environment values.

Every field is described in the configuration reference: https://jakubdyszkiewicz.github.io/isodev/docs/configuration/

```text
isodev config show
```

## `isodev integration`

Optional integrations with Git worktrees, Paseo and Zed. The container commands work without them.

**Aliases:** `int`

**Commands:**

- [`git-worktree`](#isodev-integration-git-worktree): Replace a worktree's link with an independent repository, and restore it
- [`paseo`](#isodev-integration-paseo): Paseo worktree hooks and agent providers
- [`zed`](#isodev-integration-zed): Zed worktree task and ACP agents

### `isodev integration git-worktree`

A linked worktree's `.git` file points into the main repository's Git directory, which a container that only mounts the worktree doesn't have. With `git_worktree.mode: replace`, `isodev up` and the Paseo and Zed setup replace the link with an independent repository, and `isodev down` and the teardowns restore it. These commands do the same by hand.

**Commands:**

- [`replace`](#isodev-integration-git-worktree-replace): Replace a worktree's link with an independent repository
- [`restore`](#isodev-integration-git-worktree-restore): Restore a replaced worktree's link

### `isodev integration git-worktree replace`

Replace a linked Git worktree's `.git` file with an independent repository. The workspace files, current branch and commit stay in place. New commits are stored in the worktree's own repository, and the main checkout gets a remote, `isodev-NAME`, to fetch them from. `NAME` is the worktree's name in the main repository, the directory under `.git/worktrees/`.

The worktree must be on a branch and have no staged or unstaged tracked changes. Untracked files are kept. The main checkout's cached `origin/*` refs and the branch's upstream are copied without contacting the remote; run `git fetch origin` to refresh them. The original worktree registration stays in place so the link can be restored.

```text
isodev integration git-worktree replace [flags]
```

**Flags:**

- `--source-path string`: main checkout the worktree belongs to (default: found from the worktree)
- `--worktree-path string`: path to the worktree (required)

**Examples:**

```sh
isodev int git-worktree replace --worktree-path /path/to/worktree
```

### `isodev integration git-worktree restore`

Restore a replaced worktree's link. The workspace files stay in place, and the independent repository is saved in a new directory in `.isodev-git-archive/` in the workspace.

First, the worktree's branches are fetched into `isodev-NAME/BRANCH` refs in the main checkout, and its HEAD into `isodev-NAME/HEAD`. If the worktree is still on its original branch, or still detached, and its new commits extend the original commit, the restored worktree is fast-forwarded to them and keeps its staged changes. Otherwise the branch stays where it was and a warning says so; the commits are still in those refs. The `isodev-NAME` remote is then removed, and its refs stay.

The original worktree registration must still exist, and so must Isodev's record of the replacement in `~/.local/state/isodev/worktrees/`. `.isodev-git-archive` is added to the repository's `info/exclude`.

```text
isodev integration git-worktree restore [flags]
```

**Flags:**

- `--worktree-path string`: path to the worktree (required)

**Examples:**

```sh
isodev int git-worktree restore --worktree-path /path/to/worktree
```

### `isodev integration paseo`

Worktree hooks and agent providers for Paseo (https://paseo.sh). Add the `setup` and `teardown` hooks to a project's `paseo.json`, and install a provider for each agent you use. The `isodev` binary must be on Paseo's `PATH`.

**Commands:**

- [`setup`](#isodev-integration-paseo-setup): Start a Paseo worktree's container
- [`teardown`](#isodev-integration-paseo-teardown): Remove a Paseo worktree's container and restore the worktree
- [`exec`](#isodev-integration-paseo-exec): Run an agent in a Paseo workspace's container
- [`install-provider`](#isodev-integration-paseo-install-provider): Install a Paseo agent provider
- [`enter`](#isodev-integration-paseo-enter): Open a shell in the current workspace's container

### `isodev integration paseo setup`

Paseo worktree setup hook; add it to `paseo.json` as `worktree.setup`.

Start the container of the worktree in `PASEO_WORKTREE_PATH`, with the workspace mounted at the same path as on the host, as `isodev up` does. `git_worktree.mode` must be set; with `replace`, the worktree is replaced from `PASEO_SOURCE_CHECKOUT_PATH` (or `PASEO_ROOT_PATH`).

```text
isodev integration paseo setup
```

### `isodev integration paseo teardown`

Paseo worktree teardown hook; add it to `paseo.json` as `worktree.teardown`.

Remove the container of the worktree in `PASEO_WORKTREE_PATH` and, if its link was replaced, restore it so Paseo can archive the worktree, as `isodev down` does. The worktree is fast-forwarded to its new commits when possible, and they are fetched into `isodev-NAME/*` refs in the main checkout; see `isodev int git-worktree restore`.

```text
isodev integration paseo teardown
```

### `isodev integration paseo exec`

Run an agent for an Isodev provider. Paseo runs this command; you don't run it yourself.

The command runs in the container of the workspace in `PASEO_AGENT_CWD`, waiting for the setup hook to finish if needed. Without `PASEO_AGENT_CWD`, Paseo is probing the provider, and a separate probe container is used. `PASEO_AGENT_ID` and `PASEO_AGENT_CWD` are forwarded along with `lifecycle.on_exec.forward_env_vars`.

```text
isodev integration paseo exec COMMAND [ARG...]
```

### `isodev integration paseo install-provider`

Add or update an Isodev provider in `~/.paseo/config.json`, backing up the file first. Run `paseo reload` afterwards and choose the provider in Paseo.

Providers run the agent in the workspace's container, with Paseo agent tools disabled. The container image must include the agent and its credentials.

```text
isodev integration paseo install-provider [flags]
```

**Flags:**

- `--type string`: provider type (codex, claude, or opencode-acp)

**Examples:**

```sh
isodev int paseo install-provider --type=claude
isodev int paseo install-provider --type=codex
isodev int paseo install-provider --type=opencode-acp
```

### `isodev integration paseo enter`

Open a shell in the container of the workspace that contains the current directory, for example from a Paseo workspace terminal.

```text
isodev integration paseo enter
```

### `isodev integration zed`

A worktree setup task and ACP agents for Zed (https://zed.dev).

**Commands:**

- [`setup`](#isodev-integration-zed-setup): Prepare a Zed worktree and its container
- [`teardown`](#isodev-integration-zed-teardown): Remove a Zed worktree's container and restore the worktree
- [`acp`](#isodev-integration-zed-acp): Run an ACP agent in the workspace's container
- [`install-provider`](#isodev-integration-zed-install-provider): Install a Zed ACP agent
- [`install-task`](#isodev-integration-zed-install-task): Install the Zed worktree setup task

### `isodev integration zed setup`

Start a Zed worktree's container with the workspace at the same path as on the host, as `isodev up` does. `git_worktree.mode` must be set; with `replace`, the worktree is replaced from the main checkout, preserving a detached HEAD.

Run it from the task that `isodev int zed install-task` installs; without flags it reads Zed's task variables.

```text
isodev integration zed setup [flags]
```

**Flags:**

- `--source string`: main checkout path (or ZED_MAIN_GIT_WORKTREE)
- `--workspace string`: worktree path (or ZED_WORKTREE_ROOT)

### `isodev integration zed teardown`

Remove the container and, if the worktree's link was replaced, restore it, as `isodev down` does. Run this before Zed archives or deletes the worktree.

The worktree is fast-forwarded to its new commits when possible, and they are fetched into `isodev-NAME/*` refs in the main checkout; see `isodev int git-worktree restore`.

```text
isodev integration zed teardown [flags]
```

**Flags:**

- `--workspace string`: worktree path (or ZED_WORKTREE_ROOT)

### `isodev integration zed acp`

Bridge Zed to OpenCode or Codex ACP in the running container of the workspace that contains `--workspace`. Zed runs this command; configure it with `isodev int zed install-provider`.

The container must mount the workspace at its host path, as `isodev int zed setup` does. Sessions outside the workspace are rejected, and Zed's host-side terminal and file capabilities are disabled.

```text
isodev integration zed acp [flags]
```

**Flags:**

- `--agent string`: ACP agent (opencode or codex) (default `opencode`)
- `--workspace string`: workspace directory containing the running container (default `.`)

### `isodev integration zed install-provider`

Add or update an Isodev ACP agent server in Zed's `settings.json` on this machine, preserving comments and other settings and backing up the file.

For SSH projects, the Zed application and the remote server have separate settings; use `--path` to choose the file Zed reads.

```text
isodev integration zed install-provider [flags]
```

**Flags:**

- `--path string`: Zed settings file (default: this machine's global settings)
- `--type string`: provider type (opencode-acp or codex)

### `isodev integration zed install-task`

Add or update the manually triggered "Prepare isodev worktree" task in Zed's global `tasks.json` on this machine, preserving comments and other tasks and backing up the file. Run it from Zed's **Task: Spawn** after opening a worktree.

```text
isodev integration zed install-task [flags]
```

**Flags:**

- `--path string`: Zed tasks file (default: this machine's global tasks)

<!-- END GENERATED REFERENCE -->
