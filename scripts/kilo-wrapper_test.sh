#!/bin/bash
# Plain-Bash test harness for scripts/kilo-wrapper-lib.sh.
#
# Verifies the three things that matter:
#   1. TMPDIR/TMP/TEMP are exported pointing to <cwd>/tmp.
#   2. <cwd>/tmp is created on demand.
#   3. <repo>/.gitignore ends up with a tmp/ entry (idempotently).
#
# The wrapper at scripts/kilo-wrapper.sh is a thin shell that sources
# the lib and calls setup_kilo_tmpdir in all three of its exec paths,
# so verifying the lib's behaviour is sufficient.
#
# Run from the repo root:
#   ./scripts/kilo-wrapper_test.sh

set -eu

REPO_ROOT="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
LIB="$REPO_ROOT/scripts/kilo-wrapper-lib.sh"

TEST_ROOT="$REPO_ROOT/tmp/wrapper-test"
rm -rf "$TEST_ROOT"
mkdir -p "$TEST_ROOT"

PASS=0
FAIL=0

assert_eq() {
    local label="$1" expected="$2" actual="$3"
    if [ "$expected" = "$actual" ]; then
        printf '  ok  %s\n' "$label"
        PASS=$((PASS + 1))
    else
        printf '  FAIL %s — expected %q got %q\n' "$label" "$expected" "$actual"
        FAIL=$((FAIL + 1))
        return 1
    fi
}

assert_file_exists() {
    local label="$1" path="$2"
    if [ -e "$path" ]; then
        printf '  ok  %s\n' "$label"
        PASS=$((PASS + 1))
    else
        printf '  FAIL %s — %s does not exist\n' "$label" "$path"
        FAIL=$((FAIL + 1))
        return 1
    fi
}

assert_file_contains() {
    local label="$1" path="$2" needle="$3"
    if [ -f "$path" ] && grep -Fq "$needle" -- "$path"; then
        printf '  ok  %s\n' "$label"
        PASS=$((PASS + 1))
    else
        printf '  FAIL %s — %q not in %s\n' "$label" "$needle" "$path"
        FAIL=$((FAIL + 1))
        return 1
    fi
}

# --- case 1: ENVs set + ./tmp created ---------------------------------

printf '\n=== ENVs (TMPDIR/TMP/TEMP) point to <cwd>/tmp and dir is created ===\n'
do_case1() {
    local cwd="$TEST_ROOT/01-env/work"
    mkdir -p "$cwd"
    (
        cd "$cwd"
        # shellcheck source=/dev/null
        . "$LIB"
        setup_kilo_tmpdir
        printf 'TMPDIR=%s\nTMP=%s\nTEMP=%s\n' "$TMPDIR" "$TMP" "$TEMP"
    ) > "$TEST_ROOT/01-env/out"

    assert_eq "TMPDIR=<cwd>/tmp" "$cwd/tmp" "$(grep '^TMPDIR=' "$TEST_ROOT/01-env/out" | cut -d= -f2-)"
    assert_eq "TMP=<cwd>/tmp"    "$cwd/tmp" "$(grep '^TMP='    "$TEST_ROOT/01-env/out" | cut -d= -f2-)"
    assert_eq "TEMP=<cwd>/tmp"   "$cwd/tmp" "$(grep '^TEMP='   "$TEST_ROOT/01-env/out" | cut -d= -f2-)"
    assert_file_exists "<cwd>/tmp created" "$cwd/tmp"
}
do_case1

# --- case 2: git repo gets tmp/ in .gitignore (idempotent) ------------

printf '\n=== git repo: tmp/ appended to .gitignore, idempotent on rerun ===\n'
do_case2() {
    local cwd="$TEST_ROOT/02-gitignore/repo"
    mkdir -p "$cwd"
    ( cd "$cwd" && git init -q )

    # First invocation: creates the entry and the tmp dir.
    ( cd "$cwd" && . "$LIB" && setup_kilo_tmpdir ) >/dev/null
    assert_file_exists  "tmp/ created"       "$cwd/tmp"
    assert_file_exists  ".gitignore created" "$cwd/.gitignore"
    assert_file_contains "tmp/ listed"       "$cwd/.gitignore" "tmp/"

    # Second invocation: must not add a duplicate tmp/ line.
    ( cd "$cwd" && . "$LIB" && setup_kilo_tmpdir ) >/dev/null
    local count
    count="$(grep -cE '^tmp/$' -- "$cwd/.gitignore")"
    assert_eq "exactly one tmp/ line after re-run" "1" "$count"
}
do_case2

# --- summary ----------------------------------------------------------

printf '\n----------------------------------------\n'
printf 'PASS: %d   FAIL: %d\n' "$PASS" "$FAIL"
[ "$FAIL" -eq 0 ]
