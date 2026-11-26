---
title: Zed
weight: 2
---

Isodev can prepare [Zed](https://zed.dev) worktrees with their own container and run ACP agents inside that container.

## Worktree task

```sh
isodev int zed install-task
```

This adds a manually triggered **Prepare isodev worktree** task to Zed's global `tasks.json` (`~/.config/zed/tasks.json`, or `$XDG_CONFIG_HOME/zed/tasks.json` on Linux). Use `--path` to select a different file. The installer preserves comments and other tasks and backs up the existing file.

A project can instead define the task in `.zed/tasks.json`; put it in an array if you create a new file:

```json
{
  "label": "Prepare isodev worktree",
  "command": "isodev",
  "args": ["int", "zed", "setup", "--workspace", "$ZED_WORKTREE_ROOT", "--source", "$ZED_MAIN_GIT_WORKTREE"],
  "reevaluate_context": true,
  "reveal": "no_focus",
  "hide": "on_success"
}
```

Open a worktree, then run the task from **Task: Spawn**. Setup starts a container with the workspace at the same absolute path, handling the worktree as [`git_worktree.mode`](../../git-worktrees) says. Setup fails if the mode isn't set.

{{< callout type="info" >}}
When Zed connects over SSH, the Zed application and the SSH server have separate settings. Check which task file Zed loads.
{{< /callout >}}

### Teardown

Before Zed archives or deletes the worktree, run:

```sh
isodev int zed teardown --workspace /path/to/worktree
```

Teardown removes the container and, if the worktree was replaced, restores it. With `mode: replace`, it fetches the worktree's commits, including a detached HEAD, into `isodev-NAME/*` refs in the main checkout and fast-forwards the restored worktree to them when it can; see [Restore](../../git-worktrees#restore). The independent repository is archived under `.isodev-git-archive/`, and is lost if Zed deletes the worktree directory, but the fetched commits stay in the main checkout.

## ACP agents

```sh
isodev int zed install-provider --type=opencode-acp   # or --type=codex
```

This adds an agent to Zed's `settings.json` on this machine, preserving comments and other settings and backing up the existing file. Use `--path` to select another file. The equivalent manual entry, using the absolute path to `isodev` on the machine running the Zed server, is:

```json
"agent_servers": {
  "OpenCode (isodev)": {
    "type": "custom",
    "command": "/home/you/.local/bin/isodev",
    "args": ["int", "zed", "acp"]
  },
  "Codex (isodev)": {
    "type": "custom",
    "command": "/home/you/.local/bin/isodev",
    "args": ["int", "zed", "acp", "--agent", "codex"]
  }
}
```

Open the prepared worktree as the Zed project and select the agent.

- The container must already be running and mount the workspace at the same absolute path, as the worktree task does.
- The container needs OpenCode, or `@agentclientprotocol/codex-acp`, which bundles a compatible Codex, together with the agent's credentials.
- The bridge picks the container from the agent's working directory and rejects sessions outside that workspace, so multi-root projects may need separate windows.
- Zed's host-side ACP terminal and filesystem capabilities are disabled.
