package utils

import (
	"fmt"
	"os"
)

// Log prints a message to stderr with [kilo-docker] prefix.
func Log(format string, args ...interface{}) {
	if len(args) > 0 {
		fmt.Fprintf(os.Stderr, "[kilo-docker] "+format, args...)
	} else {
		fmt.Fprintf(os.Stderr, "[kilo-docker] %s", format)
	}
}

// LogError prints an error message to stderr with [kilo-docker] prefix.
func LogError(format string, args ...interface{}) {
	if len(args) > 0 {
		fmt.Fprintf(os.Stderr, "[kilo-docker] Error: "+format, args...)
	} else {
		fmt.Fprintf(os.Stderr, "[kilo-docker] Error: %s", format)
	}
}

// LogWarn prints a warning message to stderr with [kilo-docker] prefix.
func LogWarn(format string, args ...interface{}) {
	if len(args) > 0 {
		fmt.Fprintf(os.Stderr, "[kilo-docker] Warning: "+format, args...)
	} else {
		fmt.Fprintf(os.Stderr, "[kilo-docker] Warning: %s", format)
	}
}