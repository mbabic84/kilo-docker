#!/bin/sh
set -e
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
case "$ARCH" in x86_64) ARCH="amd64";; aarch64|arm64) ARCH="arm64";; esac
INSTALL_DIR="${HOME}/.local/bin"
mkdir -p "$INSTALL_DIR"
curl -fsSL "https://github.com/mbabic84/kilo-docker/releases/latest/download/kilo-docker-${OS}-${ARCH}" \
  -o "${INSTALL_DIR}/kilo-docker"
chmod +x "${INSTALL_DIR}/kilo-docker"
exec "${INSTALL_DIR}/kilo-docker" install "$@"
