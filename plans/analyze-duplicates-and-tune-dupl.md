# Plan: Analyze Duplicates and Tune dupl

## Context
- The repository now uses `golangci-lint` v2 syntax in `.golangci.yml`, but duplication detection is still configured conservatively with `dupl.threshold: 120`.
- There is also an uncommitted `.golangci-test-dupl.yml` file using outdated config keys, so it is not yet a reliable low-threshold duplication probe.
- The task is to inspect the Go codebase for real duplicate blocks and only then tune `dupl` so the linter catches those duplicates instead of relying on guesswork.

## Changes
1. Run duplication-focused scans with `golangci-lint` using `dupl` only and lower thresholds to identify candidate duplicate blocks.
2. Read the implicated files and confirm whether the reported blocks are meaningful duplications worth flagging.
3. Update duplication-specific lint configuration to valid `golangci-lint` v2 syntax and set a threshold that catches confirmed duplicates.
4. Re-run the duplication scan to confirm the chosen threshold reports the intended duplicate blocks.

## Files Modified
| File | Change |
|------|--------|
| `plans/analyze-duplicates-and-tune-dupl.md` | Record the duplication-analysis and tuning approach |
| `.golangci.yml` | Potentially lower `dupl` threshold if confirmed duplicates are being missed |
| `.golangci-test-dupl.yml` | Convert to valid v2 syntax for low-threshold duplication-only scans |

## Verification
- `golangci-lint run --default=none -E dupl ./... --timeout=5m` loads successfully.
- Confirmed duplicate blocks are reported by `dupl` at the selected threshold.
- The resulting threshold is justified by actual duplicate code in the repo, not arbitrary tightening.
