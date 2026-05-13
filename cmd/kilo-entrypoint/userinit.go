package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
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
func runUserInit() error {
	utils.Log("[userinit] starting initialization\n")

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

	// Interactive login (required every time)
	if loginRes.UserID == "" {
		utils.Log("[kilo-docker] Starting Ainstruct authentication\n", utils.WithOutput())
		newLoginRes, err := runLoginInteractive()
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

	// Add user to host supplementary groups so workspace files with
	// group-level permissions are accessible after privilege drop.
	joinHostGroups(username)

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

	// Persist user configuration for re-attach
	// Store info in a file on the volume so it survives container restarts
	userConfigPath := filepath.Join(homeDir, ".local/share/kilo/.user-config.json")
	utils.Log("[userinit] Saving user config to: %s\n", userConfigPath)
	userConfig := map[string]string{
		"homeDir":  homeDir,
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

	// Mark initialized only after all user init work has completed.
	utils.Log("[userinit] Marking container as initialized: %s\n", initMarker)
	_ = os.WriteFile(initMarker, []byte("1\n"), 0o600)

	utils.Log("[userinit] Starting zellij\n")
	return execZellij()
}

// createOSUser creates an OS user with the given name and UID/GID.
func createOSUser(username, uidStr, gidStr string) error {
	_ = exec.Command("deluser", username).Run()
	cmd := exec.Command("addgroup", "--gid", gidStr, username)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("addgroup: %v: %s", err, out)
	}
	cmd = exec.Command("adduser", "--uid", uidStr, "--ingroup", username, "--disabled-password", "--shell", "/bin/sh", username)
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

// joinHostGroups adds the container user to supplementary groups matching
// the host user's group IDs (passed via PGIDS env var). This enables access
// to workspace files with group-level permissions after privilege drop.
func joinHostGroups(username string) {
	pgidsStr := os.Getenv("PGIDS")
	if pgidsStr == "" {
		utils.Log("[userinit] joinHostGroups: no PGIDS env var\n")
		return
	}
	utils.Log("[userinit] joinHostGroups: host supplementary groups: %s\n", pgidsStr)

	var groupNames []string
	for _, gidStr := range strings.Split(pgidsStr, ",") {
		gidStr = strings.TrimSpace(gidStr)
		if gidStr == "" {
			continue
		}
		out, err := exec.Command("getent", "group", gidStr).Output()
		if err == nil {
			parts := strings.SplitN(strings.TrimSpace(string(out)), ":", 2)
			if len(parts) > 0 && parts[0] != "" {
				groupNames = append(groupNames, parts[0])
			}
		} else {
			groupName := "kilo-host-gid-" + gidStr
			if err := exec.Command("addgroup", "--gid", gidStr, groupName).Run(); err != nil {
				utils.LogWarn("[userinit] joinHostGroups: failed to create group %s (GID %s): %v\n", groupName, gidStr, err)
				continue
			}
			utils.Log("[userinit] joinHostGroups: created group %s with GID %s\n", groupName, gidStr)
			groupNames = append(groupNames, groupName)
		}
	}

	if len(groupNames) == 0 {
		utils.Log("[userinit] joinHostGroups: no groups to join\n")
		return
	}

	joined := strings.Join(groupNames, ",")
	utils.Log("[userinit] joinHostGroups: adding %s to groups: %s\n", username, joined)
	if err := exec.Command("usermod", "--append", "--groups", joined, username).Run(); err != nil {
		utils.LogWarn("[userinit] joinHostGroups: failed to add %s to groups: %v\n", username, err)
	} else {
		utils.Log("[userinit] joinHostGroups: added %s to groups: %s\n", username, joined)
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

// initTokens merges the MCP token from login with any existing encrypted
// tokens on the volume and re-saves them. Also stores sync JWT tokens for
// the file sync subsystem.
func initTokens(homeDir, userID string, loginRes loginResult) error {
	var context7Token, ainstructToken string
	var syncToken, syncRefreshToken, syncTokenExpiry string
	var ainstructPATExpiry string

	if loginRes.AinstructPAT != "" {
		ainstructToken = loginRes.AinstructPAT
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
	if loginRes.AinstructPATExpiry > 0 {
		ainstructPATExpiry = strconv.FormatInt(loginRes.AinstructPATExpiry, 10)
	}

	storedContext7, storedAinstruct, storedSync, storedSyncRefresh, storedSyncExpiry, storedPatExpiry, loadErr := loadEncryptedTokens(homeDir, userID)
	if loadErr == nil {
		ainstructToken = coalesce(ainstructToken, storedAinstruct)
		context7Token = coalesce(context7Token, storedContext7)
		syncToken = coalesce(syncToken, storedSync)
		syncRefreshToken = coalesce(syncRefreshToken, storedSyncRefresh)
		syncTokenExpiry = coalesce(syncTokenExpiry, storedSyncExpiry)
		ainstructPATExpiry = coalesce(ainstructPATExpiry, storedPatExpiry)
	}

	if context7Token == "" && ainstructToken == "" && syncToken == "" {
		return nil
	}

	return saveEncryptedTokens(homeDir, userID, context7Token, ainstructToken, syncToken, syncRefreshToken, syncTokenExpiry, ainstructPATExpiry)
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

// coalesce returns the first non-empty string from the given values.
func coalesce(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
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

// runMCPTokens interactively prompts for Context7 and Ainstruct MCP tokens
// and saves them to encrypted storage alongside existing sync tokens.
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
			c7, aInst, sTok, sRef, sExp, _, _ := parseTokenEnv(string(decrypted))
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

	if err := saveEncryptedTokens(homeDir, userID, context7Token, ainstructToken, syncToken, syncRefresh, syncExpiry, ""); err != nil {
		utils.LogWarn("[userinit] failed to save tokens: %v\n", err)
	}

	utils.Log("[kilo-docker] MCP tokens saved\n", utils.WithOutput())
	return nil
}

// checkRemoteHasConfig queries the ainstruct API to determine whether a
// kilo.jsonc config file already exists in the remote collection. Used during
// first-time init to decide whether to push the local config template.
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

	_, _, syncToken, _, _, _, _ := parseTokenEnv(string(decrypted))
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

	_, _, syncToken, _, _, _, _ := parseTokenEnv(string(decrypted))
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
			Metadata   struct {
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

// maskToken truncates and masks a token string for safe logging.
// Returns "***" for tokens shorter than 8 chars.
func maskToken(token string) string {
	if len(token) <= 8 {
		return strings.Repeat("*", len(token))
	}
	return token[:4] + strings.Repeat("*", len(token)-8) + token[len(token)-4:]
}
