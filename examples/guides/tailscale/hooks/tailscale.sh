#!/bin/sh
# Runs whenever Isodev creates or starts the container.
set -eu
# Start the Tailscale daemon if it isn't running, then log in once.
mkdir -p /var/log/isodev /var/lib/tailscale
if ! tailscale status --json >/dev/null 2>&1; then
  tailscaled --state=/var/lib/tailscale/tailscaled.state </dev/null >/var/log/isodev/tailscaled.log 2>&1 &
fi
attempt=0
until tailscale status --json >/dev/null 2>&1; do
  attempt=$((attempt + 1))
  if [ "$attempt" -ge 30 ]; then
    echo "tailscaled did not start; recent log:" >&2
    tail -n 20 /var/log/isodev/tailscaled.log >&2
    exit 1
  fi
  sleep 1
done
if [ "$(tailscale status --json | jq -r .BackendState)" != Running ]; then
  tailscale up --auth-key=file:/run/secrets/tailscale-auth-key --timeout=60s
fi
