# Plan: Workflow Improvements

## Context
Review of `.github/workflows/` identified several issues: missing error handling, potential script injection, no concurrency controls, inconsistent image tags, and narrow test scope.

## Changes

### 1. `check-kilo-version.yml` — Error handling + injection fix

**Line 22:** Add `-f` flag to `curl` and pipe through error check:
```yaml
LATEST=$(curl -sf https://api.github.com/repos/Kilo-Org/kilocode/releases/latest | jq -r '.tag_name' | sed 's/^v//')
if [ -z "$LATEST" ]; then
  echo "::error::Failed to fetch latest Kilo version"
  exit 1
fi
```

**Line 45:** Use environment variable instead of direct interpolation:
```yaml
- name: Update Dockerfile
  if: steps.compare.outputs.needs_update == 'true'
  env:
    NEW_VERSION: ${{ steps.latest.outputs.version }}
  run: |
    sed -i "s/ARG KILO_VERSION=.*/ARG KILO_VERSION=$NEW_VERSION/" Dockerfile
```

### 2. `ci.yml` — Concurrency + test scope + image tag alignment

**After line 6:** Add concurrency group:
```yaml
concurrency:
  group: ci-${{ github.ref }}
  cancel-in-progress: true
```

**Line 30:** Expand test scope:
```yaml
- name: Run tests
  run: go test ./...
```

**Line 40:** Use `github.repository` instead of hardcoded owner:
```yaml
tags: ghcr.io/${{ github.repository }}:latest
```

### 3. `release.yml` — Concurrency group

**After line 6:** Add concurrency group (never cancel a release):
```yaml
concurrency:
  group: release
  cancel-in-progress: false
```

## Files Modified

| File | Change |
|------|--------|
| `.github/workflows/check-kilo-version.yml` | Add curl error handling (`-sf` + empty check), use env var for sed to prevent injection |
| `.github/workflows/ci.yml` | Add concurrency group, expand test scope to `./...`, align image tag with `github.repository`, explicit `cache: true` on setup-go |
| `.github/workflows/release.yml` | Add concurrency group (`cancel-in-progress: false`), explicit `cache: true` on setup-go |

## Verification
- Review the diff to confirm all changes are syntactically valid YAML
- No test commands apply (workflow files), but can validate with `yamllint` or `actionlint` if available
