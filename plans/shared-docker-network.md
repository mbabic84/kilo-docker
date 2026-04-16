# Plan: Shared Docker Network for Kilo Containers

## Context

Currently, Kilo containers default to Docker's `bridge` network, which means:
- Containers are isolated unless users pass explicit network flags
- Name-based container-to-container communication is inconsistent across sessions
- Sidecar-style workflows (for example, Playwright MCP sharing services/output) are harder to operate

Goal:
- Always attach Kilo containers to a shared network (`kilo-shared`)
- Preserve support for user-specified additional networks
- Standardize networking behavior across sessions
- Support a shared Playwright MCP container and shared output volume

Docker CLI validation (official docs):
- `docker run` supports repeating `--network` to connect to multiple networks at create time
- `docker network connect` can attach additional networks to existing/running containers

## Design Decisions

- Use a single constant source for shared resource names (`SharedNetworkName`, `PlaywrightVolumeName`)
- Always include `kilo-shared` as the first network when launching Kilo containers
- Keep `--network` repeatable for user-provided networks
- Normalize network lists during serialization/mismatch checks (dedupe, stable order, ignore implicit default)
- Manage shared Playwright container lifecycle explicitly (create-on-demand, reuse-if-running)

## Changes

### 1. Centralize shared constants
**Files**: `cmd/kilo-docker/network.go` (or shared constants file used by all call sites)

Define constants once and reference everywhere:

```go
const SharedNetworkName = "kilo-shared"
const PlaywrightVolumeName = "kilo-playwright-output"
const SharedPlaywrightContainerName = "kilo-playwright-mcp"
```

### 2. Update `--network` flag to support multiple values
**File**: `cmd/kilo-docker/flags.go`

Change config field from single network to slice:

```go
type config struct {
    // ... existing fields
    networks []string
}
```

Update `--network` flag handling to append values and serialize repeatably:

```go
{
    Names:       []string{"--network"},
    Description: "Connect to a Docker network (repeatable). 'kilo-shared' is always included.",
    setField:    func(c *config, v string) { c.networks = append(c.networks, v) },
    serializeArgs: func(c config) []string {
        normalized := normalizeNetworks(c.networks)
        return flatten("--network", normalized)
    },
    buildDockerArgs: func(c config) []string {
        normalized := normalizeNetworks(c.networks)
        return flatten("--network", normalized)
    },
}
```

`normalizeNetworks` behavior:
- Always include `SharedNetworkName`
- Remove duplicates
- Keep deterministic order (shared network first, then user-provided order)

### 3. Add idempotent ensure functions with wrapped errors
**File**: `cmd/kilo-docker/network.go`

Add:
- `EnsureSharedNetwork()`
- `EnsurePlaywrightVolume()`

Implementation requirements:
- Use inspect-first/create-if-missing behavior (idempotent)
- Wrap command failures with context (`fmt.Errorf("...: %w", err)`)
- Log warnings/errors via `utils.LogWarn` / `utils.LogError` in calling flow

### 4. Ensure shared resources in one orchestration point
**File**: `cmd/kilo-docker/main.go`

Before container creation:
- Call `EnsureSharedNetwork()` for all runs
- Call `EnsurePlaywrightVolume()` only when `--playwright` is enabled

Flow:
1. Parse flags
2. Ensure network/volume
3. Build args
4. Run container

Avoid duplicating ensure calls in both `main.go` and `args.go`.

### 5. Serialization and mismatch detection normalization
**File**: `cmd/kilo-docker/flags.go`

When comparing stored args vs current args:
- Treat `kilo-shared` as implicit default
- Normalize both sides with the same function
- Do not report mismatch if only difference is implicit shared network representation

### 6. Playwright integration: shared container + shared volume
**File**: `cmd/kilo-docker/playwright.go`

Changes:
1. Remove `--playwright`/`--network` mutual exclusivity
2. Use shared network model (container attached to `kilo-shared`)
3. Reuse existing `kilo-playwright-mcp` container when available
4. Use shared volume `kilo-playwright-output`
5. Keep `--isolated` behavior for per-session browser isolation

Lifecycle policy:
- If container exists and running: reuse
- If exists but stopped: start (or recreate if incompatible)
- If missing: create
- If image/config drift is detected: recreate with clear log message

### 7. Kilo container volume mount when Playwright enabled
**File**: `cmd/kilo-docker/args.go`

Only when `--playwright` is used:
- Mount `kilo-playwright-output` into Kilo container
- Use correct user-home target path format:
  - `/home/kd-<hash>/playwright-output`
- Preserve compatibility path/symlink behavior used by existing Playwright MCP integration

### 8. Optional cleanup command
**Files**: command routing file + network helpers

Add optional `kilo-docker networks cleanup` command with safe behavior:
- Check whether containers are attached before deletion
- Refuse destructive cleanup when resources are in use
- Print actionable message for manual detachment/stop sequence

## Files Modified

| File | Change |
|------|--------|
| `cmd/kilo-docker/flags.go` | Change `network` to `networks []string`; repeatable `--network`; normalization for build + serialization |
| `cmd/kilo-docker/network.go` | Shared constants; idempotent ensure helpers for network/volume |
| `cmd/kilo-docker/main.go` | Single orchestration point for ensure calls before container run |
| `cmd/kilo-docker/playwright.go` | Shared Playwright container lifecycle; remove network mutual exclusivity; shared volume integration |
| `cmd/kilo-docker/args.go` | Conditional Playwright output volume mount to Kilo container |
| `cmd/kilo-docker/<commands>.go` (optional) | Add `networks cleanup` command |

## Verification

1. Build:
```bash
go build -o bin/ ./...
```

2. Tests:
```bash
go test ./...
```

3. Lint:
```bash
golangci-lint run ./...
```

4. Manual networking checks:
- Run `kilo-docker` without `--network`
- Verify container is attached to `kilo-shared`:
  - `docker inspect <container> | jq .NetworkSettings.Networks`
- Run `kilo-docker --network my-network`
- Verify container is attached to both `kilo-shared` and `my-network`
- Run a second Kilo container and verify both share `kilo-shared`
- Verify container-to-container name resolution/connectivity:
  - `docker exec <container1> ping -c 1 <container2>`

5. Manual Playwright checks:
- Run `kilo-docker --playwright`
- Verify shared Playwright container exists (`kilo-playwright-mcp`)
- Run a second Kilo container with `--playwright` and verify Playwright container is reused
- Verify shared output path in Kilo container:
  - `/home/kd-<hash>/playwright-output`

6. Idempotency/error checks:
- Run Kilo multiple times and verify network/volume creation does not fail when resources already exist
- Simulate Docker failure (daemon down / denied permission) and verify wrapped error messages + `utils.LogWarn`/`utils.LogError` output
- Verify flag mismatch detection does not trigger due only to implicit `kilo-shared`
