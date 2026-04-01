package main

import (
	"io"
	"os"
	"path/filepath"

	"github.com/mbabic84/kilo-docker/pkg/constants"
)

// runLoadTokens reads the token environment file from the volume and writes
// its contents to stdout. Returns nil (with no output) if the file doesn't
// exist, allowing the host to detect empty tokens.
func runLoadTokens() error {
	home := constants.GetHomeDir()
	tokenFile := filepath.Join(home, ".local", "share", "kilo", ".tokens.env")
	data, err := os.ReadFile(tokenFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	os.Stdout.Write(data)
	return nil
}

// runSaveTokens reads KEY=VALUE pairs from stdin and writes them to the
// token file with mode 0600. Creates the directory structure if needed.
// runSaveTokens reads KEY=VALUE pairs from stdin and writes them to the
// token file with mode 0600. Creates the directory structure if needed.
func runSaveTokens() error {
	home := constants.GetHomeDir()
	tokenDir := filepath.Join(home, ".local", "share", "kilo")
	tokenFile := filepath.Join(tokenDir, ".tokens.env")
	if err := os.MkdirAll(tokenDir, 0700); err != nil {
		return err
	}
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		return err
	}
	if err := os.WriteFile(tokenFile, data, 0600); err != nil {
		return err
	}
	return nil
}
