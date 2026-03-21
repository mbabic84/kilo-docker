#!/bin/sh
set -e

TOKEN_FILE="/home/user/.local/share/kilo/.tokens.env"

if [ -f "$TOKEN_FILE" ]; then
    . "$TOKEN_FILE"
    export CONTEXT7_TOKEN
    export AINSTRUCT_TOKEN
fi

exec kilo "$@"
