---
title: Overview
weight: 1
---

Isodev gives each Git worktree its own Docker container, so you can run development environments and coding agents, such as Claude Code, Codex and OpenCode, side by side on the same repository. Each container has its own filesystem, network, processes and, if you want, its own Docker daemon, while the workspace files stay on the host.

A linked Git worktree points back to the main repository's `.git` directory, which isn't available inside the container. Isodev can [mount that directory, or replace the worktree's link](git-worktrees) with an independent repository and restore it when you're done.

Isodev runs on Linux and macOS hosts and requires Docker. It integrates with [Paseo](https://paseo.sh) and [Zed](https://zed.dev).

{{< callout type="warning" >}}
Isodev isolates workspaces from each other and from the host. It is **not** a sandbox for untrusted code; see [Security](security).
{{< /callout >}}

{{< cards >}}
  {{< card link="getting-started" title="Getting started" subtitle="Install isodev and create your first container." >}}
  {{< card link="guides" title="Guides" subtitle="Add agents, GitHub authentication, Docker-in-Docker, Tailscale and more, one at a time." >}}
  {{< card link="configuration" title="Configuration" subtitle="Every config.yaml field, with editor completion." >}}
  {{< card link="git-worktrees" title="Git worktrees" subtitle="Make Git work in a worktree's container." >}}
  {{< card link="integrations" title="Integrations" subtitle="Paseo and Zed." >}}
{{< /cards >}}
