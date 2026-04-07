#!/bin/bash
# Wrapper script that loads tokens from encrypted storage before exec-ing real kilo
# This ensures tokens are available automatically without user configuration
#
# Security: Environment variables are set ONLY immediately before Kilo starts,
# minimizing token exposure in the shell environment.

# Shared logging function (mimics kilo-docker logging format)
log() {
    echo "[kilo-wrapper] $*" >&2
}

# Check if we need to skip token loading (e.g., if called from kilo-entrypoint)
if [ "$1" = "--no-token-load" ]; then
    shift
    exec /usr/local/bin/kilo-real "$@"
fi

# Step 1: Apply MCP enabled states by reading from encrypted storage
# This updates opencode.json before Kilo starts
# NOTE: See docs/MCP_ENABLED_KNOWN_ISSUE.md for details about container-specific issue
log "Applying MCP enabled states..."
if command -v kilo-entrypoint &>/dev/null; then
    kilo-entrypoint mcp-config || true
fi

# Step 2: Load tokens from encrypted storage
# These are NOT exported yet - they'll only be set for the Kilo process
log "Loading tokens..."
env_output=""
if command -v kilo-entrypoint &>/dev/null; then
    env_output=$(kilo-entrypoint print-env 2>/dev/null || echo "")
    if [ -n "$env_output" ]; then
        log "Tokens loaded, will set environment before starting Kilo"
    else
        log "No tokens found in storage"
    fi
fi

# Step 3: Start Kilo with tokens set in environment
# This is the ONLY place where KD_MCP_* env vars are exported
# Using bash -c ensures tokens are only available to Kilo, not the wrapper shell
log "Starting kilo..."
if [ -n "$env_output" ]; then
    # Export tokens and exec Kilo in a single step
    # The bash -c creates a minimal shell, eval sets the exports, then exec replaces with Kilo
    exec bash -c '
        eval "$1"
        shift
        exec "$@"
    ' _ "$env_output" /usr/local/bin/kilo-real "$@"
else
    exec /usr/local/bin/kilo-real "$@"
fi
