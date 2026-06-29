package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/mbabic84/kilo-docker/pkg/utils"
)

// mcpServerDefaults defines the canonical MCP server entries.
// When a server is enabled and the entry is missing from kilo.jsonc
// (e.g. user edited it out, or config predates a template addition),
// the entry is inserted with these defaults before the enabled toggle.
var mcpServerDefaults = map[string]map[string]any{
	"playwright": {
		"type":    "remote",
		"url":     "http://kilo-playwright-mcp:8931/mcp",
		"enabled": false,
	},
	"gitnexus": {
		"type":    "local",
		"command": []any{"gitnexus", "mcp"},
		"enabled": false,
	},
	"github": {
		"type":    "local",
		"command": []any{"gh", "mcp"},
		"enabled": false,
	},
	"ainstruct": {
		"type":    "remote",
		"url":     "{env:KD_AINSTRUCT_BASE_URL}/mcp",
		"enabled": false,
		"headers": map[string]any{
			"Authorization": "Bearer {env:KD_MCP_AINSTRUCT_TOKEN}",
		},
	},
	"context7": {
		"type":    "remote",
		"url":     "https://mcp.context7.com/mcp",
		"enabled": false,
		"headers": map[string]any{
			"CONTEXT7_API_KEY": "{env:KD_MCP_CONTEXT7_TOKEN}",
		},
	},
}

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

	// Determine enabled states for env-var-controlled servers
	servicesEnv := os.Getenv("KD_SERVICES")
	enabled := map[string]bool{
		"playwright": os.Getenv("PLAYWRIGHT_ENABLED") == "1",
		"gitnexus":   os.Getenv("GITNEXUS_ENABLED") == "1",
		"github":     strings.Contains(","+servicesEnv+",", ",gh,"),
	}

	// context7 and ainstruct tokens are read from encrypted storage only
	var context7Token, ainstructToken string

	if homeDir != "" {
		_, _, _, userID := loadUserConfig()
		if userID != "" {
			if encData, err := os.ReadFile(filepath.Join(homeDir, ".local/share/kilo-docker/.tokens.env.enc")); err == nil {
				if decrypted, err := decryptAES(encData, userID); err == nil {
					c7, aInst, _, _, _, _, _ := parseTokenEnv(string(decrypted))
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

	enabled["context7"] = context7Token != ""
	enabled["ainstruct"] = ainstructToken != ""

	context7TokenLen := len(context7Token)
	aintTokenLen := len(ainstructToken)

	utils.Log("[MCP Config] Environment check - PLAYWRIGHT_ENABLED=%v, GITNEXUS_ENABLED=%v, context7=[%d chars], ainstruct=[%d chars]\n",
		enabled["playwright"], enabled["gitnexus"], context7TokenLen, aintTokenLen)
	utils.Log("[MCP Config] Determined enabled states - playwright=%v, gitnexus=%v, context7=%v, ainstruct=%v\n",
		enabled["playwright"], enabled["gitnexus"], enabled["context7"], enabled["ainstruct"])

	// Check if we have homeDir for config update
	if homeDir == "" {
		utils.LogWarn("[MCP Config] No homeDir available, cannot update MCP config\n")
		return nil
	}

	configPath := filepath.Join(homeDir, ".config", "kilo", "kilo.jsonc")
	utils.Log("[MCP Config] Target config file: %s\n", configPath)

	// Ensure MCP server entries exist before toggling enabled state.
	// kilo.jsonc persists across container restarts/recreations — existing
	// users may be missing entries for servers added after their config
	// was created, or may have removed entries manually.
	for name, entryDef := range mcpServerDefaults {
		if enabled[name] {
			if err := ensureMCPEntry(configPath, name, entryDef); err != nil {
				utils.LogWarn("[MCP Config] Failed to ensure %s MCP entry: %v\n", name, err)
			}
		}
	}

	if err := migrateContext7Header(configPath); err != nil {
		utils.LogWarn("[MCP Config] Failed to migrate context7 header: %v\n", err)
	}

	if err := updateMCPEnabledStates(configPath, enabled); err != nil {
		utils.LogWarn("[MCP Config] Failed to update %s: %v\n", configPath, err)
		return err
	}

	utils.Log("[MCP Config] Successfully applied MCP enabled states\n")
	return nil
}

// updateMCPEnabledStates rewrites opencode.json with updated MCP enabled states.
// It preserves all other configuration fields (url, headers, type, etc).
// Servers are enabled/disabled based on the enabled map passed in.
func updateMCPEnabledStates(configPath string, enabled map[string]bool) error {
	utils.Log("[MCP Config] Reading config file: %s\n", configPath)

	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			utils.LogWarn("[MCP Config] Config file does not exist: %s\n", configPath)
			return nil
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
		return nil
	}
	mcp, ok := mcpRaw.(map[string]any)
	if !ok {
		utils.LogError("[MCP Config] 'mcp' section is not an object\n")
		return nil
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

		if val, known := enabled[key]; known {
			entry["enabled"] = val
			utils.Log("[MCP Config] %s: enabled=%v (was %v)\n", key, val, oldEnabled)
			updatedCount++
		} else {
			utils.Log("[MCP Config] Skipping unknown MCP server '%s'\n", key)
		}

		mcp[key] = entry
	}

	out, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		utils.LogError("[MCP Config] Failed to marshal JSON: %v\n", err)
		return err
	}

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

// ensureMCPEntry adds entryDef to config["mcp"][name] if it doesn't already exist.
// Writes back to configPath only when a change was made.
// Uses the same temp-file+rename atomic write pattern as updateMCPEnabledStates.
func ensureMCPEntry(configPath, name string, entryDef map[string]any) error {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}

	var configObj map[string]any
	if err := json.Unmarshal(data, &configObj); err != nil {
		return err
	}

	mcpRaw, ok := configObj["mcp"]
	if !ok {
		configObj["mcp"] = map[string]any{}
		mcpRaw = configObj["mcp"]
	}
	mcp, ok := mcpRaw.(map[string]any)
	if !ok {
		return nil
	}

	if _, exists := mcp[name]; exists {
		utils.Log("[MCP Config] MCP entry %q already exists, skipping insert\n", name)
		return nil
	}

	utils.Log("[MCP Config] Inserting missing MCP entry %q\n", name)
	mcp[name] = entryDef
	configObj["mcp"] = mcp

	out, err := json.MarshalIndent(configObj, "", "  ")
	if err != nil {
		return err
	}

	tmpPath := configPath + ".tmp"
	if err := os.WriteFile(tmpPath, out, 0o600); err != nil {
		return err
	}
	return os.Rename(tmpPath, configPath)
}

// migrateContext7Header migrates the old Context7 authorization header format
// ("Authorization: Bearer <key>") to the new format ("CONTEXT7_API_KEY: <key>").
// This is idempotent — no-ops if already correct.
func migrateContext7Header(configPath string) error {
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var config map[string]any
	if err := json.Unmarshal(data, &config); err != nil {
		return err
	}

	mcpRaw, ok := config["mcp"]
	if !ok {
		return nil
	}
	mcp, ok := mcpRaw.(map[string]any)
	if !ok {
		return nil
	}

	ctx7Raw, ok := mcp["context7"]
	if !ok {
		return nil
	}
	ctx7, ok := ctx7Raw.(map[string]any)
	if !ok {
		return nil
	}

	headersRaw, ok := ctx7["headers"]
	if !ok {
		return nil
	}
	headers, ok := headersRaw.(map[string]any)
	if !ok {
		return nil
	}

	authVal, hasAuth := headers["Authorization"]
	ctx7Val, hasCtx7Key := headers["CONTEXT7_API_KEY"]

	migrated := false

	// Migrate from old "Authorization: Bearer <key>" format
	if hasAuth && !hasCtx7Key {
		val, _ := authVal.(string)
		headers["CONTEXT7_API_KEY"] = strings.TrimPrefix(val, "Bearer ")
		delete(headers, "Authorization")
		migrated = true
	}

	// Fix already-migrated header that still contains "Bearer " prefix
	if hasCtx7Key {
		if val, ok := ctx7Val.(string); ok && strings.HasPrefix(val, "Bearer ") {
			headers["CONTEXT7_API_KEY"] = strings.TrimPrefix(val, "Bearer ")
			migrated = true
		}
	}

	if migrated {
		utils.Log("[MCP Config] Migrating context7 header: Authorization -> CONTEXT7_API_KEY\n")

		out, err := json.MarshalIndent(config, "", "  ")
		if err != nil {
			return err
		}

		tmpPath := configPath + ".tmp"
		if err := os.WriteFile(tmpPath, out, 0o600); err != nil {
			return err
		}
		if err := os.Rename(tmpPath, configPath); err != nil {
			return err
		}
	}

	return nil
}