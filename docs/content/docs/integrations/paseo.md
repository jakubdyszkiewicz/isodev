---
title: Paseo
weight: 1
---

Isodev provides [Paseo](https://paseo.sh) worktree hooks and agent providers that run inside the workspace container. The `isodev` binary must be on Paseo's `PATH`.

{{% steps %}}

### Add the worktree hooks

Add both hooks to a project's `paseo.json`:

```json
{
  "worktree": {
    "setup": "isodev int paseo setup",
    "teardown": "isodev int paseo teardown"
  }
}
```

- **Setup** runs for every new worktree, regardless of provider. It starts a container with the workspace mounted at the same path, handling the worktree as [`git_worktree.mode`](../../git-worktrees) says. Setup fails if the mode isn't set.
- **Teardown** removes the container and, if the worktree was replaced, restores it so Paseo can archive it.

With `mode: replace`, teardown fetches the worktree's commits into `isodev-NAME/*` refs in the main checkout and fast-forwards the restored worktree to them when it can; see [Restore](../../git-worktrees#restore). The commits stay in the main checkout after Paseo removes the directory.

### Install a provider

```sh
isodev int paseo install-provider --type=claude        # Claude (Isodev)
isodev int paseo install-provider --type=codex         # Codex (Isodev)
isodev int paseo install-provider --type=opencode-acp  # OpenCode ACP (Isodev)
paseo reload --json
```

The installer backs up `~/.paseo/config.json` before changing it. Paseo agent tools are disabled for all Isodev providers.

### Prepare the image

The container image must include the agent and its credentials; follow the guide for your agent:

- [Claude Code](../../guides/claude-code). Paseo reads Claude transcripts from the host's `~/.claude` to restore and rewind conversations, so the guide shares that directory with the container.
- [Codex](../../guides/codex)
- [OpenCode](../../guides/opencode). ACP terminal and file capabilities are disabled on the host.

{{% /steps %}}

## How agents find their container

Providers select the container from `PASEO_AGENT_CWD`, waiting for the setup hook to finish if needed. When no workspace is selected, Paseo probes the provider in a separate probe container whose state is in `~/.local/state/isodev/probe.json`. To recreate the probe container, run `isodev down --workspace PATH` with the `cwd` from that file.

In a Paseo workspace terminal, `isodev int paseo enter` opens a shell in that workspace's container.
