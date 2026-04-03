package constants

import (
	"os"
	"path/filepath"
)

const KiloHome = "/home/kilo-t8x3m7kp"

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
