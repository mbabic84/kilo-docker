# Plan: Fix golangci-lint findings

## Context
- `golangci-lint run ./... --timeout=5m` currently reports duplicate code, unchecked response closes, high cyclomatic complexity, staticcheck package comments, and a large set of `gosec` findings.
- Context7 documentation confirms two important constraints for current tooling:
- `golangci-lint` v2 uses `linters.exclusions.rules` for path/text-based exclusions.
- `gosec` supports targeted `#nosec <RULE> -- justification` comments for manually verified safe patterns.
- This repository is a CLI/container orchestration codebase, so some exec/path findings are expected and should be documented, while real defects like unchecked body closes, unsafe downloads, tar extraction safety, and inotify bounds checks should be fixed in code.

## Changes
1. Update `.golangci.yml` to keep strict linting but reduce noise only where justified.
   - Keep `dupl`, `errcheck`, `staticcheck`, `gocyclo`, and `gosec` enabled.
   - Add documented exclusions for `_test.go` security noise and package-level safe patterns where code changes are not appropriate.
   - Adjust complexity handling only as far as needed for this orchestration-heavy codebase.
2. Remove duplicate helper logic.
   - Refactor repeated flag definitions in `cmd/kilo-docker/flags.go`.
   - Consolidate prompt-confirm logic between `cmd/kilo-docker/main.go` and `cmd/kilo-docker/setup.go`.
   - Consolidate duplicated log option parsing/output handling in `pkg/utils/log.go`.
3. Fix concrete correctness and hardening issues.
   - Replace subprocess-based downloader in `cmd/kilo-docker/handlers.go` with `net/http` download logic.
   - Check `resp.Body.Close()` errors in `cmd/kilo-entrypoint/userinit.go`.
   - Harden tar restore path handling in `cmd/kilo-entrypoint/backup.go`.
   - Add inotify buffer bounds and fd/watch-descriptor conversion guards in `cmd/kilo-entrypoint/watcher.go`.
4. Apply targeted `#nosec` suppressions with justifications for intentional container orchestration patterns.
   - Focus on fixed-binary `exec.Command`, rooted config-path reads/writes, and validated URL/client flows.
5. Add package comments for `pkg/constants`, `pkg/services`, and `pkg/utils` to satisfy `staticcheck` ST1000.

## Files Modified
| File | Change |
|------|--------|
| `.golangci.yml` | Update lint thresholds and targeted exclusions |
| `cmd/kilo-docker/flags.go` | Remove duplicate flag definitions |
| `cmd/kilo-docker/main.go` | Reuse prompt helper and reduce lint findings |
| `cmd/kilo-docker/setup.go` | Centralize prompt helper / safe terminal handling |
| `cmd/kilo-docker/handlers.go` | Replace curl/wget subprocess download |
| `cmd/kilo-docker/handle_backup.go` | Harden archive path handling or annotate intentional exec |
| `cmd/kilo-docker/docker.go` | Add justified gosec suppressions for docker exec wrappers |
| `cmd/kilo-docker/volume.go` | Add justified gosec suppressions for docker volume helpers |
| `cmd/kilo-docker/ssh.go` | Add targeted suppressions and safe path handling notes |
| `cmd/kilo-entrypoint/backup.go` | Fix archive extraction safety issues |
| `cmd/kilo-entrypoint/userinit.go` | Fix unchecked body closes and annotate intentional flows |
| `cmd/kilo-entrypoint/watcher.go` | Fix bounds checks and guarded conversions |
| `pkg/constants/home.go` | Add package comment |
| `pkg/services/services.go` | Add package comment |
| `pkg/utils/log.go` | Add package comment and deduplicate logging helpers |
| `pkg/utils/parse.go` | Rely on package comment coverage |
| `pkg/utils/redact.go` | Rely on package comment coverage |

## Verification
- Run `golangci-lint run ./... --timeout=5m`
- Run `go test ./...`
- If config or security-related code changes require extra confidence, run `go vet ./...`
