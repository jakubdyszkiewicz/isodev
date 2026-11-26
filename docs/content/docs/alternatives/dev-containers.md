---
title: Dev Containers
weight: 2
---

[Dev Containers](https://containers.dev/) are the closest alternative to Isodev, which started as a wrapper around them before switching to plain Docker. Both run a project in a Docker container with the workspace mounted. The Dev Container specification describes a project's environment in a `devcontainer.json` file, and is implemented by several IDEs and cloud development environments.

A development environment combines two things:

- **Project requirements,** such as Go 1.24 or jq.
- **Developer preferences,** such as Claude Code, or Vim with your dotfiles.

Dev Containers handle project requirements well. For preferences they offer Features and dotfiles, but there is **no overlay or merging** of configurations ([spec#22](https://github.com/devcontainers/spec/issues/22), [#3279](https://github.com/microsoft/vscode-remote-release/issues/3279), [#8436](https://github.com/microsoft/vscode-remote-release/issues/8436)), so personal setup ends up in the shared file or isn't possible:

- Personal mounts, such as secrets for Claude Code, fail for teammates who don't have those files ([#5322](https://github.com/microsoft/vscode-remote-release/issues/5322), [spec#132](https://github.com/devcontainers/spec/issues/132)).
- Docker networks can only be set in the project's `runArgs` or Compose file, so you can't change them for yourself. A network setting ([#4272](https://github.com/microsoft/vscode-remote-release/issues/4272)) and a CLI flag ([cli#274](https://github.com/devcontainers/cli/pull/274)) were rejected, and connecting two projects' containers is still an open question ([spec#538](https://github.com/devcontainers/spec/issues/538)).
- GPUs and other `runArgs` are static and shared ([#3972](https://github.com/microsoft/vscode-remote-release/issues/3972)).
- Machine-specific setup, such as a corporate CA certificate, has no place to go ([#6092](https://github.com/microsoft/vscode-remote-release/issues/6092)).
- Features must be published to a registry or committed to each repository's `.devcontainer/` folder; you can't keep a personal Feature on your machine.
- Editor-only workarounds, such as VS Code's `dev.containers.defaultFeatures`, aren't part of the specification and have leaked into projects ([#11616](https://github.com/microsoft/vscode-remote-release/issues/11616)).

The project also picks the image, so your preferences may not work in it.

Isodev takes the other side: it handles developer preferences and leaves project requirements to tools such as [mise](https://mise.jdx.dev). A well-crafted mise configuration runs the project both inside and outside a container.

## Simplicity

Isodev is configured **per user**, not per repository, which keeps it simple: one YAML file with a schema, a standard Dockerfile and shell scripts. That makes it easy for a coding agent to customize, and easy to keep in your dotfiles.

Dev Containers have six lifecycle hooks: `initializeCommand` on the host, then `onCreateCommand`, `updateContentCommand`, `postCreateCommand`, `postStartCommand` and `postAttachCommand` in the container. Isodev has two, each a list of scripts that run in the container: `on_create` when it's created, and `on_start` whenever it starts.

## Networking

A Dev Container built from an image or Dockerfile joins Docker's default bridge network, where containers can reach each other. Only the project's `runArgs` can change that (see above). Isodev creates a network for each container, so containers can't reach each other.

Dev Containers forward ports to the host: the ones listed in `forwardPorts`, and in VS Code, the ones it detects. Forwarding is done by the editor; the `devcontainer` CLI doesn't forward ports ([cli#22](https://github.com/devcontainers/cli/issues/22)). Isodev by design doesn't forward or publish any ports, so servers in different worktrees can use the same port without clashing. To reach them from your devices, use container IPs or [Tailscale](../../guides/tailscale), which gives each container its own address on your tailnet.

## Dependencies

Isodev is a single binary with no runtime dependencies besides Docker. The reference implementation of Dev Containers, the [`devcontainer` CLI](https://github.com/devcontainers/cli), is a Node.js program and needs Node.js 20 or later.

## ADE integrations

Isodev integrates with agentic development environments (ADEs), [Paseo](../../integrations/paseo) and [Zed](../../integrations/zed), which start containers automatically, and configures them to run coding agents inside the container.

## Which to use

- **Dev Containers,** when a team shares a project's environment, or you use an editor, Codespaces or another service that builds on the specification.
- **Isodev,** when you want the same personal environment in every project, run agents in many worktrees in parallel, or work in repositories that don't have a Dev Container configuration.
