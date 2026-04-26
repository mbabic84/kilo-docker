// Package constants defines shared fixed paths and service endpoints.
package constants

import (
	"os"
	"path/filepath"
)

// KiloHome is the fallback home directory when $HOME is unset.
// Actual container home is dynamically generated as /home/kd-<hash>.
const KiloHome = "/home/kd-default"

const (
	AinstructBaseURL    = "https://ainstruct-dev.kralicinora.cz"
	AinstructAPIBaseURL = AinstructBaseURL + "/api/v1"
)

func GetHomeDir() string {
	home := os.Getenv("HOME")
	if home == "" {
		return KiloHome
	}
	return home
}

func GetKiloConfigDir() string {
	return filepath.Join(GetHomeDir(), ".config", "kilo")
}
