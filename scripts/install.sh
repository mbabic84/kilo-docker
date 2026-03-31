#!/bin/sh
set -e

OS=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$OS" in
  linux|darwin) ;;
  *)
    echo "Error: unsupported OS '$OS'. Supported: linux, darwin" >&2
    exit 1
    ;;
esac

ARCH=$(uname -m)
case "$ARCH" in
  x86_64)          ARCH="amd64" ;;
  aarch64|arm64)   ARCH="arm64" ;;
  *)
    echo "Error: unsupported architecture '$ARCH'. Supported: x86_64, aarch64/arm64" >&2
    exit 1
    ;;
esac

INSTALL_DIR="${HOME}/.local/bin"
TARGET="${INSTALL_DIR}/kilo-docker"
mkdir -p "$INSTALL_DIR"

curl -fsSL "https://github.com/mbabic84/kilo-docker/releases/latest/download/kilo-docker-${OS}-${ARCH}" \
  -o "$TARGET"

if [ ! -s "$TARGET" ]; then
  echo "Error: download failed or binary is empty." >&2
  exit 1
fi

chmod +x "$TARGET"

# Pull Docker image during install
if command -v docker >/dev/null 2>&1; then
  echo "Pulling Docker image..."
  docker pull ghcr.io/mbabic84/kilo-docker:latest 2>/dev/null || true
fi
