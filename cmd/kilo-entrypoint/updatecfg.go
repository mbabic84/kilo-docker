package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/mbabic84/kilo-docker/pkg/constants"
)

// runUpdateConfig downloads the latest opencode.json template from the
// repository and deep-merges it with the existing config. Template defaults
// are preserved for new keys; existing customizations override template values.
// This replicates the behavior of `jq -s ".[0] * .[1]"` from the original bash.
func runUpdateConfig() error {
	configPath := filepath.Join(constants.GetKiloConfigDir(), "opencode.json")

	templateURL := "https://raw.githubusercontent.com/mbabic84/kilo-docker/main/configs/opencode.json"
	resp, err := http.Get(templateURL)
	if err != nil {
		return fmt.Errorf("failed to download config template: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("failed to download config template: HTTP %d", resp.StatusCode)
	}

	templateData, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read template: %w", err)
	}

	var template map[string]any
	if err := json.Unmarshal(templateData, &template); err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}

	if _, err := os.Stat(configPath); err == nil {
		existingData, err := os.ReadFile(configPath)
		if err != nil {
			return fmt.Errorf("failed to read existing config: %w", err)
		}
		var existing map[string]any
		if err := json.Unmarshal(existingData, &existing); err != nil {
			return fmt.Errorf("failed to parse existing config: %w", err)
		}
		merged := mergeJSON(template, existing)
		out, _ := json.MarshalIndent(merged, "", "  ")
		tmpPath := configPath + ".tmp"
		if err := os.WriteFile(tmpPath, out, 0644); err != nil {
			return err
		}
		if err := os.Rename(tmpPath, configPath); err != nil {
			return err
		}
		fmt.Println("Config merged. Existing customizations preserved, new servers added.")
	} else {
		if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
			return err
		}
		if err := os.WriteFile(configPath, templateData, 0644); err != nil {
			return err
		}
		fmt.Println("Config created from template.")
	}
	return nil
}

// mergeJSON recursively merges src into dst. Nested maps are merged recursively;
// scalar values from src overwrite dst. Keys present only in dst are preserved.
// This replicates jq's recursive object merge operator `*`.
// mergeJSON recursively merges src into dst. Nested maps are merged recursively;
// scalar values from src overwrite dst. Keys present only in dst are preserved.
// This replicates jq's recursive object merge operator `*`.
func mergeJSON(dst, src map[string]any) map[string]any {
	result := make(map[string]any)
	for k, v := range dst {
		result[k] = v
	}
	for k, srcVal := range src {
		if dstVal, ok := result[k]; ok {
			if dstMap, isDstMap := dstVal.(map[string]any); isDstMap {
				if srcMap, isSrcMap := srcVal.(map[string]any); isSrcMap {
					result[k] = mergeJSON(dstMap, srcMap)
					continue
				}
			}
		}
		result[k] = srcVal
	}
	return result
}
