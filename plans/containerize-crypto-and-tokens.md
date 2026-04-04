# Plan: Containerize Crypto and Token Management

## Context

Sensitive data (tokens, passwords, user_id) is currently passed from host to container via `docker run -e` environment variables. This exposes them via `docker inspect`, `/proc/*/environ`, and Docker daemon logs. The host also handles encryption/decryption and token file I/O.

**Goal**: Move all crypto, token storage, and authentication into the container so no sensitive data crosses the host→container boundary via env vars.

## Design

### Principles

1. **Ainstruct account is required** — no `--password` flag, no unprotected mode
2. **Single static volume**: `kilo-docker-data` — shared by all containers on the host
3. **Container user home derived from `user_id`** — hash derivative, not user_id directly
4. **`user_id` never persisted** — lives in `kilo-entrypoint` process memory only
5. **Volume stores only encrypted MCP tokens** — useless without `user_id` from login
6. **Non-persistent init marker** — `/tmp/.kilo-initialized` (container filesystem)
7. **Login on every container recreation** — trade-off for not storing `user_id`
8. **Single container** — entrypoint handles login, crypto, token management

### Security Model

| Data | Location | Lifetime | Accessible by |
|---|---|---|---|
| `user_id` | Process memory | Container lifetime | kilo-entrypoint process only |
| Decrypted MCP tokens | Process memory | Container lifetime | kilo-entrypoint process only |
| JWT tokens (access/refresh) | Process memory | Container lifetime | kilo-entrypoint process only |
| Encrypted MCP tokens | Volume `.tokens.env.enc` | Persistent | Anyone with volume (useless without key) |
| Init marker | `/tmp/.kilo-initialized` | Container lifetime | Anyone in container (no sensitive data) |

**Nothing sensitive in**:
- Persistent storage (volume) — only encrypted blobs
- Docker environment variables (`-e`) — eliminated
- Process arguments (`ps`) — eliminated
- Container environment (`/proc/*/environ`) — eliminated

### Volume

- **Name**: `kilo-docker-data` (static, single volume per host)
- **Mount**: `kilo-docker-data:/home`
- **Structure**:
  ```
  /home/
  ├── kilo-<sha256(user_id)[:12]>/         # derived home, NOT user_id directly
  │   ├── .config/kilo/                     # kilo config (rules, commands, agents)
  │   ├── .local/share/kilo/
  │   │   ├── .tokens.env.enc               # encrypted MCP tokens (AES-256-CBC)
  │   │   └── .tokens.skip                  # skip marker
  │   └── ...
  ├── kilo-<sha256(user_id_2)[:12]>/        # second user (multi-user)
  │   └── ...
  ```

No `.user_id` file. No plaintext tokens. The derived directory name is computed from `sha256(user_id)` but the actual `user_id` is never written to disk.

### Container Lifecycle

#### Start (always detached, no TTY)
```
Host:  docker run -d -v kilo-docker-data:/home image

Entrypoint (runInit — no subcommand):
  1. User/group setup (PUID/PGID from env)
  2. Service installation
  3. Config directory creation
  4. Privilege drop (syscall.Setuid/Setgid)
  5. Exec sleep infinity (keeps container alive)
```

#### First exec (no init marker)
```
Host:  docker exec -it container kilo-entrypoint zellij-attach

zellij-attach:
  1. Check /tmp/.kilo-initialized → NOT EXISTS
  2. Scan /home/kilo-*/.local/share/kilo/.tokens.env.enc
     ├── Found encrypted tokens → existing user, go to step 3
     └── Not found → new user, go to step 3 (login will create)

  3. LOGIN (via exec's TTY):
     - Prompt username (fmt.Scanln via stderr)
     - Prompt password (term.ReadPassword via stderr)
     - HTTP POST /auth/login → access_token, refresh_token
     - HTTP GET /auth/profile → user_id
     - HTTP ensurePAT → MCP token
     - user_id stays in process memory

  4. SETUP USER:
     - derived_name = "kilo-" + sha256(user_id)[:12]
     - home = /home/<derived_name>/
     - Create home + config directories
     - Create OS user from derived_name
     - Set HOME=/home/<derived_name>
     - If new user: prompt for additional MCP tokens (Context7) via TTY
     - Encrypt ALL MCP tokens with user_id → write .tokens.env.enc

  5. START SYNC:
     - Decrypt .tokens.env.enc with user_id (in memory)
     - Start ainstruct-sync as goroutine with tokens in Go memory
     - No env vars — tokens passed via function arguments

  6. MARK INITIALIZED:
     - Write /tmp/.kilo-initialized (no sensitive data)

  7. EXEC ZELLIJ:
     - zellij attach --create kilo-docker
     - user_id discarded from this process (sync has its own copy)
```

#### Subsequent exec (marker exists)
```
Host:  docker exec -it container kilo-entrypoint zellij-attach

zellij-attach:
  1. Check /tmp/.kilo-initialized → EXISTS
  2. Exec zellij attach --create kilo-docker
```

#### Container recreated (shared volume)
```
Host:  docker run -d -v kilo-docker-data:/home image
Host:  docker exec -it container kilo-entrypoint zellij-attach

zellij-attach:
  1. Check /tmp/.kilo-initialized → NOT EXISTS (new container)
  2. Scan /home/kilo-*/.local/share/kilo/.tokens.env.enc → FOUND (from previous container)
  3. LOGIN REQUIRED (user_id not stored, must re-authenticate)
  4. Verify derived_name matches existing directory
  5. Re-encrypt tokens if needed
  6. Start sync, mark initialized, exec zellij
```

### Token Passing to ainstruct-sync

Ainstruct-sync is part of the `kilo-entrypoint` binary (not a separate process). Tokens are passed directly in Go memory — no env vars needed:

```go
func startSync(homeDir string, userId string) error {
    // Decrypt MCP tokens from volume
    encData, _ := os.ReadFile(filepath.Join(homeDir, ".local/share/kilo/.tokens.env.enc"))
    decrypted, _ := decryptAES(encData, userId)
    // Parse tokens, get JWT access_token, refresh_token, expiry

    // Create syncer with tokens in Go memory
    syncer := NewSyncerWithTokens(accessToken, refreshToken, expiry, homeDir)
    go syncer.Run()
    return nil
}
```

No env vars. No subprocess. Tokens exist only in Go memory within the `kilo-entrypoint` process.

### What the Host Passes to `docker run`

| Env Var | Purpose | Sensitive? |
|---|---|---|
| `PUID` | Host UID for file ownership | No |
| `PGID` | Host GID for file ownership | No |
| `KD_SERVICES` | Enabled services list | No |
| `SSH_AUTH_SOCK` | SSH agent socket path | No |
| `TERM`, `LANG`, `TZ` | Terminal/locale config | No |

No sensitive data. Login happens inside the container via `docker exec -it` TTY.

### Kilo CLI Environment Variables

Kilo's `opencode.json` uses `{env:KD_AINSTRUCT_TOKEN}` and `{env:KD_CONTEXT7_TOKEN}` placeholders resolved at runtime from the process environment. These must be set before Kilo starts.

**Approach**: `zellij-attach` sets these via `os.Setenv()` before exec'ing zellij. They're inherited by the zellij/kilo process tree.

| Env Var | Set by | Purpose |
|---|---|---|
| `KD_AINSTRUCT_TOKEN` | `os.Setenv()` in zellij-attach | MCP auth for Kilo CLI |
| `KD_CONTEXT7_TOKEN` | `os.Setenv()` in zellij-attach | MCP auth for Kilo CLI |
| `KD_AINSTRUCT_BASE_URL` | `os.Setenv()` in zellij-attach | API URL for Kilo CLI |

These are process-level env vars — NOT `docker run -e`:
- NOT visible via `docker inspect`
- NOT inherited by `docker exec` processes (separate process tree)
- Inherited by zellij and all child processes (kilo) in the same session
- Lost when container is removed

This is the minimum required by Kilo's config format. No alternative exists without changing Kilo's `{env:...}` syntax.

## Changes

### Host Binary (`cmd/kilo-docker/`)

#### Delete Files
| File | Reason |
|---|---|
| `crypto.go` | Crypto moves to container |
| `tokens.go` | Token I/O moves to container |
| `ainstruct.go` | Login moves to container |

#### `flags.go`
- Remove `--password` flag
- Remove `--ainstruct` flag (ainstruct is always required)
- Remove `encrypted` field from config struct
- Remove `encrypted` from `parseArgs()`

#### `setup.go`
- `resolveVolume()`: Always return `"kilo-docker-data"` (remove `encrypted`/`once` branching for volume name)
- Remove `promptMissingTokens()` — container handles MCP token prompting
- Remove `saveSkipMarker()` — container handles
- Remove `savePlainSkipMarker()` — container handles
- Remove `--password` and `--ainstruct` from help text
- Update help examples

#### `main.go`
- Remove `ainstructSyncToken`, `ainstructSyncRefreshToken`, `ainstructSyncTokenExpiry` variables
- Remove `ainstructLogin()` call
- Remove `loadTokens()` / `saveTokens()` calls
- Remove `os.Setenv("VOLUME_PASSWORD", ...)`
- Remove `promptMissingTokens()` call
- Login is no longer host's responsibility — container handles it on exec
- Exec `kilo-entrypoint zellij-attach` instead of `zellij attach`

#### `args.go`
- Change volume mount from `volume+":"+kiloHome` to `"kilo-docker-data:/home"`
- Remove: `KD_CONTEXT7_TOKEN`, `KD_AINSTRUCT_TOKEN`, `KD_AINSTRUCT_SYNC_TOKEN`, `KD_AINSTRUCT_SYNC_REFRESH_TOKEN`, `KD_AINSTRUCT_SYNC_TOKEN_EXPIRY`
- Remove: `KD_AINSTRUCT_ENABLED`, `KD_AINSTRUCT_BASE_URL`

#### `volume.go`
- Remove `deriveVolumeName()` function
- Keep `deriveContainerName()`, `volumeExists()`, `createVolume()`, `removeVolume()`, `listVolumes()`

#### `handle_backup.go`
- Volume name is always `"kilo-docker-data"` (remove `resolveVolume` call for encrypted branching)
- Remove `.enc` suffix from backup filename logic

#### `docker_test.go`, `tokens_test.go`
- Remove tests for deleted functions
- Update remaining tests

### Container Binary (`cmd/kilo-entrypoint/`)

#### New: `crypto.go`
Copy from host `cmd/kilo-docker/crypto.go`:
- `encryptAES(plaintext []byte, password string) ([]byte, error)`
- `decryptAES(ciphertext []byte, password string) ([]byte, error)`
- `pkcs7Pad(data []byte, blockSize int) []byte`
- `pkcs7Unpad(data []byte) ([]byte, error)`

Dependency: `golang.org/x/crypto` (already in go.mod)

#### New: `zellijattach.go` — zellij-attach subcommand
Key new file. Contains:

```go
func runZellijAttach() error {
    const initMarker = "/tmp/.kilo-initialized"

    // Already initialized in this container?
    if _, err := os.Stat(initMarker); err == nil {
        return execZellij()
    }

    // Find existing user data on volume
    userId := findExistingUser()

    if userId == "" {
        // New user — full login
        var err error
        userId, err = runLoginInteractive()
        if err != nil { return err }
    } else {
        // Existing user — re-authenticate (user_id not stored)
        var err error
        userId, err = runLoginInteractive()
        if err != nil { return err }
        // Verify derived name matches
    }

    // Setup user environment
    derived := deriveHomeName(userId)
    home := "/home/" + derived
    setupUserEnvironment(home, userId)

    // Start sync
    startSync(home, userId)

    // Mark initialized
    os.WriteFile(initMarker, []byte("1\n"), 0644)

    // Exec zellij
    return execZellij()
}
```

Helper functions:
- `findExistingUser()`: Scan `/home/kilo-*/.local/share/kilo/.tokens.env.enc`, return `""` or the derived name
- `deriveHomeName(userId)`: `"kilo-" + sha256hex(userId)[:12]`
- `setupUserEnvironment(home, userId)`: Create OS user, set HOME, create config dirs
- `startSync(home, userId)`: Decrypt tokens, start ainstruct-sync with process env vars
- `execZellij()`: `syscall.Exec("/usr/local/bin/zellij", ["zellij", "attach", "--create", "kilo-docker"], os.Environ())`

#### `login.go` — `runAinstructLogin()`
Change from env-var-based to TTY-based:
- Remove `os.Getenv("USERNAME")`, `os.Getenv("PASSWORD")` reads
- Add `promptUsername()` (fmt.Scanln via stderr)
- Add `promptPassword()` (term.ReadPassword via stderr)
- Keep HTTP calls unchanged
- Remove `os.Getenv("API_URL")` — use constant directly
- Return `(user_id string, accessToken string, refreshToken string, expiresIn int64, mcpToken string, err error)`

#### `login.go` — `ensurePAT()`
No changes needed — it's called internally by `runAinstructLogin()`.

#### `init.go` — `runInit()`
Simplified — remove sync startup:
- Keep: user/group setup, service installation, config dirs, privilege drop
- Remove: `os.Getenv("KD_AINSTRUCT_ENABLED")` check and sync startup
- Remove: sync startup (moved to zellij-attach)
- Keep: exec sleep infinity

#### `main.go`
- Add `"zellij-attach"` to subcommands map
- Add case for `"zellij-attach"` calling `runZellijAttach()`

#### `loadsave.go`
No changes needed — subcommands still available for standalone use, but primary token flow uses crypto.go directly.

#### `sync_content.go`
Add `NewSyncerWithTokens(accessToken, refreshToken, expiry, homeDir)` constructor that accepts tokens directly instead of reading from env vars. The existing `NewSyncer()` (env-var-based) can be removed.

## Files Modified

| File | Change |
|---|---|
| `cmd/kilo-docker/crypto.go` | DELETE |
| `cmd/kilo-docker/tokens.go` | DELETE |
| `cmd/kilo-docker/ainstruct.go` | DELETE |
| `cmd/kilo-docker/volume.go` | Remove `deriveVolumeName()` |
| `cmd/kilo-docker/flags.go` | Remove `--password`, `--ainstruct`, `encrypted` |
| `cmd/kilo-docker/setup.go` | Static volume, remove token prompting, update help |
| `cmd/kilo-docker/main.go` | Remove all token/login logic, exec zellij-attach |
| `cmd/kilo-docker/args.go` | Remove token env vars, volume → `kilo-docker-data:/home` |
| `cmd/kilo-docker/handle_backup.go` | Simplify (no encrypted suffix logic) |
| `cmd/kilo-docker/docker_test.go` | Remove tests for deleted code |
| `cmd/kilo-docker/tokens_test.go` | DELETE (tests crypto.go which moves to container) |
| `cmd/kilo-entrypoint/crypto.go` | NEW — copy from host |
| `cmd/kilo-entrypoint/zellijattach.go` | NEW — login, init, sync, zellij |
| `cmd/kilo-entrypoint/login.go` | TTY-based login (no env var reads) |
| `cmd/kilo-entrypoint/init.go` | Remove sync startup |
| `cmd/kilo-entrypoint/main.go` | Add `zellij-attach` subcommand |
| `cmd/kilo-entrypoint/sync_content.go` | Add `NewSyncerWithTokens()`, remove `NewSyncer()` env reads |

## Verification

1. `go build ./cmd/kilo-docker && go build ./cmd/kilo-entrypoint`
2. `docker build -t kilo-docker:test .`
3. First exec: login via TTY → verify derived home created, encrypted tokens on volume
4. Second exec: marker exists → verify zellij starts directly
5. `docker inspect <container>` → verify NO sensitive env vars (no tokens, no passwords, no user_id)
6. `docker exec container cat /tmp/.kilo-initialized` → marker only, no sensitive data
7. `docker exec container ls /home/kilo-*/.local/share/kilo/` → only `.tokens.env.enc`
8. Remove container, new container with same volume → verify login required again
9. Token refresh → verify re-encrypted tokens on volume after JWT refresh
10. Multi-user: second account → verify separate derived home directory
11. Verify ainstruct-sync works with tokens passed in Go memory (no env vars)
12. `go test ./cmd/...`
13. Update documentation (README, help text, inline comments) to reflect new architecture

## Documentation Updates

Files to update after implementation:

| File | Changes |
|---|---|
| `README.md` | Remove `--password`, `--ainstruct` flags from usage. Update flow description. |
| `cmd/kilo-docker/setup.go` (help text) | Remove `--password`, `--ainstruct` from options and examples |
| `cmd/kilo-docker/main.go` (package doc) | Update module-level comment to reflect new architecture |
| `cmd/kilo-entrypoint/main.go` (package doc) | Add `zellij-attach` to subcommand list |
| `cmd/kilo-entrypoint/zellijattach.go` | Add package-level doc comment explaining the login/init flow |
| Inline comments in modified files | Update to reflect new token/crypto flow |
