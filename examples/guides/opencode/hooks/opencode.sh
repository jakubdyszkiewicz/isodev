#!/bin/sh
# Runs once in each new container.
set -eu
# Copy the host credentials into the container's own OpenCode data directory.
mkdir -p "$HOME/.local/share/opencode"
install -m 600 /run/secrets/opencode-auth.json "$HOME/.local/share/opencode/auth.json"
