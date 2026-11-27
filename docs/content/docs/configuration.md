---
title: Configuration
weight: 5
---

Isodev reads an optional `~/.config/isodev/config.yaml`, which applies to every new container. All fields are optional, and unknown fields are rejected. `isodev config show` prints the effective configuration after defaults and overrides, including any environment values.

## Example

```yaml
image: isodev-worker:latest
build:
  context: image          # Relative to ~/.config/isodev
  dockerfile: Dockerfile
privileged: true          # Only when needed, for example for Docker-in-Docker
command: ["sleep", "infinity"]
environment:
  CODEX_HOME: /root/.codex
volumes:
  - type: bind
    source: ~/.codex/auth.json
    target: /run/secrets/codex-auth.json  # Copied to $CODEX_HOME by on-create.sh
    read_only: true
  - type: volume                          # Private to each container, for Docker-in-Docker
    target: /var/lib/docker
lifecycle:
  on_create:
    scripts:
      - file: on-create.sh
  on_start:
    scripts:
      - file: on-start.sh
  on_exec:
    forward_env_vars: [ANTHROPIC_API_KEY]
```

The [guides](../guides) have complete, tested configurations for common setups.

## Editor support

[`config.schema.json`](https://isodev.dyszkiewicz.me/config.schema.json) is a JSON Schema for the file. To get completion, hover documentation and validation in editors that use yaml-language-server, such as Zed, VS Code and Neovim, start your `config.yaml` with:

```yaml
# yaml-language-server: $schema=https://isodev.dyszkiewicz.me/config.schema.json
```

## Paths

In `build.context`, `volumes[].source` and script `file` fields, `~/` means the host user's home, and relative paths are relative to `~/.config/isodev`. Other `~` forms, such as `~user/`, are not supported. `build.dockerfile` is relative to the build context.

## Fields

<!-- BEGIN GENERATED REFERENCE: edit the field comments in internal/config/config.go and run `go generate ./...` -->

### `image`

string · default `ubuntu:24.04`

Image for new containers. The `ISODEV_IMAGE` environment variable overrides it. Required when `build` is set, because it names the built image.

### `build`

object

Build the image before creating each new container, with `docker build --tag IMAGE --file DOCKERFILE CONTEXT`. Docker reuses cached layers and build output goes to stderr. A failed build prevents creation. Existing containers are not rebuilt.

### `build.context`

string · required

Build context directory, which must exist. It is sent to Docker, so keep credentials and hook scripts outside it.

### `build.dockerfile`

string · default `Dockerfile`

Dockerfile path, relative to the context.

### `command`

list of strings · default `["sleep","infinity"]`

Command arguments passed to the image. The container must keep running so that lifecycle scripts and agent commands can run in it. Containers run with `--init`.

### `privileged`

boolean

Run containers with `--privileged`, for example for Docker-in-Docker. Privileged containers have broad access to the host.

### `environment`

map of strings

Fixed environment variables set when a container is created. Values are strings; variable interpolation is not supported.

### `volumes`

list of objects

Mounts added to each new container, in the Compose long form: host directories and files, and Docker volumes. Isodev adds the workspace mount itself. Mounted sources are visible to the container even when read-only.

### `volumes[].type`

`bind` or `volume` · required

`bind` mounts a host directory or file. `volume` mounts a Docker volume, which lives on Docker's own disk, so it works for data that needs a real Linux filesystem, such as an inner Docker daemon's `/var/lib/docker`, on macOS too.

### `volumes[].source`

string

For `bind`, the host path, which must exist; `~/` is the host user's home, and relative paths are relative to `~/.config/isodev`. For `volume`, the name of a Docker volume shared by all containers and kept when they are removed. Without a source, a `volume` is private to the container: it is kept by `isodev up --recreate`, emptied by `up --recreate --renew-volumes`, and removed by `isodev down`.

### `volumes[].target`

string · required

Absolute path in the container. It must not overlap the workspace.

### `volumes[].read_only`

boolean

Mount read-only.

### `lifecycle`

object

What Isodev does when it creates or starts a container, and when it runs a command in one.

### `lifecycle.on_create`

object

Runs once in each new container, after `on_start`. If a script fails, the rest are skipped and the new container and its network are removed. Changes apply only to new containers.

### `lifecycle.on_create.scripts`

list of objects

Scripts that run in order, each streamed to `sh` in the container with the workspace as the working directory.

### `lifecycle.on_create.scripts[].file`

string · required

Script path. `~/` is the host user's home, and relative paths are relative to `~/.config/isodev`. The script runs with `sh`, with standard input from `/dev/null`; its shebang is ignored. `ISODEV_PROBE` is `1` in the Paseo probe container and `0` otherwise. Output goes to stderr.

### `lifecycle.on_start`

object

Runs in each new container before `on_create`, and again whenever Isodev starts a stopped container with `up`, `start`, or Paseo or Zed setup, using the current configuration. If a script fails, the rest are skipped: a new container and its network are removed, and a started container is stopped again. Starting a container with `docker start` skips it.

### `lifecycle.on_start.scripts`

list of objects

Scripts that run in order, each streamed to `sh` in the container with the workspace as the working directory.

### `lifecycle.on_start.scripts[].file`

string · required

Script path. `~/` is the host user's home, and relative paths are relative to `~/.config/isodev`. The script runs with `sh`, with standard input from `/dev/null`; its shebang is ignored. `ISODEV_PROBE` is `1` in the Paseo probe container and `0` otherwise. Output goes to stderr.

### `lifecycle.on_exec`

object

Applied each time Isodev runs a command in a container: `isodev exec`, and Paseo and Zed agent launches. Changes apply without recreating the container.

### `lifecycle.on_exec.forward_env_vars`

list of strings

Names of environment variables copied from the invoking process into each command. Values are read on every invocation and passed by name, so they don't appear in process arguments. Unset variables are omitted; empty values are forwarded. Forward secrets only to containers and commands you trust.

### `git_worktree`

object

How Isodev handles a workspace that is a linked Git worktree. A worktree's `.git` file points into the main repository's Git directory, which the container doesn't have unless Isodev provides it.

### `git_worktree.mode`

`none` or `mount-common-dir` or `replace`

`none` mounts only the worktree, so Git doesn't work in the container. `mount-common-dir` also mounts the main repository's Git directory, Git's common directory, read-write at its host path, and the workspace at its host path. `replace` replaces the worktree's link with an independent repository before the container is created, and `isodev down` restores it. The mode applies when a container is created, so switch modes with `isodev up --recreate`; a replaced worktree is then restored first if the new mode isn't `replace`. When unset in a worktree, `isodev up` warns and uses `none`, and Paseo and Zed setup fail.

<!-- END GENERATED REFERENCE -->

## Applying changes

Changes to `image`, `build`, `command`, `privileged`, `environment` and `volumes` apply only to new containers. Run `isodev up --recreate` to replace a workspace's container; its workspace files and private volumes stay, but the rest of its private filesystem is removed. Add `--renew-volumes` to empty the private volumes too. Changes to `on_start` apply on the next start, and changes to `on_exec` on the next command. A change to `git_worktree.mode` also applies only to new containers; with `isodev up --recreate`, a replaced worktree is restored first if the new mode isn't `replace`.
