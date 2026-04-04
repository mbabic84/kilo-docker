package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
)

const initMarker = "/tmp/.kilo-initialized"

// runZellijAttach is the entry point for the "zellij-attach" subcommand.
// If the container is already initialized, it execs zellij directly.
// Otherwise it runs the first-time user init flow.
func runZellijAttach() error {
	if _, err := os.Stat(initMarker); err == nil {
		fmt.Fprintf(os.Stderr, "[kilo-docker] Container already initialized, attaching zellij\n")
		return execZellij()
	}
	fmt.Fprintf(os.Stderr, "[kilo-docker] First-time initialization\n")
	return runUserInit()
}

// loadUserConfig loads the persisted user configuration from the volume.
// Returns the home directory, username, shell, and userID, or empty strings if not found.
func loadUserConfig() (homeDir, username, shell, userID string) {
	// Try to find the user config in any kd-* directory
	baseDir := "/home"
	entries, err := os.ReadDir(baseDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[kilo-docker] Error reading /home: %v\n", err)
		return "", "", "", ""
	}
	fmt.Fprintf(os.Stderr, "[kilo-docker] Scanning /home for user config: %d entries\n", len(entries))
	for _, entry := range entries {
		if !entry.IsDir() || !filepath.HasPrefix(entry.Name(), "kd-") {
			continue
		}
		configPath := filepath.Join(baseDir, entry.Name(), ".local/share/kilo/.user-config.json")
		fmt.Fprintf(os.Stderr, "[kilo-docker] Checking for user config: %s\n", configPath)
		if _, err := os.Stat(configPath); err == nil {
			fmt.Fprintf(os.Stderr, "[kilo-docker] Found user config: %s\n", configPath)
			data, err := os.ReadFile(configPath)
			if err != nil {
				fmt.Fprintf(os.Stderr, "[kilo-docker] Error reading user config: %v\n", err)
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
					fmt.Fprintf(os.Stderr, "[kilo-docker] Loaded user config: homeDir=%s, username=%s, shell=%s\n", homeDir, username, shell)
					return homeDir, username, shell, userID
				}
			}
		}
	}
	fmt.Fprintf(os.Stderr, "[kilo-docker] No user config found\n")
	return "", "", "", ""
}

// execZellij replaces the current process with zellij attach.
// It sets the correct HOME, USER, LOGNAME, and SHELL environment variables
// from the persisted user config, and drops privileges to the user.
func execZellij() error {
	// Load user configuration to set environment variables and drop privileges
	homeDir, username, shell, userID := loadUserConfig()
	
	// If no user config found, we can't properly run as user
	if homeDir == "" || username == "" {
		fmt.Fprintf(os.Stderr, "[kilo-docker] Warning: No user config found, running as root\n")
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
						fmt.Fprintf(os.Stderr, "[kilo-docker] Dropping privileges to UID=%d, GID=%d\n", uid, gid)
						syscall.Setgid(gid)
						syscall.Setuid(uid)
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
	
	// Load MCP token env vars from encrypted storage so they are available
	// to zellij and child processes (Kilo). The tokens were set via
	// os.Setenv() during first-time init but are absent in subsequent
	// docker exec processes — we must re-read them from the volume.
	if homeDir != "" && userID != "" {
		if context7, ainstruct, syncToken, syncRefresh, syncExpiry, err := loadEncryptedTokens(homeDir, userID); err == nil {
			fmt.Fprintf(os.Stderr, "[kilo-docker] Loaded MCP tokens from encrypted storage\n")
			fmt.Fprintf(os.Stderr, "[kilo-docker] Token status: ainstruct=%s context7=%s sync=%s\n",
				maskToken(ainstruct), maskToken(context7), maskToken(syncToken))
			if ainstruct != "" {
				env = appendOrReplaceEnv(env, "KD_AINSTRUCT_TOKEN", ainstruct)
			}
			if context7 != "" {
				env = appendOrReplaceEnv(env, "KD_CONTEXT7_TOKEN", context7)
			}
			if syncToken != "" {
				env = appendOrReplaceEnv(env, "KD_AINSTRUCT_SYNC_TOKEN", syncToken)
			}
			if syncRefresh != "" {
				env = appendOrReplaceEnv(env, "KD_AINSTRUCT_SYNC_REFRESH_TOKEN", syncRefresh)
			}
			if syncExpiry != "" {
				env = appendOrReplaceEnv(env, "KD_AINSTRUCT_SYNC_TOKEN_EXPIRY", syncExpiry)
			}
		} else {
			fmt.Fprintf(os.Stderr, "[kilo-docker] Warning: failed to load MCP tokens: %v\n", err)
		}
	} else {
		fmt.Fprintf(os.Stderr, "[kilo-docker] Warning: cannot load tokens — homeDir=%q userID=%q\n", homeDir, userID)
	}
	
	fmt.Fprintf(os.Stderr, "[kilo-docker] Executing zellij with HOME=%s, USER=%s\n", homeDir, username)
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

// maskToken returns a masked preview of a token for safe logging.
func maskToken(token string) string {
	if token == "" {
		return "<empty>"
	}
	if len(token) <= 8 {
		return "***"
	}
	return token[:4] + "..." + token[len(token)-4:]
}
