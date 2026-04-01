package constants

import "os"

const KiloHome = "/home/kilo-t8x3m7kp"

func GetHomeDir() string {
	home := os.Getenv("HOME")
	if home == "" {
		return KiloHome
	}
	return home
}
