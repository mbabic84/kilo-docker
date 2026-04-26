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
## Quality Workflow (CI Parity)

The local developer workflow should mirror CI for consistency. Before committing or creating a PR, run the complete quality sequence:

```bash
# 1. Build check
go build ./...

# 2. Tests
go test ./...

# 3. Vet
go vet ./...

# 4. Lint (requires golangci-lint)
golangci-lint run ./... --timeout=5m

# 5. Race detector
go test -race ./...
```

These steps are enforced in `.github/workflows/ci.yml`. All must pass before a PR merges.

> **Note:** For quick iteration, `scripts/build.sh test` runs only `go test ./...`. The full quality workflow should be run before committing changes.

## Linter Configuration

This repository uses `.golangci.yml` with:
- `staticcheck`, `errcheck`, `dupl`, `gocyclo`, `gosec` enabled
- Thresholds: gocyclo ≥15, dupl ≥120
- False-positive exclusions for safe patterns (exec.Command, file perms, etc.)
