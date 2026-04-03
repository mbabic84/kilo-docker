# Plan: Prompt on Flag Mismatch for Existing Sessions

## Context

When a user runs `kilo-docker <flags>` from a directory that already has a running container, the tool currently attaches to the existing container without checking whether the flags differ. The container's `kilo.args` label stores the original flags, but this is never compared against the newly requested flags. The user should be prompted to choose between reusing the existing session or recreating it with the new flags.

## Current Behavior (main.go:248)

```go
if containerState == "running" {
    execDockerInteractive(containerName, "kilo-t8x3m7kp", "zellij", "attach", "--create", "kilo-docker")
    handleSessionEnd(containerName, cfg.once)
}
```

No flag comparison happens — it just attaches.

## Changes

### 1. Extract `serializeArgs()` function

**File:** `cmd/kilo-docker/args.go`

Extract the session-args serialization logic (currently inline in `buildContainerArgs` at lines 34-67) into a standalone function `serializeArgs(cfg config, sshAuthSock string) string`. This produces the same format as the `kilo.args` label, enabling direct string comparison.

```go
func serializeArgs(cfg config, sshAuthSock string) string {
    var sessionArgs string
    if cfg.once {
        sessionArgs += "--once "
    }
    for _, svcName := range cfg.enabledServices {
        svc := getService(svcName)
        if svc != nil && svc.Flag != "" {
            sessionArgs += svc.Flag + " "
        }
    }
    if cfg.playwright {
        sessionArgs += "--playwright "
    }
    if sshAuthSock != "" {
        sessionArgs += "--ssh "
    }
    if cfg.encrypted && !cfg.ainstruct {
        sessionArgs += "-p "
    }
    if cfg.ainstruct {
        sessionArgs += "--ainstruct "
    }
    if cfg.mcp {
        sessionArgs += "--mcp "
    }
    if cfg.network != "" {
        sessionArgs += "--network " + cfg.network + " "
    }
    for _, port := range cfg.ports {
        sessionArgs += "--port " + port + " "
    }
    if len(cfg.args) > 0 {
        sessionArgs += strings.Join(cfg.args, " ") + " "
    }
    return strings.TrimSpace(sessionArgs)
}
```

Update `buildContainerArgs` to call `serializeArgs()` instead of duplicating the logic.

### 2. Add `getContainerLabel()` helper

**File:** `cmd/kilo-docker/docker.go`

Add a helper to retrieve a specific label from a container:

```go
func getContainerLabel(container, label string) string {
    val, _ := dockerInspect(container, "{{index .Config.Labels \"" + label + "\"}}")
    return strings.TrimSpace(val)
}
```

### 3. Add flag mismatch check in `runContainer()`

**File:** `cmd/kilo-docker/main.go`

After determining `containerState == "running"` (around line 248), add flag comparison logic:

1. Serialize current flags via `serializeArgs(cfg, sshAuthSock)`
2. Retrieve stored flags via `getContainerLabel(containerName, "kilo.args")`
3. If they differ:
   - Show the user what changed:
     ```
     Existing session uses different flags.
       Existing: --mcp --docker
       Current:  --mcp --ssh --docker
     ```
   - Use `promptConfirm("Recreate with new flags? [y/N]: ")`:
     - User enters "y" → remove container, fall through to create new
     - User enters anything else / empty → reuse existing (attach as before)
   - `--yes/-y` auto-confirms Yes → recreates the container (matches user's requested flags)

### 4. Add tests for `serializeArgs()`

**File:** `cmd/kilo-docker/args_test.go`

Add unit tests:
- `TestSerializeArgsEmpty` — no flags → empty string
- `TestSerializeArgsOnce` — `--once` only
- `TestSerializeArgsServices` — service flags like `--docker`, `--go`
- `TestSerializeArgsCombined` — mixed flags
- `TestSerializeArgsPorts` — port mappings
- `TestSerializeArgsNetwork` — network flag

## Files Modified

| File | Change |
|------|--------|
| `cmd/kilo-docker/args.go` | Extract `serializeArgs()`, refactor `buildContainerArgs` to use it |
| `cmd/kilo-docker/docker.go` | Add `getContainerLabel()` helper |
| `cmd/kilo-docker/main.go` | Add flag mismatch detection + prompt before attaching to running container |
| `cmd/kilo-docker/args_test.go` | Unit tests for `serializeArgs()` |

## Verification

```bash
cd /home/mbabic/projects/github/kilo-docker
go test ./cmd/kilo-docker/ -run "TestSerializeArgs" -v
go vet ./cmd/kilo-docker/
go build ./cmd/kilo-docker/
```
