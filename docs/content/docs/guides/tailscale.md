---
title: Tailscale
weight: 12
---

Put each container on your [Tailscale](https://tailscale.com) network. You can then reach servers running in a container from your other devices, and the container can reach services on your tailnet.

## Create an auth key

In the Tailscale admin console, [generate an auth key](https://login.tailscale.com/admin/settings/keys) that is:

- **Reusable**, so every container can use it.
- **Ephemeral**, so Tailscale removes a container's device 30 to 60 minutes after it goes offline, for example after `isodev down`.
- **Pre-approved** and **tagged**, for example with `tag:isodev`, so you can control what the containers reach with access rules.

Save it as `~/.config/isodev/tailscale-auth-key`, readable only by you (`chmod 600`).

## Set up

Use the files in [`examples/guides/tailscale`](https://github.com/jakubdyszkiewicz/isodev/tree/main/examples/guides/tailscale):

{{< example-file path="guides/tailscale/config.yaml" lang="yaml" name="~/.config/isodev/config.yaml" >}}

{{< example-file path="guides/tailscale/image/Dockerfile" lang="dockerfile" name="~/.config/isodev/image/Dockerfile" >}}

{{< example-file path="guides/tailscale/hooks/tailscale.sh" lang="sh" name="~/.config/isodev/hooks/tailscale.sh" >}}

The hook runs whenever Isodev starts the container, so Tailscale comes back after `isodev stop` and `isodev up`. Tailscale's state is in a [private volume](../../configuration#volumessource), so the container keeps its device, name and address across `isodev stop`, `isodev up` and `isodev up --recreate`, instead of joining as a new device each time. `isodev down` removes the volume, and the ephemeral key lets Tailscale remove the device.

If a container stays stopped for longer than about 30 minutes, Tailscale removes its ephemeral device. The hook then logs in again with the auth key when the container starts, and the container may get a new address.

To also use [subnet routes](https://tailscale.com/kb/1019/subnets) advertised on your tailnet, add `--accept-routes` to `tailscale up`.

## Use it

```sh
isodev up --recreate
isodev exec tailscale status
```

Each container joins as a device named after its container, `isodev-<hash>`; `isodev ls` shows which workspace it belongs to. Servers in the container are reachable at the address from `isodev exec tailscale ip -4`, or at the device name with [MagicDNS](https://tailscale.com/kb/1081/magicdns).

## Trade-offs

- **Containers are privileged.** See [Security](../../security).
- **Containers are on your tailnet.** Use the key's tag in your access rules to limit what they can reach, and who can reach them.
- **Without an ephemeral key, removed containers stay on the tailnet** as offline devices until you remove them in the admin console.
