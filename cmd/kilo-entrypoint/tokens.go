package main

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// TokenStore is the in-memory representation of the encrypted token file.
// The on-disk format is one KEY=VALUE line per entry, so a map is a natural
// fit and lets us preserve keys we do not know about (forward compatibility).
type TokenStore map[string]string

const (
	tokenKeyContext7      = "KD_MCP_CONTEXT7_TOKEN"
	tokenKeyAinstruct     = "KD_MCP_AINSTRUCT_TOKEN"
	tokenKeySync          = "KD_AINSTRUCT_SYNC_TOKEN"
	tokenKeySyncRefresh   = "KD_AINSTRUCT_SYNC_REFRESH_TOKEN"
	tokenKeySyncExpiry    = "KD_AINSTRUCT_SYNC_TOKEN_EXPIRY"
	tokenKeyPATExtiry     = "KD_AINSTRUCT_PAT_EXPIRY"
	tokenKeyGiteaToken    = "KD_MCP_GITEA_TOKEN"
	tokenKeyGiteaHost     = "KD_GITEA_HOST"
	tokenKeyGiteaInsecure = "KD_GITEA_INSECURE"
)

// Get returns the value for key, or "" when unset.
func (s TokenStore) Get(key string) string {
	return s[key]
}

// Set stores value under key. Empty values are kept so callers can persist an
// explicit "cleared" state without losing the key itself.
func (s TokenStore) Set(key, value string) {
	s[key] = value
}

// Has reports whether key is present (even with an empty value).
func (s TokenStore) Has(key string) bool {
	_, ok := s[key]
	return ok
}

func tokenFilePath(home string) string {
	return filepath.Join(home, ".local/share/kilo-docker/.tokens.env.enc")
}

// parseTokenEnv parses KEY=VALUE formatted data. Unknown keys are preserved so
// tokens written by a newer version survive a read/write round trip.
func parseTokenEnv(data []byte) TokenStore {
	store := TokenStore{}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSuffix(line, "\r")
		key, value, ok := strings.Cut(line, "=")
		if !ok || key == "" {
			continue
		}
		store[key] = value
	}
	return store
}

// loadTokenStore reads and decrypts the token file. A missing or unreadable
// file returns an empty (non-nil) store together with the error so callers can
// decide whether to treat it as fatal.
func loadTokenStore(home, userID string) (TokenStore, error) {
	encData, err := os.ReadFile(tokenFilePath(home))
	if err != nil {
		return TokenStore{}, err
	}
	decrypted, err := decryptAES(encData, userID)
	if err != nil {
		return TokenStore{}, err
	}
	return parseTokenEnv(decrypted), nil
}

// encode serializes the store as KEY=VALUE lines in sorted key order for
// deterministic output.
func (s TokenStore) encode() []byte {
	keys := make([]string, 0, len(s))
	for key := range s {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	var b strings.Builder
	for _, key := range keys {
		b.WriteString(key)
		b.WriteString("=")
		b.WriteString(s[key])
		b.WriteString("\n")
	}
	return []byte(b.String())
}

// saveTokenStore encrypts the store with userID as the key and writes it to
// <home>/.local/share/kilo-docker/.tokens.env.enc.
func saveTokenStore(home, userID string, store TokenStore) error {
	encData, err := encryptAES(store.encode(), userID)
	if err != nil {
		return err
	}
	tokenPath := tokenFilePath(home)
	// Write atomically (temp + rename) so a concurrent reader never sees a
	// truncated/empty token file and a crash mid-write cannot corrupt it.
	tmp, err := os.CreateTemp(filepath.Dir(tokenPath), ".tokens.env.enc.*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	if _, err := tmp.Write(encData); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return err
	}
	if err := tmp.Chmod(0600); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	if err := os.Rename(tmpPath, tokenPath); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	return nil
}

// saveSyncTokensToEncrypted loads the existing store, updates only the SYNC
// token fields, and re-encrypts. Every other key (MCP tokens, Gitea settings,
// and unknown future keys) is preserved by the read-modify-write cycle.
func saveSyncTokensToEncrypted(home, userID, syncToken, syncRefreshToken, syncTokenExpiry string) error {
	store, err := loadTokenStore(home, userID)
	if err != nil {
		// Only a missing file is safe to treat as empty. Any other failure
		// (unreadable or undecryptable store) must abort instead of replacing
		// the existing tokens with an empty store.
		if !os.IsNotExist(err) {
			return err
		}
		store = TokenStore{}
	}
	store.Set(tokenKeySync, syncToken)
	store.Set(tokenKeySyncRefresh, syncRefreshToken)
	store.Set(tokenKeySyncExpiry, syncTokenExpiry)
	return saveTokenStore(home, userID, store)
}

// clearSyncTokensFromEncrypted loads the existing store, clears only the SYNC
// token fields, and re-encrypts. Every other key is preserved.
func clearSyncTokensFromEncrypted(home, userID string) error {
	store, err := loadTokenStore(home, userID)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		store = TokenStore{}
	}
	store.Set(tokenKeySync, "")
	store.Set(tokenKeySyncRefresh, "")
	store.Set(tokenKeySyncExpiry, "")
	return saveTokenStore(home, userID, store)
}
