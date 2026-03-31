package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// runConfig toggles MCP server enabled/disabled state in opencode.json
// based on environment variables. It patches both the user config and
// any workspace config so Kilo reads the correct state regardless of CWD.
func runConfig() error {
	mapping := map[string]string{
		"ainstruct": "KD_AINSTRUCT_TOKEN",
		"context7":  "KD_CONTEXT7_TOKEN",
	}

	playwrightEnabled := os.Getenv("PLAYWRIGHT_ENABLED") == "1"

	configPaths := []string{
		filepath.Join(os.Getenv("HOME"), ".config", "kilo", "opencode.json"),
		filepath.Join(os.Getenv("PWD"), "configs", "opencode.json"),
	}

	for _, configPath := range configPaths {
		if err := applyConfigFilter(configPath, mapping, playwrightEnabled); err != nil {
			fmt.Fprintf(os.Stderr, "[kilo-docker] Warning: config error for %s: %v\n", configPath, err)
		}
	}
	return nil
}

// applyConfigFilter reads a JSON config file and toggles MCP server entries
// based on environment variables. The mapping connects server names to their
// token env vars; Playwright is toggled separately via PLAYWRIGHT_ENABLED.
func applyConfigFilter(configPath string, mapping map[string]string, playwrightEnabled bool) error {
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
			entry["enabled"] = os.Getenv(envVar) != ""
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
