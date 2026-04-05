package main

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/mbabic84/kilo-docker/pkg/utils"
)

// applyMCPEnabledFromEnv updates opencode.json MCP enabled states based on environment variables.
// This function should be called AFTER the kilo wrapper script has set KD_MCP_* tokens as env vars.
//
// NOTE: There is a known issue where Kilo CLI 7.1.20 does not respect the enabled field in
// container environments (works fine on host). See docs/MCP_ENABLED_KNOWN_ISSUE.md for details.
// Users must manually enable MCPs via Ctrl+P until this is fixed upstream.
// It enables/disables MCP servers according to whether their respective tokens are present:
//   - context7: enabled if KD_MCP_CONTEXT7_TOKEN is non-empty
//   - ainstruct: enabled if KD_MCP_AINSTRUCT_TOKEN is non-empty
//   - playwright: enabled if PLAYWRIGHT_ENABLED == "1"
// The homeDir parameter can be empty - if so, it will be auto-detected via loadUserConfig().
func applyMCPEnabledFromEnv(homeDir string) error {
	utils.Log("[MCP Config] Starting MCP enabled state application\n")

	// Determine enabled states from environment variables (set by kilo-wrapper.sh)
	playwrightEnabled := os.Getenv("PLAYWRIGHT_ENABLED") == "1"
	context7Token := os.Getenv("KD_MCP_CONTEXT7_TOKEN")
	context7Enabled := context7Token != ""
	context7TokenLen := len(context7Token)

	ainstructToken := os.Getenv("KD_MCP_AINSTRUCT_TOKEN")
	ainstructEnabled := ainstructToken != ""
	aintTokenLen := len(ainstructToken)

	// Log what we found (token lengths only, not actual values)
	utils.Log("[MCP Config] Environment check - PLAYWRIGHT_ENABLED=%v, KD_MCP_CONTEXT7_TOKEN=[%d chars], KD_MCP_AINSTRUCT_TOKEN=[%d chars]\n",
		playwrightEnabled, context7TokenLen, aintTokenLen)
	utils.Log("[MCP Config] Determined enabled states - playwright=%v, context7=%v, ainstruct=%v\n",
		playwrightEnabled, context7Enabled, ainstructEnabled)

	// Auto-detect homeDir if not provided
	if homeDir == "" {
		utils.Log("[MCP Config] homeDir not provided, auto-detecting via loadUserConfig()\n")
		homeDir, _, _, _ = loadUserConfig()
		if homeDir != "" {
			utils.Log("[MCP Config] Auto-detected homeDir: %s\n", homeDir)
		} else {
			utils.LogWarn("[MCP Config] Failed to auto-detect homeDir, skipping MCP config update\n")
		}
	} else {
		utils.Log("[MCP Config] Using provided homeDir: %s\n", homeDir)
	}

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
	if err := os.WriteFile(tmpPath, out, 0644); err != nil {
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
