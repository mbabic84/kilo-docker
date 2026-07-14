package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mbabic84/kilo-docker/pkg/utils"
)

func loadCustomEnvs(home, userID string) (map[string]string, error) {
	customPath := filepath.Join(home, ".local/share/kilo-docker/.custom-envs.env.enc")
	encData, err := os.ReadFile(customPath)
	if err != nil {
		return nil, err
	}
	decrypted, err := decryptAES(encData, userID)
	if err != nil {
		return nil, err
	}
	return parseEnvMap(string(decrypted)), nil
}

func saveCustomEnvs(home, userID string, envs map[string]string) error {
	data := serializeEnvMap(envs)
	encData, err := encryptAES([]byte(data), userID)
	if err != nil {
		return err
	}
	customPath := filepath.Join(home, ".local/share/kilo-docker/.custom-envs.env.enc")
	_ = os.MkdirAll(filepath.Dir(customPath), 0o700)
	return os.WriteFile(customPath, encData, 0600)
}

func parseEnvMap(data string) map[string]string {
	envs := make(map[string]string)
	for _, line := range strings.Split(data, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 || parts[0] == "" {
			continue
		}
		envs[parts[0]] = parts[1]
	}
	return envs
}

func serializeEnvMap(envs map[string]string) string {
	keys := make([]string, 0, len(envs))
	for k := range envs {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var sb strings.Builder
	for _, k := range keys {
		sb.WriteString(k)
		sb.WriteByte('=')
		sb.WriteString(envs[k])
		sb.WriteByte('\n')
	}
	return sb.String()
}

func runCustomEnvsList(homeDir, userID string) {
	envs, err := loadCustomEnvs(homeDir, userID)
	if err != nil {
		utils.LogError("[kilo-docker] Failed to load custom envs: %v\n", err, utils.WithOutput())
		return
	}

	if len(envs) == 0 {
		utils.Log("[kilo-docker] No custom envs stored\n", utils.WithOutput())
		return
	}

	utils.Log("[kilo-docker] Custom envs:\n", utils.WithOutput())

	maxKeyLen := 0
	for k := range envs {
		if len(k) > maxKeyLen {
			maxKeyLen = len(k)
		}
	}

	keys := make([]string, 0, len(envs))
	for k := range envs {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		masked := maskToken(envs[k])
		utils.Log("  %-*s  %s\n", maxKeyLen, k, masked, utils.WithOutput())
	}
}

func runCustomEnvsGet(homeDir, userID, key string) {
	envs, err := loadCustomEnvs(homeDir, userID)
	if err != nil {
		utils.LogError("[kilo-docker] Failed to load custom envs: %v\n", err, utils.WithOutput())
		return
	}
	if val, ok := envs[key]; ok {
		fmt.Println(val)
	}
}

func runCustomEnvsAdd(homeDir, userID, key, value string) {
	if key == "" {
		utils.LogError("[kilo-docker] Key is required\n", utils.WithOutput())
		os.Exit(1)
	}

	envs, err := loadCustomEnvs(homeDir, userID)
	if err != nil {
		if !os.IsNotExist(err) {
			utils.LogError("[kilo-docker] Failed to load custom envs: %v\n", err, utils.WithOutput())
			os.Exit(1)
		}
		envs = make(map[string]string)
	}

	if _, exists := envs[key]; exists {
		utils.LogError("[kilo-docker] Key %q already exists — use edit to change it\n", key, utils.WithOutput())
		os.Exit(1)
	}

	envs[key] = value
	if err := saveCustomEnvs(homeDir, userID, envs); err != nil {
		utils.LogError("[kilo-docker] Failed to save custom envs: %v\n", err, utils.WithOutput())
		os.Exit(1)
	}
	utils.Log("[kilo-docker] Added %s\n", key, utils.WithOutput())
}

func runCustomEnvsEdit(homeDir, userID, key, value string) {
	if key == "" {
		utils.LogError("[kilo-docker] Key is required\n", utils.WithOutput())
		os.Exit(1)
	}

	envs, err := loadCustomEnvs(homeDir, userID)
	if err != nil {
		if os.IsNotExist(err) {
			utils.LogError("[kilo-docker] Key %q does not exist — use add to create it\n", key, utils.WithOutput())
			os.Exit(1)
		}
		utils.LogError("[kilo-docker] Failed to load custom envs: %v\n", err, utils.WithOutput())
		os.Exit(1)
	}

	if _, exists := envs[key]; !exists {
		utils.LogError("[kilo-docker] Key %q does not exist — use add to create it\n", key, utils.WithOutput())
		os.Exit(1)
	}

	envs[key] = value
	if err := saveCustomEnvs(homeDir, userID, envs); err != nil {
		utils.LogError("[kilo-docker] Failed to save custom envs: %v\n", err, utils.WithOutput())
		os.Exit(1)
	}
	utils.Log("[kilo-docker] Updated %s\n", key, utils.WithOutput())
}

func runCustomEnvsRemove(homeDir, userID, key string) {
	if key == "" {
		utils.LogError("[kilo-docker] Key is required\n", utils.WithOutput())
		os.Exit(1)
	}

	envs, err := loadCustomEnvs(homeDir, userID)
	if err != nil {
		if os.IsNotExist(err) {
			utils.LogError("[kilo-docker] Key %q does not exist\n", key, utils.WithOutput())
			os.Exit(1)
		}
		utils.LogError("[kilo-docker] Failed to load custom envs: %v\n", err, utils.WithOutput())
		os.Exit(1)
	}

	if _, exists := envs[key]; !exists {
		utils.LogError("[kilo-docker] Key %q does not exist\n", key, utils.WithOutput())
		os.Exit(1)
	}

	delete(envs, key)
	if err := saveCustomEnvs(homeDir, userID, envs); err != nil {
		utils.LogError("[kilo-docker] Failed to save custom envs: %v\n", err, utils.WithOutput())
		os.Exit(1)
	}
	utils.Log("[kilo-docker] Removed %s\n", key, utils.WithOutput())
}

// showCustomEnvsCompletions prints tab-completion candidates for custom env
// keys to stdout, one per line.
func showCustomEnvsCompletions() {
	homeDir, _, _, userID := loadUserConfig()
	if homeDir == "" || userID == "" {
		return
	}
	envs, err := loadCustomEnvs(homeDir, userID)
	if err != nil {
		return
	}
	keys := make([]string, 0, len(envs))
	for k := range envs {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Println(k)
	}
}
