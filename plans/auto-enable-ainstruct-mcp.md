# Plan: Auto-enable Ainstruct MCP on `--ainstruct` (without `--mcp`)

## Context

Currently, `--ainstruct` handles login and file sync, but the Ainstruct MCP server only activates when `--mcp` is also used (which prompts for both Context7 and Ainstruct API tokens). The user wants `--ainstruct` alone to automatically provision and enable the Ainstruct MCP server via PAT, with `--mcp` reserved for other MCP servers (e.g., Context7).

There are two separate token systems:
- **Sync token** (`KD_AINSTRUCT_SYNC_TOKEN`): JWT from `/auth/login` — used for file sync
- **MCP token** (`KD_AINSTRUCT_TOKEN`): PAT — used by the Ainstruct MCP server

The PAT can be auto-provisioned after login, eliminating manual token entry.

## Behavioral Change

| Flags | Before | After |
|-------|--------|-------|
| `--ainstruct` | Login + sync only, no MCP | Login + sync + Ainstruct MCP auto-enabled (PAT auto-provisioned) |
| `--mcp` | Context7 + Ainstruct MCP (manual tokens) | Context7 MCP (manual token) — Ainstruct MCP handled by `--ainstruct` |
| `--ainstruct --mcp` | Login + sync + both MCP servers (manual tokens) | Login + sync + Ainstruct MCP (auto-PAT) + Context7 MCP (manual token) |

## API Endpoints

| Endpoint | Purpose |
|----------|---------|
| `GET /api/v1/auth/pat` | List user's PATs (returns `PatListResponse` with `tokens[]`) |
| `POST /api/v1/auth/pat` | Create new PAT (returns `PatResponse` with `token` value) |
| `POST /api/v1/auth/pat/{pat_id}/rotate` | Rotate existing PAT (returns `PatResponse` with new `token` value) |

All require `Authorization: Bearer <JWT>`.

`PatListItem` schema: `{ pat_id, label, user_id, scopes, created_at, expires_at, last_used }`

## Changes

### 1. Host-side: PAT label helper — `cmd/kilo-docker/ainstruct.go`

Add `buildPATLabel()` using `os/user` and `os.Hostname()`:

```go
func buildPATLabel() string {
    u, _ := user.Current()
    hostname, _ := os.Hostname()
    username := "unknown"
    if u != nil { username = u.Username }
    if hostname == "" { hostname = "unknown" }
    return fmt.Sprintf("kilo-docker | %s@%s", username, hostname)
}
```

Add `MCPToken string` to `loginResult` struct. Pass `PAT_LABEL` env var to the `dockerRun` call in `ainstructLogin()`. Parse `MCP_TOKEN` from container output.

### 2. Container-side: PAT lifecycle — `cmd/kilo-entrypoint/login.go`

Add `patListItem` struct, `listPATs()`, and `ensurePAT()` functions. Extend `runAinstructLogin()`:

1. Read `PAT_LABEL` from env
2. After successful login, call `ensurePAT(apiURL, accessToken, label)`
3. `ensurePAT` flow:
   - `GET /api/v1/auth/pat` — list all PATs
   - Find PAT where `label == PAT_LABEL`
   - **Found**: `POST /api/v1/auth/pat/{pat_id}/rotate` with `{"expires_in_days": 30}` → return new token
   - **Not found**: `POST /api/v1/auth/pat` with `{"label": PAT_LABEL, "expires_in_days": 30}` → return token
4. Output `MCP_TOKEN=<token>` on success, nothing on failure (non-fatal)

### 3. Host-side: restructure `main.go` token flow — `cmd/kilo-docker/main.go`

**Current** (lines 187-221): Ainstruct block only sets sync tokens; MCP token loading gated by `cfg.mcp`.

**New**: After `ainstructLogin()`, auto-provision ainstruct MCP token regardless of `--mcp`:

```go
if cfg.ainstruct {
    result, err := ainstructLogin(repoURL + ":latest")
    // ... existing error handling ...
    dataVolume = deriveVolumeName(result.UserID)
    ainstructSyncToken = result.AccessToken
    ainstructSyncRefreshToken = result.RefreshToken
    ainstructSyncTokenExpiry = time.Now().Unix() + result.ExpiresIn
    os.Setenv("VOLUME_PASSWORD", result.UserID)

    // Auto-provision Ainstruct MCP token
    if result.MCPToken != "" {
        kdAinstructToken = result.MCPToken
    }
    // Load saved ainstruct token from volume (overrides PAT if present)
    if kdAinstructToken == "" && dataVolume != "" {
        _, savedAinstruct := loadTokens(repoURL+":latest", dataVolume, true, result.UserID)
        if savedAinstruct != "" {
            kdAinstructToken = savedAinstruct
        }
    }
    // Save auto-PAT to volume if no saved token existed
    if kdAinstructToken != "" {
        // Save (preserving any existing context7 token)
        saveTokens(repoURL+":latest", dataVolume, kdContext7Token, kdAinstructToken, true, result.UserID)
    }

    // --mcp: load/provision Context7 token only
    if cfg.mcp {
        if kdContext7Token == "" {
            token1, _ := loadTokens(repoURL+":latest", dataVolume, true, result.UserID)
            if token1 != "" { kdContext7Token = token1 }
        }
        if kdContext7Token == "" {
            promptMissingTokens(dataVolume, true, result.UserID)
            kdContext7Token = os.Getenv("KD_CONTEXT7_TOKEN")
        }
    }
}
```

Key changes:
- Ainstruct MCP token is auto-provisioned from PAT when `--ainstruct` is used (no `--mcp` needed)
- `--mcp` only handles Context7 token (load/prompt)
- `promptMissingTokens` only fires for Context7 when `--ainstruct` provides ainstruct token automatically
- Volume token still overrides PAT if it exists (handles token rotation gracefully)

### 4. Host-side: update env var gating — `cmd/kilo-docker/args.go`

Change `KD_AINSTRUCT_TOKEN` gating at line 80-82:

```go
// Before:
if cfg.mcp && kdAinstructToken != "" {

// After:
if (cfg.mcp || cfg.ainstruct) && kdAinstructToken != "" {
```

`KD_CONTEXT7_TOKEN` stays gated by `cfg.mcp` only.

### 5. Host-side: adjust prompt condition in `main.go` — `cmd/kilo-docker/main.go`

When `--ainstruct --mcp` together, change the condition from:
```go
if kdContext7Token == "" || kdAinstructToken == "" {
    promptMissingTokens(...)
}
```
to:
```go
if kdContext7Token == "" {
    promptMissingTokens(...)
}
```
The ainstruct token is already auto-provisioned, so we only prompt when Context7 is missing. The user can leave the ainstruct prompt empty within `promptMissingTokens` (it's already set).

### 6. Host-side: update help text — `cmd/kilo-docker/setup.go`

Line 151: Change `--mcp` description from:
`"Enable MCP servers (prompts for Context7 and Ainstruct API tokens)"`
to:
`"Enable remote MCP servers (prompts for Context7 API token)"`

## Files Modified

| File | Change |
|------|--------|
| `cmd/kilo-entrypoint/login.go` | Add `patListItem` struct, `listPATs()`, `ensurePAT()`; extend `runAinstructLogin()` to auto-provision PAT |
| `cmd/kilo-docker/ainstruct.go` | Add `buildPATLabel()`, `MCPToken` field, pass `PAT_LABEL`, parse `MCP_TOKEN` |
| `cmd/kilo-docker/main.go` | Restructure ainstruct block: auto-provision MCP token, relegate `--mcp` to Context7 only |
| `cmd/kilo-docker/args.go` | Change `KD_AINSTRUCT_TOKEN` gate from `cfg.mcp` to `cfg.mcp || cfg.ainstruct` |
| `cmd/kilo-docker/setup.go` | Update `--mcp` help text (ainstruct no longer prompted) |

## Verification

1. `kilo-docker --ainstruct` → login → PAT auto-created → Ainstruct MCP enabled (no token prompt, no `--mcp` needed)
2. `kilo-docker --ainstruct --mcp` → login → PAT auto-created → prompted for Context7 token only → both MCP servers enabled
3. `kilo-docker --mcp` (no ainstruct) → prompted for Context7 token (ainstruct unchanged)
4. Second `--ainstruct` run → token loaded from volume → no API call → Ainstruct MCP enabled
5. Volume lost → PAT API finds existing → rotates → saves → no duplicate
6. PAT API failure → non-fatal → ainstruct MCP disabled (sync still works)