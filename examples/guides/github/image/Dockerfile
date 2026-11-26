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

# GitHub CLI, also used by Git as the credential helper for github.com. `gh`
# resolves to the wrapper in /usr/local/bin.
RUN mise use --global gh \
    && git config --system credential.https://github.com.helper '!gh auth git-credential'
