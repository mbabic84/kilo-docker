package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
)

// runAinstructLogin authenticates with the Ainstruct API using credentials
// from environment variables (USERNAME, PASSWORD, API_URL). It performs a
// two-step flow: POST /auth/login to get tokens, then GET /auth/profile to
// get the user_id. Results are written to stdout as KEY=VALUE lines:
//
//	STATUS=success
//	USER_ID=...
//	ACCESS_TOKEN=...
//	REFRESH_TOKEN=...
//	EXPIRES_IN=...
//
// On error, it outputs STATUS=error\nERROR=... and exits with code 1.
// HTTP status codes are mapped to specific error messages:
//   - 200: success
//   - 401: invalid credentials
//   - 403: account disabled
//   - 0: connection failed
func runAinstructLogin() error {
	username := os.Getenv("USERNAME")
	password := os.Getenv("PASSWORD")
	apiURL := os.Getenv("API_URL")

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

// parseLoginOutput parses KEY=VALUE formatted lines from the ainstruct-login
// subcommand into a map for structured access.
func parseLoginOutput(output string) map[string]string {
	result := make(map[string]string)
	for _, line := range bytes.Split([]byte(output), []byte{'\n'}) {
		if len(line) == 0 {
			continue
		}
		parts := bytes.SplitN(line, []byte{'='}, 2)
		if len(parts) == 2 {
			result[string(parts[0])] = string(parts[1])
		}
	}
	return result
}

// parseInt64 parses a string as a 64-bit integer, returning 0 on error.
func parseInt64(s string) int64 {
	v, _ := strconv.ParseInt(s, 10, 64)
	return v
}
