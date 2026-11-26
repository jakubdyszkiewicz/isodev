---
title: Guides
weight: 3
---

Each guide adds one feature to your containers and comes with a complete, tested configuration that you can copy as it is. The files live in the repository's [`examples/guides`](https://github.com/jakubdyszkiewicz/isodev/tree/main/examples/guides) directory.

## Agents

{{< cards >}}
  {{< card link="claude-code" title="Claude Code" subtitle="Your host login, settings and transcripts." >}}
  {{< card link="codex" title="Codex" subtitle="Your host login and sessions." >}}
  {{< card link="opencode" title="OpenCode" subtitle="Your host credentials." >}}
{{< /cards >}}

## Git and GitHub

{{< cards >}}
  {{< card link="github" title="GitHub authentication" subtitle="gh and git push with a token you control." >}}
  {{< card link="git-config" title="Git config" subtitle="Your host Git config: identity, settings and global ignore file." >}}
{{< /cards >}}

## Tools

{{< cards >}}
  {{< card link="docker" title="Docker-in-Docker" subtitle="A private Docker daemon per container." >}}
  {{< card link="docker-hub-cache" title="Docker Hub cache" subtitle="Pull Docker Hub images through a cache on the host." >}}
  {{< card link="mise" title="Project toolchains" subtitle="Each project's tools, installed with mise." >}}
  {{< card link="caches" title="Shared caches" subtitle="Download and build caches shared by all containers." >}}
  {{< card link="dotfiles" title="Shell and dotfiles" subtitle="Your shell, its configuration and tools, from your dotfiles." >}}
  {{< card link="host-tools" title="Host tools" subtitle="A command-line tool and its credentials from the host." >}}
{{< /cards >}}

## Network

{{< cards >}}
  {{< card link="tailscale" title="Tailscale" subtitle="Each container on your tailnet." >}}
{{< /cards >}}

## Use a guide

Each guide's configuration replaces `~/.config/isodev`. Back up your current configuration, then copy the guide's directory from a clone of the repository, for example:

```sh
cp -r examples/guides/claude-code/. ~/.config/isodev/
isodev config show
isodev up --recreate
```

Containers run as `root`, so `~` in the container is `/root`.

## Tools come from mise

Every guide's image installs [mise](https://mise.jdx.dev) and uses it for tools, such as Claude Code, Codex, OpenCode, `gh` and Tailscale. mise installs prebuilt releases of thousands of tools, pins their versions, and puts them on `PATH` through its shims. To add a tool, add a step to the Dockerfile:

```dockerfile
RUN mise use --global ripgrep
```

`mise use --global TOOL@VERSION` pins a version; without one, each image build gets the latest. Only system packages, such as Docker Engine, come from `apt`.

## Combine guides

All guides start from the same Ubuntu image, so you can merge several into one configuration:

- **`image/Dockerfile`:** append each guide's steps after the shared first lines, which install mise. Several `mise use --global` steps can become one, such as `RUN mise use --global claude-code gh`.
- **`config.yaml`:** merge the fields. Add `volumes` and `forward_env_vars` entries to your lists, and `environment` entries to your map; `privileged: true` is needed if any guide sets it.
- **Scripts:** copy each guide's `hooks/` scripts, and add an entry for each to the `scripts` list of `on_create` or `on_start`. Scripts run in the order they're listed.

For every field, see the [configuration reference](../configuration).
