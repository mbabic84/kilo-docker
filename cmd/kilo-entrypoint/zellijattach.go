package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/mbabic84/kilo-docker/pkg/constants"
	"github.com/mbabic84/kilo-docker/pkg/utils"
	"golang.org/x/term"
)

// initMarker and userInitInProgressMarker are variables (not constants) so
// tests can redirect them to a temporary directory.
var (
	initMarker               = "/tmp/.kilo-initialized"
	userInitInProgressMarker = "/tmp/.kilo-user-init-in-progress"
	userInitLockFile         = "/tmp/.kilo-user-init.lock"
)

const initReadyTimeoutEnvVar = "KD_INIT_READY_TIMEOUT"

const defaultInitReadyTimeout = 5 * time.Minute

// errStaleUserInitMarker signals that the in-progress marker's owner is
// provably gone (PID absent or start-time mismatch). It is distinct from a
// timeout so callers can reclaim the marker instead of blocking.
var errStaleUserInitMarker = errors.New("stale user-init marker")

// userInitClaimed records whether this process won the user-init race, so the
// cancellation handler only removes a marker it actually owns.
var userInitClaimed atomic.Bool

// runZellijAttach is the entry point for the "zellij-attach" subcommand.
// If the container is already initialized, it execs zellij directly.
// Otherwise it runs the first-time user init flow.
func runZellijAttach() error {
	utils.Log("[zellijattach] Entry\n")

	installLoginSignalHandler()

	if err := waitForMarker(initReadyMarker, initReadyTimeout(),
		"[kilo-docker] Waiting for container initialization to finish...\n"); err != nil {
		return err
	}

	timeout := initReadyTimeout()

	if _, err := os.Stat(initMarker); err == nil {
		utils.Log("[zellijattach] Init marker exists, checking PAT expiry\n")
		if needsAinstructPATRefresh() {
			utils.Log("[kilo-docker] Ainstruct PAT expiring soon, re-authentication required\n", utils.WithOutput())
			claimed, err := acquireUserInit(timeout)
			if err != nil {
				return err
			}
			if !claimed {
				ensureSyncForCurrentUser()
				return execZellij()
			}
			defer removeOwnedInitMarker()
			return refreshPATOnly()
		}
		ensureSyncForCurrentUser()
		return execZellij()
	}

	claimed, err := acquireUserInit(timeout)
	if err != nil {
		return err
	}
	if !claimed {
		ensureSyncForCurrentUser()
		return execZellij()
	}
	defer removeOwnedInitMarker()

	// Container not initialized - run full user init
	return runUserInit()
}

// installLoginSignalHandler installs SIGINT/SIGTERM/SIGHUP handlers for the
// interactive attach. On signal it restores terminal echo, removes the
// in-progress marker if this process owns it, prints "login cancelled", and
// exits non-zero. It is a no-op once the process is replaced by zellij.
func installLoginSignalHandler() {
	stdinFD := int(os.Stdin.Fd())
	savedState, _ := term.GetState(stdinFD)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
	go func() {
		sig := <-sigCh
		utils.LogWarn("\n[kilo-docker] login cancelled (%s)\n", sig, utils.WithOutput())
		if savedState != nil {
			_ = term.Restore(stdinFD, savedState)
		}
		removeOwnedInitMarker()
		os.Exit(130)
	}()
}

// needsAinstructPATRefresh checks whether the stored Ainstruct PAT is
// expiring within the rotation threshold or is missing entirely.
func needsAinstructPATRefresh() bool {
	homeDir, _, _, userID := loadUserConfig()
	if homeDir == "" || userID == "" {
		utils.Log("[zellijattach] No user config, cannot check PAT expiry\n")
		return false
	}

	store, err := loadTokenStore(homeDir, userID)
	if err != nil {
		utils.Log("[zellijattach] Cannot load encrypted tokens: %v\n", err)
		return false
	}
	patExpiryStr := store.Get(tokenKeyPATExtiry)
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

// refreshPATOnly performs a lightweight silent Ainstruct PAT rotation on the
// re-attach path when the stored PAT is expiring. Unlike runUserInit it does
// not recreate the OS user, reinstall services, or clear live zellij sessions;
// it only refreshes tokens, ensures sync, and execs zellij. PAT failures are
// degraded (warn + disable Ainstruct MCP) rather than fatal, matching the
// interactive login behavior.
func refreshPATOnly() error {
	homeDir, _, _, userID := loadUserConfig()
	if homeDir == "" || userID == "" {
		utils.LogWarn("[zellijattach] Cannot refresh PAT: no user config\n")
		return execZellij()
	}

	set, ok := loadUsableSyncTokens(homeDir, userID, constants.AinstructAPIBaseURL)
	if !ok {
		utils.LogWarn("[zellijattach] No usable stored tokens for PAT refresh\n")
		return execZellij()
	}

	label := buildAinstructPATLabel()
	pat, patExpiry, patErr := ensureAinstructPAT(constants.AinstructAPIBaseURL, set.accessToken, label, set.ainstruct, formatExpiry(set.patExpiry))
	if patErr != nil {
		if isAuthRejection(patErr) {
			utils.LogWarn("[zellijattach] Stored credentials rejected; skipping PAT refresh\n")
		} else {
			warnPATDisabled(patErr)
		}
		pat, patExpiry = set.ainstruct, set.patExpiry
	}

	// Persist the token set even when the PAT call failed: loadUsableSyncTokens
	// may have silently rotated the refresh token, and that update must not be
	// dropped or the next attach can no longer refresh.
	store, loadErr := loadTokenStore(homeDir, userID)
	if loadErr != nil {
		store = TokenStore{}
	}
	store.Set(tokenKeyContext7, set.context7)
	store.Set(tokenKeyAinstruct, pat)
	store.Set(tokenKeySync, set.accessToken)
	store.Set(tokenKeySyncRefresh, set.refreshToken)
	store.Set(tokenKeySyncExpiry, formatExpiry(set.syncExpiry))
	store.Set(tokenKeyPATExtiry, formatExpiry(patExpiry))
	if err := saveTokenStore(homeDir, userID, store); err != nil {
		utils.LogWarn("[zellijattach] Failed to save rotated tokens: %v\n", err)
	} else {
		chownTokenFile(homeDir)
	}

	ensureSyncForCurrentUser()
	return execZellij()
}

// chownTokenFile hands ownership of the encrypted token file to the configured
// user so the (unprivileged) sync process can read tokens this root-owned
// attach just wrote. Failures are non-fatal (the file simply keeps its mode).
func chownTokenFile(homeDir string) {
	uid, gid, ok := userIDsFromConfig(homeDir)
	if !ok {
		return
	}
	tokenPath := filepath.Join(homeDir, ".local/share/kilo-docker/.tokens.env.enc")
	if err := os.Chown(tokenPath, uid, gid); err != nil {
		utils.LogWarn("[zellijattach] Failed to chown token file: %v\n", err)
	}
}

// userIDsFromConfig reads uid/gid from the persisted user config.
func userIDsFromConfig(homeDir string) (int, int, bool) {
	configPath := filepath.Join(homeDir, ".local/share/kilo-docker/.user-config.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return 0, 0, false
	}
	var config map[string]string
	if err := json.Unmarshal(data, &config); err != nil {
		return 0, 0, false
	}
	uid, uidErr := strconv.Atoi(config["uid"])
	gid, gidErr := strconv.Atoi(config["gid"])
	if uidErr != nil || gidErr != nil {
		return 0, 0, false
	}
	return uid, gid, true
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

// initMarkerInfo identifies the process that owns the in-progress marker.
// startTime is field 22 of /proc/<pid>/stat and guards against PID reuse.
type initMarkerInfo struct {
	pid       int
	startTime uint64
}

// initMarkerContent renders the current process identity as "<pid> <startTime>".
func initMarkerContent() (string, error) {
	startTime, err := readProcStartTime(os.Getpid())
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%d %d\n", os.Getpid(), startTime), nil
}

// writeInitMarker records the current process identity in the marker file.
func writeInitMarker(f *os.File) error {
	content, err := initMarkerContent()
	if err != nil {
		return err
	}
	_, err = f.WriteString(content)
	return err
}

// parseInitMarker parses the "<pid> <startTime>" marker format. A marker that
// predates this format (or is momentarily empty) yields an error so callers
// treat it as un-reclaimable rather than guessing.
func parseInitMarker(data string) (initMarkerInfo, error) {
	fields := strings.Fields(data)
	if len(fields) != 2 {
		return initMarkerInfo{}, fmt.Errorf("unexpected marker content %q", strings.TrimSpace(data))
	}
	pid, err := strconv.Atoi(fields[0])
	if err != nil {
		return initMarkerInfo{}, fmt.Errorf("invalid marker pid %q: %w", fields[0], err)
	}
	startTime, err := strconv.ParseUint(fields[1], 10, 64)
	if err != nil {
		return initMarkerInfo{}, fmt.Errorf("invalid marker start time %q: %w", fields[1], err)
	}
	return initMarkerInfo{pid: pid, startTime: startTime}, nil
}

// readProcStartTime returns field 22 (starttime) of /proc/<pid>/stat. The
// comm field (field 2) may contain spaces and parentheses, so parsing starts
// after the last ')' where fields 3.. follow.
func readProcStartTime(pid int) (uint64, error) {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return 0, err
	}
	s := string(data)
	closeIdx := strings.LastIndexByte(s, ')')
	if closeIdx < 0 || closeIdx+2 > len(s) {
		return 0, fmt.Errorf("malformed /proc/%d/stat", pid)
	}
	fields := strings.Fields(s[closeIdx+2:])
	const startTimeIdx = 22 - 3 // fields[0] is field 3 (state)
	if len(fields) <= startTimeIdx {
		return 0, fmt.Errorf("malformed /proc/%d/stat: only %d fields", pid, len(fields))
	}
	return strconv.ParseUint(fields[startTimeIdx], 10, 64)
}

// initProcAlive reports whether the process recorded in the marker is still
// alive. It returns false when the process is gone or when its start time no
// longer matches (PID reuse). On any parse failure it conservatively returns
// true so an unreadable marker is never reclaimed.
func initProcAlive(pid int, startTime uint64) bool {
	current, err := readProcStartTime(pid)
	if err == nil {
		return current == startTime
	}
	// Process gone (ENOENT) means not alive; any other failure (e.g. an
	// unreadable /proc entry) is treated conservatively as still alive.
	return !os.IsNotExist(err)
}

// staleInitMarker reports whether the marker's owner is provably gone. An
// unparseable marker is never considered stale.
func staleInitMarker(data string) bool {
	info, err := parseInitMarker(data)
	if err != nil {
		return false
	}
	return !initProcAlive(info.pid, info.startTime)
}

// markerOwnerGone reports whether the on-disk in-progress marker belongs to a
// process that no longer exists. Missing/unreadable markers return false.
func markerOwnerGone() bool {
	data, err := os.ReadFile(userInitInProgressMarker)
	if err != nil {
		return false
	}
	return staleInitMarker(string(data))
}

// removeOwnedInitMarker removes the in-progress marker, but only when this
// process is its owner, so a waiting attach never deletes another process's
// marker.
func removeOwnedInitMarker() {
	if userInitClaimed.Load() {
		_ = os.Remove(userInitInProgressMarker)
	}
}

// tryClaimUserInit performs a single atomic claim. It writes the marker
// content to a temp file and hard-links it into place, so the marker is never
// observable empty (a crash mid-write cannot leave an unparseable marker) and
// the link fails atomically if another process already holds the marker.
func tryClaimUserInit() (bool, error) {
	dir := filepath.Dir(userInitInProgressMarker)
	tmpFile, err := os.CreateTemp(dir, filepath.Base(userInitInProgressMarker)+".claim.*")
	if err != nil {
		return false, fmt.Errorf("failed to create init marker temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer func() { _ = os.Remove(tmpPath) }()

	if werr := writeInitMarker(tmpFile); werr != nil {
		_ = tmpFile.Close()
		return false, fmt.Errorf("failed to write init marker: %w", werr)
	}
	if cerr := tmpFile.Close(); cerr != nil {
		return false, fmt.Errorf("failed to close init marker temp file: %w", cerr)
	}

	if err := os.Link(tmpPath, userInitInProgressMarker); err != nil {
		if os.IsExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("failed to claim user initialization: %w", err)
	}

	userInitClaimed.Store(true)
	utils.Log("[zellijattach] Claimed user initialization\n")
	return true, nil
}

// claimUserInit atomically creates a user-init-in-progress marker. A marker
// whose owner is provably gone is reclaimed and the claim retried. Serialized
// with an exclusive flock so the read-decide-remove sequence cannot race two
// concurrent reclaimers into both owning the marker. Returns true if this
// process won the race, false otherwise.
func claimUserInit() (bool, error) {
	lock, lockErr := utils.Acquire(userInitLockFile, true)
	if lockErr != nil {
		// Without the lock we still have the atomic O_EXCL/link claim, so
		// proceed rather than failing the attach.
		utils.LogWarn("[zellijattach] Could not acquire user-init lock: %v\n", lockErr)
		return tryClaimUserInit()
	}
	defer func() { _ = lock.Release() }()

	claimed, err := tryClaimUserInit()
	if err != nil || claimed {
		return claimed, err
	}

	data, readErr := os.ReadFile(userInitInProgressMarker)
	if readErr != nil || !staleInitMarker(string(data)) {
		return false, nil
	}

	utils.LogWarn("[zellijattach] Reclaiming stale user-init marker (owner gone)\n")
	if rmErr := os.Remove(userInitInProgressMarker); rmErr != nil && !os.IsNotExist(rmErr) {
		return false, fmt.Errorf("failed to reclaim stale marker: %w", rmErr)
	}
	return tryClaimUserInit()
}

// acquireUserInit claims exclusive ownership of user init. When another attach
// already owns the marker it waits for completion; a marker whose owner is
// provably gone is reclaimed and the claim retried. Returns claimed=true when
// this process owns the marker and should run init.
func acquireUserInit(timeout time.Duration) (bool, error) {
	const maxReclaimAttempts = 3
	for attempt := 0; attempt < maxReclaimAttempts; attempt++ {
		claimed, err := claimUserInit()
		if err != nil {
			return false, err
		}
		if claimed {
			return true, nil
		}

		waitErr := waitForCompletedUserInit(timeout)
		if waitErr == nil {
			return false, nil
		}
		if errors.Is(waitErr, errStaleUserInitMarker) {
			utils.LogWarn("[zellijattach] Stale user-init owner detected, retrying claim\n")
			continue
		}
		return false, waitErr
	}
	return false, fmt.Errorf("could not acquire user initialization after %d attempts", maxReclaimAttempts)
}

// waitForCompletedUserInit polls for the /tmp/.kilo-initialized marker file
// which signals that the winning zellij-attach process has finished user init.
// If the in-progress marker's owner is provably gone it returns
// errStaleUserInitMarker so the caller can reclaim instead of blocking.
func waitForCompletedUserInit(timeout time.Duration) error {
	if _, err := os.Stat(initMarker); err == nil {
		return nil
	}
	if markerOwnerGone() {
		return fmt.Errorf("user initialization owner is gone: %w", errStaleUserInitMarker)
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
		if markerOwnerGone() {
			return fmt.Errorf("user initialization owner is gone: %w", errStaleUserInitMarker)
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
		configPath := filepath.Join(baseDir, entry.Name(), ".local/share/kilo-docker/.user-config.json")
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
	workspace := "kilo-docker"
	if wd, err := os.Getwd(); err == nil {
		if base := filepath.Base(wd); base != "" && base != "/" {
			workspace = base
		}
	}

	hostname := os.Getenv("KD_HOSTNAME")
	if hostname == "" {
		hostname = "unknown"
	}
	sessionName := fmt.Sprintf("%s | %s", hostname, workspace)

	// If no user config found, we can't properly run as user
	if homeDir == "" || username == "" {
		utils.LogWarn("[zellijattach] No user config found, running as root\n")
	} else {
		// Load UID/GID from config to drop privileges
		configPath := filepath.Join(homeDir, ".local/share/kilo-docker/.user-config.json")
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

	// Set HOME, USER, LOGNAME, SHELL, BASH_ENV from persisted config
	if homeDir != "" {
		env = appendOrReplaceEnv(env, "HOME", homeDir)
		env = appendOrReplaceEnv(env, "BASH_ENV", filepath.Join(homeDir, ".bashrc"))
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
