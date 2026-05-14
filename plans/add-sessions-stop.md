# Plan: Add `sessions stop` Command

## Context
When using `kilo-docker --ports` to map host ports into a container (e.g., `-p 8080:8080`), those ports remain bound to the running container until the container exits or is removed. Currently, the only way to free ports for reuse is:
- `sessions cleanup` — removes the container entirely (destructive, loses container state)
- `docker stop` / `docker kill` externally — not integrated into the CLI

A `sessions stop` command is needed to gracefully stop a running container, freeing its ports while preserving the container and volume for later restart via `kilo-docker sessions <name>`.

## Analysis: Port Conflict Handling (Current)

Container names are derived from workspace (`deriveContainerName`), not from ports. When two different workspaces try to use the same host port:

1. `runContainer` at `main.go:132` derives a *different* container name for workspace B
2. `dockerState()` returns `"not_found"` for that name → no collision detected
3. Falls to `default` case at `main.go:225-232` → `docker run -d -p 8080:8080`
4. Docker rejects with `"port is already allocated"` — the error is blindly logged via `LogError` at line 231 and exits with code 1

**No pre-flight check exists.** The user gets a raw Docker error with no suggestion about which session holds the port. Since `kilo.args` labels on containers contain port info, we can detect conflicts proactively.

## Changes

### 1. `cmd/kilo-docker/handle_sessions.go` — Add `stop` subcommand
After `recreateMode` handling (~line 124), add a `stopMode` handling block:
- Parse `stop` subcommand, resolve target by name/index
- Only stop running containers (skip already stopped)
- Call `docker stop <container>` (sends SIGTERM, Docker's default 10s timeout)
- Print confirmation message indicating the container was stopped and can be reattached

### 2. `cmd/kilo-docker/setup.go` — Add help text
- Add `sessions stop` case to `printCommandHelp`
- Add `stop` to the `sessions` help header line

### 3. `cmd/kilo-docker/main.go` — Port conflict pre-flight check
In `runContainer`, before `buildContainerArgs` (~line 209), add a function that:
- Collects all kilo-docker sessions via `getSessions()`
- Parses each session's stored `kilo.args` label for `--port`/`-p` values
- For each port the current config requests, checks if any *other* running session has it mapped
- On conflict, prints a user-friendly message: "Port 8080 is in use by session 'X' (workspace /path). Stop it first: kilo-docker sessions stop <name>"
- Exits with code 1 (preventing the opaque Docker error)

### 3. Tests
- Unit tests for the port extraction and conflict-checking logic (parse port args from stored session data)
- Tests parse serialized args from sessions, extract ports, and verify conflict detection against config ports

## Changes

### 1. `cmd/kilo-docker/handle_sessions.go` — Add `stop` subcommand
After `recreateMode` handling (~line 124), add a `stopMode` handling block:
- Parse `stop` subcommand, resolve target by name/index
- Only stop running containers (skip already stopped)
- Call `docker stop <container>` (sends SIGTERM, Docker's default 10s timeout)
- Print confirmation message indicating the container was stopped and can be reattached

### 2. `cmd/kilo-docker/setup.go` — Add help text
- Add `sessions stop` case to `printCommandHelp`:
  ```
  Usage: kilo-docker sessions stop <name|index>
  
  Stop a running session, freeing its ports while preserving the container.
  
  Use 'kilo-docker sessions <name>' to restart later.
  ```
- Add `stop` to the `sessions` help header line as a listed subcommand (alongside `cleanup`, `recreate`)

### 3. Tests (optional — see Verification)
Since `docker stop` can't be unit-tested without Docker, the main verification is the build/test/lint cycle. The logic is minimal and follows the exact same `resolveTarget` → `dockerRun("stop", ...)` pattern used by `cleanup` with `dockerRun("rm", "-f", ...)`.

## Files Modified
| File | Change |
|------|--------|
| `cmd/kilo-docker/handle_sessions.go` | Add `stop` subcommand parsing and execution |
| `cmd/kilo-docker/setup.go` | Add `sessions stop` help text, update `sessions` help |
| `cmd/kilo-docker/main.go` | Add port conflict pre-flight check in `runContainer` |
| `cmd/kilo-docker/*_test.go` | Tests for port conflict detection logic |

## Verification
```bash
go build ./...
go test ./...
go vet ./...
golangci-lint run ./... --timeout=5m
```