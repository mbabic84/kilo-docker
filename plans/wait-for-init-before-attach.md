# Plan: Move Init Waiting Into Entrypoint

## Context
- `kilo-docker` should stay a simple host-side tool for starting and managing sessions, without internal readiness polling loops.
- On slower hosts, the root init path may still be installing services or creating service groups when attach begins.
- This can race `runUserInit()` against `setupServiceGroups()` and lead to missing supplementary group access for the mounted workspace.
- Concurrent attaches can also race against each other while first-time user initialization is still running.

## Changes
1. Keep a dedicated root-init marker in `cmd/kilo-entrypoint/init.go` after root init completes service installation and service-group setup.
2. Make `cmd/kilo-entrypoint/zellijattach.go` wait for root init to finish before proceeding, with a user-visible waiting message.
3. Serialize first-time user initialization with an in-progress marker so concurrent attaches wait until full user init is done.
4. Move the final initialized marker in `cmd/kilo-entrypoint/userinit.go` to the end of user init so attach waits for the full setup, not just an early partial state.
5. Remove host-side readiness polling from `cmd/kilo-docker/` and keep failures explicit from the entrypoint.

## Files Modified
| File | Change |
|------|--------|
| cmd/kilo-entrypoint/init.go | Write root-init marker after service setup completes |
| cmd/kilo-entrypoint/zellijattach.go | Wait for root/user init and serialize first-time user initialization |
| cmd/kilo-entrypoint/userinit.go | Mark initialization complete only after full user init finishes |
| cmd/kilo-docker/docker.go | Remove obsolete host-side init polling helper |
| cmd/kilo-docker/main.go | Remove host-side readiness waiting |
| cmd/kilo-docker/handle_sessions.go | Remove host-side readiness waiting |
| plans/wait-for-init-before-attach.md | Document context, implementation, and verification |

## Verification
- Run `go test ./...`.
- Run `go vet ./...`.
- Confirm session startup waits inside `kilo-entrypoint` instead of from `kilo-docker`.
- Validate behavior conceptually: on slow hosts, attach starts only after root init and full first-time user init are ready.
