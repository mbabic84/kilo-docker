package main

import (
	"fmt"
	"os"
	"strings"

	"golang.org/x/term"
	"github.com/kilo-org/kilo-docker/pkg/utils"
)

// loginResult holds the authentication tokens returned by Ainstruct login.
type loginResult struct {
	UserID       string
	AccessToken  string
	RefreshToken string
	ExpiresIn    int64
}

// ainstructLogin runs the full Ainstruct authentication flow on the host side.
// It prompts for username/password (via TTY), calls the container's ainstruct-login
// subcommand via docker run, and returns the parsed login result containing
// user_id, tokens, and expiry.
func ainstructLogin(image string) (loginResult, error) {
	var result loginResult

	fmt.Fprintf(os.Stderr, "\n=== Ainstruct Authentication ===\n")
	fmt.Fprintf(os.Stderr, "Sign in with your Ainstruct account to enable:\n")
	fmt.Fprintf(os.Stderr, "  - Encrypted volume (derived from your user_id)\n")
	fmt.Fprintf(os.Stderr, "  - File sync (push/pull config, commands, agents)\n")
	fmt.Fprintf(os.Stderr, "  - MCP server tokens (stored encrypted in volume)\n\n")

	username := promptUsername()
	password := promptPassword()

	apiURL := "https://ainstruct-dev.kralicinora.cz/api/v1"

	output, err := dockerRun(
		"-e", "USERNAME="+username,
		"-e", "PASSWORD="+password,
		"-e", "API_URL="+apiURL,
		image,
		"ainstruct-login",
	)
	if err != nil {
		return result, err
	}

	parsed := utils.ParseKeyValueOutput(output)

	if parsed["STATUS"] != "success" {
		errMsg := parsed["ERROR"]
		if errMsg == "" {
			errMsg = "unknown error"
		}
		return result, fmt.Errorf("%s", errMsg)
	}

	result.UserID = parsed["USER_ID"]
	result.AccessToken = parsed["ACCESS_TOKEN"]
	result.RefreshToken = parsed["REFRESH_TOKEN"]
	if v := parsed["EXPIRES_IN"]; v != "" {
		result.ExpiresIn = utils.ParseInt64(v)
	}

	fmt.Fprintf(os.Stderr, "\nSigned in successfully.\n")
	fmt.Fprintf(os.Stderr, "Volume derived from user_id, tokens will be stored encrypted.\n\n")

	return result, nil
}

// promptUsername reads the Ainstruct account username from stdin, retrying if empty.
func promptUsername() string {
	for {
		fmt.Fprintf(os.Stderr, "Ainstruct username: ")
		var username string
		fmt.Scanln(&username)
		username = strings.TrimSpace(username)
		if username != "" {
			return username
		}
		fmt.Fprintf(os.Stderr, "Username cannot be empty.\n")
	}
}

// promptPassword reads the Ainstruct account password from the terminal without echoing.
func promptPassword() string {
	for {
		fmt.Fprintf(os.Stderr, "Ainstruct password: ")
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

// promptConfirm displays a yes/no prompt and returns true if the user enters "y".
// Returns true automatically when --yes flag is set or stdin is not a TTY.
func promptConfirm(message string) bool {
	if autoConfirm {
		fmt.Fprintf(os.Stderr, "%sy\n", message)
		return true
	}
	fmt.Print(message)
	var response string
	fmt.Scanln(&response)
	return strings.ToLower(strings.TrimSpace(response)) == "y"
}

// promptToken reads an API token from the terminal without echoing.
func promptToken(label string) string {
	fmt.Printf("%s: ", label)
	token, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Println()
	if err != nil {
		return ""
	}
	return string(token)
}

// readPassword reads a password from the terminal without echoing.
func readPassword() string {
	password, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Println()
	if err != nil {
		return ""
	}
	return string(password)
}

// confirmPassword reads a password from the terminal and verifies it matches
// the existing password.
func confirmPassword(existing string) bool {
	fmt.Print("Confirm: ")
	confirm, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Println()
	if err != nil {
		return false
	}
	return string(confirm) == existing
}
