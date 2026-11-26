FROM ubuntu:24.04
ARG DEBIAN_FRONTEND=noninteractive
ENV PATH=/root/.local/share/mise/shims:/root/.local/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates curl git jq \
    && rm -rf /var/lib/apt/lists/*

# Dotfiles, here https://github.com/jakubdyszkiewicz/dotfishy: replace the URL
# and branch with yours. Docker checks the branch on every build and rebuilds
# from here when it has new commits.
ADD https://github.com/jakubdyszkiewicz/dotfishy.git#master /root/.dotfishy
# An empty profile selects dotfishy's server role: Fish and other apt packages,
# the dotfile links and the CLI tools, without development toolchains.
RUN : > /root/.dotfishy/profile \
    && /root/.dotfishy/install.sh
