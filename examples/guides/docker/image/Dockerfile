FROM ubuntu:24.04
ARG DEBIAN_FRONTEND=noninteractive
# mise installs tools to ~/.local/share/mise and puts them on PATH through its
# shims. /usr/local/bin comes first, so files mounted there win.
ENV PATH=/root/.local/bin:/usr/local/sbin:/usr/local/bin:/root/.local/share/mise/shims:/usr/sbin:/usr/bin:/sbin:/bin
RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates curl git \
    && rm -rf /var/lib/apt/lists/*

# mise, which installs the tools.
RUN curl -fsSL https://mise.run | MISE_INSTALL_PATH=/usr/local/bin/mise sh

# Docker Engine with Buildx and Compose. The daemon is a system package, so it
# comes from apt.
RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        docker.io docker-buildx docker-compose-v2 iptables \
    && rm -rf /var/lib/apt/lists/*
