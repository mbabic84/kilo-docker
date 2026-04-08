package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/mbabic84/kilo-docker/pkg/utils"
)

const initMarker = "/tmp/.kilo-initialized"

// runZellijAttach is the entry point for the "zellij-attach" subcommand.
// If the container is already initialized, it execs zellij directly.
// Otherwise it runs the first-time user init flow.
func runZellijAttach() error {
	utils.Log("[zellijattach] Checking init marker at %s\n", initMarker)
	if _, err := os.Stat(initMarker); err == nil {
		utils.Log("[zellijattach] Container already initialized\n")
		return execZellij()
	}
	utils.Log("[zellijattach] Running first-time initialization\n")
	return runUserInit()
}

// loadUserConfig loads the persisted user configuration from the volume.
// Returns the home directory, username, shell, and userID, or empty strings if not found.
func loadUserConfig() (homeDir, username, shell, userID string) {
	// Try to find the user config in any kd-* directory
	baseDir := "/home"
	entries, err := os.ReadDir(baseDir)
	if err != nil {
		utils.LogError("[zellijattach] Error reading /home: %v\n", err)
		return "", "", "", ""
	}
	utils.Log("[zellijattach] Scanning /home for user config: %d entries\n", len(entries))
	for _, entry := range entries {
		if !entry.IsDir() || !strings.HasPrefix(entry.Name(), "kd-") {
			continue
		}
		configPath := filepath.Join(baseDir, entry.Name(), ".local/share/kilo/.user-config.json")
		utils.Log("[zellijattach] Checking for user config: %s\n", configPath)
		if _, err := os.Stat(configPath); err == nil {
			utils.Log("[zellijattach] Found user config: %s\n", configPath)
			data, err := os.ReadFile(configPath)
			if err != nil {
				utils.LogError("[zellijattach] Error reading user config: %v\n", err)
				continue
			}
			var config map[string]string
			if err := json.Unmarshal(data, &config); err == nil {
				if hd, ok := config["homeDir"]; ok {
					homeDir = hd
				}
				if u, ok := config["username"]; ok {
					username = u
				}
				if s, ok := config["shell"]; ok {
					shell = s
				}
				if uid, ok := config["userID"]; ok {
					userID = uid
				}
				if homeDir != "" && username != "" {
					utils.Log("[zellijattach] Loaded user config: homeDir=%s, username=%s, shell=%s\n", homeDir, username, shell)
					return homeDir, username, shell, userID
				}
			}
		}
	}
	utils.Log("[zellijattach] No user config found\n")
	return "", "", "", ""
}

// execZellij replaces the current process with zellij attach.
// It sets the correct HOME, USER, LOGNAME, and SHELL environment variables
// from the persisted user config, and drops privileges to the user.
func execZellij() error {
	// Load user configuration to set environment variables and drop privileges
	homeDir, username, shell, _ := loadUserConfig()
	
	// If no user config found, we can't properly run as user
	if homeDir == "" || username == "" {
		utils.LogWarn("[zellijattach] No user config found, running as root\n")
	} else {
		// Load UID/GID from config to drop privileges
		configPath := filepath.Join(homeDir, ".local/share/kilo/.user-config.json")
		data, err := os.ReadFile(configPath)
		if err == nil {
			var config map[string]string
			if err := json.Unmarshal(data, &config); err == nil {
				if uidStr, ok := config["uid"]; ok {
					if gidStr, ok := config["gid"]; ok {
						uid, _ := strconv.Atoi(uidStr)
						gid, _ := strconv.Atoi(gidStr)
						utils.Log("[zellijattach] Dropping privileges to UID=%d, GID=%d\n", uid, gid)
						_ = syscall.Setgid(gid)
						_ = syscall.Setuid(uid)
					}
				}
			}
		}
	}
	
	// Create a copy of the environment
	env := os.Environ()
	
	// Set HOME, USER, LOGNAME, SHELL from persisted config
	if homeDir != "" {
		env = appendOrReplaceEnv(env, "HOME", homeDir)
	}
	if username != "" {
		env = appendOrReplaceEnv(env, "USER", username)
		env = appendOrReplaceEnv(env, "LOGNAME", username)
	}
	if shell != "" {
		env = appendOrReplaceEnv(env, "SHELL", shell)
	}
	
	utils.Log("[zellijattach] Executing zellij with HOME=%s, USER=%s\n", homeDir, username)
	return syscall.Exec("/usr/local/bin/zellij", []string{"zellij", "attach", "--create", "kilo-docker"}, env)
}

// appendOrReplaceEnv appends or replaces an environment variable in the env slice.
func appendOrReplaceEnv(env []string, key, value string) []string {
	prefix := key + "="
	for i, e := range env {
		if len(e) > len(prefix) && e[:len(prefix)] == prefix {
			env[i] = prefix + value
			return env
		}
	}
	return append(env, prefix+value)
}
