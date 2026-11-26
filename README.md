# Isodev

Isodev gives each Git worktree its own Docker container, so you can run development environments and coding agents, such as Claude Code, Codex and OpenCode, side by side on the same repository. Each container has its own filesystem, network, processes and, if you want, its own Docker daemon, while the workspace files stay on the host.

It makes Git work in linked worktrees inside the container, and integrates with [Paseo](https://paseo.sh) and [Zed](https://zed.dev). Isodev runs on Linux and macOS and requires Docker.

**📖 Documentation: https://jakubdyszkiewicz.github.io/isodev/**

> [!WARNING]
> Isodev isolates workspaces from each other and from the host. It is not a sandbox for untrusted code; see [Security](https://jakubdyszkiewicz.github.io/isodev/docs/security/).

## Install

Download an archive from [GitHub Releases](https://github.com/jakubdyszkiewicz/isodev/releases), or install with Go 1.24 or later:

```sh
go install github.com/jakubdyszkiewicz/isodev@latest
```

## Quick start

```sh
cd ~/src/my-project
isodev up        # Create and start the container
isodev enter     # Open a shell in it
isodev down      # Remove the container; workspace files stay
```

The default image is plain `ubuntu:24.04`. To add agents, credentials, Docker-in-Docker and more, follow the [guides](https://jakubdyszkiewicz.github.io/isodev/docs/guides/).

## Contributing

See [DEVELOPMENT.md](DEVELOPMENT.md) for building, testing, the code layout and the documentation site.

## License

[Apache License 2.0](LICENSE)
