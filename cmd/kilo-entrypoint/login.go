package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/mbabic84/kilo-docker/pkg/constants"
	"github.com/mbabic84/kilo-docker/pkg/utils"
	"golang.org/x/term"
)

// maxLoginAttempts is the number of interactive credential attempts before
// the attach gives up and exits.
const maxLoginAttempts = 3

// authHTTPClient bounds every Ainstruct auth/PAT/refresh call. The default
// client has no timeout, which would let a stalled endpoint block the attach
// indefinitely (the refresh call now runs on the automatic skip-login path).
var authHTTPClient = &http.Client{Timeout: 30 * time.Second}

// errLoginCancelled signals that the user aborted the interactive login (EOF
// on a prompt). Callers print "login cancelled" and exit non-zero.
var errLoginCancelled = errors.New("login cancelled")

// errUnauthorized and errForbidden tag server rejections of stored
// credentials so callers can fall back to the interactive prompt.
var (
	errUnauthorized = errors.New("unauthorized")
	errForbidden    = errors.New("forbidden")
)

// loginErrorKind classifies a failed login attempt to drive the retry policy.
type loginErrorKind int

const (
	loginErrTransient loginErrorKind = iota
	loginErrInvalidCredentials
	loginErrAccountDisabled
)

// loginError wraps an authentication failure with its retry classification.
type loginError struct {
	Kind loginErrorKind
	Err  error
}

func (e *loginError) Error() string { return e.Err.Error() }
func (e *loginError) Unwrap() error { return e.Err }

// statusError maps an HTTP status to a typed error, tagging 401/403 so callers
// can detect a rejected credential rather than a transient failure.
func statusError(action string, code int) error {
	switch code {
	case http.StatusUnauthorized:
		return fmt.Errorf("%w: %s failed with status %d", errUnauthorized, action, code)
	case http.StatusForbidden:
		return fmt.Errorf("%w: %s failed with status %d", errForbidden, action, code)
	default:
		return fmt.Errorf("%s failed with status %d", action, code)
	}
}

// isAuthRejection reports whether err represents a server rejection of the
// supplied credential (401/403).
func isAuthRejection(err error) bool {
	return errors.Is(err, errUnauthorized) || errors.Is(err, errForbidden)
}

// warnPATDisabled logs a non-fatal PAT failure and the degradation notice.
func warnPATDisabled(patErr error) {
	utils.LogWarn("[login] Warning: Failed to refresh Ainstruct PAT: %v\n", patErr)
	utils.Log("[kilo-docker] Ainstruct MCP server will be disabled.\n", utils.WithOutput())
}

// formatExpiry renders a unix expiry as a decimal string, or "" for zero.
func formatExpiry(unix int64) string {
	if unix <= 0 {
		return ""
	}
	return strconv.FormatInt(unix, 10)
}

// loginResponse is the JSON body returned by POST /api/v1/auth/login.
type loginResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
}

// profileResponse is the JSON body returned by GET /api/v1/auth/profile.
type profileResponse struct {
	UserID string `json:"user_id"`
}

// loginResult holds the outcome of a completed login flow.
type loginResult struct {
	UserID             string
	AccessToken        string
	RefreshToken       string
	ExpiresIn          int64
	AinstructPAT       string
	AinstructPATExpiry int64
}

// --- Ainstruct PAT management ---

const ainstructPATRotationThreshold = 24 * time.Hour // rotate if expiring within 1 day

// flexTime handles timestamps with or without timezone info.
// The API returns timestamps like "2026-04-02T13:37:06.201389" (no TZ),
// while Go's default time.Time expects RFC3339 with timezone.
type flexTime struct {
	time.Time
}

func (ft *flexTime) UnmarshalJSON(data []byte) error {
	s := strings.Trim(string(data), "\"")
	if s == "null" || s == "" {
		return nil
	}
	// Try RFC3339 first (with TZ), then without TZ, then with microseconds
	formats := []string{
		"2006-01-02T15:04:05Z07:00",
		"2006-01-02T15:04:05.000000",
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05.000000",
		"2006-01-02 15:04:05",
	}
	for _, f := range formats {
		t, err := time.Parse(f, s)
		if err == nil {
			ft.Time = t
			return nil
		}
	}
	return fmt.Errorf("cannot parse time %q", s)
}

type patListItem struct {
	PatID     string    `json:"pat_id"`
	Label     string    `json:"label"`
	UserID    string    `json:"user_id"`
	Scopes    []string  `json:"scopes"`
	CreatedAt flexTime  `json:"created_at"`
	ExpiresAt *flexTime `json:"expires_at"`
	LastUsed  *flexTime `json:"last_used"`
}

type patListResponse struct {
	Tokens []patListItem `json:"tokens"`
}

type patResponse struct {
	PatID     string    `json:"pat_id"`
	Token     string    `json:"token"`
	Label     string    `json:"label"`
	UserID    string    `json:"user_id"`
	Scopes    []string  `json:"scopes"`
	CreatedAt flexTime  `json:"created_at"`
	ExpiresAt *flexTime `json:"expires_at"`
}

func listPATs(apiURL, accessToken string) ([]patListItem, error) {
	req, err := http.NewRequest("GET", apiURL+"/auth/pat", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := authHTTPClient.Do(req)
	if err != nil {
		utils.LogError("[login] PAT list request failed: %v\n", err)
		return nil, err
	}

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		utils.Log("[login] PAT list returned status %d: %s\n", resp.StatusCode, utils.Redact(string(body)))
		return nil, statusError("list PATs", resp.StatusCode)
	}

	var listResp patListResponse
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err := json.Unmarshal(body, &listResp); err != nil {
		return nil, err
	}

	return listResp.Tokens, nil
}

func ainstructPATRequest(accessToken, url string, body map[string]interface{}, action string, validStatus func(int) bool) (string, int64, error) {
	jsonBody, _ := json.Marshal(body)
	req, err := http.NewRequest("POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return "", 0, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := authHTTPClient.Do(req)
	if err != nil {
		utils.LogError("[login] Ainstruct PAT %s request failed: %v\n", action, err)
		return "", 0, err
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, _ := io.ReadAll(resp.Body)
	if !validStatus(resp.StatusCode) {
		utils.Log("[login] Ainstruct PAT %s returned status %d: %s\n", action, resp.StatusCode, utils.Redact(string(respBody)))
		return "", 0, statusError(action+" Ainstruct PAT", resp.StatusCode)
	}

	var result patResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", 0, err
	}

	expiry := int64(0)
	if result.ExpiresAt != nil {
		expiry = result.ExpiresAt.Unix()
	}
	utils.Log("[login] Ainstruct PAT %sd successfully (id=%s, expiry=%d)\n", action, result.PatID, expiry)
	return result.Token, expiry, nil
}

func rotateAinstructPAT(apiURL, accessToken, patID string) (string, int64, error) {
	return ainstructPATRequest(
		accessToken,
		apiURL+"/auth/pat/"+patID+"/rotate",
		map[string]interface{}{"expires_in_days": 7},
		"rotate",
		func(code int) bool { return code == 200 },
	)
}

func createAinstructPAT(apiURL, accessToken, label string) (string, int64, error) {
	return ainstructPATRequest(
		accessToken,
		apiURL+"/auth/pat",
		map[string]interface{}{"label": label, "expires_in_days": 7},
		"create",
		func(code int) bool { return code == 200 || code == 201 },
	)
}

func ensureAinstructPAT(apiURL, accessToken, label, storedToken, storedExpiry string) (string, int64, error) {
	utils.Log("[login] Ensuring Ainstruct PAT with label: %q\n", label)

	// Always check the server — local expiry is only used to decide
	// whether to rotate, not to skip the server call entirely.
	pats, err := listPATs(apiURL, accessToken)
	if err != nil {
		return "", 0, err
	}

	// If no stored token but we DO have a stored expiry, we might have
	// a stale cache. Clear the expiry to force a fresh decision.
	if storedToken == "" && storedExpiry != "" {
		storedExpiry = ""
	}

	for _, pat := range pats {
		if pat.Label == label {
			if storedToken != "" {
				if pat.ExpiresAt == nil || time.Until(pat.ExpiresAt.Time) <= ainstructPATRotationThreshold {
					utils.LogWarn("[login] Existing PAT expiring soon or no expiry, rotating\n")
				} else {
					expiry := pat.ExpiresAt.Unix()
					utils.Log("[login] Existing PAT still valid (expires %s), using stored token\n", pat.ExpiresAt.Format("2006-01-02"))
					return storedToken, expiry, nil
				}
				return rotateAinstructPAT(apiURL, accessToken, pat.PatID)
			} else {
				if pat.ExpiresAt != nil {
					utils.Log("[login] No stored token, rotating existing PAT (expires %s)\n", pat.ExpiresAt.Format("2006-01-02"))
				} else {
					utils.Log("[login] No stored token, rotating existing PAT (no expiry)\n")
				}

				return rotateAinstructPAT(apiURL, accessToken, pat.PatID)
			}
		}
	}

	utils.Log("[login] No existing PAT found, creating new one...\n")
	return createAinstructPAT(apiURL, accessToken, label)
}

// --- Interactive login ---

// runLoginInteractive prompts for username/password via TTY, authenticates
// with the Ainstruct API, fetches the user profile, and obtains an MCP PAT.
// SYNC tokens are always saved to encrypted storage for persistent sync.
func runLoginInteractive() (loginResult, error) {
	utils.Log("[kilo-docker] === Ainstruct Authentication ===\n", utils.WithOutput())
	utils.Log("[kilo-docker] Sign in at %s\n", constants.AinstructBaseURL, utils.WithOutput())
	utils.Log("[kilo-docker] Enables:\n", utils.WithOutput())
	utils.Log("[kilo-docker]   - File sync (push/pull config, commands, agents, instructions)\n", utils.WithOutput())
	utils.Log("[kilo-docker]   - MCP server tokens (stored encrypted in volume)\n", utils.WithOutput())

	result, err := loginWithRetries(promptCredentials, func(username, password string) (loginResult, error) {
		return authenticateAinstruct(constants.AinstructAPIBaseURL, username, password)
	})
	if err != nil {
		return loginResult{}, err
	}

	homeDir := "/home/" + deriveHomeName(result.UserID)
	_ = os.MkdirAll(homeDir, 0o700)

	var storedAinstruct, storedPatExpiry string
	encPath := filepath.Join(homeDir, ".local/share/kilo-docker/.tokens.env.enc")
	if encData, err := os.ReadFile(encPath); err == nil {
		if decrypted, decErr := decryptAES(encData, result.UserID); decErr == nil {
			_, storedAinstruct, _, _, _, storedPatExpiry, _ = parseTokenEnv(string(decrypted))
		}
	}

	ainstructPATLabel := buildAinstructPATLabel()
	ainstructPAT, ainstructPATExpiry, ainstructPATErr := ensureAinstructPAT(constants.AinstructAPIBaseURL, result.AccessToken, ainstructPATLabel, storedAinstruct, storedPatExpiry)
	if ainstructPATErr != nil {
		warnPATDisabled(ainstructPATErr)
	} else if ainstructPAT != "" {
		result.AinstructPAT = ainstructPAT
		result.AinstructPATExpiry = ainstructPATExpiry
	}

	syncExpiry := strconv.FormatInt(time.Now().Unix()+result.ExpiresIn, 10)
	if err := saveSyncTokensToEncrypted(homeDir, result.UserID, result.AccessToken, result.RefreshToken, syncExpiry); err != nil {
		utils.LogWarn("[login] Failed to save sync tokens: %v\n", err)
	} else {
		utils.Log("[login] Sync tokens saved to encrypted storage\n")
	}

	utils.Log("[kilo-docker] Signed in successfully.\n", utils.WithOutput())
	utils.Log("[kilo-docker] Tokens encrypted and stored in volume.\n", utils.WithOutput())

	return result, nil
}

// credentialsPrompt acquires a username and password for one attempt.
type credentialsPrompt func() (username, password string, err error)

// authenticateFunc performs one authentication attempt with the given
// credentials.
type authenticateFunc func(username, password string) (loginResult, error)

// loginWithRetries prompts for credentials up to maxLoginAttempts times. It
// re-prompts on invalid credentials (401) and transient network/5xx errors and
// aborts immediately on account-disabled (403) or prompt cancellation.
func loginWithRetries(prompt credentialsPrompt, auth authenticateFunc) (loginResult, error) {
	var lastErr error
	for attempt := 1; attempt <= maxLoginAttempts; attempt++ {
		utils.Log("[kilo-docker] Attempt %d of %d\n", attempt, maxLoginAttempts, utils.WithOutput())

		username, password, err := prompt()
		if err != nil {
			return loginResult{}, err
		}

		result, authErr := auth(username, password)
		if authErr == nil {
			return result, nil
		}

		var le *loginError
		if errors.As(authErr, &le) {
			switch le.Kind {
			case loginErrAccountDisabled:
				utils.LogError("[kilo-docker] Account disabled — cannot sign in.\n", utils.WithOutput())
				return loginResult{}, authErr
			case loginErrInvalidCredentials:
				utils.LogWarn("[kilo-docker] Invalid username or password. Please try again.\n", utils.WithOutput())
			default:
				utils.LogWarn("[kilo-docker] Login failed due to a temporary error. Please try again.\n", utils.WithOutput())
			}
			lastErr = authErr
			continue
		}

		return loginResult{}, authErr
	}

	return loginResult{}, fmt.Errorf("login failed after %d attempts: %w", maxLoginAttempts, lastErr)
}

// authenticateAinstruct performs a single login request followed by a profile
// fetch, returning the resulting tokens. Failures are classified as
// loginError so the retry loop can decide whether to re-prompt.
func authenticateAinstruct(apiBaseURL, username, password string) (loginResult, error) {
	var result loginResult

	loginBody, _ := json.Marshal(map[string]string{
		"username": username,
		"password": password,
	})

	resp, err := authHTTPClient.Post(apiBaseURL+"/auth/login", "application/json", bytes.NewReader(loginBody))
	if err != nil {
		return result, &loginError{Kind: loginErrTransient, Err: fmt.Errorf("connection failed: %w", err)}
	}

	loginRespBytes, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusUnauthorized:
		return result, &loginError{Kind: loginErrInvalidCredentials, Err: fmt.Errorf("invalid credentials")}
	case http.StatusForbidden:
		return result, &loginError{Kind: loginErrAccountDisabled, Err: fmt.Errorf("account disabled")}
	default:
		return result, &loginError{Kind: loginErrTransient, Err: fmt.Errorf("login failed with status %d", resp.StatusCode)}
	}

	var loginResp loginResponse
	if err := json.Unmarshal(loginRespBytes, &loginResp); err != nil {
		return result, &loginError{Kind: loginErrTransient, Err: fmt.Errorf("failed to parse login response: %w", err)}
	}
	if loginResp.AccessToken == "" {
		return result, &loginError{Kind: loginErrTransient, Err: fmt.Errorf("login succeeded but no access_token in response")}
	}

	profileReq, err := http.NewRequest("GET", apiBaseURL+"/auth/profile", nil)
	if err != nil {
		return result, &loginError{Kind: loginErrTransient, Err: fmt.Errorf("connection failed: %w", err)}
	}
	profileReq.Header.Set("Authorization", "Bearer "+loginResp.AccessToken)
	profileResp, err := authHTTPClient.Do(profileReq)
	if err != nil {
		return result, &loginError{Kind: loginErrTransient, Err: fmt.Errorf("connection failed: %w", err)}
	}

	profileBody, _ := io.ReadAll(profileResp.Body)
	_ = profileResp.Body.Close()

	switch profileResp.StatusCode {
	case http.StatusOK:
	case http.StatusUnauthorized:
		return result, &loginError{Kind: loginErrInvalidCredentials, Err: fmt.Errorf("profile fetch returned status %d", profileResp.StatusCode)}
	case http.StatusForbidden:
		return result, &loginError{Kind: loginErrAccountDisabled, Err: fmt.Errorf("account disabled")}
	default:
		return result, &loginError{Kind: loginErrTransient, Err: fmt.Errorf("profile fetch failed with status %d", profileResp.StatusCode)}
	}

	var profileRespParsed profileResponse
	if err := json.Unmarshal(profileBody, &profileRespParsed); err != nil {
		return result, &loginError{Kind: loginErrTransient, Err: fmt.Errorf("failed to parse profile response: %w", err)}
	}
	if profileRespParsed.UserID == "" {
		return result, &loginError{Kind: loginErrTransient, Err: fmt.Errorf("profile fetched but no user_id in response")}
	}

	result.UserID = profileRespParsed.UserID
	result.AccessToken = loginResp.AccessToken
	result.RefreshToken = loginResp.RefreshToken
	result.ExpiresIn = loginResp.ExpiresIn
	return result, nil
}

// storedTokenSet is the decrypted token material read from the volume.
type storedTokenSet struct {
	context7     string
	ainstruct    string
	accessToken  string
	refreshToken string
	syncExpiry   int64
	patExpiry    int64
}

// loadUsableSyncTokens loads stored tokens and returns usable sync credentials.
// When the access token is missing or expired it performs a single silent
// refresh using the refresh token. Returns ok=false when no usable credentials
// can be produced (forcing the interactive prompt).
func loadUsableSyncTokens(homeDir, userID, apiBaseURL string) (storedTokenSet, bool) {
	var set storedTokenSet
	if homeDir == "" || userID == "" {
		return set, false
	}

	context7, ainstruct, syncToken, syncRefresh, syncExpiry, patExpiry, err := loadEncryptedTokens(homeDir, userID)
	if err != nil {
		utils.Log("[login] Cannot load stored tokens: %v\n", err)
		return set, false
	}

	set.context7 = context7
	set.ainstruct = ainstruct
	set.accessToken = syncToken
	set.refreshToken = syncRefresh
	set.syncExpiry, _ = strconv.ParseInt(strings.TrimSpace(syncExpiry), 10, 64)
	set.patExpiry, _ = strconv.ParseInt(strings.TrimSpace(patExpiry), 10, 64)

	if set.accessToken == "" && set.refreshToken == "" {
		utils.Log("[login] No stored sync tokens to reuse\n")
		return set, false
	}

	if set.accessToken == "" || set.syncExpiry <= time.Now().Unix() {
		if set.refreshToken == "" {
			utils.Log("[login] Stored sync token expired and no refresh token\n")
			return set, false
		}
		refreshed, refreshErr := refreshSyncTokens(apiBaseURL, set.refreshToken)
		if refreshErr != nil {
			utils.LogWarn("[login] Silent token refresh failed: %v\n", refreshErr)
			return set, false
		}
		set.accessToken = refreshed.AccessToken
		if refreshed.RefreshToken != "" {
			set.refreshToken = refreshed.RefreshToken
		}
		if refreshed.ExpiresIn > 0 {
			set.syncExpiry = time.Now().Unix() + refreshed.ExpiresIn
		}
		utils.Log("[login] Refreshed stored sync token silently\n")
	}

	return set, true
}

// refreshSyncTokens performs a single silent sync-token refresh.
func refreshSyncTokens(apiBaseURL, refreshToken string) (loginResponse, error) {
	var result loginResponse

	reqBody, _ := json.Marshal(map[string]string{"refresh_token": refreshToken})
	resp, err := authHTTPClient.Post(apiBaseURL+"/auth/refresh", "application/json", bytes.NewReader(reqBody))
	if err != nil {
		return result, fmt.Errorf("token refresh request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return result, statusError("token refresh", resp.StatusCode)
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return result, fmt.Errorf("token refresh decode failed: %w", err)
	}
	if result.AccessToken == "" {
		return result, fmt.Errorf("token refresh returned empty access token")
	}
	return result, nil
}

// resolveCredentials attempts to reuse stored credentials without prompting.
// It reuses an unexpired sync access token (silently refreshing an expired one
// when a refresh token exists) and then validates the credentials with the
// first live call (list PATs). Returns ok=false when the caller must prompt:
// no usable tokens, a failed silent refresh, or a 401/403 from the live call.
func resolveCredentials(homeDir, userID, apiBaseURL string) (loginResult, bool) {
	set, ok := loadUsableSyncTokens(homeDir, userID, apiBaseURL)
	if !ok {
		return loginResult{}, false
	}

	label := buildAinstructPATLabel()
	pat, patExpiry, patErr := ensureAinstructPAT(apiBaseURL, set.accessToken, label, set.ainstruct, formatExpiry(set.patExpiry))
	if patErr != nil {
		if isAuthRejection(patErr) {
			utils.LogWarn("[login] Stored credentials rejected by server, prompting for login\n")
			return loginResult{}, false
		}
		warnPATDisabled(patErr)
		pat, patExpiry = set.ainstruct, set.patExpiry
	}

	remaining := set.syncExpiry - time.Now().Unix()
	if remaining < 0 {
		remaining = 0
	}

	return loginResult{
		UserID:             userID,
		AccessToken:        set.accessToken,
		RefreshToken:       set.refreshToken,
		ExpiresIn:          remaining,
		AinstructPAT:       pat,
		AinstructPATExpiry: patExpiry,
	}, true
}

func promptCredentials() (string, string, error) {
	username, err := promptUsername()
	if err != nil {
		return "", "", err
	}
	password, err := promptPassword()
	if err != nil {
		return "", "", err
	}
	return username, password, nil
}

func promptUsername() (string, error) {
	for {
		utils.Log("[kilo-docker] Ainstruct username: ", utils.WithOutput())
		var username string
		_, err := fmt.Scanln(&username)
		if errors.Is(err, io.EOF) {
			return "", errLoginCancelled
		}
		username = strings.TrimSpace(username)
		if username != "" {
			return username, nil
		}
		utils.LogWarn("[login] Username cannot be empty.\n")
	}
}

func promptPassword() (string, error) {
	for {
		utils.Log("[kilo-docker] Ainstruct password: ", utils.WithOutput())
		password, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Println()
		if err != nil {
			return "", errLoginCancelled
		}
		if len(password) == 0 {
			utils.LogWarn("[login] Password cannot be empty.\n")
			continue
		}
		if len(password) < 4 {
			utils.LogWarn("[login] Password must be at least 4 characters.\n")
			continue
		}
		return string(password), nil
	}
}

func buildAinstructPATLabel() string {
	username := os.Getenv("KD_USERNAME")
	hostname := os.Getenv("KD_HOSTNAME")
	if username == "" {
		username = "unknown"
	}
	if hostname == "" {
		hostname = "unknown"
	}
	return fmt.Sprintf("kilo-docker | %s@%s", username, hostname)
}
