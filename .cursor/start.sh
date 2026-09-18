#!/usr/bin/env bash
# Per-boot startup for the Traicr Cloud Agent environment.
# Starts the Docker daemon (required for `docker compose up` and the e2e tests)
# in the nested VM using the fuse-overlayfs storage driver. Idempotent: it
# exits early when the daemon is already responding.
set -euo pipefail

log() { printf '==> %s\n' "$1"; }

if docker info >/dev/null 2>&1; then
  log "Docker daemon already running"
  exit 0
fi

log "Starting Docker daemon (fuse-overlayfs)"
sudo mkdir -p /var/log
sudo bash -c 'nohup dockerd --storage-driver=fuse-overlayfs >/var/log/dockerd.log 2>&1 &'

# Wait for the daemon socket to become responsive.
for _ in $(seq 1 30); do
  if sudo docker info >/dev/null 2>&1; then
    break
  fi
  sleep 1
done

if ! sudo docker info >/dev/null 2>&1; then
  log "Docker daemon failed to start; recent log:"
  sudo tail -n 30 /var/log/dockerd.log || true
  exit 1
fi

# Let the agent user use the daemon without sudo for the rest of the session.
sudo chmod 666 /var/run/docker.sock || true

log "Docker daemon ready"
docker version --format '{{.Server.Version}}' 2>/dev/null || true
