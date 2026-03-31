package main

import (
	"fmt"
	"strings"
)

// loadTokens reads API tokens from the Docker volume. If encrypted is true,
// it reads the encrypted file and decrypts it on the host (crypto never runs
// inside the container). Returns (context7Token, ainstructToken).
func loadTokens(image, volume string, encrypted bool, password string) (string, string) {
	const kiloHome = "/home/kilo-t8x3m7kp"
	volumeMount := volume + ":" + kiloHome

	if encrypted {
		output, err := dockerRun(
			"-v", volumeMount,
			image,
			"cat", kiloHome+"/.local/share/kilo/.tokens.env.enc",
		)
		if err != nil || output == "" {
			return "", ""
		}
		decrypted, err := decryptAES([]byte(output), password)
		if err != nil {
			return "", ""
		}
		return parseTokenEnv(string(decrypted))
	}

	output, err := dockerRun(
		"-v", volumeMount,
		image,
		"load-tokens",
	)
	if err != nil || output == "" {
		return "", ""
	}
	return parseTokenEnv(output)
}

// saveTokens writes API tokens to the Docker volume. If encrypted is true,
// the tokens are AES-256-CBC encrypted on the host before writing.
func saveTokens(image, volume string, token1, token2 string, encrypted bool, password string) error {
	const kiloHome = "/home/kilo-t8x3m7kp"
	volumeMount := volume + ":" + kiloHome
	tokenData := fmt.Sprintf("KD_CONTEXT7_TOKEN=%s\nKD_AINSTRUCT_TOKEN=%s\n", token1, token2)

	if encrypted {
		encData, err := encryptAES([]byte(tokenData), password)
		if err != nil {
			return err
		}
		encPath := kiloHome + "/.local/share/kilo/.tokens.env.enc"
		_, err = dockerRunWithStdin(string(encData),
			"-v", volumeMount,
			image,
			"sh", "-c", fmt.Sprintf("mkdir -p \"$(dirname '%s')\" && cat > '%s' && chmod 600 '%s'", encPath, encPath, encPath),
		)
		return err
	}

	_, err := dockerRunWithStdin(tokenData,
		"-v", volumeMount,
		image,
		"save-tokens",
	)
	return err
}

// parseTokenEnv extracts KD_CONTEXT7_TOKEN and KD_AINSTRUCT_TOKEN values
// from KEY=VALUE formatted string data.
func parseTokenEnv(data string) (string, string) {
	var token1, token2 string
	for _, line := range strings.Split(data, "\n") {
		if strings.HasPrefix(line, "KD_CONTEXT7_TOKEN=") {
			token1 = strings.TrimPrefix(line, "KD_CONTEXT7_TOKEN=")
		} else if strings.HasPrefix(line, "KD_AINSTRUCT_TOKEN=") {
			token2 = strings.TrimPrefix(line, "KD_AINSTRUCT_TOKEN=")
		}
	}
	return token1, token2
}
