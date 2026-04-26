package main

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/mbabic84/kilo-docker/pkg/utils"
)

// applyMCPEnabledFromEnv updates opencode.json MCP enabled states by reading tokens
// from encrypted storage. It does NOT read from environment variables for security - 
// tokens are only available in env vars during the Kilo session managed by kilo-wrapper.sh.
//
// This function reads tokens directly from the encrypted file to determine which MCP
// servers should be enabled, ensuring consistent behavior whether called during startup
// or manually.
func applyMCPEnabledFromEnv(homeDir string) error {
	utils.Log("[MCP Config] Starting MCP enabled state application\n")

	// Auto-detect homeDir
	if homeDir == "" {
		utils.Log("[MCP Config] homeDir not provided, auto-detecting via loadUserConfig()\n")
		homeDir, _, _, _ = loadUserConfig()
		if homeDir != "" {
			utils.Log("[MCP Config] Auto-detected homeDir: %s\n", homeDir)
		}
	}

	// Determine enabled states
	// playwright is controlled by env var (not stored in encrypted tokens)
	playwrightEnabled := os.Getenv("PLAYWRIGHT_ENABLED") == "1"

	// context7 and ainstruct tokens are read from encrypted storage only
	var context7Token, ainstructToken string

	if homeDir != "" {
		// We need userID to decrypt
		_, _, _, userID := loadUserConfig()
		if userID != "" {
			if encData, err := os.ReadFile(filepath.Join(homeDir, ".local/share/kilo/.tokens.env.enc")); err == nil {
				if decrypted, err := decryptAES(encData, userID); err == nil {
					c7, aInst, _, _, _, _ := parseTokenEnv(string(decrypted))
					context7Token = c7
					ainstructToken = aInst
					utils.Log("[MCP Config] Loaded tokens from encrypted storage\n")
				} else {
					utils.Log("[MCP Config] Failed to decrypt token file: %v\n", err)
				}
			} else {
				utils.Log("[MCP Config] No encrypted token file found: %v\n", err)
			}
		} else {
			utils.Log("[MCP Config] No userID available to decrypt tokens\n")
		}
	} else {
		utils.Log("[MCP Config] No homeDir available to load encrypted tokens\n")
	}

	context7Enabled := context7Token != ""
	context7TokenLen := len(context7Token)

	ainstructEnabled := ainstructToken != ""
	aintTokenLen := len(ainstructToken)

	// Log what we found (token lengths only, not actual values)
	utils.Log("[MCP Config] Environment check - PLAYWRIGHT_ENABLED=%v, context7=[%d chars], ainstruct=[%d chars]\n",
		playwrightEnabled, context7TokenLen, aintTokenLen)
	utils.Log("[MCP Config] Determined enabled states - playwright=%v, context7=%v, ainstruct=%v\n",
		playwrightEnabled, context7Enabled, ainstructEnabled)

	// Check if we have homeDir for config update
	if homeDir == "" {
		utils.LogWarn("[MCP Config] No homeDir available, cannot update MCP config\n")
		return nil
	}

	configPath := filepath.Join(homeDir, ".config", "kilo", "opencode.json")
	utils.Log("[MCP Config] Target config file: %s\n", configPath)

	if err := updateMCPEnabledStates(configPath, playwrightEnabled, context7Enabled, ainstructEnabled); err != nil {
		utils.LogWarn("[MCP Config] Failed to update %s: %v\n", configPath, err)
		return err
	}

	utils.Log("[MCP Config] Successfully applied MCP enabled states\n")
	return nil
}

// updateMCPEnabledStates rewrites opencode.json with updated MCP enabled states.
// It preserves all other configuration fields (url, headers, type, etc).
// Servers are enabled/disabled based solely on the boolean parameters passed.
func updateMCPEnabledStates(configPath string, playwrightEnabled, context7Enabled, ainstructEnabled bool) error {
	utils.Log("[MCP Config] Reading config file: %s\n", configPath)

	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			utils.LogWarn("[MCP Config] Config file does not exist: %s\n", configPath)
			return nil // Config doesn't exist, nothing to update
		}
		utils.LogError("[MCP Config] Failed to read config file: %v\n", err)
		return err
	}
	utils.Log("[MCP Config] Config file read successfully (%d bytes)\n", len(data))

	var config map[string]any
	if err := json.Unmarshal(data, &config); err != nil {
		utils.LogError("[MCP Config] Failed to parse JSON: %v\n", err)
		return err
	}
	utils.Log("[MCP Config] JSON parsed successfully\n")

	mcpRaw, ok := config["mcp"]
	if !ok {
		utils.LogWarn("[MCP Config] No 'mcp' section found in config, nothing to update\n")
		return nil // No MCP section, nothing to update
	}
	mcp, ok := mcpRaw.(map[string]any)
	if !ok {
		utils.LogError("[MCP Config] 'mcp' section is not an object\n")
		return nil // MCP section is not an object
	}
	utils.Log("[MCP Config] Found MCP section with %d server(s)\n", len(mcp))

	updatedCount := 0
	for key, entryRaw := range mcp {
		entry, ok := entryRaw.(map[string]any)
		if !ok {
			utils.LogWarn("[MCP Config] Skipping invalid MCP entry for '%s' (not an object)\n", key)
			continue
		}

		oldEnabled := entry["enabled"]

		// Update enabled state based on server type
		switch key {
		case "playwright":
			entry["enabled"] = playwrightEnabled
			utils.Log("[MCP Config] playwright: enabled=%v (was %v)\n", playwrightEnabled, oldEnabled)
			updatedCount++
		case "context7":
			entry["enabled"] = context7Enabled
			utils.Log("[MCP Config] context7: enabled=%v (was %v)\n", context7Enabled, oldEnabled)
			updatedCount++
		case "ainstruct":
			entry["enabled"] = ainstructEnabled
			utils.Log("[MCP Config] ainstruct: enabled=%v (was %v)\n", ainstructEnabled, oldEnabled)
			updatedCount++
		default:
			utils.Log("[MCP Config] Skipping unknown MCP server '%s'\n", key)
		}

		mcp[key] = entry
	}

	out, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		utils.LogError("[MCP Config] Failed to marshal JSON: %v\n", err)
		return err
	}

	// Atomic write: write to temp file first, then rename
	tmpPath := configPath + ".tmp"
	utils.Log("[MCP Config] Writing to temp file: %s\n", tmpPath)
	if err := os.WriteFile(tmpPath, out, 0o600); err != nil {
		utils.LogError("[MCP Config] Failed to write temp file: %v\n", err)
		return err
	}

	utils.Log("[MCP Config] Renaming temp file to: %s\n", configPath)
	if err := os.Rename(tmpPath, configPath); err != nil {
		utils.LogError("[MCP Config] Failed to rename temp file: %v\n", err)
		return err
	}

	utils.Log("[MCP Config] Successfully updated %d MCP server(s)\n", updatedCount)
	return nil
}
