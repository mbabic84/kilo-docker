package main

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/mbabic84/kilo-docker/pkg/constants"
	"github.com/mbabic84/kilo-docker/pkg/utils"
)

func syncMCPConfig() error {
	playwrightEnabled := os.Getenv("PLAYWRIGHT_ENABLED") == "1"

	context7Set := false
	ainstructSet := false

	// Load tokens from encrypted file (source of truth)
	homeDir, _, _, userID := loadUserConfig()
	if homeDir != "" && userID != "" {
		if encData, err := os.ReadFile(filepath.Join(homeDir, ".local/share/kilo/.tokens.env.enc")); err == nil {
			if decrypted, err := decryptAES(encData, userID); err == nil {
				c7, aInst, _, _, _, _ := parseTokenEnv(string(decrypted))
				context7Set = c7 != ""
				ainstructSet = aInst != ""
			}
		}
	}

	configPath := filepath.Join(constants.GetKiloConfigDir(), "opencode.json")
	if err := applyConfigFilter(configPath, playwrightEnabled, context7Set, ainstructSet); err != nil {
		utils.LogWarn("config error for %s: %v\n", configPath, err)
	}
	return nil
}

func applyConfigFilter(configPath string, playwrightEnabled, context7Set, ainstructSet bool) error {
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
		} else if key == "context7" {
			entry["enabled"] = context7Set
		} else if key == "ainstruct" {
			entry["enabled"] = ainstructSet
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
