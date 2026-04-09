# Go Development Guide

This project contains Go code in `cmd/` and `pkg/` directories.

## Logging

**Always use `utils.Log()` for logging** — do not use `fmt.Printf()` or direct file logging.

Three log levels available:

```go
utils.Log("[module] message: %s\n", value)       // [LOG] prefix
utils.LogWarn("[module] warning: %s\n", value)  // [WARN] prefix
utils.LogError("[module] error: %s\n", value)   // [ERROR] prefix
```

To show output to stderr (user visibility), append `utils.WithOutput()`:

```go
utils.Log("[module] user message\n", utils.WithOutput())
```

Logs are written to `~/.config/kilo/kilo-docker.log` for persistence across container recreations.

## Running Tests

```bash
go test ./...
```

## Running Vet

```bash
go vet ./...
```

## Running Lint

Requires [golangci-lint](https://golangci-lint.run/):

```bash
golangci-lint run ./...
```

## Code Structure

- `cmd/kilo-docker/` - Main CLI application
- `cmd/kilo-entrypoint/` - Container entrypoint application
- `pkg/` - Shared packages (constants, services, utils)

## Building

```bash
go build -o bin/ ./...
```

This builds all binaries to the `bin/` directory.

## Common Issues

The linter may report:
- **errcheck**: Unchecked error returns (e.g., `resp.Body.Close()`)
- **staticcheck**: Code quality suggestions
- **unused**: Unused functions

These are warnings, not errors. The code compiles and tests pass.