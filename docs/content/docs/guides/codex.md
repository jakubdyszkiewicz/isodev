---
title: Codex
weight: 2
---

Run [Codex](https://github.com/openai/codex) in the container, sharing your host login and sessions.

## Set up

Run `codex login` on the host first; it stores the login in `~/.codex/auth.json`. Then use the files in [`examples/guides/codex`](https://github.com/jakubdyszkiewicz/isodev/tree/main/examples/guides/codex):

{{< example-file path="guides/codex/config.yaml" lang="yaml" name="~/.config/isodev/config.yaml" >}}

{{< example-file path="guides/codex/image/Dockerfile" lang="dockerfile" name="~/.config/isodev/image/Dockerfile" >}}

Because the whole `~/.codex` directory is shared, token refreshes in a container also update the host. To use an API key instead, export `OPENAI_API_KEY` on the host.

## Use it

```sh
isodev up --recreate
isodev exec codex
```

With [Paseo](../../integrations/paseo), install the provider and choose **Codex (Isodev)**. The provider runs Codex with approvals disabled and full access inside the container.

```sh
isodev int paseo install-provider --type=codex
paseo reload --json
```

With [Zed](../../integrations/zed#acp-agents), run `isodev int zed install-provider --type=codex`. Zed uses the `codex-acp` adapter from the last step of the Dockerfile.

## Trade-offs

- **The container can modify your Codex configuration.** It can read and change everything in `~/.codex`, including sessions and `config.toml`. Mount a separate directory with only a copy of `auth.json` if you need stronger separation; token refreshes then stay in the container.
