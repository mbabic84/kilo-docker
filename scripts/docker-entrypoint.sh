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

CONFIG="/home/kilo/.config/kilo/opencode.json"

# Disable MCP servers whose required token env vars are not set.
# Mapping: MCP server key → required env var name.
# Uses jq's env object — undefined vars evaluate to null.
# To add a new MCP server, add its key and env var to the $mapping object.
if [ -f "$CONFIG" ]; then
    jq '
      {"ainstruct":"AINSTRUCT_TOKEN","context7":"CONTEXT7_TOKEN"} as $mapping |
      .mcp |= with_entries(
        if $mapping[.key] and (env[$mapping[.key]] // null) == null then
          .value.enabled = false
        else
          .
        end
      )
    ' "$CONFIG" > "${CONFIG}.tmp" && mv "${CONFIG}.tmp" "$CONFIG"
fi

exec kilo "$@"
