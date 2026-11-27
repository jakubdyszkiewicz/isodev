# Docker Hub pull-through cache

Caches public Docker Hub images for the inner Docker daemons of the [Docker Hub cache guide](https://isodev.dyszkiewicz.me/docs/guides/docker-hub-cache/), so each container doesn't download them again.

It listens on port 5000 of every host interface over plain HTTP, and pulls anonymously. Cached entries expire after `proxy.ttl` in `config.yml`.
