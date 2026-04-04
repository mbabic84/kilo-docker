package main

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/mbabic84/kilo-docker/pkg/constants"
	"github.com/mbabic84/kilo-docker/pkg/utils"
)

// runConfig toggles MCP server enabled/disabled state in the user's
// opencode.json based on environment variables.
func runConfig() error {
	mcpEnabled := os.Getenv("KD_MCP_ENABLED") == "1"

	mapping := map[string]string{
		"ainstruct": "KD_AINSTRUCT_TOKEN",
		"context7":  "KD_CONTEXT7_TOKEN",
	}

	playwrightEnabled := os.Getenv("PLAYWRIGHT_ENABLED") == "1"

	configPath := filepath.Join(constants.GetKiloConfigDir(), "opencode.json")
	if err := applyConfigFilter(configPath, mapping, mcpEnabled, playwrightEnabled); err != nil {
		utils.LogWarn("config error for %s: %v\n", configPath, err)
	}
	return nil
}

// applyConfigFilter reads a JSON config file and toggles MCP server entries
// based on environment variables. The mapping connects server names to their
// token env vars; when mcpEnabled is true, servers with non-empty token env
// vars are enabled; Playwright is toggled separately via PLAYWRIGHT_ENABLED.
func applyConfigFilter(configPath string, mapping map[string]string, mcpEnabled, playwrightEnabled bool) error {
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

	for key, entryRaw := range mcp {
		entry, ok := entryRaw.(map[string]any)
		if !ok {
			continue
		}

		if key == "playwright" {
			entry["enabled"] = playwrightEnabled
		} else if envVar, exists := mapping[key]; exists {
			tokenSet := os.Getenv(envVar) != ""
			entry["enabled"] = mcpEnabled && tokenSet
		}

		mcp[key] = entry
	}

	out, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	tmpPath := configPath + ".tmp"
	if err := os.WriteFile(tmpPath, out, 0644); err != nil {
		return err
	}
	return os.Rename(tmpPath, configPath)
}
