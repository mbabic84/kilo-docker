package main

import (
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/mbabic84/kilo-docker/pkg/utils"
	"golang.org/x/term"
)

// defaultGiteaHost is offered when no Gitea host has been stored yet.
const defaultGiteaHost = "https://gitea.com"

// runMCPTokens interactively prompts for MCP tokens and saves them to
// encrypted storage. Gitea host/token/TLS prompts are shown only when the
// gitea service is enabled with --gitea.
func runMCPTokens() error {
	homeDir, _, _, userID := loadUserConfig()
	if homeDir == "" || userID == "" {
		return fmt.Errorf("no user config found")
	}

	store, loadErr := loadTokenStore(homeDir, userID)
	if loadErr != nil {
		if !os.IsNotExist(loadErr) {
			return fmt.Errorf("cannot read token store: %w", loadErr)
		}
		store = TokenStore{}
	}

	// An explicit env var wins over the stored Ainstruct token (legacy behavior).
	if ainstructToken := os.Getenv("KD_MCP_AINSTRUCT_TOKEN"); ainstructToken != "" {
		store.Set(tokenKeyAinstruct, ainstructToken)
	}

	utils.Log("[kilo-docker] MCP Token Management\n", utils.WithOutput())
	utils.Log("[kilo-docker] ====================\n", utils.WithOutput())
	utils.Log("[kilo-docker] \n", utils.WithOutput())

	context7Token := store.Get(tokenKeyContext7)

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
	store.Set(tokenKeyContext7, context7Token)

	if isGiteaServiceEnabled() {
		promptGiteaSettings(store)
	}

	if err := saveTokenStore(homeDir, userID, store); err != nil {
		utils.LogWarn("[userinit] failed to save tokens: %v\n", err)
	}

	utils.Log("[kilo-docker] MCP tokens saved\n", utils.WithOutput())
	return nil
}

// giteaInputs is the resolved starting state for the Gitea prompts.
// *FromEnv records whether an environment variable supplied the value, in
// which case the corresponding prompt is skipped.
type giteaInputs struct {
	Host            string
	Token           string
	Insecure        bool
	HostFromEnv     bool
	TokenFromEnv    bool
	InsecureFromEnv bool
}

// resolveGiteaInputs merges stored values with environment overrides.
func resolveGiteaInputs(store TokenStore) giteaInputs {
	inputs := giteaInputs{
		Host:     store.Get(tokenKeyGiteaHost),
		Token:    store.Get(tokenKeyGiteaToken),
		Insecure: store.Get(tokenKeyGiteaInsecure) == "true",
	}
	if v := strings.TrimSpace(os.Getenv("KD_GITEA_HOST")); v != "" {
		inputs.Host = v
		inputs.HostFromEnv = true
	}
	if v := strings.TrimSpace(os.Getenv("KD_MCP_GITEA_TOKEN")); v != "" {
		inputs.Token = v
		inputs.TokenFromEnv = true
	}
	if v, ok := os.LookupEnv("KD_GITEA_INSECURE"); ok && strings.TrimSpace(v) != "" {
		inputs.Insecure = isTruthy(v)
		inputs.InsecureFromEnv = true
	}
	return inputs
}

// promptGiteaSettings prompts for host, token, and TLS verification and writes
// the results into store. Values supplied through the environment skip their
// prompt.
func promptGiteaSettings(store TokenStore) {
	inputs := resolveGiteaInputs(store)

	host := inputs.Host
	if !inputs.HostFromEnv {
		host = promptGiteaHost(host)
	}
	store.Set(tokenKeyGiteaHost, host)

	token := inputs.Token
	if !inputs.TokenFromEnv {
		token = promptGiteaToken(token)
	}
	store.Set(tokenKeyGiteaToken, token)

	insecure := inputs.Insecure
	if !inputs.InsecureFromEnv {
		insecure = promptGiteaInsecure()
	}
	if insecure {
		store.Set(tokenKeyGiteaInsecure, "true")
	} else {
		store.Set(tokenKeyGiteaInsecure, "")
	}

	if token == "" {
		utils.Log("[kilo-docker] No Gitea token set; the Gitea MCP server stays disabled until one is configured\n", utils.WithOutput())
	} else {
		utils.Log("[kilo-docker] Tip: run `tea login add --name kilo` once to use the tea CLI with these credentials\n", utils.WithOutput())
	}
}

func promptGiteaHost(current string) string {
	display := current
	if display == "" {
		display = defaultGiteaHost
	}

	utils.Log("[kilo-docker] \n", utils.WithOutput())
	utils.Log("[kilo-docker] Gitea host: %s\n", display, utils.WithOutput())
	utils.Log("[kilo-docker]   - Press Enter to keep current\n", utils.WithOutput())
	utils.Log("[kilo-docker]   - Type a new URL to update\n", utils.WithOutput())
	utils.Log("[kilo-docker]   - Type 'clear' to unset\n", utils.WithOutput())

	for {
		utils.Log("[kilo-docker] > ", utils.WithOutput())
		var input string
		_, _ = fmt.Scanln(&input)
		input = strings.TrimSpace(input)

		switch input {
		case "":
			return display
		case "clear":
			return ""
		}

		normalized, err := normalizeGiteaHost(input)
		if err != nil {
			utils.Log("[kilo-docker] Invalid host: %v\n", err, utils.WithOutput())
			continue
		}
		return normalized
	}
}

func promptGiteaToken(current string) string {
	currentDisplay := "[not set]"
	if current != "" {
		currentDisplay = maskToken(current)
	}

	utils.Log("[kilo-docker] \n", utils.WithOutput())
	utils.Log("[kilo-docker] Gitea access token: %s\n", currentDisplay, utils.WithOutput())
	utils.Log("[kilo-docker]   - Press Enter to keep current\n", utils.WithOutput())
	utils.Log("[kilo-docker]   - Type a new token to update\n", utils.WithOutput())
	utils.Log("[kilo-docker]   - Type 'clear' to disable\n", utils.WithOutput())
	utils.Log("[kilo-docker] > ", utils.WithOutput())

	input := strings.TrimSpace(readSecretInput())

	switch input {
	case "":
		return current
	case "clear":
		return ""
	default:
		return input
	}
}

// readSecretInput reads a secret without echoing it when stdin is a terminal.
// Piped input still works through the plain line reader.
func readSecretInput() string {
	if term.IsTerminal(int(os.Stdin.Fd())) {
		secret, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Println()
		if err == nil {
			return string(secret)
		}
	}
	var input string
	_, _ = fmt.Scanln(&input)
	return input
}

func promptGiteaInsecure() bool {
	utils.Log("[kilo-docker] Disable TLS verification for Gitea? [y/N]: ", utils.WithOutput())
	var input string
	_, _ = fmt.Scanln(&input)
	return isTruthy(input)
}

// normalizeGiteaHost validates and normalizes a Gitea base URL. The scheme must
// be http or https, a host must be present, and trailing slashes are removed.
func normalizeGiteaHost(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", fmt.Errorf("host is empty")
	}

	normalized := strings.TrimRight(trimmed, "/")
	parsed, err := url.Parse(normalized)
	if err != nil {
		return "", fmt.Errorf("invalid URL: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("URL must start with http:// or https://")
	}
	if parsed.Host == "" {
		return "", fmt.Errorf("URL must include a host")
	}
	if parsed.User != nil {
		return "", fmt.Errorf("URL must not include credentials")
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", fmt.Errorf("URL must not include a query or fragment")
	}
	return normalized, nil
}

func isTruthy(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "y", "on":
		return true
	}
	return false
}

// maskToken truncates and masks a token string for safe logging.
// Returns "***" for tokens shorter than 8 chars.
func maskToken(token string) string {
	if len(token) <= 8 {
		return strings.Repeat("*", len(token))
	}
	return token[:4] + "****" + token[len(token)-4:]
}
