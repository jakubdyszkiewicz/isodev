---
title: Project toolchains
weight: 8
---

Install each project's tools, such as Go, Node.js or Python, when its container is created, from the project's [mise](https://mise.jdx.dev) configuration. Different worktrees can use different versions without affecting each other.

Every guide's image already has mise for its global tools; this guide adds a hook that installs the project's own tools too.

## Set up

Use the files in [`examples/guides/mise`](https://github.com/jakubdyszkiewicz/isodev/tree/main/examples/guides/mise):

{{< example-file path="guides/mise/config.yaml" lang="yaml" name="~/.config/isodev/config.yaml" >}}

{{< example-file path="guides/mise/image/Dockerfile" lang="dockerfile" name="~/.config/isodev/image/Dockerfile" >}}

{{< example-file path="guides/mise/hooks/mise.sh" lang="sh" name="~/.config/isodev/hooks/mise.sh" >}}

The hook installs the tools from a `mise.toml`, `.mise.toml`, `.tool-versions` or `.config/mise/config.toml` in the workspace. Workspaces without one get no tools. `mise trust` marks the project's configuration as trusted, which mise requires before it runs it.

## Use it

```sh
isodev up --recreate
isodev exec mise ls
```

The tools are on `PATH` through mise's shims, in `isodev exec`, `isodev enter` and agents. After changing the project's mise configuration, run `isodev exec mise install`. To avoid downloading the same tools in every container, add [shared caches](../caches).

## Trade-offs

- **Project configuration runs in the container.** `mise trust` accepts whatever the project's configuration does, including environment variables and tasks. Only use it with projects you trust.
