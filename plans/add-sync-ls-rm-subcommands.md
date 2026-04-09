# Plan: Add kilo-entrypoint sync subcommands for listing and removing ainstruct sync files

## Context
- The kilo-entrypoint binary currently supports sync and resync subcommands
- Users need a way to list and remove individual sync files without manual file operations
- This addresses GitHub issue #134: feat: Add kilo-entrypoint subcommands for listing and removing ainstruct sync files

## Changes
1. Add `sync ls` subcommand to list all ainstruct sync files with name, size, and modified date
2. Add `sync rm <file>` subcommand to remove a specific sync file (both local and remote copies)
3. Implement confirmation prompt before deletion in rm subcommand
4. Handle errors gracefully (file not found, permission denied, etc.)
5. Update help text to document new subcommands
6. Add necessary imports and helper functions

## Files Modified
| File | Change |
|------|--------|
| cmd/kilo-entrypoint/main.go | Add sync subcommand handling for ls and rm, update help text |
| cmd/kilo-entrypoint/sync.go | Add helper functions for listing and removing sync files |
| cmd/kilo-entrypoint/sync_content.go | Add helper functions for listing and removing sync files via API |

## Verification
- Run `go test ./...` to ensure all tests pass
- Run `go vet ./...` to check for any vet issues
- Run `golangci-lint run ./...` to check for lint issues
- Manual verification:
  - Test `kilo-entrypoint sync ls` shows proper file listing
  - Test `kilo-entrypoint sync rm <file>` prompts for confirmation and removes file
  - Test error handling for non-existent files
  - Test that both local and remote copies are removed