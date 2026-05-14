# Plan: Fix session attach for exited containers

## Context
When running `kilo-docker sessions <target>` to attach to an exited container, the code does `docker start -d` then immediately `docker exec`, but:
- The error from `docker start` is silently discarded
- The container may start but immediately exit again (e.g., if `execZellij()` fails inside the entrypoint)
- `docker exec` then fails with "container is not running"

Same issue exists in `runContainer()` in `main.go` for the equivalent code path.

## Changes
1. Add a `waitForContainerRunning()` helper that polls container state after start
2. Update both `handle_sessions.go` and `main.go` to check `docker start` errors and wait for container to be running before exec-ing

## Files Modified
| File | Change |
|------|--------|
| `cmd/kilo-docker/docker.go` | Add `waitForContainerRunning()` helper |
| `cmd/kilo-docker/handle_sessions.go` | Check `docker start` error, add wait loop before exec |
| `cmd/kilo-docker/main.go` | Same fix in `runContainer()` |

## Verification
```bash
go build ./... && go test ./... && go vet ./...
```