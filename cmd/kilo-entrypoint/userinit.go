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

	existingUser := findExistingUser()
	if existingUser != "" {
		utils.Log("Found existing user data: %s\n", existingUser)
	}

	// Login
	utils.Log("Starting Ainstruct authentication\n")
	loginRes, err := runLoginInteractive()
	if err != nil {
		return fmt.Errorf("login failed: %w", err)
	}
	userID := loginRes.UserID

	if existingUser != "" {
		derived := deriveHomeName(userID)
		if derived != existingUser {
			return fmt.Errorf("user mismatch: expected %s, got %s", existingUser, derived)
		}
		utils.Log("User verified: %s\n", derived)
	}

	username := deriveHomeName(userID)
	homeDir := "/home/" + username

	// Create OS user
	utils.Log("Creating user: %s (UID=%s, GID=%s)\n", username, puidStr, pgidStr)
	if err := createOSUser(username, puidStr, pgidStr); err != nil {
		return fmt.Errorf("failed to create user %s: %w", username, err)
	}

	// Add user to service groups
	joinServiceGroups(username)

	// Create home directory and config structure
	utils.Log("Creating home directory: %s\n", homeDir)
	os.MkdirAll(homeDir, 0755)
	os.MkdirAll(filepath.Join(homeDir, ".config/kilo/commands"), 0755)
	os.MkdirAll(filepath.Join(homeDir, ".config/kilo/agents"), 0755)
	os.MkdirAll(filepath.Join(homeDir, ".config/kilo/plugins"), 0755)
	os.MkdirAll(filepath.Join(homeDir, ".config/kilo/skills"), 0755)
	os.MkdirAll(filepath.Join(homeDir, ".config/kilo/tools"), 0755)
	os.MkdirAll(filepath.Join(homeDir, ".config/kilo/rules"), 0755)
	os.MkdirAll(filepath.Join(homeDir, ".local/share/kilo"), 0755)
	os.MkdirAll(filepath.Join(homeDir, ".ssh"), 0700)

	// Create .bashrc if missing
	bashrc := filepath.Join(homeDir, ".bashrc")
	if _, err := os.Stat(bashrc); os.IsNotExist(err) {
		os.WriteFile(bashrc, []byte("# ~/.bashrc\n"), 0644)
	}

	// Copy default config templates if user doesn't have them
	copyFileIfMissing("/etc/kilo/opencode.json", filepath.Join(homeDir, ".config/kilo/opencode.json"))
	copyFileIfMissing("/etc/zellij/config.kdl", filepath.Join(homeDir, ".config/zellij/config.kdl"))

	// Setup SSH known_hosts
	utils.Log("Setting up SSH known_hosts\n")
	setupKnownHosts(homeDir)

	// Copy service configs to the new home
	utils.Log("Copying service configs\n")
	copyServiceConfigs(homeDir)

	// Install user-scoped services (NVM, etc.) with HOME set to user home
	utils.Log("Installing user-scoped services\n")
	installUserServices(homeDir)

	// Set HOME temporarily so GetKiloConfigDir() and token paths resolve correctly
	savedHome := os.Getenv("HOME")
	os.Setenv("HOME", homeDir)

	// MCP token initialization and config
	// Note: KD_MCP_* tokens should ONLY be set by the kilo wrapper script,
	// not globally. Load stored encrypted tokens to check if context7 is configured.
	// Note: KD_MCP_CONTEXT7_TOKEN should ONLY be set by the kilo wrapper script
	var context7TokenExists bool
	var context7TokenEmpty bool
	tokenFilePath := filepath.Join(homeDir, ".local/share/kilo/.tokens.env.enc")
	if _, encErr := os.Stat(tokenFilePath); encErr == nil {
		if encData, readErr := os.ReadFile(tokenFilePath); readErr == nil {
			if decrypted, decErr := decryptAES(encData, userID); decErr == nil {
				c7, _, sTok, sRef, sExp, _ := parseTokenEnv(string(decrypted))
				context7TokenExists = true
				if c7 == "" {
					context7TokenEmpty = true
				}
				if sTok != "" {
					os.Setenv("KD_AINSTRUCT_SYNC_TOKEN", sTok)
				}
				if sRef != "" {
					os.Setenv("KD_AINSTRUCT_SYNC_REFRESH_TOKEN", sRef)
				}
				if sExp != "" {
					os.Setenv("KD_AINSTRUCT_SYNC_TOKEN_EXPIRY", sExp)
				}
			}
		}
	}

	// Prompt for Context7 token only if never configured (not stored or empty)
	if context7TokenExists && !context7TokenEmpty {
		// Token exists and is set - nothing to do
	} else if context7TokenEmpty {
		// User explicitly disabled context7, keep it disabled
		utils.Log("Context7 token explicitly disabled, skipping prompt\n")
	} else {
		// Not set (never configured), prompt user to get token
		utils.Log("Initializing MCP tokens\n")
		promptContext7Token()
	}

	// Save all tokens (ainstruct PAT from login + context7 + sync tokens)
	utils.Log("Saving MCP tokens\n")
	if err := initTokens(homeDir, userID, loginRes); err != nil {
		utils.LogWarn("token init failed: %v\n", err)
	}

	// Chown SSH agent socket to the new user
	if sshAuthSock := os.Getenv("SSH_AUTH_SOCK"); sshAuthSock != "" {
		utils.Log("Setting SSH agent socket ownership: %s\n", sshAuthSock)
		os.Chown(sshAuthSock, puid, pgid)
		os.Chmod(sshAuthSock, 0600)
	}

	os.Setenv("HOME", savedHome)

	// Mark initialized
	os.WriteFile(initMarker, []byte("1\n"), 0644)

	// Persist user configuration for re-attach
	// Store info in a file on the volume so it survives container restarts
	userConfigPath := filepath.Join(homeDir, ".local/share/kilo/.user-config.json")
	utils.Log("Saving user config to: %s\n", userConfigPath)
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
	if err := os.WriteFile(userConfigPath, configJSON, 0644); err != nil {
		utils.LogWarn("failed to save user config: %v\n", err)
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
		utils.Log("Saved user config: %s\n", string(redactedJSON))
	}

	// Set environment
	os.Setenv("HOME", homeDir)
	os.Setenv("USER", username)
	os.Setenv("LOGNAME", username)
	os.Setenv("SHELL", userConfig["shell"])

	// Chown everything to the new user (after all root-level file writes)
	utils.Log("Setting ownership: %s\n", homeDir)
	chownRecursive(homeDir, puid, pgid)

	// Drop privileges and exec zellij
	utils.Log("Dropping privileges to %s (UID=%s, GID=%s)\n", username, puidStr, pgidStr)
	syscall.Setgid(pgid)
	syscall.Setuid(puid)

	// Start file sync as the user (after privilege drop so files are owned correctly)
	utils.Log("Starting file sync\n")
	startSyncWithTokens(homeDir, userID)

	// Delete stale zellij sessions from previous container lifecycle
	utils.Log("Clearing old zellij sessions\n")
	clearZellijSessions()

	utils.Log("Starting zellij\n")
	return execZellij()
}

// createOSUser creates an OS user with the given name and UID/GID.
func createOSUser(username, uidStr, gidStr string) error {
	exec.Command("deluser", username).Run()
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
	os.MkdirAll(filepath.Dir(dst), 0755)
	os.WriteFile(dst, data, 0644)
	utils.Log("Copied %s\n", filepath.Base(src))
}

// joinServiceGroups adds the user to service groups created by init.
func joinServiceGroups(username string) {
	servicesEnv := os.Getenv("KD_SERVICES")
	if servicesEnv == "" {
		return
	}
	for _, svcName := range strings.Split(servicesEnv, ",") {
		svc := getService(svcName)
		if svc == nil || svc.RequiresSocket == "" {
			continue
		}
		exec.Command("addgroup", username, svc.Name).Run()
	}
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
		utils.LogWarn("failed to delete zellij sessions: %v\n", err)
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
		return ""
	}
	for _, entry := range entries {
		if !entry.IsDir() || !strings.HasPrefix(entry.Name(), "kd-") {
			continue
		}
		tokenPath := filepath.Join(baseDir, entry.Name(), ".local/share/kilo/.tokens.env.enc")
		if _, err := os.Stat(tokenPath); err == nil {
			return entry.Name()
		}
	}
	return ""
}

// startSyncWithTokens decrypts tokens from the volume, sets them as
// process env vars for the sync child process, and starts sync in background.
func startSyncWithTokens(homeDir, userID string) error {
	encPath := filepath.Join(homeDir, ".local/share/kilo/.tokens.env.enc")
	if _, err := os.Stat(encPath); err != nil {
		return nil
	}

	encData, err := os.ReadFile(encPath)
	if err != nil {
		return nil
	}

	decrypted, err := decryptAES(encData, userID)
	if err != nil {
		return nil
	}

	_, _, syncToken, syncRefresh, syncExpiry, _ := parseTokenEnv(string(decrypted))

	// Sync tokens are needed globally for file sync - set via os.Setenv
	if syncToken != "" {
		os.Setenv("KD_AINSTRUCT_SYNC_TOKEN", syncToken)
	}
	if syncRefresh != "" {
		os.Setenv("KD_AINSTRUCT_SYNC_REFRESH_TOKEN", syncRefresh)
	}
	if syncExpiry != "" {
		os.Setenv("KD_AINSTRUCT_SYNC_TOKEN_EXPIRY", syncExpiry)
	}
	// MCP tokens (context7, ainstruct) should ONLY be set by the kilo wrapper script
	// Not here - not even for the sync subprocess

	go func() {
		cmd := exec.Command("kilo-entrypoint", "sync")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Start()
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

	fmt.Println("MCP Token Management")
	fmt.Println("====================")
	fmt.Println()

	currentContext7 := "[not set]"
	if context7Token != "" {
		currentContext7 = maskToken(context7Token)
	}
	fmt.Printf("Context7 token: %s\n", currentContext7)
	fmt.Println("  - Press Enter to keep current")
	fmt.Println("  - Type a new token to update")
	fmt.Println("  - Type 'clear' to disable")
	fmt.Print("> ")

	var input string
	fmt.Scanln(&input)
	input = strings.TrimSpace(input)

	if input == "clear" {
		context7Token = ""
		fmt.Println("Context7 token disabled")
	} else if input != "" {
		context7Token = input
		fmt.Println("Context7 token updated")
	}

	if err := saveEncryptedTokens(homeDir, userID, context7Token, ainstructToken, syncToken, syncRefresh, syncExpiry); err != nil {
		utils.LogWarn("failed to save tokens: %v\n", err)
	}

	fmt.Println("MCP tokens saved")
	return nil
}

func maskToken(token string) string {
	if len(token) <= 8 {
		return strings.Repeat("*", len(token))
	}
	return token[:4] + strings.Repeat("*", len(token)-8) + token[len(token)-4:]
}
