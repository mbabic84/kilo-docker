# Plan: Add Version Status to Update Command

## Context

The `kilo-docker update` command currently downloads the latest binary but doesn't show version information. PR #130 introduced a `.versions` file in releases containing both `KILO_DOCKER_VERSION` and `KILO_VERSION`.

## Changes

1. **Add function to fetch latest versions from .versions file**
   - New function `getLatestVersions()` in `handlers.go`
   - Download from `https://github.com/mbabic84/kilo-docker/releases/latest/download/default.versions`
   - Parse simple key=value format:
     ```
     KILO_DOCKER_VERSION=v2.9.0
     KILO_VERSION=7.1.22
     ```
   - Return both versions (or error if fetch fails)

2. **Use existing `version` and `kiloVersion` variables for current versions**
   - The `version` variable in `flags.go:16` holds the current kilo-docker version
   - The `kiloVersion` variable in `flags.go:17` holds the current Kilo CLI version
   - Both are set at build time via ldflags

3. **Modify `handleUpdate()` to print version status**
   - Before download: Show both kilo-docker and Kilo version changes
   - Example output:
     ```
     kilo-docker: v1.0.0 → v2.9.0
     Kilo CLI: 0.8.0 → 7.1.22
     ```
   - If already latest: "Already on latest version (v2.9.0). No update needed."
   - After successful update: "Updated ✓"

## Files Modified

| File | Change |
|------|--------|
| `cmd/kilo-docker/handlers.go` | Add version fetching function, modify `handleUpdate()` |

## Verification

```bash
go build -o kilo-docker ./cmd/kilo-docker
./kilo-docker update
go vet ./...
go test ./...
```
