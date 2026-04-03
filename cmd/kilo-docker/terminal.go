package main

import (
	"os"
	"os/exec"
)

// resetTerminal resets the terminal to a sane state after a docker exec
// session with zellij.
func resetTerminal() {
	stty := exec.Command("/bin/sh", "-c", "stty sane </dev/tty 2>/dev/null || true")
	stty.Run()

	reset := exec.Command("/bin/sh", "-c", "reset 2>/dev/null || true")
	reset.Stdin = os.Stdin
	reset.Stdout = os.Stdout
	reset.Stderr = os.Stderr
	reset.Run()
}
