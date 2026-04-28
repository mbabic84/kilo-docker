# Plan: Fail Fast On Inaccessible Workspace During Reattach

## Context
- `kilo-docker` creates containers with `-w <workspace>` so `docker exec` enters the mounted project directory.
- `kilo-entrypoint zellij-attach` then drops from root to the mapped host user and execs `zellij attach --create kilo-docker`.
- On some remote hosts, the mounted workspace is accessible only through supplementary groups such as `docker`, so the reattach path can lose access after dropping from root to the mapped user.

## Changes
1. Update `cmd/kilo-entrypoint/zellijattach.go` to restore supplementary groups during the reattach privilege-drop path, matching the initialization flow.
2. Validate the inherited working directory after privileges are dropped.
3. If the workspace is inaccessible, return an explicit error so the entrypoint prints it and exits instead of letting zellij fail later.
4. Log group restoration and confirmed working directory decisions so the reason is visible in the persistent kilo log without exposing sensitive data.

## Files Modified
| File | Change |
|------|--------|
| cmd/kilo-entrypoint/zellijattach.go | Restore supplementary groups on reattach and fail fast if the workspace is inaccessible |
| plans/fix-zellij-cwd-permission-denied.md | Document the bug, fix, and verification |

## Verification
- Run `go test ./...`.
- Run `go vet ./...`.
- Confirm the entrypoint package builds and the new guard compiles.
- Validate behavior conceptually: when workspace access depends on supplementary groups, reattach preserves those groups during zellij startup; if the workspace is still inaccessible, the entrypoint prints an explicit error and exits.
