---
title: OpenCode
weight: 3
---

Run [OpenCode](https://opencode.ai) in the container with your host credentials.

## Set up

Run `opencode auth login` on the host first. Then use the files in [`examples/guides/opencode`](https://github.com/jakubdyszkiewicz/isodev/tree/main/examples/guides/opencode):

{{< example-file path="guides/opencode/config.yaml" lang="yaml" name="~/.config/isodev/config.yaml" >}}

{{< example-file path="guides/opencode/image/Dockerfile" lang="dockerfile" name="~/.config/isodev/image/Dockerfile" >}}

{{< example-file path="guides/opencode/hooks/opencode.sh" lang="sh" name="~/.config/isodev/hooks/opencode.sh" >}}

Each container gets a copy of the credentials, so its sessions and data stay private to it. After logging in again on the host, run `isodev up --recreate` to copy the new credentials.

Providers that read API keys from the environment work too: add the variables, such as `ANTHROPIC_API_KEY`, to `lifecycle.on_exec.forward_env_vars`.

## Use it

```sh
isodev up --recreate
isodev exec opencode
```

With [Paseo](../../integrations/paseo), install the provider and choose **OpenCode ACP (Isodev)**:

```sh
isodev int paseo install-provider --type=opencode-acp
paseo reload --json
```

With [Zed](../../integrations/zed#acp-agents), run `isodev int zed install-provider --type=opencode-acp`.
