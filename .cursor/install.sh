#!/usr/bin/env bash
# Idempotent bootstrap for the Traicr Cloud Agent environment.
# Installs the Go toolchain and Docker engine the repository needs, then
# downloads modules and builds both binaries. Safe to run repeatedly.
set -euo pipefail

GO_VERSION="1.27.1"
GO_ROOT="/usr/local/go"

log() { printf '==> %s\n' "$1"; }

ensure_go() {
  local current=""
  if [ -x "${GO_ROOT}/bin/go" ]; then
    current="$("${GO_ROOT}/bin/go" env GOVERSION 2>/dev/null || true)"
  fi
  if [ "${current}" = "go${GO_VERSION}" ]; then
    log "Go ${GO_VERSION} already installed"
    return
  fi
  log "Installing Go ${GO_VERSION}"
  local tarball="/tmp/go${GO_VERSION}.linux-amd64.tar.gz"
  curl -fsSL "https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz" -o "${tarball}"
  sudo rm -rf "${GO_ROOT}"
  sudo tar -C /usr/local -xzf "${tarball}"
  rm -f "${tarball}"
}

ensure_docker() {
  if command -v docker >/dev/null 2>&1 && command -v fuse-overlayfs >/dev/null 2>&1; then
    log "Docker and fuse-overlayfs already installed"
  else
    log "Installing Docker engine and fuse-overlayfs"
    sudo apt-get update -qq
    sudo DEBIAN_FRONTEND=noninteractive apt-get install -y -qq \
      -o Dpkg::Options::="--force-confold" \
      docker.io docker-compose-v2 fuse-overlayfs fuse3 uidmap iptables ca-certificates
  fi
  # Allow the agent user to talk to the daemon without sudo.
  if getent group docker >/dev/null 2>&1; then
    sudo usermod -aG docker "$(id -un)" || true
  fi
}

ensure_path() {
  # Make the Go toolchain available in interactive shells for this environment.
  local line='export PATH=/usr/local/go/bin:$HOME/go/bin:$PATH'
  local profile="${HOME}/.bashrc"
  if [ -f "${profile}" ] && ! grep -qF "${line}" "${profile}"; then
    printf '\n%s\n' "${line}" >> "${profile}"
  fi
}

ensure_go
ensure_docker
ensure_path

export PATH="/usr/local/go/bin:${HOME}/go/bin:${PATH}"

log "Downloading Go modules"
go mod download

log "Building collector and server binaries"
make build

log "Install complete"
go version
docker --version
