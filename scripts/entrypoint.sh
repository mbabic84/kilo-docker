#!/bin/sh
set -e

# Ensure terminal capability variables are set for TUI formatting
# Without these, the TUI falls back to markdown-style rendering (e.g., "_Thinking:_")
export TERM="${TERM:-xterm-256color}"
export COLORTERM="${COLORTERM:-truecolor}"

TOKEN_FILE="/home/user/.local/share/kilo/.tokens.env"

if [ -f "$TOKEN_FILE" ]; then
    . "$TOKEN_FILE"
    export CONTEXT7_TOKEN
    export AINSTRUCT_TOKEN
fi

exec kilo "$@"
