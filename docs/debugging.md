# Debugging Kilo Docker

This guide explains how to debug and diagnose issues in Kilo Docker, both locally and in CI.

## Logging Infrastructure

Kilo Docker uses centralized logging in `pkg/utils/log.go`:

- `utils.Log()` — [LOG] prefix
- `utils.LogWarn()` — [WARN] prefix  
- `utils.LogError()` — [ERROR] prefix

Add `utils.WithOutput()` for user-visible messages (stderr).

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
