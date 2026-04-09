# Plan: Remember Me Functionality for Ainstruct Login

## Context
Add "remember me" opt-in feature for ainstruct login using `--remember` flag. **Only SYNC tokens are affected** — MCP tokens remain unchanged.

**Scope (only SYNC tokens):**
- `--remember=false` (default): SYNC tokens in memory only, lost on container restart
- `--remember=true` on new login: Save SYNC tokens to encrypted storage
- `--remember=true` on restart with valid tokens: Auto-login with notice

**Unchanged (MCP tokens):**
- Passed as env vars to Kilo via `print-env` — works as before
- Never affected by `--remember` flag
- `sync_content.go` already reads SYNC tokens from encrypted storage

## Changes

### 1. Add --remember flag to runInit/runUserInit
- Parse `--remember` flag in `runInit()` (default: `false`)
- Pass `remember bool` to `runUserInit()`
- Pass `remember bool` to `runLoginInteractive()`

### 2. Modify runLoginInteractive to accept remember flag
- Update signature: `func runLoginInteractive(remember bool) (loginResult, error)`
- On successful login:
  - If `remember=true`: Save SYNC tokens to encrypted storage via `saveSyncTokensToEncrypted()`
  - If `remember=false`: Do not persist — tokens live in memory only
- MCP tokens (PAT) always saved via `initTokens()` regardless of remember flag

### 3. Add auto-login for existing users with --remember
- New function `checkAutoLogin(homeDir, userID string) (loginResult, bool, error)`:
  - Returns `(loginResult, autoLoggedIn bool, error)`
  - Load SYNC tokens from encrypted storage
  - Check if access token expired (`tokenExpiry` vs `time.Now().Unix()`)
  - If expired but refresh token valid → call `POST /auth/refresh`, update stored tokens
  - If tokens valid → return `loginResult` with `autoLoggedIn=true`
  - If refresh fails or no tokens → return `autoLoggedIn=false`, caller falls back to login

- In `runUserInit()` flow:
  ```
  IF remember=false AND SYNC tokens exist in storage:
    DELETE SYNC tokens from encrypted storage (keep MCP tokens)
  
  IF remember=true AND existing user AND SYNC tokens exist:
    result, autoLogin, err := checkAutoLogin(homeDir, userID)
    IF autoLogin:
      utils.Log("[kilo-docker] Signed in automatically using saved session\n", utils.WithOutput())
      continue with result
    IF error: proceed to interactive login
  
  ELSE: proceed to interactive login
  ```

- **User notification**: On auto-login success: "Signed in automatically using saved session"

### 3b. Clear SYNC tokens when remember=false
- If `remember=false` and SYNC tokens exist in encrypted storage, remove them
- New function: `clearSyncTokensFromEncrypted(homeDir, userID) error`
  - Load existing encrypted tokens
  - Set SYNC token fields to empty strings
  - Re-encrypt and save
  - MCP tokens remain untouched

### 4. Create saveSyncTokensToEncrypted helper
- New function: `saveSyncTokensToEncrypted(homeDir, userID, syncToken, refreshToken, expiry int64) error`
- Reads existing encrypted tokens, updates only SYNC fields, re-encrypts
- Used by both `runLoginInteractive()` and `checkAutoLogin()` (for token refresh)

## Files Modified
| File | Change |
|------|--------|
| cmd/kilo-entrypoint/main.go | Parse `--remember` flag |
| cmd/kilo-entrypoint/userinit.go | Add `checkAutoLogin()`, update `runUserInit()` to use remember flag |
| cmd/kilo-entrypoint/login.go | Update `runLoginInteractive()` to accept remember param |
| cmd/kilo-entrypoint/tokens.go | Add `saveSyncTokensToEncrypted()`, `clearSyncTokensFromEncrypted()` |

## Files Verified (No Change Needed)
| File | Status |
|------|--------|
| cmd/kilo-entrypoint/sync_content.go | Already reads from encrypted storage |
| cmd/kilo-entrypoint/main.go | `print-env` unchanged — MCP tokens only |
| scripts/kilo-wrapper.sh | Unchanged — reads MCP tokens only |

## Verification
- `--remember=false`: Login prompts for credentials, tokens NOT saved to `.tokens.env.enc`
- `--remember=false` (with existing SYNC tokens): SYNC tokens deleted from storage, MCP tokens preserved
- `--remember=true` (new user): Login prompts, on success tokens saved, no auto-login notice
- `--remember=true` (existing user, valid tokens): No login prompt, shows "Signed in automatically using saved session"
- `--remember=true` (existing user, expired token): Login prompt, refresh succeeds, tokens updated
- `--remember=true` (existing user, invalid refresh): Login prompt, user must re-authenticate
- Verify SYNC tokens never in env vars: `env | grep KD_AINSTRUCT_SYNC` returns nothing
- Verify MCP tokens still work: Kilo MCP functionality unaffected
- Run `go vet ./...` and `golangci-lint run ./...`