---
title: Security
weight: 8
---

Isodev isolates workspaces from each other and from the host. It is **not** a sandbox for untrusted code.

- **Kernel:** containers share the host kernel. With `privileged: true`, for example for Docker-in-Docker, a container has broad access to the host.
- **Mounts:** every configured mount is visible to the container, read-only or not. Mounting credentials or `~/.claude` gives the agent those credentials and that data.
- **Git worktrees:** with `git_worktree.mode: mount-common-dir`, the container can write to the main repository's `.git`, including hooks that later run on the host; see [Git worktrees](../git-worktrees#mount-common-dir). With `replace`, it only has the worktree's own repository.
- **Workspace files run on the host too:** the container can change everything in the workspace, including its `.git`. Git on the host in that workspace, from your shell, your editor or Paseo, runs the configuration and hooks it finds there, such as `core.fsmonitor`, with your access. This applies in every `git_worktree` mode; with `replace`, the worktree's repository is in the workspace. The same is true of other files that host tools run, such as `.envrc` or build scripts.
- **Network:** each container gets its own Docker bridge network with ordinary outbound access.
- **Host-side tools:** Paseo terminals, file browsing and Git operations, and Zed's own terminal and editor processes, still run on the host. For its providers, Isodev disables Paseo agent tools and the host-side ACP terminal and file capabilities, so agent commands and file edits happen in the container. Tools that run on the host, such as MCP servers configured in your editor, keep their own access.
- **Forwarded environment:** variables in `lifecycle.on_exec.forward_env_vars` are passed to every command Isodev runs in a container. Forward secrets only to containers and commands you trust.
