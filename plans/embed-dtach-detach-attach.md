# Plan: Embed dtach for Detach/Attach Functionality

## Context

Docker's native `attach` has issues:
- Ctrl+C on attach sends SIGINT to the container process
- Window resize doesn't propagate properly
- Attaching to a detached container can cause unexpected signal delivery
- No persistent session across disconnects

**dtach** is a simple C program (~600 lines total across 3 files) that provides screen-like detach/attach functionality without terminal emulation. It:
- Uses Unix domain sockets for client/master communication
- Forwards raw PTY bytes without re-encoding
- Supports multiple simultaneous clients
- Sessions persist on disk until explicitly removed

## Changes

### 1. Embed dtach source into kilo-docker

Create `cmd/kilo-entrypoint/dtach/` with:
- `main.c` - entry point and argument parsing
- `master.c` - master process managing PTY and clients
- `attach.c` - attach client logic
- `dtach.h` - header file with constants and types
- `config.h` - minimal config for building

### 2. Add go:generate directive for compilation

In `cmd/kilo-entrypoint/main.go` or a new `dtach.go` file:
```go
//go:generate sh compile_dtach.sh
```

Create `cmd/kilo-entrypoint/compile_dtach.sh`:
```bash
#!/bin/sh
# Compiles dtach as a static binary using musl-gcc
musl-gcc -static -o dtach main.c master.c attach.c
```

### 3. Modify Dockerfile to install dtach

Add to the container build:
```dockerfile
COPY cmd/kilo-entrypoint/dtach/dtach /usr/local/bin/dtach
```

### 4. Add dtach subcommand to kilo-entrypoint

In `cmd/kilo-entrypoint/main.go`, add handling for:
- `dtach-new <session> <cmd...>` - create and attach
- `dtach-attach <session>` - attach to existing session
- `dtach-detach` - detach from current session (sends detach char)
- `dtach-list` - list sessions

### 5. Modify kilo-docker host CLI

In `cmd/kilo-docker/docker.go`:
- Replace `execDockerAttach()` with running `docker exec` to invoke `dtach-attach` inside container
- Or use `docker exec` with PTY allocation: `docker exec -it <container> dtach-attach <session>`

**New approach using docker exec + dtach:**
```go
func execDockerDtachAttach(container, session string) error {
    cmd := exec.Command("docker", "exec", "-it", container, "dtach-attach", session)
    cmd.Stdin = os.Stdin
    cmd.Stdout = os.Stdout
    cmd.Stderr = os.Stderr
    return cmd.Run()
}
```

### 6. Store session socket in persistent location

Use `/var/run/dtach/` or `$KILO_HOME/dtach/` for session sockets to survive container restarts:
```bash
dtach -c /var/run/dtach/myapp /bin/bash
```

### 7. Modify container startup

When starting a new container session, instead of `docker run -it`, use:
```bash
docker run --detach <image> dtach-new <session> <command>
```

Then attach via `docker exec -it <container> dtach-attach <session>`

## Files Modified

| File | Change |
|------|--------|
| `cmd/kilo-entrypoint/dtach/main.c` | New - dtach entry point |
| `cmd/kilo-entrypoint/dtach/master.c` | New - dtach master process |
| `cmd/kilo-entrypoint/dtach/attach.c` | New - dtach attach logic |
| `cmd/kilo-entrypoint/dtach/dtach.h` | New - dtach header |
| `cmd/kilo-entrypoint/dtach/config.h` | New - minimal config |
| `cmd/kilo-entrypoint/compile_dtach.sh` | New - build script |
| `cmd/kilo-entrypoint/main.go` | Add dtach subcommands |
| `cmd/kilo-docker/docker.go` | Add `execDockerDtachAttach()` |
| `cmd/kilo-docker/main.go` | Use dtach attach instead of docker attach |
| `Dockerfile` | Copy dtach binary to container |

## Session Lifecycle

1. **Start new session**: `dtach -c /var/run/dtach/session /bin/bash`
2. **Detach**: Press `^\` (Ctrl+\\) - session continues running
3. **Reattach**: `dtach -a /var/run/dtach/session`
4. **Multiple clients**: Multiple terminals can attach to same session simultaneously

## Benefits Over Docker Attach

1. **True detach** - Press `^\` to detach without stopping the process
2. **Persistent sessions** - Session survives client disconnect
3. **Multiple clients** - Can attach from multiple terminals simultaneously
4. **Better signal handling** - Ctrl+C only sent when explicitly pressed
5. **No window size issues** - WINCH signals handled properly

## Verification

1. Build the container with dtach: `docker build -t kilo-test .`
2. Start a session: `docker run --detach kilo-test dtach-new mysession /bin/bash`
3. Attach: `docker exec -it <container> dtach-attach mysession`
4. Detach with `^\`
5. Reattach from same or different terminal
6. Verify session persists after `docker exec` exits

## Build Dependencies

- `musl-gcc` for static compilation (available in Alpine: `apk add musl-dev gcc`)
- Or use `交叉编译` with Docker: `docker run --rm -v $PWD:/src alpine:latest sh -c "apk add musl-dev gcc && cd /src && musl-gcc -static -o dtach main.c master.c attach.c"`
