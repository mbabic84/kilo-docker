package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mbabic84/kilo-docker/pkg/utils"
)

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

	if encData, err := os.ReadFile(filepath.Join(homeDir, ".local/share/kilo-docker/.tokens.env.enc")); err == nil {
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

// maskToken truncates and masks a token string for safe logging.
// Returns "***" for tokens shorter than 8 chars.
func maskToken(token string) string {
	if len(token) <= 8 {
		return strings.Repeat("*", len(token))
	}
	return token[:4] + strings.Repeat("*", len(token)-8) + token[len(token)-4:]
}
