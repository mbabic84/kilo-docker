# Plan: Exec-Based Session Management

## Context

Currently, kilo-docker uses `docker run`/`docker attach`/`docker start -ai` to connect users to Zellij sessions. Zellij is the container's main process (via `kilo-entrypoint` → `runInit()` → `syscall.Exec("/usr/local/bin/zellij")`). When the user detaches (Ctrl+P Ctrl+Q) or zellij exits, the container process ends and may enter "exited" state.

**Problem**: The container's lifecycle is tied to the Zellij process. If zellij crashes or the user detaches in a way that terminates the process, the container stops. There's no clean way to keep a session alive independently of the Zellij attachment state.

**Solution**: Decouple the container lifecycle from Zellij by:
1. Making the container's long-running process `sleep infinity` instead of zellij
2. Using `docker exec` to start/attach zellij sessions inside the running container
3. When zellij detaches/exits, only the exec process dies — the container keeps running
4. Users re-attach via `docker exec -it container zellij attach`

## Changes

### 1. `cmd/kilo-entrypoint/init.go` — Change default behavior

Modify `runInit()` (line 144-152): replace the zellij exec with `sleep infinity`.

**Before:**
```go
if len(os.Args) <= 1 {
    if _, err := user.Lookup("kilo-t8x3m7kp"); err == nil {
        return syscall.Exec("/usr/bin/sudo", []string{"sudo", "-E", "-u", "kilo-t8x3m7kp", "-s", "/usr/local/bin/zellij"}, os.Environ())
    }
    return syscall.Exec("/usr/local/bin/zellij", []string{"zellij"}, os.Environ())
}
```

**After:**
```go
if len(os.Args) <= 1 {
    if _, err := user.Lookup("kilo-t8x3m7kp"); err == nil {
        return syscall.Exec("/usr/bin/sudo", []string{"sudo", "-E", "-u", "kilo-t8x3m7kp", "-s", "/bin/sleep", "infinity"}, os.Environ())
    }
    return syscall.Exec("/bin/sleep", []string{"sleep", "infinity"}, os.Environ())
}
```

All initialization (user setup, service install, privilege drop, config dirs, etc.) still runs before the exec. The init is idempotent, so container restarts (`docker start`) re-run init harmlessly.

### 2. `cmd/kilo-docker/docker.go` — Add `execDockerInteractive` function

Add a new function for interactive `docker exec` sessions:

```go
func execDockerInteractive(args ...string) error {
    cmd := exec.Command("docker", args...)
    cmd.Stdin = os.Stdin
    cmd.Stdout = os.Stdout
    cmd.Stderr = os.Stderr
    return cmd.Run()
}
```

This is similar to `execDockerAttach` but without `SysProcAttr` (no TTY-detach handling needed for exec — when zellij detaches, the exec exits cleanly).

### 3. `cmd/kilo-docker/main.go` — Change `runContainer()` flow

Replace the container attachment logic (lines 247-262) to use exec-based zellij management.

**Before:**
```go
if containerState == "running" {
    execDockerAttach("attach", containerName)
    handleSessionEnd(containerName, cfg.once)
} else if containerState == "exited" || containerState == "created" {
    execDockerAttach("start", "-ai", containerName)
    handleSessionEnd(containerName, cfg.once)
} else {
    runArgs := buildRunArgs(containerArgs, image, cfg.args, isTerminal())
    execDocker(runArgs...)
    handleSessionEnd(containerName, cfg.once)
}
```

**After:**
```go
if containerState == "running" {
    attachToSession(containerName, isTerminal())
    handleSessionEnd(containerName, cfg.once)
} else if containerState == "exited" || containerState == "created" {
    dockerRun("start", containerName)
    attachToSession(containerName, isTerminal())
    handleSessionEnd(containerName, cfg.once)
} else {
    runArgs := buildRunArgs(containerArgs, image, []string{"sleep", "infinity"}, isTerminal())
    execDocker(runArgs...)
    handleSessionEnd(containerName, cfg.once)
}
```

Key changes:
- New containers are started with `sleep infinity` as the command (overrides entrypoint's default)
- "exited" containers are started non-interactively with `docker start`, then exec'd into
- All zellij interaction happens via `docker exec`

Add the `attachToSession` helper:

```go
func attachToSession(containerName string, terminal bool) error {
    args := []string{"exec"}
    if terminal {
        args = append(args, "-it")
    } else {
        args = append(args, "-i")
    }
    args = append(args, "--user", "kilo-t8x3m7kp", containerName, "zellij")
    return execDockerInteractive(args...)
}
```

### 4. `cmd/kilo-docker/main.go` — Simplify `handleSessionEnd()`

With exec-based sessions, the container is always "running" after zellij exits (since sleep infinity keeps it alive). Simplify the function:

```go
func handleSessionEnd(containerName string, onceMode bool) {
    resetTerminal()
    state := dockerState(containerName)
    if state == "running" {
        if onceMode {
            dockerRun("rm", "-f", containerName)
            fmt.Fprintf(os.Stderr, "\nSession '%s' ended and removed.\n", containerName)
        } else {
            fmt.Fprintf(os.Stderr, "\nDetached from session '%s'.\n", containerName)
            fmt.Fprintf(os.Stderr, "To re-attach, run: kilo-docker sessions %s\n\n", containerName)
        }
    } else {
        fmt.Fprintf(os.Stderr, "\nSession '%s' ended.\n", containerName)
    }
}
```

### 5. `cmd/kilo-docker/handle_sessions.go` — Update session attach logic

Update `handleSessions()` (lines 173-203) to use exec-based attachment:

**Before:**
```go
case "running":
    execDockerAttach("attach", containerToAttach)
    handleSessionEnd(containerToAttach, false)
case "exited", "created":
    // SSH setup...
    execDockerAttach("start", "-ai", containerToAttach)
    handleSessionEnd(containerToAttach, false)
```

**After:**
```go
case "running":
    attachToSession(containerToAttach, isTerminal())
    handleSessionEnd(containerToAttach, false)
case "exited", "created":
    // SSH setup...
    dockerRun("start", containerToAttach)
    attachToSession(containerToAttach, isTerminal())
    handleSessionEnd(containerToAttach, false)
```

### 6. `cmd/kilo-docker/terminal.go` — Simplify terminal reset

With `docker exec`, the terminal doesn't need the complex escape sequences that `docker attach` required. Review and simplify `resetTerminal()` — may only need `stty sane` and `reset`.

### 7. `cmd/kilo-docker/terminal.go` — Simplify terminal reset

With `docker exec`, the terminal is not put into raw mode like `docker attach` does. The escape sequences (`\033c\033[2J\033[H`) can be removed. Keep `stty sane` and `reset` as a safety net since zellij may still leave terminal in a non-default state.

### 8. `cmd/kilo-docker/args.go` — No changes needed

Container args (labels, volumes, env vars, etc.) remain the same. The `--init` flag stays — tini reaps zombies from exec sessions.

## Behavior Summary

| Scenario | Old Behavior | New Behavior |
|----------|-------------|--------------|
| First run | `docker run` → zellij (PID 1) | `docker run` → sleep infinity, then `docker exec` zellij |
| Container running, re-attach | `docker attach` | `docker exec` zellij attach |
| Container exited, restart | `docker start -ai` → zellij | `docker start` → sleep infinity, then `docker exec` zellij attach |
| Detach (Ctrl+P Ctrl+Q) | Container keeps running | N/A — detach via zellij's `Ctrl+O D` |
| Zellij exits normally | Container stops | `docker exec` exits, container keeps running |
| --once mode, user detaches | Container keeps running (wasteful) | Container is removed after exec exits |

## Files Modified

| File | Change |
|------|--------|
| `cmd/kilo-entrypoint/init.go:144-152` | Replace zellij exec with sleep infinity |
| `cmd/kilo-docker/docker.go` | Add `execDockerInteractive()` function |
| `cmd/kilo-docker/main.go` | Add `attachToSession()`, modify `runContainer()` and `handleSessionEnd()` |
| `cmd/kilo-docker/handle_sessions.go:173-203` | Use exec-based attach for session management |
| `cmd/kilo-docker/terminal.go` | Simplify terminal reset (remove raw-mode escape sequences) |

## Verification

1. `go build ./cmd/kilo-docker` — verify host binary compiles
2. `go build ./cmd/kilo-entrypoint` — verify entrypoint binary compiles
3. `go test ./...` — run all existing tests
4. Manual testing:
   - `kilo-docker` — new container starts, zellij opens via exec
   - Detach from zellij (Ctrl+O D) — container stays running, exec exits
   - `kilo-docker sessions` — shows running session
   - `kilo-docker sessions <name>` — re-attaches via exec
   - Exit zellij — in once mode, container is removed; in normal mode, container stays
   - `kilo-docker sessions cleanup` — removes exited containers (should find fewer now since containers stay running)
