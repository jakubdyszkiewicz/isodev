---
title: Sandboxes
weight: 1
---

A sandbox limits what an agent can reach: which files it can write, which hosts it can connect to, and which credentials it can see. Isodev is not a sandbox. It gives each worktree its own environment, but a container shares the host kernel, can reach any network, and can read whatever you mount into it; see [Security](../../security).

If your main concern is containing an agent rather than giving it a full development environment, a sandbox is the better tool.

## MicroVM (e.g. Docker Sandboxes)

[Docker Sandboxes](https://docs.docker.com/ai/sandboxes/) run each agent in a microVM with its own kernel and its own Docker Engine, through the `sbx` command-line tool on macOS, Windows and Linux.

- **Stronger isolation.** A microVM doesn't share the host kernel, and the agent can run Docker without a privileged container.
- **Network and credential proxy.** Traffic goes through a proxy with allow and deny rules, and blocked destinations can be approved one at a time. You store a key with `sbx secret set`, and the proxy adds it to outbound requests, so the agent never sees it.

Isodev's [Docker-in-Docker](../../guides/docker) needs a privileged container. On Linux, [Sysbox](https://github.com/nestybox/sysbox) can run Docker in an unprivileged container, but it hasn't been tested with Isodev.

## Overhead

On Linux, an Isodev container's processes are ordinary host processes. Containers have no memory limit by default, take memory as they need it and give it back when they free it, share the host's kernel and file cache, and use the host filesystem directly. Ten idle containers cost almost nothing.

A microVM runs its own kernel and has a memory limit, by default half of host memory, which you set with `sbx run --memory`; too low and builds run out of memory. Memory the guest frees goes back to the host, but the guest's file cache stays in host memory until the host runs short ([sbx#828](https://github.com/docker/sbx-releases/issues/828)). The workspace is reached through a shared filesystem, which is slow for builds: one user measured a Maven build at 2.5 minutes on the host and 13 minutes in a sandbox ([sbx#31](https://github.com/docker/sbx-releases/issues/31)). That is the cost of stronger isolation.

## Sharing caches

Sandboxes keep each agent's caches separate, so a new sandbox downloads and builds everything again. Sharing one writable directory between microVMs isn't safe either: file locks don't work across VMs.

Isodev containers share the host kernel, so locks work across them and they can share [caches](../../guides/caches).

## Sharing agent configuration

Sandboxes also keep each agent's configuration separate ([sbx#113](https://github.com/docker/sbx-releases/issues/113)). That protects your host, but it costs:

- **Memory and history.** Agents keep project memory, session history and transcripts in their configuration directory. With a copy per sandbox, they're lost when it's removed.
- **Settings.** Skills, MCP servers, permissions and plugins have to be copied into each sandbox and kept in sync.
- **Tools on the host.** Paseo needs the agent's directory to show a conversation's full history, and resuming a session on the host needs the session files there.

The cost of sharing is the reverse: a container that shares the directory can read transcripts from every project and change settings, hooks and plugins that your host sessions load. See [Claude Code](../../guides/claude-code#trade-offs).

## Which to use

- **Docker Sandboxes,** when an agent runs unattended with access you don't want it to misuse, and you need network and credential controls.
- **Isodev,** when you run several agents or development environments in parallel and want each to have your full environment.
