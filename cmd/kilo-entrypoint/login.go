package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"time"

	"github.com/mbabic84/kilo-docker/pkg/constants"
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
	UserID       string
	AccessToken  string
	RefreshToken string
	ExpiresIn    int64
	MCPToken     string
}

// --- PAT management ---

const patRotationThreshold = 7 * 24 * time.Hour // rotate if expiring within 7 days

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
	fmt.Fprintf(os.Stderr, "[kilo-docker] Listing PATs: %s/auth/pat\n", apiURL)
	req, err := http.NewRequest("GET", apiURL+"/auth/pat", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[kilo-docker] PAT list request failed: %v\n", err)
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		fmt.Fprintf(os.Stderr, "[kilo-docker] PAT list returned status %d: %s\n", resp.StatusCode, string(body))
		return nil, fmt.Errorf("list PATs failed with status %d", resp.StatusCode)
	}

	var listResp patListResponse
	body, _ := io.ReadAll(resp.Body)
	if err := json.Unmarshal(body, &listResp); err != nil {
		return nil, err
	}

	return listResp.Tokens, nil
}

func ensurePAT(apiURL, accessToken, label, storedToken string) (string, error) {
	fmt.Fprintf(os.Stderr, "[kilo-docker] Ensuring PAT with label: %q\n", label)
	pats, err := listPATs(apiURL, accessToken)
	if err != nil {
		return "", err
	}

	for _, pat := range pats {
		if pat.Label == label {
			// PAT exists.  If we have a stored token and the PAT is not
			// expired or expiring soon, return the stored value — no
			// rotation needed.
			if storedToken != "" {
				if pat.ExpiresAt == nil || time.Until(pat.ExpiresAt.Time) <= patRotationThreshold {
					fmt.Fprintf(os.Stderr, "[kilo-docker] Existing PAT expiring soon or no expiry, rotating\n")
				} else {
					fmt.Fprintf(os.Stderr, "[kilo-docker] Existing PAT still valid (expires %s), using stored token\n", pat.ExpiresAt.Time.Format("2006-01-02"))
					return storedToken, nil
				}
			} else {
				// No stored token — must rotate to obtain the value.
				if pat.ExpiresAt != nil {
					fmt.Fprintf(os.Stderr, "[kilo-docker] No stored token, rotating existing PAT (expires %s)\n", pat.ExpiresAt.Time.Format("2006-01-02"))
				} else {
					fmt.Fprintf(os.Stderr, "[kilo-docker] No stored token, rotating existing PAT (no expiry)\n")
				}
			}
			rotateReq := map[string]interface{}{
				"expires_in_days": 30,
			}
			rotateJSON, _ := json.Marshal(rotateReq)
			req, err := http.NewRequest("POST", apiURL+"/auth/pat/"+pat.PatID+"/rotate", bytes.NewReader(rotateJSON))
			if err != nil {
				return "", err
			}
			req.Header.Set("Authorization", "Bearer "+accessToken)
			req.Header.Set("Content-Type", "application/json")

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				fmt.Fprintf(os.Stderr, "[kilo-docker] PAT rotate request failed: %v\n", err)
				return "", err
			}
			defer resp.Body.Close()

			if resp.StatusCode != 200 {
				body, _ := io.ReadAll(resp.Body)
				fmt.Fprintf(os.Stderr, "[kilo-docker] PAT rotate returned status %d: %s\n", resp.StatusCode, string(body))
				return "", fmt.Errorf("rotate PAT failed with status %d", resp.StatusCode)
			}

			var rotateResp patResponse
			body, _ := io.ReadAll(resp.Body)
			if err := json.Unmarshal(body, &rotateResp); err != nil {
				return "", err
			}
			fmt.Fprintf(os.Stderr, "[kilo-docker] PAT rotated successfully (id=%s)\n", rotateResp.PatID)
			return rotateResp.Token, nil
		}
	}

	fmt.Fprintf(os.Stderr, "[kilo-docker] No existing PAT found, creating new one...\n")
	createReq := map[string]interface{}{
		"label":           label,
		"expires_in_days": 30,
	}
	createJSON, _ := json.Marshal(createReq)
	req, err := http.NewRequest("POST", apiURL+"/auth/pat", bytes.NewReader(createJSON))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[kilo-docker] PAT create request failed: %v\n", err)
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 201 && resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		fmt.Fprintf(os.Stderr, "[kilo-docker] PAT create returned status %d: %s\n", resp.StatusCode, string(body))
		return "", fmt.Errorf("create PAT failed with status %d", resp.StatusCode)
	}

	var createResp patResponse
	body, _ := io.ReadAll(resp.Body)
	if err := json.Unmarshal(body, &createResp); err != nil {
		return "", err
	}
	fmt.Fprintf(os.Stderr, "[kilo-docker] PAT created successfully (id=%s)\n", createResp.PatID)
	return createResp.Token, nil
}

// --- Interactive login ---

// runLoginInteractive prompts for username/password via TTY, authenticates
// with the Ainstruct API, fetches the user profile, and obtains an MCP PAT.
func runLoginInteractive() (loginResult, error) {
	var result loginResult

	fmt.Fprintf(os.Stderr, "\n=== Ainstruct Authentication ===\n")
	fmt.Fprintf(os.Stderr, "Sign in at %s\n\n", constants.AinstructBaseURL)
	fmt.Fprintf(os.Stderr, "Enables:\n")
	fmt.Fprintf(os.Stderr, "  - File sync (push/pull config, commands, agents, instructions)\n")
	fmt.Fprintf(os.Stderr, "  - MCP server tokens (stored encrypted in volume)\n\n")

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
	defer resp.Body.Close()

	loginRespBytes, _ := io.ReadAll(resp.Body)

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
	defer profileResp.Body.Close()

	if profileResp.StatusCode != 200 {
		return result, fmt.Errorf("profile fetch failed with status %d", profileResp.StatusCode)
	}

	profileBody, _ := io.ReadAll(profileResp.Body)
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

	// Load stored encrypted ainstruct token so ensurePAT can skip
	// rotation when the existing PAT is still valid.
	var storedAinstruct string
	homeDir := "/home/" + deriveHomeName(result.UserID)
	encPath := filepath.Join(homeDir, ".local/share/kilo/.tokens.env.enc")
	if encData, err := os.ReadFile(encPath); err == nil {
		if decrypted, decErr := decryptAES(encData, result.UserID); decErr == nil {
			_, storedAinstruct, _, _, _, _ = parseTokenEnv(string(decrypted))
		}
	}

	patLabel := buildPATLabel()
	patToken, patErr := ensurePAT(constants.AinstructAPIBaseURL, loginResp.AccessToken, patLabel, storedAinstruct)
	if patErr != nil {
		fmt.Fprintf(os.Stderr, "\nWarning: Failed to create Ainstruct MCP token: %v\n", patErr)
		fmt.Fprintf(os.Stderr, "Ainstruct MCP server will be disabled.\n")
	} else if patToken != "" {
		result.MCPToken = patToken
	}

	fmt.Fprintf(os.Stderr, "\nSigned in successfully.\n")
	fmt.Fprintf(os.Stderr, "Tokens encrypted and stored in volume.\n\n")

	return result, nil
}

func promptUsername() string {
	for {
		fmt.Fprint(os.Stderr, "Ainstruct username: ")
		var username string
		fmt.Scanln(&username)
		username = strings.TrimSpace(username)
		if username != "" {
			return username
		}
		fmt.Fprintf(os.Stderr, "Username cannot be empty.\n")
	}
}

func promptPassword() string {
	for {
		fmt.Fprint(os.Stderr, "Ainstruct password: ")
		password, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Println()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: Failed to read password.\n")
			os.Exit(1)
		}
		if len(password) == 0 {
			fmt.Fprintf(os.Stderr, "Error: Password cannot be empty.\n")
			continue
		}
		if len(password) < 4 {
			fmt.Fprintf(os.Stderr, "Password must be at least 4 characters.\n")
			continue
		}
		return string(password)
	}
}

// promptContext7Token prompts for a Context7 API token via TTY.
// Empty input is accepted (token is optional — Context7 MCP will be disabled).
func promptContext7Token() string {
	fmt.Fprint(os.Stderr, "Context7 API key (leave empty to skip): ")
	var token string
	fmt.Scanln(&token)
	return strings.TrimSpace(token)
}

func buildPATLabel() string {
	u, _ := user.Current()
	hostname, _ := os.Hostname()
	username := "unknown"
	if u != nil {
		username = u.Username
	}
	if hostname == "" {
		hostname = "unknown"
	}
	return fmt.Sprintf("kilo-docker | %s@%s", username, hostname)
}

// --- Env-var-based login (for ainstruct-login subcommand) ---

// runAinstructLogin authenticates with the Ainstruct API using credentials
// from environment variables (USERNAME, PASSWORD, API_URL). It performs a
// two-step flow: POST /auth/login to get tokens, then GET /auth/profile to
// get the user_id. Results are written to stdout as KEY=VALUE lines.
func runAinstructLogin() error {
	username := os.Getenv("USERNAME")
	password := os.Getenv("PASSWORD")
	apiURL := os.Getenv("API_URL")
	patLabel := os.Getenv("PAT_LABEL")

	if username == "" || password == "" || apiURL == "" {
		return fmt.Errorf("missing required env vars: USERNAME, PASSWORD, API_URL")
	}

	loginBody := map[string]string{
		"username": username,
		"password": password,
	}
	loginJSON, _ := json.Marshal(loginBody)

	resp, err := http.Post(apiURL+"/auth/login", "application/json", bytes.NewReader(loginJSON))
	if err != nil {
		return fmt.Errorf("connection failed")
	}
	defer resp.Body.Close()

	loginResp, _ := io.ReadAll(resp.Body)

	switch resp.StatusCode {
	case 200:
		var loginResult struct {
			AccessToken  string `json:"access_token"`
			RefreshToken string `json:"refresh_token"`
			ExpiresIn    int64  `json:"expires_in"`
		}
		if err := json.Unmarshal(loginResp, &loginResult); err != nil {
			return fmt.Errorf("failed to parse login response: %w", err)
		}
		if loginResult.AccessToken == "" {
			return fmt.Errorf("login succeeded but no access_token in response")
		}

		req, _ := http.NewRequest("GET", apiURL+"/auth/profile", nil)
		req.Header.Set("Authorization", "Bearer "+loginResult.AccessToken)
		profileResp, err := http.DefaultClient.Do(req)
		if err != nil {
			return fmt.Errorf("connection failed")
		}
		defer profileResp.Body.Close()

		if profileResp.StatusCode != 200 {
			return fmt.Errorf("profile fetch failed with status %d", profileResp.StatusCode)
		}

		var profileResult struct {
			UserID string `json:"user_id"`
		}
		profileBody, _ := io.ReadAll(profileResp.Body)
		if err := json.Unmarshal(profileBody, &profileResult); err != nil {
			return fmt.Errorf("failed to parse profile response: %w", err)
		}
		if profileResult.UserID == "" {
			return fmt.Errorf("profile fetched but no user_id in response")
		}

		fmt.Printf("STATUS=success\n")
		fmt.Printf("USER_ID=%s\n", profileResult.UserID)
		fmt.Printf("ACCESS_TOKEN=%s\n", loginResult.AccessToken)
		fmt.Printf("REFRESH_TOKEN=%s\n", loginResult.RefreshToken)
		if loginResult.ExpiresIn > 0 {
			fmt.Printf("EXPIRES_IN=%d\n", loginResult.ExpiresIn)
		}

		if patLabel != "" {
			mcpToken, err := ensurePAT(apiURL, loginResult.AccessToken, patLabel, "")
			if err == nil && mcpToken != "" {
				fmt.Printf("MCP_TOKEN=%s\n", mcpToken)
			}
		}

		return nil
	case 401:
		return fmt.Errorf("invalid credentials")
	case 403:
		return fmt.Errorf("account disabled")
	case 0:
		return fmt.Errorf("connection failed")
	default:
		var errResp struct {
			Detail struct {
				Message string `json:"message"`
			} `json:"detail"`
		}
		json.Unmarshal(loginResp, &errResp)
		msg := errResp.Detail.Message
		if msg == "" {
			msg = "unknown error"
		}
		return fmt.Errorf("login failed with status %d: %s", resp.StatusCode, msg)
	}
}
