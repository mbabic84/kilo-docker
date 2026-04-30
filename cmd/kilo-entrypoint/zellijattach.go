package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/mbabic84/kilo-docker/pkg/utils"
)

const initMarker = "/tmp/.kilo-initialized"

const userInitInProgressMarker = "/tmp/.kilo-user-init-in-progress"

const initReadyTimeoutEnvVar = "KD_INIT_READY_TIMEOUT"

const defaultInitReadyTimeout = 5 * time.Minute

// runZellijAttach is the entry point for the "zellij-attach" subcommand.
// If the container is already initialized, it execs zellij directly.
// Otherwise it runs the first-time user init flow (which handles auto-login if remember=true).
func runZellijAttach(remember bool) error {
	utils.Log("[zellijattach] Entry: remember=%v\n", remember)

	if err := waitForMarker(initReadyMarker, initReadyTimeout(),
		"[kilo-docker] Waiting for container initialization to finish...\n"); err != nil {
		return err
	}

	if _, err := os.Stat(initMarker); err == nil {
		utils.Log("[zellijattach] Init marker exists, skipping userinit\n")
		if !remember {
			clearStoredSyncTokens()
		}
		return execZellij()
	}

	claimed, err := claimUserInit()
	if err != nil {
		return err
	}
	if !claimed {
		if err := waitForCompletedUserInit(initReadyTimeout()); err != nil {
			return err
		}
		if !remember {
			clearStoredSyncTokens()
		}
		return execZellij()
	}
	defer func() { _ = os.Remove(userInitInProgressMarker) }()

	// Container not initialized - run full user init
	// runUserInit handles auto-login if remember=true and tokens are valid
	return runUserInit(remember)
}

func initReadyTimeout() time.Duration {
	value := strings.TrimSpace(os.Getenv(initReadyTimeoutEnvVar))
	if value == "" {
		return defaultInitReadyTimeout
	}

	timeout, err := time.ParseDuration(value)
	if err != nil {
		utils.LogWarn("[zellijattach] Invalid %s=%q, using default %s\n", initReadyTimeoutEnvVar, value, defaultInitReadyTimeout)
		return defaultInitReadyTimeout
	}
	if timeout <= 0 {
		utils.LogWarn("[zellijattach] Invalid %s=%q, using default %s\n", initReadyTimeoutEnvVar, value, defaultInitReadyTimeout)
		return defaultInitReadyTimeout
	}

	return timeout
}

func waitForMarker(marker string, timeout time.Duration, waitingMsg string) error {
	if _, err := os.Stat(marker); err == nil {
		return nil
	}

	utils.Log(waitingMsg, utils.WithOutput())
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(marker); err == nil {
			return nil
		}
		time.Sleep(250 * time.Millisecond)
	}

	return fmt.Errorf("initialization marker %q did not appear in %s", marker, timeout)
}

func claimUserInit() (bool, error) {
	f, err := os.OpenFile(userInitInProgressMarker, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err == nil {
		_ = f.Close()
		utils.Log("[zellijattach] Claimed user initialization\n")
		return true, nil
	}
	if os.IsExist(err) {
		return false, nil
	}
	return false, fmt.Errorf("failed to claim user initialization: %w", err)
}

func waitForCompletedUserInit(timeout time.Duration) error {
	if _, err := os.Stat(initMarker); err == nil {
		return nil
	}

	utils.Log("[kilo-docker] Waiting for user initialization to finish...\n", utils.WithOutput())
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(initMarker); err == nil {
			return nil
		}
		if _, err := os.Stat(userInitInProgressMarker); os.IsNotExist(err) {
			return fmt.Errorf("user initialization ended without completing")
		}
		time.Sleep(250 * time.Millisecond)
	}

	return fmt.Errorf("user initialization did not complete in %s", timeout)
}

// clearStoredSyncTokens clears sync tokens from encrypted storage if they exist.
// Called when --remember is not used to ensure no leftover tokens.
func clearStoredSyncTokens() {
	homeDir, _, _, userID := loadUserConfig()
	if homeDir == "" || userID == "" {
		return
	}
	_, _, storedSync, storedRefresh, _, loadErr := loadEncryptedTokens(homeDir, userID)
	if loadErr != nil {
		return
	}
	if storedSync == "" && storedRefresh == "" {
		utils.Log("[zellijattach] No stored sync tokens to clear\n")
		return
	}
	if err := clearSyncTokensFromEncrypted(homeDir, userID); err != nil {
		utils.LogWarn("[zellijattach] Failed to clear sync tokens: %v\n", err)
	} else {
		utils.Log("[zellijattach] Cleared stored sync tokens\n")
	}
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
						suppGroups := getUserGroups(username)
						if len(suppGroups) > 0 {
							utils.Log("[zellijattach] Restoring supplementary groups for %s: %v\n", username, suppGroups)
							if err := syscall.Setgroups(suppGroups); err != nil {
								utils.LogWarn("[zellijattach] Failed to restore supplementary groups: %v\n", err)
							}
						}
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

	if err := ensureAccessibleWorkingDir(); err != nil {
		return err
	}

	utils.Log("[zellijattach] Executing zellij with HOME=%s, USER=%s\n", homeDir, username)
	return syscall.Exec("/usr/local/bin/zellij", []string{"zellij", "attach", "--create", "kilo-docker"}, env)
}

func ensureAccessibleWorkingDir() error {
	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("workspace is not accessible after privilege drop: %w", err)
	}
	if _, err := os.Stat("."); err != nil {
		return fmt.Errorf("workspace %q is not accessible after privilege drop: %w", wd, err)
	}
	utils.Log("[zellijattach] Using working directory: %s\n", wd)
	return nil
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
