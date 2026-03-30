#!/bin/sh
# kilo-docker entrypoint — Container initialization and user setup.
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
    if [ "${ZELLIJ_ENABLED:-}" = "1" ] && ! command -v zellij >/dev/null 2>&1; then
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
    # Fix SSH agent socket ownership for the non-root user
    if [ -n "${SSH_AUTH_SOCK:-}" ] && [ -S "${SSH_AUTH_SOCK}" ]; then
        chown kilo-t8x3m7kp "${SSH_AUTH_SOCK}" 2>/dev/null || true
    fi
    # Pre-populate known_hosts to avoid interactive prompts
    mkdir -p /home/kilo-t8x3m7kp/.ssh
    chmod 700 /home/kilo-t8x3m7kp/.ssh
    ssh-keyscan -H github.com gitlab.com bitbucket.com >> /home/kilo-t8x3m7kp/.ssh/known_hosts 2>/dev/null || true
    mkdir -p /home/kilo-t8x3m7kp/.config/kilo/commands /home/kilo-t8x3m7kp/.config/kilo/agents \
             /home/kilo-t8x3m7kp/.config/kilo/plugins /home/kilo-t8x3m7kp/.config/kilo/skills \
             /home/kilo-t8x3m7kp/.config/kilo/tools /home/kilo-t8x3m7kp/.config/kilo/rules
    chown -R kilo-t8x3m7kp:kilo-t8x3m7kp /home/kilo-t8x3m7kp/.ssh /home/kilo-t8x3m7kp/.config \
             /home/kilo-t8x3m7kp/.local /workspace
    if [ "${KD_AINSTRUCT_ENABLED:-}" = "1" ]; then
        su-exec kilo-t8x3m7kp sh -c 'exec ainstruct-sync &'
        echo "[kilo-docker] Ainstruct sync started" >&2
    fi
    exec su-exec kilo-t8x3m7kp "$0" "$@"
fi

. "$SCRIPT_DIR/setup-kilo-config.sh"

if [ "${ZELLIJ_ENABLED:-}" = "1" ]; then
    mkdir -p "$HOME/.config/zellij"
    if [ ! -f "$HOME/.config/zellij/config.kdl" ]; then
        cp /etc/zellij/config.kdl "$HOME/.config/zellij/config.kdl"
    fi
fi

if [ $# -eq 0 ]; then
    exec sh
else
    exec "$@"
fi
