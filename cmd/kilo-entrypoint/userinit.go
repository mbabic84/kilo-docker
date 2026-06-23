package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/mbabic84/kilo-docker/pkg/utils"
)

// migrateContainerPaths moves persistent data files from the old kilo
// namespace to the new kilo-docker namespace. Called once during user init
// after home directory creation, before any read operations on old paths.
// If the new path already contains a file, the old file is renamed with a
// .migrated suffix rather than overwritten.
func migrateContainerPaths(homeDir string) {
	migrations := []struct {
		oldRel string
		newRel string
		label  string
	}{
		{".local/share/kilo/.tokens.env.enc", ".local/share/kilo-docker/.tokens.env.enc", "encrypted tokens"},
		{".local/share/kilo/.custom-envs.env.enc", ".local/share/kilo-docker/.custom-envs.env.enc", "custom envs"},
		{".local/share/kilo/.user-config.json", ".local/share/kilo-docker/.user-config.json", "user config"},
		{".config/kilo/.ainstruct-hashes", ".config/kilo-docker/.ainstruct-hashes", "sync hashes"},
	}

	for _, m := range migrations {
		oldPath := filepath.Join(homeDir, m.oldRel)
		newPath := filepath.Join(homeDir, m.newRel)

		_, oldErr := os.Stat(oldPath)
		oldExists := oldErr == nil

		_, newErr := os.Stat(newPath)
		newExists := newErr == nil

		if !oldExists {
			continue
		}

		if newExists {
			utils.Log("[kilo-docker] %s already at new location, renaming old file\n", m.label, utils.WithOutput())
			utils.Log("[kilo-docker]   Old: %s → %s.migrated\n", oldPath, oldPath, utils.WithOutput())
			if err := os.Rename(oldPath, oldPath+".migrated"); err != nil {
				utils.LogWarn("[kilo-docker] Warning: failed to rename old %s: %v\n", m.label, err, utils.WithOutput())
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(newPath), 0o700); err != nil {
			utils.LogWarn("[kilo-docker] Warning: failed to create directory for %s: %v\n", m.label, err, utils.WithOutput())
			continue
		}

		if err := os.Rename(oldPath, newPath); err != nil {
			utils.LogWarn("[kilo-docker] Warning: failed to migrate %s: %v\n", m.label, err, utils.WithOutput())
			utils.LogWarn("[kilo-docker]   Old: %s\n", oldPath, utils.WithOutput())
			utils.LogWarn("[kilo-docker]   New: %s\n", newPath, utils.WithOutput())
		} else {
			utils.Log("[kilo-docker] Migrated %s to kilo-docker namespace\n", m.label, utils.WithOutput())
			utils.Log("[kilo-docker]   %s → %s\n", oldPath, newPath, utils.WithOutput())
		}
	}
}

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
		userIDPath := filepath.Join(homeDir, ".local/share/kilo-docker/.user-config.json")
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
	_ = os.MkdirAll(filepath.Join(homeDir, ".local/share/kilo-docker"), 0o700)
	_ = os.MkdirAll(filepath.Join(homeDir, ".ssh"), 0700)

	migrateContainerPaths(homeDir)

	// Update .bashrc managed section based on KD_SERVICES (nvm, uv)
	updateBashrcManaged(homeDir)

	// Create .bash_profile that sources .bashrc for login shells (e.g. zellij panes).
	bashProfile := filepath.Join(homeDir, ".bash_profile")
	if _, err := os.Stat(bashProfile); os.IsNotExist(err) {
		profileContent := "# Source .bashrc for login shells (e.g. zellij panes)\n" +
			"if [ -f \"$HOME/.bashrc\" ]; then\n" +
			"    . \"$HOME/.bashrc\"\n" +
			"fi\n"
		_ = os.WriteFile(bashProfile, []byte(profileContent), 0o600)
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
			hashFile := filepath.Join(homeDir, ".config/kilo-docker/.ainstruct-hashes")
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
	userConfigPath := filepath.Join(homeDir, ".local/share/kilo-docker/.user-config.json")
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
	_ = os.Setenv("BASH_ENV", filepath.Join(homeDir, ".bashrc"))

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
// tokens (in either old or new path), returning the directory name if found.
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
		newPath := filepath.Join(baseDir, entry.Name(), ".local/share/kilo-docker/.tokens.env.enc")
		oldPath := filepath.Join(baseDir, entry.Name(), ".local/share/kilo/.tokens.env.enc")
		if _, err := os.Stat(newPath); err == nil {
			utils.Log("[findExistingUser] found user %s with token file (new path)\n", entry.Name())
			return entry.Name()
		}
		if _, err := os.Stat(oldPath); err == nil {
			utils.Log("[findExistingUser] found user %s with token file (old path)\n", entry.Name())
			return entry.Name()
		}
		utils.Log("[findExistingUser] dir %s exists but no token file at either path\n", entry.Name())
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

// updateBashrcManaged writes NVM and Python wrapper functions to a managed
// section of .bashrc. The section is delimited by "# >>> kilo-managed >>>"
// and "# <<< kilo-managed <<<". Content is determined by KD_SERVICES env var,
// which is set when --nvm or --uv flags are used.
//
// When multiple sessions share the same home volume, a service installed by
// one session (e.g. NVM) must not be removed from .bashrc by another session
// that does not use that service. Existing service blocks are detected by
// their sentinel comments and preserved across updates.
func updateBashrcManaged(homeDir string) {
	bashrc := filepath.Join(homeDir, ".bashrc")
	servicesEnv := os.Getenv("KD_SERVICES")

	startMarker := "# >>> kilo-managed >>>"
	endMarker := "# <<< kilo-managed <<<"

	nvmBlock := "# NVM support\n" +
		"if [ -f \"$HOME/.nvm/nvm.sh\" ]; then\n" +
		"    export NVM_DIR=\"$HOME/.nvm\"\n" +
		"    . \"$HOME/.nvm/nvm.sh\"\n" +
		"fi\n"

	uvBlock := "# Python wrappers using uv\n" +
		"if command -v uv &>/dev/null; then\n" +
		"    python() {\n" +
		"        uv run python \"$@\"\n" +
		"    }\n" +
		"    python3() {\n" +
		"        uv run python \"$@\"\n" +
		"    }\n" +
		"fi\n"

	nvmSentinel := "# NVM support"
	uvSentinel := "# Python wrappers using uv"

	// Determine which services the current session wants.
	currentServices := make(map[string]bool)
	for _, svc := range strings.Split(servicesEnv, ",") {
		currentServices[strings.TrimSpace(svc)] = true
	}

	// Read the existing managed section so that services installed by other
	// sessions on the shared volume are not lost.
	existing, err := os.ReadFile(bashrc)
	existingHasNVM := false
	existingHasUV := false
	if err == nil {
		existingStr := string(existing)
		if start := strings.Index(existingStr, startMarker); start >= 0 {
			section := existingStr[start+len(startMarker):]
			if end := strings.Index(section, endMarker); end >= 0 {
				section = section[:end]
				existingHasNVM = strings.Contains(section, nvmSentinel)
				existingHasUV = strings.Contains(section, uvSentinel)
			}
		}
	}

	nvmInstalledOnDisk := fileExists(filepath.Join(homeDir, ".nvm", "nvm.sh"))
	uvInstalledOnDisk := fileExists(filepath.Join(homeDir, ".local", "bin", "uv"))

	// Build the new managed section. A service block is included when:
	//  1. The current session requests it (e.g. --nvm), OR
	//  2. The service was already in .bashrc AND is still installed on disk.
	// This preserves services across concurrent sessions while cleaning up
	// blocks for services that have been removed from the volume.
	var sb strings.Builder
	sb.WriteString(startMarker + "\n")

	if currentServices["nvm"] || (existingHasNVM && nvmInstalledOnDisk) {
		sb.WriteString(nvmBlock)
		utils.Log("[userinit] Added NVM to .bashrc managed section\n")
	}
	if currentServices["uv"] || (existingHasUV && uvInstalledOnDisk) {
		sb.WriteString(uvBlock)
		utils.Log("[userinit] Added Python uv wrappers to .bashrc managed section\n")
	}

	sb.WriteString(endMarker + "\n")
	managedContent := sb.String()

	if err != nil {
		_ = os.WriteFile(bashrc, []byte("# ~/.bashrc\n\n"+managedContent), 0o600)
		utils.Log("[userinit] Created .bashrc with managed section\n")
		return
	}

	existingStr := string(existing)

	// Remove old managed section if present
	for {
		start := strings.Index(existingStr, startMarker)
		if start < 0 {
			break
		}
		end := strings.Index(existingStr[start:], endMarker)
		if end < 0 {
			break
		}
		end += start + len(endMarker)
		existingStr = existingStr[:start] + existingStr[end:]
	}

	existingStr = strings.TrimRight(existingStr, "\n")
	newBashrc := existingStr + "\n\n" + managedContent
	_ = os.WriteFile(bashrc, []byte(newBashrc), 0o600)
	utils.Log("[userinit] Updated .bashrc managed section (KD_SERVICES=%s)\n", servicesEnv)
}

// fileExists reports whether the given path exists and is not a directory.
func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
