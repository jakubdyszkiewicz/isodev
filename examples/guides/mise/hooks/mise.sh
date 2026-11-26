#!/bin/sh
# Runs once in each new container, in the workspace.
set -eu
# Install the project's tools when it has a mise or asdf configuration.
for config in mise.toml .mise.toml .tool-versions .config/mise/config.toml; do
  if [ -f "$config" ]; then
    mise trust
    mise install
    break
  fi
done
