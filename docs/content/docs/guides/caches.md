---
title: Shared caches
weight: 9
---

Share download and build caches between all containers, so each new container doesn't download the same tools, modules and packages again. This guide builds on [Project toolchains](../mise) and caches mise downloads, Go modules and builds, npm packages and Playwright browsers.

## Set up

Use the files in [`examples/guides/caches`](https://github.com/jakubdyszkiewicz/isodev/tree/main/examples/guides/caches):

{{< example-file path="guides/caches/config.yaml" lang="yaml" name="~/.config/isodev/config.yaml" >}}

{{< example-file path="guides/caches/image/Dockerfile" lang="dockerfile" name="~/.config/isodev/image/Dockerfile" >}}

{{< example-file path="guides/caches/hooks/mise.sh" lang="sh" name="~/.config/isodev/hooks/mise.sh" >}}

Each cache is a [named Docker volume](../../configuration#volumessource), which Docker creates the first time a container uses it. Isodev never removes named volumes, so the caches outlive every container. They live on Docker's own disk, which on macOS is faster than sharing a directory from the Mac.

Go, npm and Playwright support several containers using their caches at the same time. mise only keeps downloaded archives in its cache; the tools it installs stay private to each container.

To cache other tools, add a volume at the path where the tool keeps its cache, for example `/root/.cache/pip` for pip, or set the tool's cache variable in `environment`.

## Use it

```sh
isodev up --recreate
docker system df -v | grep isodev-cache
```

## Trade-offs

- **Containers share what they download.** A container can read and change cache entries that other containers use.
- **The caches grow.** Isodev doesn't clean them. To empty one, remove its volume while no container uses it, for example `docker volume rm isodev-cache-go-mod`; Docker creates it again, empty, for the next container.
