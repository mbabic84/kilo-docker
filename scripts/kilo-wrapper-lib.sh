#!/bin/bash
# Shared library for scripts/kilo-wrapper.sh.
#
# Centralises the kilo-docker tmp-dir policy:
#   1. Resolve <real_cwd>/tmp and create it if missing.
#   2. Ensure the worktree-local .gitignore covers it (idempotent,
#      never auto-commits, never runs `git add`).
#   3. Export TMPDIR/TMP/TEMP for the Kilo process.
#
# The lib is sourced from the wrapper (call site 1) and from inside the
# `bash -c` eval-and-exec child (call site 2, after `eval "$1"`). The
# second path guarantees the policy is applied last so it cannot be
# silently overridden by a custom encrypted env named TMPDIR.

# Emit a wrapper-style log line. Output is to stderr so it is visible
# to users in the terminal and not captured by Kilo's stdout.
log() {
    echo "[kilo-wrapper] $*" >&2
}

# Resolve the real (symlink-free) cwd. Pure Bash; no `realpath`
# dependency. `pwd -P` was chosen over `realpath "$PWD"` so the
# wrapper has no external-binary requirement.
resolve_real_cwd() {
    pwd -P 2>/dev/null || pwd
}

# Detect git worktree membership. Live-validated:
#   cwd inside a git repo  -> toplevel of that repo
#   cwd in linked worktree -> toplevel of the worktree (not main repo)
#   cwd in a submodule    -> submodule path (not superproject)
#   cwd not in git        -> empty string
detect_git_toplevel() {
    local real_cwd="$1"
    command -v git >/dev/null 2>&1 || return 1
    git -C "$real_cwd" rev-parse --show-toplevel 2>/dev/null
}

# Append "tmp/" to <repo>/.gitignore if not already covered by some
# variant of the same pattern. Idempotent, never commits, never runs
# `git add`. Read-only FS / permission errors are logged but do not
# abort the wrapper so Kilo still runs.
ensure_tmp_gitignore() {
    local repo_root="$1"
    local gi="$repo_root/.gitignore"
    local marker="tmp/"

    # Idempotency: match any of `tmp/`, `tmp`, `/tmp/`, `/tmp`,
    # `./tmp/`, `./tmp`, and nested variants. The regex intentionally
    # tolerates .gitignore comments containing the word `tmp/`; those
    # still satisfy the user's intent to ignore `tmp/`.
    if [ -f "$gi" ] && grep -Eq '(^|/|\\.)tmp(/|$)' -- "$gi" 2>/dev/null; then
        return 0
    fi

    # Append via mktemp + mv so concurrent kilo invocations cannot
    # interleave on the same .gitignore. mktemp is POSIX on
    # debian:bookworm-slim; on a read-only FS it fails and we fall
    # back to logging a warning without aborting.
    local tmp_gi
    if ! tmp_gi="$(mktemp "${repo_root}/.gitignore.kiloXXXXXX" 2>/dev/null)"; then
        log "Cannot create $gi staging file (read-only FS?); tmp/ will not be gitignored"
        return 0
    fi

    # Build the new .gitignore contents in the staging file using only
    # explicit per-line appends. This avoids the shellcheck-confusing
    # pattern of a `} > "$tmp_gi"` redirect whose block also writes to
    # `$gi` (`: > "$gi"`). Reads touch `$gi`; writes touch `$tmp_gi`
    # exclusively, which shellcheck can verify.
    if ! : > "$tmp_gi" 2>/dev/null; then
        rm -f -- "$tmp_gi"
        log "Cannot truncate staging file; tmp/ will not be gitignored"
        return 0
    fi

    if [ -f "$gi" ]; then
        if ! cat -- "$gi" >> "$tmp_gi" 2>/dev/null; then
            rm -f -- "$tmp_gi"
            log "Cannot read existing $gi; tmp/ will not be gitignored"
            return 0
        fi
        # Ensure a separator newline between pre-existing content and
        # the marker, even if $gi didn't end with one.
        if [ -s "$gi" ] && [ "$(tail -c1 -- "$gi")" != $'\n' ]; then
            if ! printf '\n' >> "$tmp_gi" 2>/dev/null; then
                rm -f -- "$tmp_gi"
                log "Cannot stage separator newline; tmp/ will not be gitignored"
                return 0
            fi
        fi
    fi

    if ! printf '# Kilo CLI workspace temp dir (managed by kilo-docker wrapper)\n' >> "$tmp_gi" 2>/dev/null \
        || ! printf '%s\n' "$marker" >> "$tmp_gi" 2>/dev/null; then
        rm -f -- "$tmp_gi"
        log "Cannot stage $gi update; tmp/ will not be gitignored"
        return 0
    fi

    if ! mv -f -- "$tmp_gi" "$gi" 2>/dev/null; then
        rm -f -- "$tmp_gi"
        log "Cannot write $gi (read-only or permission denied); tmp/ will not be gitignored"
        return 0
    fi
    log "Added '$marker' to $gi"
    return 0
}

# Resolve the workspace-local tmpdir, mkdir it, optionally update the
# worktree's .gitignore, and export TMPDIR/TMP/TEMP for the calling
# shell. The exports take effect in the caller (the wrapper), and
# because the wrapper exec's the Kilo process they propagate.
setup_kilo_tmpdir() {
    local real_cwd
    real_cwd="$(resolve_real_cwd)"

    # Pathological cwd (image default = /) -> keep upstream behavior.
    # Without this guard, "cwd + /tmp" produces "//tmp" and
    # Kilo's Path.tmp becomes "/kilo" (EACCES).
    local tmpdir
    if [ "$real_cwd" = "/" ]; then
        tmpdir="/tmp"
    else
        tmpdir="$real_cwd/tmp"
    fi

    if ! mkdir -p -- "$tmpdir" 2>/dev/null; then
        log "Cannot create $tmpdir; falling back to /tmp"
        tmpdir="/tmp"
    fi

    # Idempotent gitignore update. .gitignore is project state — the
    # wrapper never stages or commits it. The change shows up in
    # `git status` for the user to review.
    local repo_root
    if repo_root="$(detect_git_toplevel "$real_cwd")" && [ -n "$repo_root" ]; then
        ensure_tmp_gitignore "$repo_root" || true
    fi

    export TMPDIR="$tmpdir"
    export TMP="$tmpdir"
    export TEMP="$tmpdir"
}
