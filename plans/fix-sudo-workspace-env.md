# Plan: Fix sudo user switching — workspace and ENV issues

## Context

The container entrypoint (`cmd/kilo-entrypoint/init.go`) switches from root to the `kilo-t8x3m7kp` user using `sudo -i`. This causes two problems:

1. **Workspace lost**: `sudo -i` invokes a login shell that forces `cd ~`, overriding Docker's `-w /host/path` working directory. The user lands in `/home/kilo-t8x3m7kp` instead of the workspace.

2. **ENV incorrect**: `os.Environ()` is passed directly to `syscall.Exec(sudo, ...)`, which inherits `USER=root`, `LOGNAME=root`, `SHELL=/bin/sh` from the root process. `sudo -i` doesn't fully override these when they're explicitly passed in the environment. Additionally, `HOME` may not reflect the actual kilo user home in all cases.

3. **GOPATH wrong** (related): `services.go` hardcodes `GOPATH=/root/go`, which is incorrect when running as the kilo user.

## Root Cause Analysis

### Workspace issue
```go
// init.go:147
syscall.Exec("/usr/bin/sudo", []string{"sudo", "-u", "kilo-t8x3m7kp", "-i"}, os.Environ())
```
`sudo -i` always does `cd $HOME` before starting the shell. This is by design — `-i` means "initial login" which emulates a full login, including changing to home directory. Docker's `-w` sets the working directory for the entrypoint process, but `sudo -i` unconditionally overrides it.

### ENV issue
When `os.Environ()` is passed to `syscall.Exec`, the child process (sudo) inherits:
- `USER=root` (set by Docker for the root process)
- `LOGNAME=root`
- `SHELL=/bin/sh` (Alpine default for root)
- `HOME=/home/kilo-t8x3m7kp` (set in Dockerfile, this one is correct)

`sudo -i` sources the target user's login files (`~/.profile`), which may export some variables, but `USER` and `LOGNAME` from the inherited environment take precedence because they're already set.

### Subcommand path (also affected)
```go
// init.go:152
syscall.Exec(binaryPath, os.Args[1:], os.Environ())
```
After `Setuid`/`Setgid`, the process environment still has `USER=root`, `LOGNAME=root`. The working directory IS preserved here (no login shell), but the identity env vars are wrong.

## Changes

### 1. `cmd/kilo-entrypoint/init.go` — Fix ENV vars after privilege drop

After `syscall.Setgid`/`syscall.Setuid` (line 115), explicitly set user identity environment variables:

```go
syscall.Setgid(pgid)
syscall.Setuid(puid)

// Set correct user identity environment variables after privilege drop
kiloUser := "kilo-t8x3m7kp"
kiloHome := "/home/kilo-t8x3m7kp"
os.Setenv("HOME", kiloHome)
os.Setenv("USER", kiloUser)
os.Setenv("LOGNAME", kiloUser)
os.Setenv("SHELL", "/bin/sh")
```

These are set in the process environment before any `os.Environ()` call, so they'll be included correctly.

### 2. `cmd/kilo-entrypoint/init.go` — Use `sudo -s` instead of `sudo -i`

Change the interactive shell exec from:
```go
return syscall.Exec("/usr/bin/sudo", []string{"sudo", "-u", "kilo-t8x3m7kp", "-i"}, os.Environ())
```
to:
```go
return syscall.Exec("/usr/bin/sudo", []string{"sudo", "-u", "kilo-t8x3m7kp", "-s"}, os.Environ())
```

`-s` runs the shell without forcing a login shell, which means:
- The working directory is preserved (no forced `cd ~`)
- The shell is still run as the target user with correct supplementary groups
- Environment variables set above (HOME, USER, etc.) are available
- `~/.profile` is NOT sourced (but env vars are already set correctly)

### 3. `cmd/kilo-docker/services.go` — Fix GOPATH

Change:
```go
EnvVars: map[string]string{
    "GOPATH": "/root/go",
},
```
to:
```go
EnvVars: map[string]string{
    "GOPATH": "$HOME/go",
},
```

Note: Docker `-e` doesn't expand variables, so this will be literally `$HOME/go` in the env var. Go resolves `$HOME` at runtime, which is correct behavior. Alternatively, we could remove GOPATH entirely and let Go default to `$HOME/go`.

## Files Modified

| File | Change |
|------|--------|
| `cmd/kilo-entrypoint/init.go` | Set USER/LOGNAME/SHELL env vars after privilege drop; change `sudo -i` to `sudo -s` |
| `cmd/kilo-docker/services.go` | Fix GOPATH from `/root/go` to `$HOME/go` |

## Verification

1. Build: `go build ./cmd/kilo-entrypoint` and `go build ./cmd/kilo-docker`
2. Functional test: Run the container with `-w /some/workspace` and verify:
   - `pwd` returns the workspace path (not `$HOME`)
   - `echo $USER` returns `kilo-t8x3m7kp` (not `root`)
   - `echo $HOME` returns `/home/kilo-t8x3m7kp`
   - `echo $LOGNAME` returns `kilo-t8x3m7kp`
3. Subcommand test: Run `kilo-entrypoint kilo --version` and verify environment is correct
