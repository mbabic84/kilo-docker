#!/bin/sh
# =============================================================================
# BREAKING CHANGE (2026-03-27): The container user home changed from
# /home/kilo to /home/kilo-t8x3m7kp. Existing Docker volumes are incompatible
# and must be recreated with: kilo-docker init
# =============================================================================
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

PUID="${PUID:-1000}"
PGID="${PGID:-1000}"

if [ "$(id -u)" = "0" ]; then
    if [ "$PUID" != "1000" ] || [ "$PGID" != "1000" ]; then
        deluser kilo-t8x3m7kp 2>/dev/null || true
        addgroup -g "$PGID" kilo-t8x3m7kp 2>/dev/null || true
        adduser -u "$PUID" -G kilo-t8x3m7kp -D -s /bin/sh kilo-t8x3m7kp
    fi
    if [ "${DOCKER_ENABLED:-}" = "1" ]; then
        if ! command -v docker >/dev/null 2>&1; then
            echo "[kilo-docker] Downloading latest Docker client..." >&2
            DOCKER_VERSION=$(curl -fsSL "https://download.docker.com/linux/static/stable/x86_64/" | grep -oE 'docker-[0-9]+\.[0-9]+\.[0-9]+' | sort -V | tail -1 | sed 's/docker-//')
            curl -fsSL "https://download.docker.com/linux/static/stable/x86_64/docker-${DOCKER_VERSION}.tgz" | tar xzf - -C /tmp docker/docker
            mv /tmp/docker/docker /usr/local/bin/docker
            chmod +x /usr/local/bin/docker
            rm -rf /tmp/docker
        fi
        if ! command -v docker-compose >/dev/null 2>&1; then
            echo "[kilo-docker] Downloading latest Docker Compose..." >&2
            curl -fsSL "https://github.com/docker/compose/releases/latest/download/docker-compose-linux-x86_64" \
                -o /usr/local/bin/docker-compose
            chmod +x /usr/local/bin/docker-compose
            mkdir -p /usr/libexec/docker/cli-plugins
            ln -sf /usr/local/bin/docker-compose /usr/libexec/docker/cli-plugins/docker-compose
        fi
    fi
    if [ "${ZELLIJ:-}" = "1" ] && ! command -v zellij >/dev/null 2>&1; then
        echo "[kilo-docker] Downloading latest Zellij..." >&2
        wget -qO /tmp/zellij.tar.gz "https://github.com/zellij-org/zellij/releases/latest/download/zellij-x86_64-unknown-linux-musl.tar.gz"
        tar xzf /tmp/zellij.tar.gz -C /usr/local/bin
        rm -f /tmp/zellij.tar.gz
    fi
    if [ -n "${DOCKER_GID:-}" ]; then
        if addgroup -g "$DOCKER_GID" docker 2>/dev/null; then
            addgroup kilo-t8x3m7kp docker 2>/dev/null || true
        else
            DOCKER_GROUP=$(getent group "$DOCKER_GID" | cut -d: -f1)
            if [ -n "$DOCKER_GROUP" ]; then
                addgroup kilo-t8x3m7kp "$DOCKER_GROUP" 2>/dev/null || true
            fi
        fi
    fi
    mkdir -p /home/kilo-t8x3m7kp/.local /workspace
    chown -R kilo-t8x3m7kp:kilo-t8x3m7kp /home/kilo-t8x3m7kp /workspace
    exec su-exec kilo-t8x3m7kp "$0" "$@"
fi

. "$SCRIPT_DIR/setup-kilo-config.sh"

if [ "${ZELLIJ:-}" = "1" ]; then
    mkdir -p "$HOME/.config/zellij"
    if [ ! -f "$HOME/.config/zellij/config.kdl" ]; then
        cp /etc/zellij/config.kdl "$HOME/.config/zellij/config.kdl"
    fi
    exec zellij
fi

exec kilo "$@"
