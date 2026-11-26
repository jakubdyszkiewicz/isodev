---
title: Claude Code
weight: 1
---

Run [Claude Code](https://claude.com/claude-code) in the container, sharing your host login, settings and transcripts.

## Set up

Run `claude` once on the host first, so that `~/.claude` exists. Then use the files in [`examples/guides/claude-code`](https://github.com/jakubdyszkiewicz/isodev/tree/main/examples/guides/claude-code):

{{< example-file path="guides/claude-code/config.yaml" lang="yaml" name="~/.config/isodev/config.yaml" >}}

{{< example-file path="guides/claude-code/image/Dockerfile" lang="dockerfile" name="~/.config/isodev/image/Dockerfile" >}}

### Credentials

- **Linux:** `claude` stores its login in `~/.claude/.credentials.json`, which the mount provides. Token refreshes are shared through the same file.
- **macOS:** Claude Code keeps the login in the Keychain instead. Run `claude setup-token` on the host and export the result as `CLAUDE_CODE_OAUTH_TOKEN`, or export `ANTHROPIC_API_KEY`.

Variables in `forward_env_vars` are passed to commands that Isodev runs, such as `isodev exec` and Paseo agents, but not to `isodev enter` shells.

## Use it

```sh
isodev up --recreate
isodev exec claude
```

With [Paseo](../../integrations/paseo), install the provider and choose **Claude (Isodev)**:

```sh
isodev int paseo install-provider --type=claude
paseo reload --json
```

Paseo reads Claude transcripts from the host's `~/.claude` to restore and rewind conversations; the shared mount is what makes that work.

## Trade-offs

- **The container can modify your Claude configuration.** It can read all host transcripts and memory, and write settings, hooks, skills and plugins that host Claude sessions will load. Mount a separate directory instead if you need stronger separation; Paseo then can't restore or rewind container conversations.
- **`~/.claude.json` is not shared.** Claude Code replaces that file by renaming it, which breaks single-file bind mounts, so each container keeps its own.
- **Plugin paths may break.** `~/.claude/plugins` records absolute host paths, which only resolve in the container if your host home is also `/root`.
