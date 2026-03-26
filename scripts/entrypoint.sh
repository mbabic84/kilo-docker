#!/bin/sh
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

PUID="${PUID:-1000}"
PGID="${PGID:-1000}"

if [ "$(id -u)" = "0" ]; then
    if [ "$PUID" != "1000" ] || [ "$PGID" != "1000" ]; then
        deluser kilo 2>/dev/null || true
        addgroup -g "$PGID" kilo 2>/dev/null || true
        adduser -u "$PUID" -G kilo -D -s /bin/sh kilo
    fi
    if [ "${DOCKER_ENABLED:-}" = "1" ] && ! command -v docker >/dev/null 2>&1; then
        apk add --no-cache docker-cli docker-cli-compose
    fi
    if [ -n "${DOCKER_GID:-}" ]; then
        if addgroup -g "$DOCKER_GID" docker 2>/dev/null; then
            addgroup kilo docker 2>/dev/null || true
        else
            DOCKER_GROUP=$(getent group "$DOCKER_GID" | cut -d: -f1)
            if [ -n "$DOCKER_GROUP" ]; then
                addgroup kilo "$DOCKER_GROUP" 2>/dev/null || true
            fi
        fi
    fi
    mkdir -p /home/kilo/.local /workspace
    chown -R kilo:kilo /home/kilo /workspace
    exec su-exec kilo "$0" "$@"
fi

. "$SCRIPT_DIR/setup-kilo-config.sh"

exec kilo "$@"
