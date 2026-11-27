---
title: Getting started
weight: 2
---

## Install

{{< tabs >}}
  {{< tab name="Homebrew (macOS)" >}}
```sh
brew install jakubdyszkiewicz/tap/isodev
```
  {{< /tab >}}
  {{< tab name="mise" >}}
With [mise](https://mise.jdx.dev), on Linux and macOS:

```sh
mise use --global github:jakubdyszkiewicz/isodev
```
  {{< /tab >}}
  {{< tab name="Release archive" >}}
Download the archive for your platform from [GitHub Releases](https://github.com/jakubdyszkiewicz/isodev/releases), for example `isodev_0.1.0_linux_amd64.tar.gz`, and put `isodev` on your `PATH`. Archives are available for Linux and macOS on amd64 and arm64, and each release includes a SHA-256 checksums file.
  {{< /tab >}}
  {{< tab name="Go" >}}
With Go 1.24 or later:

```sh
go install github.com/jakubdyszkiewicz/isodev@latest
```
  {{< /tab >}}
  {{< tab name="From source" >}}
With [mise](https://mise.jdx.dev), from a clone of the repository:

```sh
mise install
mise run install-local-bin   # Installs isodev to ~/.local/bin
```
  {{< /tab >}}
{{< /tabs >}}

`isodev --version` shows the installed version and commit.

## Create a container

{{% steps %}}

### Start a container for a workspace

```sh
cd ~/src/my-project
isodev up
```

The workspace is mounted at `/workspaces/my-project` in the container.

### Open a shell in it

```sh
isodev enter
```

Isodev starts a shell. To run a single command instead, use `isodev exec`, for example `isodev exec git status`.

### Remove it when you're done

```sh
isodev down
```

The container and its private filesystem are removed; workspace files stay.

{{% /steps %}}

## Use your own image

The default image is plain `ubuntu:24.04`, without Git or any agent. Configure the image, mounts and setup scripts in `~/.config/isodev/config.yaml`; see the [configuration reference](../configuration).

To add agents, credentials, Docker-in-Docker or Tailscale one at a time, follow the [guides](../guides).
