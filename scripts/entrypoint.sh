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
    if [ "${KD_AINSTRUCT_ENABLED:-}" = "1" ] && ! command -v inotifywait >/dev/null 2>&1; then
        echo "[kilo-docker] Installing inotify-tools..." >&2
        apk add --no-cache inotify-tools
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

# --- Ainstruct file watcher ---
if [ "${KD_AINSTRUCT_ENABLED:-}" = "1" ]; then
    mkdir -p "$HOME/.config/kilo"
    mkdir -p "$HOME/.kilo/command" "$HOME/.kilo/agent"

    KD_AINSTRUCT_API_URL="${KD_AINSTRUCT_API_URL:-https://ainstruct-dev.kralicinora.cz/api/v1}"
    AINSTRUCT_COLLECTION_NAME="kilo-docker"
    AINSTRUCT_COLLECTION_ID=""
    AINSTRUCT_AUTH_EXPIRED=""
    AINSTRUCT_HASH_FILE="$HOME/.config/kilo/.ainstruct-hashes"

    ainstruct_hash_get() {
        local relpath="$1"
        grep -F "${relpath}=" "$AINSTRUCT_HASH_FILE" 2>/dev/null | tail -1 | cut -d= -f2-
    }

    ainstruct_hash_set() {
        local relpath="$1" hash="$2"
        mkdir -p "$(dirname "$AINSTRUCT_HASH_FILE")"
        if grep -qF "${relpath}=" "$AINSTRUCT_HASH_FILE" 2>/dev/null; then
            local tmp="${AINSTRUCT_HASH_FILE}.tmp"
            grep -vF "${relpath}=" "$AINSTRUCT_HASH_FILE" > "$tmp" 2>/dev/null
            echo "${relpath}=${hash}" >> "$tmp"
            mv "$tmp" "$AINSTRUCT_HASH_FILE"
        else
            echo "${relpath}=${hash}" >> "$AINSTRUCT_HASH_FILE"
        fi
    }

    ainstruct_hash_delete() {
        local relpath="$1"
        if [ -f "$AINSTRUCT_HASH_FILE" ]; then
            local tmp="${AINSTRUCT_HASH_FILE}.tmp"
            grep -vF "${relpath}=" "$AINSTRUCT_HASH_FILE" > "$tmp" 2>/dev/null
            mv "$tmp" "$AINSTRUCT_HASH_FILE"
        fi
    }

    # Refresh JWT access token. Updates KD_AINSTRUCT_SYNC_TOKEN in place.
    ainstruct_refresh_token() {
        if [ -z "${KD_AINSTRUCT_SYNC_REFRESH_TOKEN:-}" ]; then
            echo "[ainstruct-sync] No refresh token available" >&2
            return 1
        fi
        local refresh_body response new_access new_refresh new_expires_in
        refresh_body=$(jq -n --arg rt "$KD_AINSTRUCT_SYNC_REFRESH_TOKEN" '{"refresh_token": $rt}')
        response=$(curl -s -X POST "${KD_AINSTRUCT_API_URL}/auth/refresh" \
            -H "Content-Type: application/json" \
            -d "$refresh_body")
        new_access=$(echo "$response" | jq -r '.access_token // empty' 2>/dev/null)
        new_refresh=$(echo "$response" | jq -r '.refresh_token // empty' 2>/dev/null)
        new_expires_in=$(echo "$response" | jq -r '.expires_in // empty' 2>/dev/null)
        if [ -n "$new_access" ]; then
            KD_AINSTRUCT_SYNC_TOKEN="$new_access"
            if [ -n "$new_refresh" ]; then
                KD_AINSTRUCT_SYNC_REFRESH_TOKEN="$new_refresh"
            fi
            if [ -n "$new_expires_in" ]; then
                KD_AINSTRUCT_SYNC_TOKEN_EXPIRY="$(($(date +%s) + new_expires_in))"
            fi
            echo "[ainstruct-sync] Token refreshed" >&2
            return 0
        else
            echo "[ainstruct-sync] Token refresh failed" >&2
            return 1
        fi
    }

    # Ensure JWT is valid before an API call. Refreshes if within 60s of expiry.
    ainstruct_ensure_token() {
        if [ -z "${KD_AINSTRUCT_SYNC_TOKEN_EXPIRY:-}" ]; then
            return 0
        fi
        local now remaining
        now=$(date +%s)
        remaining=$((KD_AINSTRUCT_SYNC_TOKEN_EXPIRY - now))
        if [ "$remaining" -lt 60 ]; then
            ainstruct_refresh_token || return 1
        fi
    }

    ainstruct_api() {
        local method="$1" endpoint="$2" body="$3" response
        ainstruct_ensure_token || {
            AINSTRUCT_AUTH_EXPIRED="1"
            return 1
        }
        if [ -n "$body" ]; then
            response=$(curl -s -X "$method" "${KD_AINSTRUCT_API_URL}${endpoint}" \
                -H "Authorization: Bearer ${KD_AINSTRUCT_SYNC_TOKEN}" \
                -H "Content-Type: application/json" \
                -d "$body")
        else
            response=$(curl -s -X "$method" "${KD_AINSTRUCT_API_URL}${endpoint}" \
                -H "Authorization: Bearer ${KD_AINSTRUCT_SYNC_TOKEN}" \
                -H "Content-Type: application/json")
        fi
        if echo "$response" | grep -q '"INVALID_TOKEN"'; then
            if ainstruct_refresh_token; then
                if [ -n "$body" ]; then
                    response=$(curl -s -X "$method" "${KD_AINSTRUCT_API_URL}${endpoint}" \
                        -H "Authorization: Bearer ${KD_AINSTRUCT_SYNC_TOKEN}" \
                        -H "Content-Type: application/json" \
                        -d "$body")
                else
                    response=$(curl -s -X "$method" "${KD_AINSTRUCT_API_URL}${endpoint}" \
                        -H "Authorization: Bearer ${KD_AINSTRUCT_SYNC_TOKEN}" \
                        -H "Content-Type: application/json")
                fi
                if echo "$response" | grep -q '"INVALID_TOKEN"'; then
                    echo "[ainstruct-sync] Token invalid after refresh — stopping watcher" >&2
                    AINSTRUCT_AUTH_EXPIRED="1"
                    return 1
                fi
            else
                echo "[ainstruct-sync] Token invalid — stopping watcher" >&2
                AINSTRUCT_AUTH_EXPIRED="1"
                return 1
            fi
        fi
        printf '%s' "$response"
    }

    ainstruct_ensure_collection() {
        if [ -n "$AINSTRUCT_COLLECTION_ID" ]; then
            return 0
        fi
        local collections response
        collections=$(ainstruct_api GET "/collections")
        AINSTRUCT_COLLECTION_ID=$(echo "$collections" | jq -r --arg n "$AINSTRUCT_COLLECTION_NAME" '.collections[] | select(.name == $n) | .collection_id // empty' 2>/dev/null | head -1)
        if [ -z "$AINSTRUCT_COLLECTION_ID" ]; then
            local create_body
            create_body=$(jq -n --arg n "$AINSTRUCT_COLLECTION_NAME" '{"name": $n}')
            response=$(ainstruct_api POST "/collections" "$create_body")
            AINSTRUCT_COLLECTION_ID=$(echo "$response" | jq -r '.collection_id // empty' 2>/dev/null)
        fi
        if [ -n "$AINSTRUCT_COLLECTION_ID" ]; then
            echo "[ainstruct-sync] Collection ready: $AINSTRUCT_COLLECTION_ID" >&2
        else
            echo "[ainstruct-sync] Failed to initialize collection" >&2
            return 1
        fi
    }

    ainstruct_get_document_by_path() {
        local relpath="$1"
        local documents
        documents=$(ainstruct_api GET "/documents?collection_id=${AINSTRUCT_COLLECTION_ID}")
        echo "$documents" | jq -r --arg p "$relpath" \
            '.documents[] | select(.metadata.local_path == $p) | .document_id // empty' 2>/dev/null | head -1
    }

    ainstruct_sync_file() {
        local filepath="$1"
        local relpath="${filepath#$HOME/}"
        local basename title content existing_id response

        if [ ! -f "$filepath" ]; then
            return 0
        fi

        ainstruct_ensure_collection || return 1

        basename=$(basename "$filepath")
        title="$basename"
        content=$(cat "$filepath" 2>/dev/null)

        existing_id=$(ainstruct_get_document_by_path "$relpath")
        [ "$AINSTRUCT_AUTH_EXPIRED" = "1" ] && return 1

        local new_hash
        if [ -n "$existing_id" ]; then
            local update_body
            update_body=$(jq -n --arg c "$content" '{"content": $c}')
            response=$(ainstruct_api PATCH "/documents/${existing_id}" "$update_body")
            [ "$AINSTRUCT_AUTH_EXPIRED" = "1" ] && return 1
            new_hash=$(echo "$response" | jq -r '.content_hash // empty' 2>/dev/null)
            echo "[ainstruct-sync] Updated: $relpath" >&2
        else
            local create_body
            create_body=$(jq -n \
                --arg t "$title" \
                --arg c "$content" \
                --arg dt "markdown" \
                --arg cid "$AINSTRUCT_COLLECTION_ID" \
                --arg lp "$relpath" \
                '{"title": $t, "content": $c, "document_type": $dt, "collection_id": $cid, "metadata": {"local_path": $lp}}')
            response=$(ainstruct_api POST "/documents" "$create_body")
            [ "$AINSTRUCT_AUTH_EXPIRED" = "1" ] && return 1
            new_hash=$(echo "$response" | jq -r '.content_hash // empty' 2>/dev/null)
            echo "[ainstruct-sync] Created: $relpath" >&2
        fi

        if [ -n "$new_hash" ]; then
            ainstruct_hash_set "$relpath" "$new_hash"
        fi
    }

    ainstruct_delete_by_path() {
        local relpath="$1"
        ainstruct_ensure_collection || return 1
        local existing_id
        existing_id=$(ainstruct_get_document_by_path "$relpath")
        [ "$AINSTRUCT_AUTH_EXPIRED" = "1" ] && return 1
        if [ -n "$existing_id" ]; then
            ainstruct_api DELETE "/documents/${existing_id}"
            [ "$AINSTRUCT_AUTH_EXPIRED" = "1" ] && return 1
            ainstruct_hash_delete "$relpath"
            echo "[ainstruct-sync] Deleted: $relpath" >&2
        fi
    }

    ainstruct_pull_collection() {
        local collections documents
        collections=$(ainstruct_api GET "/collections")
        AINSTRUCT_COLLECTION_ID=$(echo "$collections" | jq -r --arg n "$AINSTRUCT_COLLECTION_NAME" '.collections[] | select(.name == $n) | .collection_id // empty' 2>/dev/null | head -1)
        if [ -z "$AINSTRUCT_COLLECTION_ID" ]; then
            echo "[ainstruct-sync] No existing collection — nothing to pull" >&2
            return 0
        fi
        echo "[ainstruct-sync] Pulling documents from collection $AINSTRUCT_COLLECTION_ID" >&2
        documents=$(ainstruct_api GET "/documents?collection_id=${AINSTRUCT_COLLECTION_ID}")
        local count
        count=$(echo "$documents" | jq -r '.documents | length' 2>/dev/null)
        if [ "$count" = "0" ] || [ -z "$count" ]; then
            echo "[ainstruct-sync] Collection is empty — nothing to pull" >&2
            return 0
        fi
        echo "$documents" | jq -c '.documents[]' 2>/dev/null | while read -r doc; do
            local doc_id relpath api_hash stored_hash abspath dir
            doc_id=$(echo "$doc" | jq -r '.document_id // empty')
            relpath=$(echo "$doc" | jq -r '.metadata.local_path // empty')
            api_hash=$(echo "$doc" | jq -r '.content_hash // empty')
            [ -z "$relpath" ] && continue
            stored_hash=$(ainstruct_hash_get "$relpath")
            if [ "$stored_hash" = "$api_hash" ]; then
                continue
            fi
            local doc_full content
            doc_full=$(ainstruct_api GET "/documents/${doc_id}")
            content=$(echo "$doc_full" | jq -r '.content // empty' 2>/dev/null)
            [ -z "$content" ] && continue
            abspath="${HOME}/${relpath}"
            dir=$(dirname "$abspath")
            mkdir -p "$dir"
            printf '%s' "$content" > "$abspath"
            ainstruct_hash_set "$relpath" "$api_hash"
            echo "[ainstruct-sync] Pulled: $relpath" >&2
        done
    }

    ainstruct_pull_collection

    LAST_SYNC=0
    AINSTRUCT_WATCH_PATHS="$HOME/.config/kilo/rules/ $HOME/.kilo/command/ $HOME/.kilo/agent/"
    if [ -f "$HOME/.config/kilo/opencode.json" ]; then
        AINSTRUCT_WATCH_PATHS="$HOME/.config/kilo/opencode.json $AINSTRUCT_WATCH_PATHS"
    fi
    (
        exec inotifywait -m -e modify,create,delete,move \
            $AINSTRUCT_WATCH_PATHS 2>/dev/null | \
        while read -r directory event filename; do
            if [ "$AINSTRUCT_AUTH_EXPIRED" = "1" ]; then
                echo "[ainstruct-sync] Watcher stopped due to expired auth token" >&2
                break
            fi
            NOW=$(date +%s)
            if [ $((NOW - LAST_SYNC)) -lt 5 ]; then
                sleep 5
            fi
            LAST_SYNC=$(date +%s)

            filepath="${directory}${filename}"
            relpath="${filepath#$HOME/}"

            case "$event" in
                DELETE|MOVED_FROM)
                    ainstruct_delete_by_path "$relpath" 2>/dev/null || true
                    ;;
                *)
                    [ -f "$filepath" ] && ainstruct_sync_file "$filepath" 2>/dev/null || true
                    ;;
            esac
        done
    ) &
    echo "[kilo-docker] Ainstruct file watcher started (PID: $!)" >&2
fi

if [ "${ZELLIJ_ENABLED:-}" = "1" ]; then
    mkdir -p "$HOME/.config/zellij"
    if [ ! -f "$HOME/.config/zellij/config.kdl" ]; then
        cp /etc/zellij/config.kdl "$HOME/.config/zellij/config.kdl"
    fi
    exec zellij "$@"
fi

# If no arguments passed, start Kilo (interactive mode by default)
# Otherwise pass through to allow testing with custom commands
if [ $# -eq 0 ]; then
    exec kilo
else
    exec "$@"
fi
