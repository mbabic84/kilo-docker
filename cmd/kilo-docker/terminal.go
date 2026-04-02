package main

import (
	"fmt"
	"os"
	"os/exec"
)

// resetTerminal resets the terminal to a sane state after a docker attach
// session.
func resetTerminal() {
	// First, try stty sane to fix the line settings - this might be enough
	// Run in a subshell to handle the raw terminal state
	stty := exec.Command("/bin/sh", "-c", "stty sane </dev/tty 2>/dev/null || true")
	stty.Run()
	
	// Then try reset
	reset := exec.Command("/bin/sh", "-c", "reset 2>/dev/null || true")
	reset.Stdin = os.Stdin
	reset.Stdout = os.Stdout
	reset.Stderr = os.Stderr
	reset.Run()
	
	// Fallback escape sequences
	fmt.Fprint(os.Stderr, "\033c\033[2J\033[H")
}
