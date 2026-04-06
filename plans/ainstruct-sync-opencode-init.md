# Plan: Fix ainstruct sync opencode.json initialization race condition

## Context

**Problem**: When a user starts a container and updates `opencode.json` before the initial sync pull completes, their changes are lost because:

1. At T0: Template `opencode.json` is copied to `~/.config/kilo/opencode.json` during user initialization
2. At T1: Sync process starts in background (with goroutine delay)
3. At T2: User edits `opencode.json` 
4. At T3: `pullCollection()` runs and **always overwrites local files** when no hash cache exists

The `pullCollection()` logic assumes local files are "stale" and remote is "source of truth". Without a hash cache (`.ainstruct-hashes`), it cannot detect local modifications made after container start.

**Current Behavior**:
```go
// userinit.go:99
copyFileIfMissing("/etc/kilo/template-opencode.json", filepath.Join(homeDir, ".config/kilo/opencode.json"))

// sync.go:36 - Pull runs AFTER sync process starts
if err := s.pullCollection(); err != nil {
    log.Printf("[ainstruct-sync] Pull failed: %v", err)
}
```

## Changes

### 1. Modify user initialization to check remote before copying template

**File**: `cmd/kilo-entrypoint/userinit.go`

**Before line 99**, add synchronous remote check logic:

```go
// Check if this is first-time initialization (no hash cache exists)
hashFile := filepath.Join(homeDir, ".config/kilo/.ainstruct-hashes")
localOpencode := filepath.Join(homeDir, ".config/kilo/opencode.json")

if _, err := os.Stat(hashFile); os.IsNotExist(err) {
    // No prior sync - check if remote collection has opencode.json
    hasRemote, checkErr := checkRemoteHasOpencode(homeDir, userID)
    if checkErr != nil || !hasRemote {
        // No remote or check failed - copy template as fallback
        copyFileIfMissing("/etc/kilo/template-opencode.json", localOpencode)
    }
    // If hasRemote is true, skip template - sync will pull from remote
} else {
    // User has synced before - don't touch existing config
    // copyFileIfMissing won't overwrite anyway, but skip the check entirely
}
```

Remove the existing line 99:
```go
// REMOVE: copyFileIfMissing("/etc/kilo/template-opencode.json", filepath.Join(homeDir, ".config/kilo/opencode.json"))
```

### 2. Add helper function to check remote for opencode.json

**File**: `cmd/kilo-entrypoint/userinit.go` (add new function)

```go
// checkRemoteHasOpencode performs a synchronous check to see if the remote
// collection contains an opencode.json document. Returns true if it exists.
// This runs during user initialization (before privilege drop) to determine
// whether to copy the template file or let sync pull from remote.
func checkRemoteHasOpencode(homeDir, userID string) (bool, error) {
    // Load sync tokens from encrypted storage
    encPath := filepath.Join(homeDir, ".local/share/kilo/.tokens.env.enc")
    encData, err := os.ReadFile(encPath)
    if err != nil {
        return false, fmt.Errorf("no encrypted tokens found: %w", err)
    }
    
    decrypted, err := decryptAES(encData, userID)
    if err != nil {
        return false, fmt.Errorf("failed to decrypt tokens: %w", err)
    }
    
    _, _, syncToken, _, _, _ := parseTokenEnv(string(decrypted))
    if syncToken == "" {
        return false, fmt.Errorf("no sync token available")
    }
    
    // Build minimal syncer to query remote
    baseURL := os.Getenv("KD_AINSTRUCT_BASE_URL")
    if baseURL == "" {
        baseURL = "https://ainstruct-dev.kralicinora.cz"
    }
    
    // Find collection by name
    collectionsURL := baseURL + "/api/v1/collections"
    req, _ := http.NewRequest("GET", collectionsURL, nil)
    req.Header.Set("Authorization", "Bearer "+syncToken)
    
    client := &http.Client{Timeout: 10 * time.Second}
    resp, err := client.Do(req)
    if err != nil {
        return false, err
    }
    defer resp.Body.Close()
    
    if resp.StatusCode != 200 {
        return false, fmt.Errorf("collections API returned %d", resp.StatusCode)
    }
    
    var result struct {
        Collections []struct {
            CollectionID string `json:"collection_id"`
            Name         string `json:"name"`
        } `json:"collections"`
    }
    
    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        return false, err
    }
    
    var collectionID string
    for _, c := range result.Collections {
        if c.Name == "kilo-docker" {
            collectionID = c.CollectionID
            break
        }
    }
    
    if collectionID == "" {
        return false, nil // No collection = no remote opencode.json
    }
    
    // Check if collection has opencode.json document
    docsURL := baseURL + "/api/v1/documents?collection_id=" + collectionID
    req, _ = http.NewRequest("GET", docsURL, nil)
    req.Header.Set("Authorization", "Bearer "+syncToken)
    
    resp, err = client.Do(req)
    if err != nil {
        return false, err
    }
    defer resp.Body.Close()
    
    if resp.StatusCode != 200 {
        return false, fmt.Errorf("documents API returned %d", resp.StatusCode)
    }
    
    var docsResult struct {
        Documents []struct {
            Metadata struct {
                LocalPath string `json:"local_path"`
            } `json:"metadata"`
        } `json:"documents"`
    }
    
    if err := json.NewDecoder(resp.Body).Decode(&docsResult); err != nil {
        return false, err
    }
    
    for _, d := range docsResult.Documents {
        if d.Metadata.LocalPath == "opencode.json" {
            return true, nil
        }
    }
    
    return false, nil
}
```

### 3. Add necessary imports to userinit.go

**File**: `cmd/kilo-entrypoint/userinit.go`

Add to imports section:
```go
import (
    // ... existing imports ...
    "encoding/json"
    "net/http"
)
```

## Files Modified

| File | Change |
|------|--------|
| `cmd/kilo-entrypoint/userinit.go` | Replace template copy with conditional logic based on remote check |
| `cmd/kilo-entrypoint/userinit.go` | Add `checkRemoteHasOpencode()` helper function |
| `cmd/kilo-entrypoint/userinit.go` | Add `encoding/json` and `net/http` imports |

## Verification

### Test Scenarios

1. **Fresh user, no remote collection**
   - Expected: Template `opencode.json` copied, sync creates collection and pushes

2. **Returning user with remote opencode.json**
   - Expected: Template skipped, sync pulls remote version

3. **Volume-mounted opencode.json (no hash cache)**
   - Expected: Remote check finds no document → but local exists → template NOT copied

4. **Network failure during remote check**
   - Expected: Falls back to template copy

### Commands to Run

```bash
# Build and test
cd /home/mbabic/projects/github/kilo-docker
go build -o /tmp/kilo-entrypoint ./cmd/kilo-entrypoint

# Run tests
go test ./cmd/kilo-entrypoint/... -v
```

## Design Decisions

1. **Synchronous check during initialization**: Ensures decision is made before user can edit files
2. **Hash file as sync state indicator**: Presence of `.ainstruct-hashes` means user has synced before
3. **Fallback to template on any error**: Network issues or auth problems don't block container startup
4. **No changes to sync process**: The existing `pullCollection()` and watcher logic remain unchanged

## Edge Cases Handled

| Scenario | Behavior |
|----------|----------|
| Hash cache exists | Skip all logic - user has synced before |
| No hash cache, remote has opencode.json | Skip template, let sync pull |
| No hash cache, no remote opencode.json | Copy template |
| No hash cache, local file exists (volume mount) | Template copy skipped by `copyFileIfMissing` semantics |
| Network/auth error | Log warning, fallback to template |
| Sync token missing | Fallback to template |
