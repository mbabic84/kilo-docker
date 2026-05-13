package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// saveEncryptedTokens encrypts MCP and sync tokens with userID as the key
// and writes them to <home>/.local/share/kilo/.tokens.env.enc.
func saveEncryptedTokens(home, userID, context7Token, ainstructToken, syncToken, syncRefreshToken, syncTokenExpiry, patExpiry string) error {
	tokenData := fmt.Sprintf("KD_MCP_CONTEXT7_TOKEN=%s\nKD_MCP_AINSTRUCT_TOKEN=%s\nKD_AINSTRUCT_SYNC_TOKEN=%s\nKD_AINSTRUCT_SYNC_REFRESH_TOKEN=%s\nKD_AINSTRUCT_SYNC_TOKEN_EXPIRY=%s\nKD_AINSTRUCT_PAT_EXPIRY=%s\n",
		context7Token, ainstructToken, syncToken, syncRefreshToken, syncTokenExpiry, patExpiry)
	encData, err := encryptAES([]byte(tokenData), userID)
	if err != nil {
		return err
	}
	tokenPath := filepath.Join(home, ".local/share/kilo/.tokens.env.enc")
	return os.WriteFile(tokenPath, encData, 0600)
}

// loadEncryptedTokens reads and decrypts MCP and sync tokens from the volume.
// Returns empty strings for any missing fields rather than an error.
func loadEncryptedTokens(home, userID string) (context7, ainstruct, syncToken, syncRefresh, syncExpiry, patExpiry string, err error) {
	tokenPath := filepath.Join(home, ".local/share/kilo/.tokens.env.enc")
	encData, err := os.ReadFile(tokenPath)
	if err != nil {
		return "", "", "", "", "", "", err
	}
	decrypted, err := decryptAES(encData, userID)
	if err != nil {
		return "", "", "", "", "", "", err
	}
	return parseTokenEnv(string(decrypted))
}

// saveSyncTokensToEncrypted loads existing encrypted tokens, updates only the
// SYNC token fields, and re-encrypts. Preserves MCP tokens (Context7, Ainstruct).
func saveSyncTokensToEncrypted(home, userID, syncToken, syncRefreshToken, syncTokenExpiry string) error {
	context7Token, ainstructToken, patExpiry := "", "", ""

	storedContext7, storedAinstruct, _, _, _, storedPatExpiry, loadErr := loadEncryptedTokens(home, userID)
	if loadErr == nil {
		if storedContext7 != "" {
			context7Token = storedContext7
		}
		if storedAinstruct != "" {
			ainstructToken = storedAinstruct
		}
		if storedPatExpiry != "" {
			patExpiry = storedPatExpiry
		}
	}

	return saveEncryptedTokens(home, userID, context7Token, ainstructToken, syncToken, syncRefreshToken, syncTokenExpiry, patExpiry)
}

// clearSyncTokensFromEncrypted loads existing encrypted tokens, clears only
// the SYNC token fields, and re-encrypts. Preserves MCP tokens.
func clearSyncTokensFromEncrypted(home, userID string) error {
	context7Token, ainstructToken, patExpiry := "", "", ""

	storedContext7, storedAinstruct, _, _, _, storedPatExpiry, loadErr := loadEncryptedTokens(home, userID)
	if loadErr == nil {
		if storedContext7 != "" {
			context7Token = storedContext7
		}
		if storedAinstruct != "" {
			ainstructToken = storedAinstruct
		}
		if storedPatExpiry != "" {
			patExpiry = storedPatExpiry
		}
	}

	return saveEncryptedTokens(home, userID, context7Token, ainstructToken, "", "", "", patExpiry)
}

// parseTokenEnv extracts token values from KEY=VALUE formatted data.
func parseTokenEnv(data string) (context7, ainstruct, syncToken, syncRefresh, syncExpiry, patExpiry string, err error) {
	for _, line := range strings.Split(data, "\n") {
		switch {
		case strings.HasPrefix(line, "KD_MCP_CONTEXT7_TOKEN="):
			context7 = strings.TrimPrefix(line, "KD_MCP_CONTEXT7_TOKEN=")
		case strings.HasPrefix(line, "KD_MCP_AINSTRUCT_TOKEN="):
			ainstruct = strings.TrimPrefix(line, "KD_MCP_AINSTRUCT_TOKEN=")
		case strings.HasPrefix(line, "KD_AINSTRUCT_SYNC_TOKEN="):
			syncToken = strings.TrimPrefix(line, "KD_AINSTRUCT_SYNC_TOKEN=")
		case strings.HasPrefix(line, "KD_AINSTRUCT_SYNC_REFRESH_TOKEN="):
			syncRefresh = strings.TrimPrefix(line, "KD_AINSTRUCT_SYNC_REFRESH_TOKEN=")
		case strings.HasPrefix(line, "KD_AINSTRUCT_SYNC_TOKEN_EXPIRY="):
			syncExpiry = strings.TrimPrefix(line, "KD_AINSTRUCT_SYNC_TOKEN_EXPIRY=")
		case strings.HasPrefix(line, "KD_AINSTRUCT_PAT_EXPIRY="):
			patExpiry = strings.TrimPrefix(line, "KD_AINSTRUCT_PAT_EXPIRY=")
		}
	}
	return context7, ainstruct, syncToken, syncRefresh, syncExpiry, patExpiry, nil
}