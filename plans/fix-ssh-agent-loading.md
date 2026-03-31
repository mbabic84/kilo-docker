# Plan: Fix SSH Agent Loading and Permissions

## Context

`kilo-docker --ssh` silently sets up SSH agent forwarding with zero user feedback. When it works, the user sees nothing. When it fails, the user sees nothing. The container-side `ssh-add -l` returns "Error connecting to agent: Permission denied", confirming the bind-mounted socket is inaccessible to the kilo user despite the `os.Chown` call in `init.go`.

### Root Causes

1. **No logging in `setupSSH()`** — Agent start, key loading, and errors are all silent
2. **`os.Chown` on bind-mounted socket silently fails** — Error at `init.go:80` is discarded (`_` return). Even when it "succeeds" (UIDs already match at 1000:1000), the socket remains inaccessible
3. **Keys not loaded into existing agent** — `ssh.go:15-18` returns immediately when `SSH_AUTH_SOCK` is set, skipping all key loading
4. **`ssh-add` errors discarded** — `ssh.go:60` ignores the return value
5. **Cleanup leaves stale socket** — `kill <pid>` doesn't remove the socket file; `ssh-agent -k` does
6. **Key scanning too broad** — Reads every non-`.pub` file in `~/.ssh/` including `config`, `known_hosts`, `authorized_keys`

## Changes

### 1. `cmd/kilo-docker/ssh.go` — Add logging, fix key loading, fix cleanup

Add `fmt` import and user-facing log messages throughout `setupSSH()`:

```go
import (
    "fmt"
    "os"
    "os/exec"
    "strings"
)
```

**Existing agent detection (lines 15-21):** Add logging and still attempt key loading:

```go
sshAuthSock := os.Getenv("SSH_AUTH_SOCK")
if sshAuthSock != "" {
    if info, err := os.Stat(sshAuthSock); err == nil {
        if info.Mode()&os.ModeSocket != 0 {
            fmt.Fprintf(os.Stderr, "[kilo-docker] Reusing existing SSH agent: %s\n", sshAuthSock)
            loadSSHKeys(sshDir())
            return sshAuthSock, true, false
        }
    }
    fmt.Fprintf(os.Stderr, "[kilo-docker] Warning: SSH_AUTH_SOCK=%s is not a valid socket\n", sshAuthSock)
}
```

**New agent startup (lines 23-26):** Log failure:

```go
output, err := exec.Command("ssh-agent", "-s").CombinedOutput()
if err != nil {
    fmt.Fprintf(os.Stderr, "[kilo-docker] Warning: failed to start ssh-agent: %v\n", err)
    return "", false, false
}
```

**After parsing new agent output (after line 43):** Log success:

```go
if newSock != "" {
    os.Setenv("SSH_AUTH_SOCK", newSock)
    os.Setenv("SSH_AGENT_PID", newPid)
    fmt.Fprintf(os.Stderr, "[kilo-docker] SSH agent started (pid=%s)\n", newPid)
    loadSSHKeys(sshDir())
    return newSock, true, true
}
```

**Extract key loading into a helper** `loadSSHKeys(dir string)`:

```go
func loadSSHKeys(sshDir string) {
    entries, err := os.ReadDir(sshDir)
    if err != nil {
        return
    }
    for _, entry := range entries {
        if entry.IsDir() || strings.HasSuffix(entry.Name(), ".pub") {
            continue
        }
        path := sshDir + "/" + entry.Name()
        data, err := os.ReadFile(path)
        if err != nil {
            continue
        }
        if strings.Contains(string(data), "PRIVATE KEY") {
            if out, err := exec.Command("ssh-add", path).CombinedOutput(); err != nil {
                fmt.Fprintf(os.Stderr, "[kilo-docker] Warning: failed to add key %s: %v\n", entry.Name(), strings.TrimSpace(string(out)))
            } else {
                fmt.Fprintf(os.Stderr, "[kilo-docker] Added SSH key: %s\n", entry.Name())
            }
        }
    }
}
```

**Fix cleanup (lines 71-76):** Use `ssh-agent -k`:

```go
func cleanupSSH(pid string) {
    if pid != "" {
        exec.Command("ssh-agent", "-k").Run()
    }
}
```

### 2. `cmd/kilo-entrypoint/init.go` — Fix socket permission handling

**Lines 78-82:** Check chown error, add logging, and use `chmod` as fallback:

```go
if sshAuthSock := os.Getenv("SSH_AUTH_SOCK"); sshAuthSock != "" {
    if info, err := os.Stat(sshAuthSock); err == nil && info.Mode()&os.ModeSocket != 0 {
        if err := os.Chown(sshAuthSock, puid, pgid); err != nil {
            fmt.Fprintf(os.Stderr, "[kilo-docker] Warning: failed to chown SSH socket: %v\n", err)
        }
        if err := os.Chmod(sshAuthSock, 0600); err != nil {
            fmt.Fprintf(os.Stderr, "[kilo-docker] Warning: failed to chmod SSH socket: %v\n", err)
        }
        // Verify accessibility
        if testFile, err := os.OpenFile(sshAuthSock, os.O_RDWR, 0); err != nil {
            fmt.Fprintf(os.Stderr, "[kilo-docker] Warning: SSH socket not accessible after fix: %v\n", err)
        } else {
            testFile.Close()
            fmt.Fprintf(os.Stderr, "[kilo-docker] SSH agent socket ready: %s\n", sshAuthSock)
        }
    } else {
        fmt.Fprintf(os.Stderr, "[kilo-docker] Warning: SSH_AUTH_SOCK=%s is not a valid socket\n", sshAuthSock)
    }
}
```

## Files Modified

| File | Change |
|------|--------|
| `cmd/kilo-docker/ssh.go` | Add stderr logging, extract `loadSSHKeys()` helper, fix key loading for existing agents, fix cleanup to use `ssh-agent -k` |
| `cmd/kilo-entrypoint/init.go` | Check `os.Chown` error, add `os.Chmod`, verify socket accessibility, add logging |

## Verification

1. **Build**: `go build ./cmd/kilo-docker/` and `go build ./cmd/kilo-entrypoint/`
2. **Test SSH flow**: Run `kilo-docker --ssh` and verify output includes:
   - `[kilo-docker] SSH agent started (pid=...)` or `[kilo-docker] Reusing existing SSH agent: ...`
   - `[kilo-docker] Added SSH key: ...` or warning if key loading fails
   - `[kilo-docker] SSH agent socket ready: /ssh-agent.sock` (inside container init)
3. **Test permission fix**: Inside container, run `ssh-add -l` — should list keys instead of "Permission denied"
4. **Test cleanup**: After exiting, verify no stale socket files remain in `/tmp/ssh-*`
5. **Test without SSH**: Run `kilo-docker` without `--ssh` — should produce no SSH output
6. **Run existing tests**: `go test ./cmd/kilo-docker/ ./cmd/kilo-entrypoint/`
