#!/bin/sh
set -e

PUID="${PUID:-1000}"
PGID="${PGID:-1000}"

if [ "$(id -u)" = "0" ]; then
    if [ "$PUID" != "1000" ] || [ "$PGID" != "1000" ]; then
        deluser kilo 2>/dev/null || true
        addgroup -g "$PGID" kilo 2>/dev/null || true
        adduser -u "$PUID" -G kilo -D -s /bin/sh kilo
    fi
    mkdir -p /home/kilo/.local /workspace
    chown -R kilo:kilo /home/kilo /workspace
    exec su-exec kilo "$0" "$@"
fi

exec kilo "$@"
