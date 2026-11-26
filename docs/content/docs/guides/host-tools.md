---
title: Host tools
weight: 11
---

Make a command-line tool from the host available in the container, together with the environment variables it needs, such as a token or a server address.

## Set up

This guide uses a small script as the tool. Save it as `~/.local/bin/mytool` on the host and make it executable with `chmod +x ~/.local/bin/mytool`:

{{< example-file path="guides/host-tools/mytool" lang="bash" name="~/.local/bin/mytool" >}}

Then use the files in [`examples/guides/host-tools`](https://github.com/jakubdyszkiewicz/isodev/tree/main/examples/guides/host-tools):

{{< example-file path="guides/host-tools/config.yaml" lang="yaml" name="~/.config/isodev/config.yaml" >}}

{{< example-file path="guides/host-tools/image/Dockerfile" lang="dockerfile" name="~/.config/isodev/image/Dockerfile" >}}

For a real tool, replace `mytool` and `MYTOOL_VAR` with its name and variables. The tool must run in the container: a script whose interpreter the image has, or a Linux binary for the container's architecture that doesn't depend on libraries the image lacks, such as a static Go binary. Otherwise, install the tool in the Dockerfile instead.

## Use it

```sh
isodev up --recreate
MYTOOL_VAR=hello isodev exec mytool
```

The tool prints `MYTOOL_VAR=hello`: Isodev reads `MYTOOL_VAR` from its own environment every time it runs a command, so the value can change without recreating the container. Forwarded variables reach `isodev exec` commands and agents, but not `isodev enter` shells.

After replacing the tool on the host, run `isodev up --recreate`: the mount keeps pointing to the file that was there when the container was created.

## Trade-offs

- **Forwarded variables are visible to everything the command runs,** including agents and the tools they start. Forward secrets only to containers and commands you trust.
