package main

import (
	"bytes"
	"encoding/json"
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
	UserID            string
	AccessToken       string
	RefreshToken      string
	ExpiresIn         int64
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

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		utils.LogError("[login] PAT list request failed: %v\n", err)
		return nil, err
	}

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		utils.Log("[login] PAT list returned status %d: %s\n", resp.StatusCode, utils.Redact(string(body)))
		return nil, fmt.Errorf("list PATs failed with status %d", resp.StatusCode)
	}

	var listResp patListResponse
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err := json.Unmarshal(body, &listResp); err != nil {
		return nil, err
	}

	return listResp.Tokens, nil
}

func rotatePAT(apiURL, accessToken, patID string) (string, int64, error) {
	rotateReq := map[string]interface{}{
		"expires_in_days": 7,
	}
	rotateJSON, _ := json.Marshal(rotateReq)
	req, err := http.NewRequest("POST", apiURL+"/auth/pat/"+patID+"/rotate", bytes.NewReader(rotateJSON))
	if err != nil {
		return "", 0, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		utils.LogError("[login] PAT rotate request failed: %v\n", err)
		return "", 0, err
	}

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		utils.Log("[login] PAT rotate returned status %d: %s\n", resp.StatusCode, utils.Redact(string(body)))
		return "", 0, fmt.Errorf("rotate PAT failed with status %d", resp.StatusCode)
	}

	var rotateResp patResponse
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err := json.Unmarshal(body, &rotateResp); err != nil {
		return "", 0, err
	}

	expiry := int64(0)
	if rotateResp.ExpiresAt != nil {
		expiry = rotateResp.ExpiresAt.Unix()
	}
	utils.Log("[login] PAT rotated successfully (id=%s, expiry=%d)\n", rotateResp.PatID, expiry)
	return rotateResp.Token, expiry, nil
}

func createPAT(apiURL, accessToken, label string) (string, int64, error) {
	createReq := map[string]interface{}{
		"label":           label,
		"expires_in_days": 7,
	}
	createJSON, _ := json.Marshal(createReq)
	req, err := http.NewRequest("POST", apiURL+"/auth/pat", bytes.NewReader(createJSON))
	if err != nil {
		return "", 0, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		utils.LogError("[login] PAT create request failed: %v\n", err)
		return "", 0, err
	}

	if resp.StatusCode != 201 && resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		utils.Log("[login] PAT create returned status %d: %s\n", resp.StatusCode, utils.Redact(string(body)))
		return "", 0, fmt.Errorf("create PAT failed with status %d", resp.StatusCode)
	}

	var createResp patResponse
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err := json.Unmarshal(body, &createResp); err != nil {
		return "", 0, err
	}

	expiry := int64(0)
	if createResp.ExpiresAt != nil {
		expiry = createResp.ExpiresAt.Unix()
	}
	utils.Log("[login] PAT created successfully (id=%s, expiry=%d)\n", createResp.PatID, expiry)
	return createResp.Token, expiry, nil
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
			return rotatePAT(apiURL, accessToken, pat.PatID)
			} else {
				if pat.ExpiresAt != nil {
					utils.Log("[login] No stored token, rotating existing PAT (expires %s)\n", pat.ExpiresAt.Format("2006-01-02"))
				} else {
utils.Log("[login] No stored token, rotating existing PAT (no expiry)\n")
				}

				return rotatePAT(apiURL, accessToken, pat.PatID)
			}
		}
	}

utils.Log("[login] No existing PAT found, creating new one...\n")
	return createPAT(apiURL, accessToken, label)
}

// --- Interactive login ---

// runLoginInteractive prompts for username/password via TTY, authenticates
// with the Ainstruct API, fetches the user profile, and obtains an MCP PAT.
// SYNC tokens are always saved to encrypted storage for persistent sync.
func runLoginInteractive() (loginResult, error) {
	var result loginResult

	utils.Log("[kilo-docker] === Ainstruct Authentication ===\n", utils.WithOutput())
	utils.Log("[kilo-docker] Sign in at %s\n", constants.AinstructBaseURL, utils.WithOutput())
	utils.Log("[kilo-docker] Enables:\n", utils.WithOutput())
	utils.Log("[kilo-docker]   - File sync (push/pull config, commands, agents, instructions)\n", utils.WithOutput())
	utils.Log("[kilo-docker]   - MCP server tokens (stored encrypted in volume)\n", utils.WithOutput())

	username := promptUsername()
	password := promptPassword()

	apiURL := constants.AinstructAPIBaseURL + "/auth/login"
	loginBody := map[string]string{
		"username": username,
		"password": password,
	}
	loginJSON, _ := json.Marshal(loginBody)

	resp, err := http.Post(apiURL, "application/json", bytes.NewReader(loginJSON))
	if err != nil {
		return result, fmt.Errorf("connection failed")
	}

	loginRespBytes, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()

	if resp.StatusCode != 200 {
		if resp.StatusCode == 401 {
			return result, fmt.Errorf("invalid credentials")
		}
		if resp.StatusCode == 403 {
			return result, fmt.Errorf("account disabled")
		}
		return result, fmt.Errorf("login failed with status %d", resp.StatusCode)
	}

	var loginResp loginResponse
	if err := json.Unmarshal(loginRespBytes, &loginResp); err != nil {
		return result, fmt.Errorf("failed to parse login response: %w", err)
	}
	if loginResp.AccessToken == "" {
		return result, fmt.Errorf("login succeeded but no access_token in response")
	}

	profileReq, err := http.NewRequest("GET", constants.AinstructAPIBaseURL+"/auth/profile", nil)
	if err != nil {
		return result, fmt.Errorf("connection failed")
	}
	profileReq.Header.Set("Authorization", "Bearer "+loginResp.AccessToken)
	profileResp, err := http.DefaultClient.Do(profileReq)
	if err != nil {
		return result, fmt.Errorf("connection failed")
	}

	profileBody, _ := io.ReadAll(profileResp.Body)
	_ = profileResp.Body.Close()

	if profileResp.StatusCode != 200 {
		return result, fmt.Errorf("profile fetch failed with status %d", profileResp.StatusCode)
	}
	var profileRespParsed profileResponse
	if err := json.Unmarshal(profileBody, &profileRespParsed); err != nil {
		return result, fmt.Errorf("failed to parse profile response: %w", err)
	}
	if profileRespParsed.UserID == "" {
		return result, fmt.Errorf("profile fetched but no user_id in response")
	}

	result.UserID = profileRespParsed.UserID
	result.AccessToken = loginResp.AccessToken
	result.RefreshToken = loginResp.RefreshToken
	result.ExpiresIn = loginResp.ExpiresIn

	homeDir := "/home/" + deriveHomeName(result.UserID)
	_ = os.MkdirAll(homeDir, 0o700)

	var storedAinstruct, storedPatExpiry string
	encPath := filepath.Join(homeDir, ".local/share/kilo/.tokens.env.enc")
	if encData, err := os.ReadFile(encPath); err == nil {
		if decrypted, decErr := decryptAES(encData, result.UserID); decErr == nil {
			_, storedAinstruct, _, _, _, storedPatExpiry, _ = parseTokenEnv(string(decrypted))
		}
	}

	ainstructPATLabel := buildAinstructPATLabel()
	ainstructPAT, ainstructPATExpiry, ainstructPATErr := ensureAinstructPAT(constants.AinstructAPIBaseURL, loginResp.AccessToken, ainstructPATLabel, storedAinstruct, storedPatExpiry)
	if ainstructPATErr != nil {
		utils.LogWarn("[login] Warning: Failed to create Ainstruct PAT: %v\n", ainstructPATErr)
		utils.Log("[kilo-docker] Ainstruct MCP server will be disabled.\n", utils.WithOutput())
	} else if ainstructPAT != "" {
		result.AinstructPAT = ainstructPAT
		result.AinstructPATExpiry = ainstructPATExpiry
	}

	if loginResp.AccessToken != "" {
		syncExpiry := strconv.FormatInt(time.Now().Unix()+loginResp.ExpiresIn, 10)
		if err := saveSyncTokensToEncrypted(homeDir, result.UserID, loginResp.AccessToken, loginResp.RefreshToken, syncExpiry); err != nil {
			utils.LogWarn("[login] Failed to save sync tokens: %v\n", err)
		} else {
			utils.Log("[login] Sync tokens saved to encrypted storage\n")
		}
	}

	utils.Log("[kilo-docker] Signed in successfully.\n", utils.WithOutput())
	utils.Log("[kilo-docker] Tokens encrypted and stored in volume.\n", utils.WithOutput())

	return result, nil
}

func promptUsername() string {
	for {
		utils.Log("[kilo-docker] Ainstruct username: ", utils.WithOutput())
		var username string
		_, _ = fmt.Scanln(&username)
		username = strings.TrimSpace(username)
		if username != "" {
			return username
		}
		utils.LogWarn("[login] Username cannot be empty.\n")
	}
}

func promptPassword() string {
	for {
		utils.Log("[kilo-docker] Ainstruct password: ", utils.WithOutput())
		password, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Println()
		if err != nil {
			utils.LogError("[login] Failed to read password.\n")
			os.Exit(1)
		}
		if len(password) == 0 {
			utils.LogWarn("[login] Password cannot be empty.\n")
			continue
		}
		if len(password) < 4 {
			utils.LogWarn("[login] Password must be at least 4 characters.\n")
			continue
		}
		return string(password)
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




