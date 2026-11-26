---
title: Docker-in-Docker
weight: 6
---

Give each container its own Docker daemon, so builds, tests and Compose stacks in one workspace don't affect the others or the host's Docker.

## Set up

Use the files in [`examples/guides/docker`](https://github.com/jakubdyszkiewicz/isodev/tree/main/examples/guides/docker):

{{< example-file path="guides/docker/config.yaml" lang="yaml" name="~/.config/isodev/config.yaml" >}}

{{< example-file path="guides/docker/image/Dockerfile" lang="dockerfile" name="~/.config/isodev/image/Dockerfile" >}}

{{< example-file path="guides/docker/hooks/docker.sh" lang="sh" name="~/.config/isodev/hooks/docker.sh" >}}

Each container's images and containers are stored in a private Docker volume, on Docker's own disk, where the inner daemon can use the overlay2 storage driver. `isodev up --recreate` keeps them, `isodev up --recreate --renew-volumes` empties them, and `isodev down` deletes them. List the volumes with `docker volume ls --filter label=isodev.managed=true`.

## Use it

```sh
isodev up --recreate
isodev exec docker run --rm hello-world
```

Each inner daemon starts with an empty image store, so every container downloads the images it uses. To keep one copy of each Docker Hub image on the host, add a [Docker Hub cache](../docker-hub-cache).

## Trade-offs

- **Containers are privileged.** The inner daemon needs `privileged: true`, which gives the container broad access to the host. See [Security](../../security).
