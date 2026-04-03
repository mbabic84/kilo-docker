# Plan: Clear Zellij Sessions on Container Recreation + Ensure Proper UID/GID in Docker Exec

## Context

When a kilo-docker container is recreated (same volume, new container), Zellij's cached session state persists in the Docker volume at `~/.cache/zellij/`. This causes Zellij to prompt the user to resurrect old processes when attaching with `zellij attach --create kilo-docker`. The config already has `session_serialization false` (configs/zellij.kdl:435), but residual session cache files can still cause prompts.

Additionally, the `dockerExec` function (`docker.go:86`) does not accept a `--user` parameter. All callers run as whatever the container's default user is. This creates a risk of permission issues, particularly when running commands that interact with user-owned files in the volume.

## Changes

### 1. Add `kiloUser` constant — `cmd/kilo-docker/flags.go`

Add a constant for the container username, matching the existing `kiloHome` constant:

```go
const (
    repoURL   = "ghcr.io/mbabic84/kilo-docker"
    kiloHome   = "/home/kilo-t8x3m7kp"
    kiloUser   = "kilo-t8x3m7kp"
)
```

### 2. Add user parameter to `dockerExec` — `cmd/kilo-docker/docker.go`

Change signature from:
```go
func dockerExec(container string, args ...string) (string, error) {
```
to:
```go
func dockerExec(container string, user string, args ...string) (string, error) {
```

Implementation — add `--user` before container name when user is non-empty:
```go
func dockerExec(container string, user string, args ...string) (string, error) {
    execArgs := []string{"exec"}
    if user != "" {
        execArgs = append(execArgs, "--user", user)
    }
    execArgs = append(execArgs, container)
    execArgs = append(execArgs, args...)
    cmd := exec.Command("docker", execArgs...)
    output, err := cmd.CombinedOutput()
    if err != nil {
        return strings.TrimSpace(string(output)), fmt.Errorf("docker exec %s %s: %w\n%s", container, strings.Join(args, " "), err, string(output))
    }
    return strings.TrimSpace(string(output)), nil
}
```

### 3. Update all `dockerExec` callers

Audit of each call site:

| File | Line | Command | User | Rationale |
|------|------|---------|------|-----------|
| `backup.go:25` | `dockerExec(container, "tar", "czf", ...)` | `""` (root) | Must read all files in volume |
| `backup.go:54` | `dockerExec(container, "tar", "xzf", ...)` | `""` (root) | Must write files with correct ownership |
| `backup.go:59` | `dockerExec(container, "chown", "-R", ...)` | `""` (root) | chown requires root |
| `session.go:25` | `dockerExec(container, "tar", "czf", ...)` | `""` (root) | Must read all files in volume |
| `session.go:53` | `dockerExec(container, "tar", "xzf", ...)` | `""` (root) | Must write files with correct ownership |
| `session.go:58` | `dockerExec(container, "chown", "-R", ...)` | `""` (root) | chown requires root |
| `playwright.go:67` | `dockerExec(container, "node", ...)` | `""` (root) | Different container (Playwright sidecar), not the kilo container |

Updated call sites:

**`cmd/kilo-docker/backup.go`** — all three calls pass `""`:
```go
dockerExec(container, "", "tar", "czf", "/tmp/backup.tar.gz", "-C", home, ".")
dockerExec(container, "", "tar", "xzf", "/tmp/backup.tar.gz", "-C", home)
dockerExec(container, "", "chown", "-R", fmt.Sprintf("%d:%d", os.Getuid(), os.Getgid()), home)
```

**`cmd/kilo-docker/session.go`** — same three calls pass `""`:
```go
dockerExec(container, "", "tar", "czf", "/tmp/backup.tar.gz", "-C", home, ".")
dockerExec(container, "", "tar", "xzf", "/tmp/backup.tar.gz", "-C", home)
dockerExec(container, "", "chown", "-R", fmt.Sprintf("%d:%d", os.Getuid(), os.Getgid()), home)
```

**`cmd/kilo-docker/playwright.go`** — passes `""`:
```go
dockerExec(playwrightContainer, "", "node", "-e", ...)
```

### 4. Add `deleteZellijSessions` helper — `cmd/kilo-docker/docker.go`

Add a new function that runs `zellij delete-all-sessions` as the kilo user:

```go
// deleteZellijSessions removes all zellij sessions inside the container.
// This prevents stale session resurrection prompts after container recreation.
// Errors are silently ignored (no sessions to delete is not a failure).
func deleteZellijSessions(container string) {
    dockerExec(container, kiloUser, "zellij", "delete-all-sessions")
}
```

Uses the `kiloUser` constant so the command runs as the correct user, creating/reading cache files with proper ownership.

### 5. Call before attach in `runContainer` — `cmd/kilo-docker/main.go`

In the "Run" section (lines 263-283), add `deleteZellijSessions` calls before attaching in the two container-recreation paths:

**a) After starting an exited/dead/created container (line 271):**
```go
} else if containerState == "exited" || containerState == "created" {
    dockerRun("start", "-d", containerName)
    time.Sleep(2 * time.Second)
    deleteZellijSessions(containerName)                          // NEW
    execDockerInteractive(containerName, kiloUser, "zellij", "attach", "--create", "kilo-docker")
```

**b) After creating a brand-new container (line 281):**
```go
} else {
    // ... create container ...
    time.Sleep(2 * time.Second)
    deleteZellijSessions(containerName)                          // NEW
    execDockerInteractive(containerName, kiloUser, "zellij", "attach", "--create", "kilo-docker")
```

**NOT added** to the `containerState == "running"` path (line 266) — the user is attaching to a live container, not recreating one.

### 6. Replace hardcoded `"kilo-t8x3m7kp"` with `kiloUser` — `cmd/kilo-docker/main.go`

Replace all `execDockerInteractive(..., "kilo-t8x3m7kp", ...)` calls with `execDockerInteractive(..., kiloUser, ...)`:

- `main.go:115` — flag mismatch, attach without recreate
- `main.go:266` — attach to running container
- `main.go:271` — attach after starting exited container
- `main.go:281` — attach after creating new container

### 7. Replace hardcoded `"kilo-t8x3m7kp"` with `kiloUser` — `cmd/kilo-docker/handle_sessions.go`

Same replacements in session handler:

- `handle_sessions.go:176` — attach to running session
- `handle_sessions.go:193` — attach to exited/created session

### 8. Call in `sessions recreate` — no additional change needed

In the `sessions recreate` handler (handle_sessions.go:102), after `runContainer(newCfg)` is called, the flow goes through `runContainer` which hits the new-container path (step 5b). This already works correctly — `dockerRun("rm", "-f", containerName)` removes the old container, then `runContainer` creates a fresh one, hitting the `else` branch with `deleteZellijSessions`.

## Files Modified

| File | Change |
|------|--------|
| `cmd/kilo-docker/flags.go` | Add `kiloUser` constant |
| `cmd/kilo-docker/docker.go` | Add `user` parameter to `dockerExec`; add `deleteZellijSessions` function |
| `cmd/kilo-docker/main.go` | Call `deleteZellijSessions` before attach in exited/created/new paths; replace hardcoded `"kilo-t8x3m7kp"` with `kiloUser` |
| `cmd/kilo-docker/handle_sessions.go` | Replace hardcoded `"kilo-t8x3m7kp"` with `kiloUser` |
| `cmd/kilo-docker/backup.go` | Update 3 `dockerExec` calls to pass `""` user |
| `cmd/kilo-docker/session.go` | Update 3 `dockerExec` calls to pass `""` user |
| `cmd/kilo-docker/playwright.go` | Update 1 `dockerExec` call to pass `""` user |

## Verification

1. `go build ./cmd/kilo-docker/` — ensure it compiles
2. `go test ./cmd/kilo-docker/...` — ensure existing tests pass
3. `go vet ./cmd/kilo-docker/` — ensure no vet issues
4. Manual test: run `kilo-docker`, exit, run `kilo-docker` again — should not prompt about old processes
