# Plan: Fix Flag Mismatch Prompt Timing

## Problem

The flag mismatch check (added in `cmd/kilo-docker/main.go:248-264`) runs AFTER all expensive setup:
- SSH setup (line 87-95)
- Service socket detection (line 98-115)
- Volume resolution (line 118)
- Token loading (line 121-149)
- Ainstruct login (line 151-185) ← causes credential prompt BEFORE mismatch check
- Playwright setup (line 188-195)
- Network selection (line 198-205)

When the user runs `kilo-docker --ainstruct` with a running session that has different flags, they first see the ainstruct authentication flow, THEN the mismatch prompt. The mismatch check should be the **first thing** shown.

## Fix

Move the flag mismatch check to right after container state is determined (line 224), before token loading and ainstruct login.

### Change 1: `cmd/kilo-docker/main.go`

**Add flag mismatch check** after line 224 (container state determination), before line 226 (workspace conflict check):

```go
// Flag mismatch check — runs before expensive setup (tokens, ainstruct, playwright)
if containerState == "running" {
    currentFlags := serializeArgs(cfg, sshAuthSock)
    storedFlags := getContainerLabel(containerName, "kilo.args")
    if currentFlags != storedFlags {
        fmt.Fprintf(os.Stderr, "Existing session uses different flags.\n")
        fmt.Fprintf(os.Stderr, "  Existing: %s\n", storedFlags)
        fmt.Fprintf(os.Stderr, "  Current:  %s\n", currentFlags)
        if cfg.yes || promptConfirm("Recreate with new flags? [y/N]: ") {
            dockerRun("rm", "-f", containerName)
            containerState = "not_found"
        } else {
            execDockerInteractive(containerName, "kilo-t8x3m7kp", "zellij", "attach", "--create", "kilo-docker")
            handleSessionEnd(containerName, cfg.once)
            return
        }
    }
}
```

Key behaviors:
- If flags match: skip check, proceed to expensive setup normally
- If flags differ + `--yes` / `-y` / non-TTY: auto-recreate without prompting
- If flags differ + user enters "y": recreate, fall through to expensive setup
- If flags differ + user enters anything else: attach to existing and return early (skip expensive setup)

**Remove mismatch check from `runContainer()`** lines 248-264. Replace with the original simple attach block:

```go
if containerState == "running" {
    execDockerInteractive(containerName, "kilo-t8x3m7kp", "zellij", "attach", "--create", "kilo-docker")
    handleSessionEnd(containerName, cfg.once)
}
```

## Files Modified

| File | Change |
|------|--------|
| `cmd/kilo-docker/main.go` | Move flag mismatch check to line ~225, remove from ~248 |

## Verification

```bash
go vet ./cmd/kilo-docker/
go build ./cmd/kilo-docker/
go test ./cmd/kilo-docker/ -run "TestSerializeArgs" -v
```
