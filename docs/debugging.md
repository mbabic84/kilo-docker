# Debugging Kilo Docker

This guide explains how to debug and diagnose issues in Kilo Docker, both locally and in CI.

## Logging Infrastructure

Kilo Docker uses centralized logging in `pkg/utils/log.go`:

- `utils.Log()` — [LOG] prefix
- `utils.LogWarn()` — [WARN] prefix  
- `utils.LogError()` — [ERROR] prefix

Add `utils.WithOutput()` for user-visible messages (stderr).

## Diagnostic Tools Inside a Session

Pass `--diagnostics` when starting a session to install lightweight system
diagnostic tools on demand (~9 MB, opt-in):

```bash
kilo-docker --diagnostics -y
```

| Tool | Package | Use case |
|------|---------|----------|
| `ps`, `top`, `pgrep`, `free`, `uptime`, `watch` | `procps` | Inspect running processes and resource usage |
| `ss` | `iproute2` | Modern socket/port inspection |
| `lsof` | `lsof` | List open files and port-bound processes |
| `netstat`, `ifconfig` | `net-tools` | Classic network diagnostics |
| `nc` | `netcat-openbsd` | TCP/UDP connectivity tests |
| `ping` | `iputils-ping` | Reachability and latency checks |
| `pstree`, `fuser`, `killall` | `psmisc` | Process tree and signal helpers |

The install step is idempotent — running `init` again on a prepared volume is a
no-op. Tools are only installed in sessions that opt in; the base image is
unchanged.

When `--diagnostics` is set, the environment variable `DIAGNOSTICS_ENABLED=1` is
available inside the session.

## Common Scenarios

### Container fails to start
Check logs: `tail -f ~/.config/kilo/kilo-docker.log`

### Token issues
Reset: `kilo-docker init`

### Race conditions
Run: `go test -race ./...` and check CI artifacts

### Linter failures
Adjust `.golangci.yml` or fix reported gocyclo/staticcheck issues

### Docker permission issues
`docker ps` to verify daemon; `groups` to check docker group membership

## CI Debugging

1. Check GitHub Actions logs for the `validate` job
2. Steps: go vet → golangci-lint → go test -race
3. Reproduce locally with the same commands

## Performance

- Slow start: check volume filesystem location
- High CPU: review watcher debounce parameters
