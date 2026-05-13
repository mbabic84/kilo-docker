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
// Otherwise it runs the first-time user init flow.
func runZellijAttach() error {
	utils.Log("[zellijattach] Entry\n")

	if err := waitForMarker(initReadyMarker, initReadyTimeout(),
		"[kilo-docker] Waiting for container initialization to finish...\n"); err != nil {
		return err
	}

	if _, err := os.Stat(initMarker); err == nil {
		utils.Log("[zellijattach] Init marker exists, checking PAT expiry\n")
		if needsAinstructPATRefresh() {
			utils.Log("[kilo-docker] Ainstruct PAT expiring soon, re-authentication required\n", utils.WithOutput())
			return runUserInit()
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
		return execZellij()
	}
	defer func() { _ = os.Remove(userInitInProgressMarker) }()

	// Container not initialized - run full user init
	return runUserInit()
}

// needsAinstructPATRefresh checks whether the stored Ainstruct PAT is
// expiring within the rotation threshold or is missing entirely.
func needsAinstructPATRefresh() bool {
	homeDir, _, _, userID := loadUserConfig()
	if homeDir == "" || userID == "" {
		utils.Log("[zellijattach] No user config, cannot check PAT expiry\n")
		return false
	}

	_, _, _, _, _, patExpiryStr, err := loadEncryptedTokens(homeDir, userID)
	if err != nil {
		utils.Log("[zellijattach] Cannot load encrypted tokens: %v\n", err)
		return false
	}
	if patExpiryStr == "" {
		utils.Log("[zellijattach] No PAT expiry stored, refreshing proactively\n")
		return true
	}

	expUnix, parseErr := strconv.ParseInt(patExpiryStr, 10, 64)
	if parseErr != nil {
		utils.Log("[zellijattach] Invalid PAT expiry: %v\n", parseErr)
		return false
	}

	remaining := expUnix - time.Now().Unix()
	utils.Log("[zellijattach] PAT remaining: %ds (threshold: %ds)\n", remaining, int64(ainstructPATRotationThreshold.Seconds()))

	if remaining <= 0 {
		utils.Log("[zellijattach] PAT already expired\n")
		return true
	}
	if remaining <= int64(ainstructPATRotationThreshold.Seconds()) {
		utils.Log("[zellijattach] PAT expiring within rotation threshold\n")
		return true
	}

	return false
}

// initReadyTimeout returns the timeout duration for waiting on init markers.
// Uses KD_INIT_READY_TIMEOUT env var if set, otherwise defaults to 5 minutes.
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

// waitForMarker polls for the existence of a marker file, printing a waiting
// message every 10 seconds, and returns an error after the timeout expires.
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

// claimUserInit atomically creates a user-init-in-progress marker file using
// O_EXCL. Returns true if this process won the race, false otherwise.
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

// waitForCompletedUserInit polls for the /tmp/.kilo-initialized marker file
// which signals that the winning zellij-attach process has finished user init.
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

	// Determine session name from current working directory before any chdir
	sessionName := "kilo-docker"
	if wd, err := os.Getwd(); err == nil {
		if base := filepath.Base(wd); base != "" && base != "/" {
			sessionName = base
		}
	}

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

	// Keep the current working directory (the workspace set by -w from docker run).
	// It is bind-mounted and should be accessible after privilege drop since the
	// container UID matches the host UID.

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

	return syscall.Exec("/usr/local/bin/zellij", []string{"zellij", "attach", "--create", sessionName}, env)
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
