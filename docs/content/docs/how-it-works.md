---
title: How it works
weight: 9
---

## Containers and networks

Each workspace gets one container and one Docker bridge network, both named after a hash of the workspace's path (`isodev-<hash>` and `isodev-<hash>-net`). The container's hostname is its name. Containers and networks carry the `isodev.managed=true` and `isodev.workspace=<path>` labels, which `isodev ls` and `isodev ps` use.

The workspace is bind-mounted into the container, so files stay on the host. Everything else in the container, including its home directory, is private and removed with the container.

## Commands in the container

`isodev exec`, `isodev enter` and agent launches use `docker exec`. Commands from `isodev exec` and agent launches run in their own process group, so interrupting Isodev also stops their child processes in the container.

## State

Isodev keeps its records in `~/.local/state/isodev`:

| Path | Contents |
| --- | --- |
| `workspaces/` | One record per workspace container |
| `probe.json`, `probe/` | The Paseo probe container's record and workspace |
| `worktrees/` | How to restore each worktree that `git_worktree.mode: replace` replaced |
| `locks/` | Locks that serialize setup and teardown of a workspace |

Private volumes, `volumes` entries of `type: volume` without a `source`, are Docker volumes named `isodev-<hash>-<target>`, such as `isodev-<hash>-var-lib-docker`, with the same labels as the container. `isodev up --recreate` reuses them, and `isodev down` removes every volume labelled with the workspace.

## Known limitations

- If setup is killed partway, the container and network can be left without a state record, so `isodev down` can't remove them, and the next `up` stops and names the container. They still appear in `isodev ls`; remove them with `docker rm -f NAME` and `docker network rm NAME-net`.
- Setup and teardown of a workspace take a lock, so concurrent runs, such as Paseo launching an agent during setup, wait for each other.
