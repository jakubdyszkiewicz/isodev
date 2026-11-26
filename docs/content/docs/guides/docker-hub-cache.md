---
title: Docker Hub cache
weight: 7
---

Pull Docker Hub images through a cache on the host, so containers don't each download the same images. This guide builds on [Docker-in-Docker](../docker).

## Run the cache

The cache is a [registry](https://distribution.github.io/distribution/) in pull-through mode, running on the host. Use the files in [`examples/services/registry-mirror`](https://github.com/jakubdyszkiewicz/isodev/tree/main/examples/services/registry-mirror), and start it with `docker compose up -d` in that directory:

{{< example-file path="services/registry-mirror/compose.yaml" lang="yaml" >}}

{{< example-file path="services/registry-mirror/config.yml" lang="yaml" >}}

## Set up

Use the files in [`examples/guides/docker-hub-cache`](https://github.com/jakubdyszkiewicz/isodev/tree/main/examples/guides/docker-hub-cache):

{{< example-file path="guides/docker-hub-cache/config.yaml" lang="yaml" name="~/.config/isodev/config.yaml" >}}

{{< example-file path="guides/docker-hub-cache/image/Dockerfile" lang="dockerfile" name="~/.config/isodev/image/Dockerfile" >}}

{{< example-file path="guides/docker-hub-cache/hooks/docker.sh" lang="sh" name="~/.config/isodev/hooks/docker.sh" >}}

The inner daemon reaches the cache through the container's network gateway, which is the host. When the cache is unreachable, it pulls from Docker Hub directly.

## Use it

```sh
isodev up --recreate
isodev exec docker info --format '{{.RegistryConfig.Mirrors}}'
isodev exec docker run --rm hello-world
curl -s http://localhost:5000/v2/_catalog
```

The catalog lists the images the cache holds, such as `library/hello-world`. Cached images expire after `proxy.ttl` in `config.yml`.

## Trade-offs

- **Only Docker Hub is cached.** Images from other registries, such as `ghcr.io`, are pulled directly.
- **Only public images.** The cache pulls anonymously.
- **The cache is reachable on your network.** It listens on port 5000 of every host interface over plain HTTP, and anyone who can reach that port can pull through it.
