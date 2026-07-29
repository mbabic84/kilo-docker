#!/bin/bash
# Wrapper script that loads tokens from encrypted storage before exec-ing real kilo
# This ensures tokens are available automatically without user configuration
#
# Security: Environment variables are set ONLY immediately before Kilo starts,
# minimizing token exposure in the shell environment.

# Tiny local log so warnings below remain functional even if the
# shared lib fails to source (e.g. missing install path). The shared
# lib also defines log() with identical semantics, which simply
# shadows this one.
log() {
    echo "[kilo-wrapper] $*" >&2
}

# Shared library containing the kilo-docker tmp-dir policy
# (workspace-local tmp + per-worktree .gitignore append).
# The lib is shipped to /usr/local/share/kilo/wrapper-lib.sh in the
# image; the sibling path is the fallback for local-dev runs and the
# wrapper test harness (./scripts/kilo-wrapper_test.sh).
SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck disable=SC1090
if [ -f /usr/local/share/kilo/wrapper-lib.sh ]; then
    . /usr/local/share/kilo/wrapper-lib.sh
elif [ -f "$SCRIPT_DIR/kilo-wrapper-lib.sh" ]; then
    . "$SCRIPT_DIR/kilo-wrapper-lib.sh"
else
    log "Cannot locate wrapper-lib.sh; tmpdir policy disabled"
fi

# Check if we need to skip token loading (e.g., if called from kilo-entrypoint)
if [ "$1" = "--no-token-load" ]; then
    shift
    setup_kilo_tmpdir
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
    # Export tokens, re-apply the kilo-docker tmp-dir policy last (so
    # user-supplied TMPDIR/TMP/TEMP cannot override the canonical
    # workspace-local tmpdir), then exec Kilo. The lib file shipped
    # at /usr/local/share/kilo/wrapper-lib.sh is reachable from the
    # bash -c child without inline copying.
    exec bash -c '
        eval "$1"
        shift
        . /usr/local/share/kilo/wrapper-lib.sh
        setup_kilo_tmpdir
        exec "$@"
    ' _ "$env_output" /usr/local/bin/kilo-real "$@"
else
    setup_kilo_tmpdir
    exec /usr/local/bin/kilo-real "$@"
fi
