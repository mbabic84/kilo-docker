#!/bin/bash
# Wrapper script that loads tokens from encrypted storage before exec-ing real kilo
# This ensures tokens are available automatically without user configuration

# Shared logging function (mimics kilo-docker logging format)
log() {
    echo "[kilo-wrapper] $*" >&2
}

# Check if we need to skip token loading (e.g., if called from kilo-entrypoint)
if [ "$1" = "--no-token-load" ]; then
    shift
    exec /usr/local/bin/kilo-real "$@"
fi

# Load tokens via kilo-entrypoint print-env if available
log "Loading tokens..."
if command -v kilo-entrypoint &>/dev/null; then
    env_output=$(kilo-entrypoint print-env 2>/dev/null || echo "")
    if [ -n "$env_output" ]; then
        log "Tokens loaded, setting environment..."
        eval "$env_output"
        log "KD_MCP_CONTEXT7_TOKEN=${KD_MCP_CONTEXT7_TOKEN:+[set]} [${#KD_MCP_CONTEXT7_TOKEN} chars]"
        log "KD_MCP_AINSTRUCT_TOKEN=${KD_MCP_AINSTRUCT_TOKEN:+[set]} [${#KD_MCP_AINSTRUCT_TOKEN} chars]"
    else
        log "No tokens found in storage"
    fi
fi

# Sync MCP config to enable/disable servers based on loaded tokens
log "Syncing MCP config..."
if command -v kilo-entrypoint &>/dev/null; then
    kilo-entrypoint mcp-config 2>/dev/null || true
fi

log "Starting kilo..."
exec /usr/local/bin/kilo-real "$@"
