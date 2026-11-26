#!/bin/sh
# Runs whenever Isodev creates or starts the container.
set -eu
# Start the inner Docker daemon if it isn't running.
if ! docker info >/dev/null 2>&1; then
  mkdir -p /var/log/isodev
  # /var/run survives a container restart. Stale PID files would make dockerd
  # wait for a containerd that is no longer running.
  rm -f /var/run/docker.pid /var/run/docker/containerd/containerd.pid
  # A volume mounted at /var/lib/docker supports overlay2. Fall back to the
  # slower vfs driver without it.
  driver=vfs
  if mountpoint -q /var/lib/docker; then
    driver=overlay2
  fi
  # Pull Docker Hub images through the cache on the host, which is this
  # container's default gateway. The daemon falls back to Docker Hub when the
  # cache is unreachable.
  gateway="$(ip -4 route show default | awk '{print $3; exit}')"
  dockerd --storage-driver="$driver" --registry-mirror="http://$gateway:$DOCKER_MIRROR_PORT" \
    </dev/null >/var/log/isodev/dockerd.log 2>&1 &
  attempt=0
  until docker info >/dev/null 2>&1; do
    attempt=$((attempt + 1))
    if [ "$attempt" -ge 60 ]; then
      echo "dockerd did not start; recent log:" >&2
      tail -n 30 /var/log/isodev/dockerd.log >&2
      exit 1
    fi
    sleep 1
  done
fi
