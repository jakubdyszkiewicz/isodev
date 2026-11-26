#!/bin/sh
# Runs once in each new container.
set -eu
# Use the host's Git identity and settings, while keeping the container's own
# ~/.gitconfig writable.
git config --global include.path /run/secrets/host-gitconfig

# Optional: commit as someone else, for example a bot account. Settings after
# the include override the host's.
# git config --global user.name 'Example Bot'
# git config --global user.email 'bot@example.com'

# Optional: use the host's global ignore file. This overrides the host's
# core.excludesFile, whose path may not exist in the container.
git config --global core.excludesFile /run/secrets/host-gitignore
