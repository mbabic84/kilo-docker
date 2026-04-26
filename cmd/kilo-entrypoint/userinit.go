package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/mbabic84/kilo-docker/pkg/constants"
	"github.com/mbabic84/kilo-docker/pkg/utils"
)

// runUserInit performs the full first-time initialization inside a container.
// It runs as root (via docker exec without --user) and handles:
//   - Interactive Ainstruct login (get user_id)
//   - OS user creation derived from user_id
//   - Home directory and config structure creation
//   - SSH known_hosts setup
//   - Service config copying
//   - SSH agent socket ownership
//   - MCP token management
//   - Sync startup
//   - Privilege drop to the created user
//   - Exec zellij
func runUserInit(remember bool) error {
	utils.Log("[userinit] starting initialization (remember=%v)\n", remember)

	puidStr := os.Getenv("PUID")
	if puidStr == "" {
		puidStr = "1000"
	}
	pgidStr := os.Getenv("PGID")
	if pgidStr == "" {
		pgidStr = "1000"
	}
	puid, _ := strconv.Atoi(puidStr)
	pgid, _ := strconv.Atoi(pgidStr)

	utils.Log("[userinit] PUID=%s, PGID=%s\n", puidStr, pgidStr)

	existingUser := findExistingUser()
	if existingUser != "" {
		utils.Log("[userinit] Found existing user data: %s\n", existingUser)
	}

	var loginRes loginResult
	var userID string
	var username string
	var homeDir string

	if existingUser != "" {
		username = existingUser
		homeDir = "/home/" + username
		userIDPath := filepath.Join(homeDir, ".local/share/kilo/.user-config.json")
		if data, err := os.ReadFile(userIDPath); err == nil {
			var config map[string]string
			if json.Unmarshal(data, &config) == nil {
				if uid, ok := config["userID"]; ok {
					userID = uid
				}
			}
		}
	}

	// Try auto-login if remember is set and we have stored credentials
	if remember && existingUser != "" && userID != "" {
		utils.Log("[kilo-docker] Attempting auto-login with saved session...\n", utils.WithOutput())
		autoRes, autoLogin, autoErr := checkAutoLogin(homeDir, userID)
		if autoLogin && autoErr == nil {
			loginRes = autoRes
			utils.Log("[kilo-docker] Signed in automatically using saved session\n", utils.WithOutput())
		} else if autoErr != nil {
			utils.Log("[kilo-docker] Auto-login failed, please sign in again\n", utils.WithOutput())
		}
	}

	// Fall back to interactive login if auto-login didn't work
	if loginRes.UserID == "" {
		utils.Log("[kilo-docker] Starting Ainstruct authentication\n", utils.WithOutput())
		newLoginRes, err := runLoginInteractive(remember)
		if err != nil {
			return fmt.Errorf("login failed: %w", err)
		}
		loginRes = newLoginRes
		userID = loginRes.UserID
	}

	if username == "" {
		username = deriveHomeName(userID)
	}
	if homeDir == "" {
		homeDir = "/home/" + username
	}

	// Create OS user
	utils.Log("[userinit] Creating user: %s (UID=%s, GID=%s)\n", username, puidStr, pgidStr)
	if err := createOSUser(username, puidStr, pgidStr); err != nil {
		return fmt.Errorf("failed to create user %s: %w", username, err)
	}

	// Add user to service groups
	joinServiceGroups(username)

	// Create home directory and config structure
	utils.Log("[userinit] Creating home directory: %s\n", homeDir)
	_ = os.MkdirAll(homeDir, 0o700)
	_ = os.MkdirAll(filepath.Join(homeDir, ".config/kilo/commands"), 0o700)
	_ = os.MkdirAll(filepath.Join(homeDir, ".config/kilo/agents"), 0o700)
	_ = os.MkdirAll(filepath.Join(homeDir, ".config/kilo/plugins"), 0o700)
	_ = os.MkdirAll(filepath.Join(homeDir, ".config/kilo/skills"), 0o700)
	_ = os.MkdirAll(filepath.Join(homeDir, ".config/kilo/tools"), 0o700)
	_ = os.MkdirAll(filepath.Join(homeDir, ".config/kilo/rules"), 0o700)
	_ = os.MkdirAll(filepath.Join(homeDir, ".local/share/kilo"), 0o700)
	_ = os.MkdirAll(filepath.Join(homeDir, ".ssh"), 0700)

	// Create .bashrc if missing
	bashrc := filepath.Join(homeDir, ".bashrc")
	if _, err := os.Stat(bashrc); os.IsNotExist(err) {
		_ = os.WriteFile(bashrc, []byte("# ~/.bashrc\n"), 0o600)
	}

	// Copy default config templates if user doesn't have them.
	// Templates use 'template-' prefix to avoid being read as system configs.
	localKilo := filepath.Join(homeDir, ".config/kilo/kilo.jsonc")
	localOpencode := filepath.Join(homeDir, ".config/kilo/opencode.json")
	utils.Log("[userinit] config init: checking for kilo.jsonc at %s\n", localKilo)

	if _, err := os.Stat(localKilo); os.IsNotExist(err) {
		utils.Log("[userinit] config init: kilo.jsonc not found, checking for opencode.json migration\n")
		// Try to migrate from opencode.json
		if _, err := os.Stat(localOpencode); err == nil {
			utils.Log("[userinit] config init: migrating opencode.json to kilo.jsonc\n")
			data, err := os.ReadFile(localOpencode)
			if err == nil {
				if err := os.WriteFile(localKilo, data, 0o600); err == nil {
					utils.Log("[userinit] config init: migrated opencode.json → kilo.jsonc\n")
					// Delete old opencode.json after successful migration
					if err := os.Remove(localOpencode); err == nil {
						utils.Log("[userinit] config init: deleted old opencode.json\n")
						// Also delete from remote
						if delErr := deleteRemoteOpencode(homeDir, userID); delErr != nil {
							utils.LogWarn("[userinit] config init: remote delete failed: %v\n", delErr)
						}
					}
				} else {
					utils.LogWarn("[userinit] config init: migration failed: %v, falling back to template\n", err)
					copyFileIfMissing("/etc/kilo/template-kilo.jsonc", localKilo)
				}
			} else {
				utils.LogWarn("[userinit] config init: read opencode.json failed: %v, falling back to template\n", err)
				copyFileIfMissing("/etc/kilo/template-kilo.jsonc", localKilo)
			}
		} else {
			hashFile := filepath.Join(homeDir, ".config/kilo/.ainstruct-hashes")
			if _, err := os.Stat(hashFile); os.IsNotExist(err) {
				utils.Log("[userinit] config init: no hash file - first time user\n")
				hasRemote, checkErr := checkRemoteHasConfig(homeDir, userID)
				if checkErr != nil {
					utils.LogWarn("[userinit] config init: remote check failed (%v), falling back to template\n", checkErr)
					copyFileIfMissing("/etc/kilo/template-kilo.jsonc", localKilo)
				} else if hasRemote {
					utils.Log("[userinit] config init: remote has kilo.jsonc, will sync from remote\n")
				} else {
					utils.Log("[userinit] config init: no remote kilo.jsonc, copying template\n")
					copyFileIfMissing("/etc/kilo/template-kilo.jsonc", localKilo)
				}
			} else {
				utils.Log("[userinit] config init: existing user (hash file found), copying template\n")
				copyFileIfMissing("/etc/kilo/template-kilo.jsonc", localKilo)
			}
		}
	} else {
		utils.Log("[userinit] config init: kilo.jsonc exists, skipping\n")
	}

	copyFileIfMissing("/etc/zellij/template-config.kdl", filepath.Join(homeDir, ".config/zellij/config.kdl"))

	// Setup SSH known_hosts
	utils.Log("[userinit] Setting up SSH known_hosts\n")
	_ = setupKnownHosts(homeDir)

	// Copy service configs to the new home
	utils.Log("[userinit] Copying service configs\n")
	_ = copyServiceConfigs(homeDir)

	// Install user-scoped services (NVM, etc.) with HOME set to user home
	utils.Log("[userinit] Installing user-scoped services\n")
	_ = installUserServices(homeDir)

	// Setup Playwright output symlink if enabled
	if os.Getenv("PLAYWRIGHT_ENABLED") == "1" {
		playwrightMount := "/mnt/playwright-output"
		if _, err := os.Stat(playwrightMount); err == nil {
			playwrightLink := filepath.Join(homeDir, ".playwright-mcp")
			_ = os.Remove(playwrightLink)
			if err := os.Symlink(playwrightMount, playwrightLink); err != nil {
				utils.LogWarn("[userinit] failed to create playwright symlink: %v\n", err)
			} else {
				utils.Log("[userinit] Created playwright symlink: %s -> %s\n", playwrightLink, playwrightMount)
			}
		}
	}

	// Set HOME temporarily so GetKiloConfigDir() and token paths resolve correctly
	savedHome := os.Getenv("HOME")
	_ = os.Setenv("HOME", homeDir)

	// Save all tokens (ainstruct PAT from login + context7 + sync tokens)
	utils.Log("[userinit] Saving MCP tokens\n")
	if err := initTokens(homeDir, userID, loginRes); err != nil {
		utils.LogWarn("[userinit] token init failed: %v\n", err)
	}

	// Chown SSH agent socket to the new user
	if sshAuthSock := os.Getenv("SSH_AUTH_SOCK"); sshAuthSock != "" {
		utils.Log("[userinit] Setting SSH agent socket ownership: %s\n", sshAuthSock)
		_ = os.Chown(sshAuthSock, puid, pgid)
		_ = os.Chmod(sshAuthSock, 0600)
	}

	_ = os.Setenv("HOME", savedHome)

	// Mark initialized
	utils.Log("[userinit] Marking container as initialized: %s\n", initMarker)
	_ = os.WriteFile(initMarker, []byte("1\n"), 0o600)

	// Persist user configuration for re-attach
	// Store info in a file on the volume so it survives container restarts
	userConfigPath := filepath.Join(homeDir, ".local/share/kilo/.user-config.json")
	utils.Log("[userinit] Saving user config to: %s\n", userConfigPath)
	userConfig := map[string]string{
		"homeDir": homeDir,
		"username": username,
		"uid":      puidStr,
		"gid":      pgidStr,
		"shell":    "/bin/bash",
		"userID":   userID,
	}
	if _, err := os.Stat("/bin/bash"); err != nil {
		userConfig["shell"] = "/bin/sh"
	}
	configJSON, _ := json.Marshal(userConfig)
	if err := os.WriteFile(userConfigPath, configJSON, 0o600); err != nil {
		utils.LogWarn("[userinit] failed to save user config: %v\n", err)
	} else {
		redactedConfig := map[string]string{
			"homeDir":  homeDir,
			"username": username,
			"uid":      puidStr,
			"gid":      pgidStr,
			"shell":    userConfig["shell"],
			"userID":   utils.RedactID(userID),
		}
		redactedJSON, _ := json.Marshal(redactedConfig)
		utils.Log("[userinit] Saved user config: %s\n", string(redactedJSON))
	}

	// Set environment
	_ = os.Setenv("HOME", homeDir)
	_ = os.Setenv("USER", username)
	_ = os.Setenv("LOGNAME", username)
	_ = os.Setenv("SHELL", userConfig["shell"])

	// Chown everything to the new user (after all root-level file writes)
	utils.Log("[userinit] Setting ownership: %s\n", homeDir)
	chownRecursive(homeDir, puid, pgid)

	// Get supplementary groups for the user before dropping privileges
	utils.Log("[userinit] Getting supplementary groups for %s\n", username)
	suppGroups := getUserGroups(username)
	utils.Log("[userinit] Supplementary groups for %s: %v\n", username, suppGroups)

	// Drop privileges and exec zellij
	utils.Log("[userinit] Dropping privileges to %s (UID=%s, GID=%s)\n", username, puidStr, pgidStr)
	
	// Set supplementary groups BEFORE setting gid/uid
	if len(suppGroups) > 0 {
		utils.Log("[userinit] Setting supplementary groups: %v\n", suppGroups)
		if err := syscall.Setgroups(suppGroups); err != nil {
			utils.LogWarn("[userinit] Failed to set supplementary groups: %v\n", err)
		}
	}
	
	_ = syscall.Setgid(pgid)
	_ = syscall.Setuid(puid)

	// Start file sync as the user (after privilege drop so files are owned correctly)
	utils.Log("[userinit] Starting file sync\n")
	_ = startSyncWithTokens(homeDir, userID)

	// Delete stale zellij sessions from previous container lifecycle
	utils.Log("[userinit] Clearing old zellij sessions\n")
	clearZellijSessions()

	utils.Log("[userinit] Starting zellij\n")
	return execZellij()
}

// createOSUser creates an OS user with the given name and UID/GID.
func createOSUser(username, uidStr, gidStr string) error {
	_ = exec.Command("deluser", username).Run()
	cmd := exec.Command("addgroup", "-g", gidStr, username)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("addgroup: %v: %s", err, out)
	}
	cmd = exec.Command("adduser", "-u", uidStr, "-G", username, "-D", "-s", "/bin/sh", username)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("adduser: %v: %s", err, out)
	}
	return nil
}

// copyFileIfMissing copies src to dst if dst doesn't already exist.
// Creates parent directories as needed.
func copyFileIfMissing(src, dst string) {
	if _, err := os.Stat(dst); err == nil {
		return
	}
	data, err := os.ReadFile(src)
	if err != nil {
		return
	}
	_ = os.MkdirAll(filepath.Dir(dst), 0o700)
	_ = os.WriteFile(dst, data, 0o600)
	utils.Log("[userinit] Copied %s\n", filepath.Base(src))
}

// joinServiceGroups adds the user to service groups for socket access.
// If a group with the target GID already exists (different name), uses that group.
func joinServiceGroups(username string) {
	servicesEnv := os.Getenv("KD_SERVICES")
	if servicesEnv == "" {
		utils.Log("[userinit] joinServiceGroups: no services enabled\n")
		return
	}
	utils.Log("[userinit] joinServiceGroups: services=%s, user=%s\n", servicesEnv, username)
	for _, svcName := range strings.Split(servicesEnv, ",") {
		svc := getService(svcName)
		if svc == nil || svc.RequiresSocket == "" {
			utils.Log("[userinit] joinServiceGroups: skipping %s (no socket required)\n", svcName)
			continue
		}

		// First try to add to the service-named group
		utils.Log("[userinit] joinServiceGroups: trying addgroup %s %s\n", username, svc.Name)
		cmd1 := exec.Command("addgroup", username, svc.Name)
		if out, err := cmd1.CombinedOutput(); err == nil {
			utils.Log("[userinit] joinServiceGroups: added %s to %s\n", username, svc.Name)
			continue
		} else {
			utils.Log("[userinit] joinServiceGroups: failed to add %s to %s: %v, output: %s\n", username, svc.Name, err, strings.TrimSpace(string(out)))
		}

		// If that failed, check if a group with the target GID already exists
		gid := os.Getenv(svc.GIDEnvVar)
		if gid == "" {
			utils.Log("[userinit] joinServiceGroups: no GID env var for %s\n", svc.Name)
			continue
		}
		utils.Log("[userinit] joinServiceGroups: looking up GID %s for service %s\n", gid, svc.Name)

		cmd := exec.Command("getent", "group", gid)
		out, err := cmd.Output()
		if err != nil {
			utils.Log("[userinit] joinServiceGroups: getent group %s failed: %v\n", gid, err)
			continue
		}

		parts := strings.SplitN(string(out), ":", 2)
		utils.Log("[userinit] joinServiceGroups: getent returned: %s\n", strings.TrimSpace(string(out)))
		if len(parts) > 0 && parts[0] != "" && parts[0] != svc.Name {
			// Add user to the existing group with matching GID
			utils.Log("[userinit] joinServiceGroups: adding %s to existing group %s\n", username, parts[0])
			if err := exec.Command("addgroup", username, parts[0]).Run(); err != nil {
				utils.Log("[userinit] joinServiceGroups: failed to add %s to %s: %v\n", username, parts[0], err)
			} else {
				utils.Log("[userinit] joinServiceGroups: successfully added %s to %s\n", username, parts[0])
			}
		}
	}
}

// getUserGroups returns a list of supplementary group IDs for the given user.
// Uses getent to read the group database.
func getUserGroups(username string) []int {
	var groups []int
	
	// Read /etc/group to find all groups the user belongs to
	data, err := os.ReadFile("/etc/group")
	if err != nil {
		utils.LogWarn("[userinit] Failed to read /etc/group: %v\n", err)
		return groups
	}
	
	for _, line := range strings.Split(string(data), "\n") {
		// Format: groupname:password:GID:user1,user2,...
		parts := strings.Split(line, ":")
		if len(parts) < 4 {
			continue
		}
		
		gid, err := strconv.Atoi(parts[2])
		if err != nil {
			continue
		}
		
		// Check if user is in this group
		members := strings.Split(parts[3], ",")
		for _, member := range members {
			if strings.TrimSpace(member) == username {
				groups = append(groups, gid)
				break
			}
		}
	}
	
	return groups
}

// checkAutoLogin attempts to auto-login using stored sync tokens.
// Returns (loginResult, autoLoggedIn bool, error).
// If auto-login fails, caller should fall back to interactive login.
func checkAutoLogin(homeDir, userID string) (loginResult, bool, error) {
	utils.Log("[checkAutoLogin] userID=%s, homeDir=%s\n", userID, homeDir)

	tokenPath := filepath.Join(homeDir, ".local/share/kilo/.tokens.env.enc")
	info, statErr := os.Stat(tokenPath)
	if statErr != nil {
		utils.LogWarn("[checkAutoLogin] token file not found at %s: %v\n", tokenPath, statErr)
		return loginResult{}, false, fmt.Errorf("token file not found: %w", statErr)
	}
	utils.Log("[checkAutoLogin] token file found: size=%d, mode=%s\n", info.Size(), info.Mode())

	_, _, storedSync, storedRefresh, storedExpiry, loadErr := loadEncryptedTokens(homeDir, userID)
	if loadErr != nil {
		utils.LogWarn("[checkAutoLogin] loadEncryptedTokens failed (likely decryption error — wrong userID?): %v\n", loadErr)
		return loginResult{}, false, fmt.Errorf("no stored tokens: %w", loadErr)
	}

	maskVal := func(s string) string {
		if len(s) > 8 {
			return s[:4] + "..." + s[len(s)-4:]
		}
		return "***"
	}
	utils.Log("[checkAutoLogin] storedSync=%s, storedRefresh=%s, storedExpiry=%s\n",
		maskVal(storedSync), maskVal(storedRefresh), storedExpiry)

	if storedSync == "" || storedRefresh == "" {
		utils.LogWarn("[checkAutoLogin] missing sync or refresh token (syncEmpty=%v, refreshEmpty=%v)\n",
			storedSync == "", storedRefresh == "")
		return loginResult{}, false, fmt.Errorf("missing sync or refresh token")
	}

	if storedExpiry != "" {
		expiryUnix, parseErr := strconv.ParseInt(storedExpiry, 10, 64)
		if parseErr != nil {
			utils.LogWarn("[checkAutoLogin] failed to parse expiry %q: %v\n", storedExpiry, parseErr)
		} else if time.Now().Unix() > expiryUnix {
			utils.Log("[checkAutoLogin] access token expired (expiry=%d, now=%d), attempting refresh\n", expiryUnix, time.Now().Unix())
			newAccess, newRefresh, newExpiry, refreshErr := refreshSyncTokens(homeDir, userID, storedRefresh)
			if refreshErr != nil {
				utils.LogWarn("[checkAutoLogin] token refresh failed: %v\n", refreshErr)
				return loginResult{}, false, refreshErr
			}
			if err := saveSyncTokensToEncrypted(homeDir, userID, newAccess, newRefresh, strconv.FormatInt(newExpiry, 10)); err != nil {
				utils.LogWarn("[checkAutoLogin] failed to save refreshed tokens: %v\n", err)
			}
			// Update ALL stored values including the new refresh token
			// to ensure the returned loginResult has the correct tokens.
			storedSync = newAccess
			storedRefresh = newRefresh
			storedExpiry = strconv.FormatInt(newExpiry, 10)
		} else {
			utils.Log("[checkAutoLogin] access token still valid (expiry=%d, now=%d, remaining=%ds)\n",
				expiryUnix, time.Now().Unix(), expiryUnix-time.Now().Unix())
		}
	} else {
		utils.Log("[checkAutoLogin] no expiry set, skipping expiration check\n")
	}

	result := loginResult{
		AccessToken:  storedSync,
		RefreshToken: storedRefresh,
		UserID:       userID,
	}
	if storedExpiry != "" {
		expiryUnix, _ := strconv.ParseInt(storedExpiry, 10, 64)
		result.ExpiresIn = expiryUnix - time.Now().Unix()
	}

	utils.Log("[userinit] checkAutoLogin: auto-login successful\n")
	return result, true, nil
}

// refreshSyncTokens calls POST /auth/refresh and returns new tokens.
func refreshSyncTokens(homeDir, userID, refreshToken string) (access, refresh string, expiry int64, err error) {
	baseURL := os.Getenv("KD_AINSTRUCT_BASE_URL")
	if baseURL == "" {
		baseURL = constants.AinstructBaseURL
	}

	body, _ := json.Marshal(map[string]string{"refresh_token": refreshToken})
	resp, err := http.Post(baseURL+"/auth/refresh", "application/json", bytes.NewReader(body))
	if err != nil {
		return "", "", 0, fmt.Errorf("refresh request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != 200 {
		respBody, _ := io.ReadAll(resp.Body)
		return "", "", 0, fmt.Errorf("refresh returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int64  `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", "", 0, fmt.Errorf("failed to decode refresh response: %w", err)
	}

	return result.AccessToken, result.RefreshToken, time.Now().Unix() + result.ExpiresIn, nil
}

// initTokens merges the MCP token from login with any existing encrypted
// tokens on the volume and re-saves them. Also stores sync JWT tokens for
// the file sync subsystem.
func initTokens(homeDir, userID string, loginRes loginResult) error {
	var context7Token, ainstructToken string
	var syncToken, syncRefreshToken, syncTokenExpiry string

	if loginRes.MCPToken != "" {
		ainstructToken = loginRes.MCPToken
	}
	if loginRes.AccessToken != "" {
		syncToken = loginRes.AccessToken
	}
	if loginRes.RefreshToken != "" {
		syncRefreshToken = loginRes.RefreshToken
	}
	if loginRes.ExpiresIn > 0 {
		syncTokenExpiry = strconv.FormatInt(time.Now().Unix()+loginRes.ExpiresIn, 10)
	}

	storedContext7, storedAinstruct, storedSync, storedSyncRefresh, storedSyncExpiry, loadErr := loadEncryptedTokens(homeDir, userID)
	if loadErr == nil {
		// Prefer login's PAT over stored ainstruct token.
		// ensurePAT() may have rotated the old token, making it invalid.
		if ainstructToken == "" && storedAinstruct != "" {
			ainstructToken = storedAinstruct
		}
		if storedContext7 != "" {
			context7Token = storedContext7
		}
		if storedSync != "" {
			syncToken = storedSync
		}
		if storedSyncRefresh != "" {
			syncRefreshToken = storedSyncRefresh
		}
		if storedSyncExpiry != "" {
			syncTokenExpiry = storedSyncExpiry
		}
	}

	// Check env var for newly prompted context7 token (set in runUserInit)
	if context7Token == "" {
		if envC7 := os.Getenv("KD_MCP_CONTEXT7_TOKEN"); envC7 != "" {
			context7Token = envC7
		}
	}

	if context7Token == "" && ainstructToken == "" && syncToken == "" {
		return nil
	}

	return saveEncryptedTokens(homeDir, userID, context7Token, ainstructToken, syncToken, syncRefreshToken, syncTokenExpiry)
}

// clearZellijSessions removes all zellij sessions to prevent stale
// session resurrection prompts after container recreation.
func clearZellijSessions() {
	cmd := exec.Command("zellij", "delete-all-sessions", "--yes", "--force")
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		utils.LogWarn("[userinit] failed to delete zellij sessions: %v\n", err)
	}
}

// deriveHomeName returns a deterministic username and directory name from a user ID.
// Uses 3 bytes (6 hex chars) of the SHA256 hash.
func deriveHomeName(userID string) string {
	hash := sha256.Sum256([]byte(userID))
	return fmt.Sprintf("kd-%x", hash[:3])
}

// findExistingUser scans /home/ for a kd-* directory containing encrypted
// tokens, returning the directory name if found.
func findExistingUser() string {
	baseDir := "/home"
	entries, err := os.ReadDir(baseDir)
	if err != nil {
		utils.LogWarn("[findExistingUser] error reading %s: %v\n", baseDir, err)
		return ""
	}
	utils.Log("[findExistingUser] scanning %s (%d entries)\n", baseDir, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() || !strings.HasPrefix(entry.Name(), "kd-") {
			continue
		}
		tokenPath := filepath.Join(baseDir, entry.Name(), ".local/share/kilo/.tokens.env.enc")
		if _, err := os.Stat(tokenPath); err == nil {
			utils.Log("[findExistingUser] found user %s with token file\n", entry.Name())
			return entry.Name()
		} else {
			utils.Log("[findExistingUser] dir %s exists but no token file: %v\n", entry.Name(), err)
		}
	}
	utils.Log("[findExistingUser] no user with token file found in %s\n", baseDir)
	return ""
}

// startSyncWithTokens starts the sync process. Tokens are loaded by
// sync_content.go directly from encrypted storage.
func startSyncWithTokens(homeDir, userID string) error {
	go func() {
		cmd := exec.Command("kilo-entrypoint", "sync")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		_ = cmd.Start()
	}()

	return nil
}

func runMCPTokens() error {
	homeDir, _, _, userID := loadUserConfig()
	if homeDir == "" || userID == "" {
		return fmt.Errorf("no user config found")
	}

	ainstructToken := os.Getenv("KD_MCP_AINSTRUCT_TOKEN")
	var context7Token string
	var syncToken, syncRefresh, syncExpiry string

	if encData, err := os.ReadFile(filepath.Join(homeDir, ".local/share/kilo/.tokens.env.enc")); err == nil {
		if decrypted, err := decryptAES(encData, userID); err == nil {
			c7, aInst, sTok, sRef, sExp, _ := parseTokenEnv(string(decrypted))
			context7Token = c7
			if ainstructToken == "" {
				ainstructToken = aInst
			}
			syncToken = sTok
			syncRefresh = sRef
			syncExpiry = sExp
		}
	}

	utils.Log("[kilo-docker] MCP Token Management\n", utils.WithOutput())
	utils.Log("[kilo-docker] ====================\n", utils.WithOutput())
	utils.Log("[kilo-docker] \n", utils.WithOutput())

	currentContext7 := "[not set]"
	if context7Token != "" {
		currentContext7 = maskToken(context7Token)
	}
	utils.Log("[kilo-docker] Context7 token: %s\n", currentContext7, utils.WithOutput())
	utils.Log("[kilo-docker]   - Press Enter to keep current\n", utils.WithOutput())
	utils.Log("[kilo-docker]   - Type a new token to update\n", utils.WithOutput())
	utils.Log("[kilo-docker]   - Type 'clear' to disable\n", utils.WithOutput())
	utils.Log("[kilo-docker] > ", utils.WithOutput())

	var input string
	_, _ = fmt.Scanln(&input)
	input = strings.TrimSpace(input)

	if input == "clear" {
		context7Token = ""
		utils.Log("[kilo-docker] Context7 token disabled\n", utils.WithOutput())
	} else if input != "" {
		context7Token = input
		utils.Log("[kilo-docker] Context7 token updated\n", utils.WithOutput())
	}

	if err := saveEncryptedTokens(homeDir, userID, context7Token, ainstructToken, syncToken, syncRefresh, syncExpiry); err != nil {
		utils.LogWarn("[userinit] failed to save tokens: %v\n", err)
	}

	utils.Log("[kilo-docker] MCP tokens saved\n", utils.WithOutput())
	return nil
}

func checkRemoteHasConfig(homeDir, userID string) (bool, error) {
	utils.Log("[userinit] checkRemote: Checking remote for kilo.jsonc (first time init)\n")

	encPath := filepath.Join(homeDir, ".local/share/kilo/.tokens.env.enc")
	encData, err := os.ReadFile(encPath)
	if err != nil {
		utils.LogWarn("[userinit] checkRemote: no encrypted tokens found: %v\n", err)
		return false, fmt.Errorf("no encrypted tokens found: %w", err)
	}

	decrypted, err := decryptAES(encData, userID)
	if err != nil {
		utils.LogWarn("[userinit] checkRemote: failed to decrypt tokens: %v\n", err)
		return false, fmt.Errorf("failed to decrypt tokens: %w", err)
	}

	_, _, syncToken, _, _, _ := parseTokenEnv(string(decrypted))
	if syncToken == "" {
		utils.LogWarn("[userinit] checkRemote: no sync token available\n")
		return false, fmt.Errorf("no sync token available")
	}

	baseURL := os.Getenv("KD_AINSTRUCT_BASE_URL")
	if baseURL == "" {
		baseURL = constants.AinstructBaseURL
	}

	collectionsURL := baseURL + "/api/v1/collections"
	req, err := http.NewRequest("GET", collectionsURL, nil)
	if err != nil {
		return false, err
	}
	req.Header.Set("Authorization", "Bearer "+syncToken)

	client := &http.Client{Timeout: 10 * time.Second}
	utils.Log("[userinit] checkRemote: GET /collections\n")
	resp, err := client.Do(req)
	if err != nil {
		utils.LogWarn("[userinit] checkRemote: collections request failed: %v\n", err)
		return false, err
	}

	if resp.StatusCode != 200 {
		_ = resp.Body.Close()
		utils.LogWarn("[userinit] checkRemote: collections API returned %d\n", resp.StatusCode)
		return false, fmt.Errorf("collections API returned %d", resp.StatusCode)
	}

	var result struct {
		Collections []struct {
			CollectionID string `json:"collection_id"`
			Name         string `json:"name"`
		} `json:"collections"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		_ = resp.Body.Close()
		utils.LogWarn("[userinit] checkRemote: failed to decode collections response: %v\n", err)
		return false, err
	}
	_ = resp.Body.Close()

	var collectionID string
	for _, c := range result.Collections {
		if c.Name == "kilo-docker" {
			collectionID = c.CollectionID
			utils.Log("[userinit] checkRemote: found collection %s\n", utils.RedactID(collectionID))
			break
		}
	}

	if collectionID == "" {
		utils.Log("[userinit] checkRemote: no kilo-docker collection found\n")
		return false, nil
	}

	docsURL := baseURL + "/api/v1/documents?collection_id=" + collectionID
	req, err = http.NewRequest("GET", docsURL, nil)
	if err != nil {
		return false, err
	}
	req.Header.Set("Authorization", "Bearer "+syncToken)

	utils.Log("[userinit] checkRemote: GET /documents for collection %s\n", utils.RedactID(collectionID))
	resp, err = client.Do(req)
	if err != nil {
		utils.LogWarn("[userinit] checkRemote: documents request failed: %v\n", err)
		return false, err
	}

	if resp.StatusCode != 200 {
		_ = resp.Body.Close()
		utils.LogWarn("[userinit] checkRemote: documents API returned %d\n", resp.StatusCode)
		return false, fmt.Errorf("documents API returned %d", resp.StatusCode)
	}

	var docsResult struct {
		Documents []struct {
			Metadata struct {
				LocalPath string `json:"local_path"`
			} `json:"metadata"`
		} `json:"documents"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&docsResult); err != nil {
		_ = resp.Body.Close()
		utils.LogWarn("[userinit] checkRemote: failed to decode documents response: %v\n", err)
		return false, err
	}
	_ = resp.Body.Close()

	for _, d := range docsResult.Documents {
		if d.Metadata.LocalPath == "kilo.jsonc" || d.Metadata.LocalPath == "kilo.json" {
			utils.Log("[userinit] checkRemote: found %s in remote collection\n", d.Metadata.LocalPath)
			return true, nil
		}
	}

	utils.Log("[userinit] checkRemote: no kilo.jsonc/kilo.json in remote collection\n")
	return false, nil
}

// deleteRemoteOpencode deletes opencode.json from the remote ainstruct collection.
// Called after successful migration to kilo.jsonc to clean up old config.
func deleteRemoteOpencode(homeDir, userID string) error {
	utils.Log("[userinit] deleteRemote: Checking for old opencode.json in remote\n")

	encPath := filepath.Join(homeDir, ".local/share/kilo/.tokens.env.enc")
	encData, err := os.ReadFile(encPath)
	if err != nil {
		utils.LogWarn("[userinit] deleteRemote: no encrypted tokens: %v\n", err)
		return err
	}

	decrypted, err := decryptAES(encData, userID)
	if err != nil {
		utils.LogWarn("[userinit] deleteRemote: decrypt failed: %v\n", err)
		return err
	}

	_, _, syncToken, _, _, _ := parseTokenEnv(string(decrypted))
	if syncToken == "" {
		return fmt.Errorf("no sync token")
	}

	baseURL := os.Getenv("KD_AINSTRUCT_BASE_URL")
	if baseURL == "" {
		baseURL = constants.AinstructBaseURL
	}

	collectionsURL := baseURL + "/api/v1/collections"
	req, err := http.NewRequest("GET", collectionsURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+syncToken)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != 200 {
		return fmt.Errorf("collections API returned %d", resp.StatusCode)
	}

	var result struct {
		Collections []struct {
			CollectionID string `json:"collection_id"`
			Name         string `json:"name"`
		} `json:"collections"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}

	var collectionID string
	for _, c := range result.Collections {
		if c.Name == "kilo-docker" {
			collectionID = c.CollectionID
			break
		}
	}

	if collectionID == "" {
		return nil
	}

	docsURL := baseURL + "/api/v1/documents?collection_id=" + collectionID
	req, err = http.NewRequest("GET", docsURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+syncToken)

	resp, err = client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != 200 {
		return fmt.Errorf("documents API returned %d", resp.StatusCode)
	}

	var docsResult struct {
		Documents []struct {
			DocumentID string `json:"document_id"`
			Metadata  struct {
				LocalPath string `json:"local_path"`
			} `json:"metadata"`
		} `json:"documents"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&docsResult); err != nil {
		return err
	}

	for _, d := range docsResult.Documents {
		if d.Metadata.LocalPath == "opencode.json" {
			utils.Log("[userinit] deleteRemote: deleting opencode.json from remote\n")
			deleteURL := baseURL + "/api/v1/documents/" + d.DocumentID
			req, err = http.NewRequest("DELETE", deleteURL, nil)
			if err != nil {
				return err
			}
			req.Header.Set("Authorization", "Bearer "+syncToken)

			resp, err = client.Do(req)
			if err != nil {
				utils.LogWarn("[userinit] deleteRemote: delete failed: %v\n", err)
				return err
			}
			_ = resp.Body.Close()
			utils.Log("[userinit] deleteRemote: deleted opencode.json from remote\n")
			return nil
		}
	}

	utils.Log("[userinit] deleteRemote: no opencode.json found in remote\n")
	return nil
}

func maskToken(token string) string {
	if len(token) <= 8 {
		return strings.Repeat("*", len(token))
	}
	return token[:4] + strings.Repeat("*", len(token)-8) + token[len(token)-4:]
}
