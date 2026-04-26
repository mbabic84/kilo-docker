# Plan: Update golangci-lint v2 Config

## Context
- The repository added `.golangci.yml` and a CI lint step, but the workflow installs `golangci-lint` `v1.64.5` while the config uses v2-style schema.
- This mismatch makes linting non-reproducible and risks immediate CI failure for configuration reasons rather than real findings.
- The latest stable release at the time of validation is `v2.11.4`, and the configuration should use current v2 keys.

## Changes
1. Update `.github/workflows/ci.yml` to install `golangci-lint` `v2.11.4` so CI matches the checked-in config generation.
2. Rewrite `.golangci.yml` to valid v2 syntax:
   - use `linters.default: none` instead of removed `disable-all`
   - move directory exclusions from `run.skip-dirs` to `linters.exclusions.paths`
   - use `output.formats.text` instead of removed `output.format`
   - use `issues.max-issues-per-linter` instead of removed `issues.max-per-linter`
3. Keep the intended linter set and thresholds unchanged where possible so revalidation reflects real code issues rather than policy drift.
4. Re-run `golangci-lint run ./... --timeout=5m` to verify the configuration loads and report remaining findings.

## Files Modified
| File | Change |
|------|--------|
| `.github/workflows/ci.yml` | Pin CI install to latest stable `golangci-lint` v2 release |
| `.golangci.yml` | Convert configuration to valid current v2 schema |
| `plans/update-golangci-lint-v2-config.md` | Record plan and validation intent |

## Verification
- `golangci-lint version` reports `v2.11.4` in CI/local validation context.
- `golangci-lint run ./... --timeout=5m` loads `.golangci.yml` without configuration-schema errors.
- Remaining failures, if any, are code findings rather than config incompatibilities.
