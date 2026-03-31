package main

import (
	"os"
	"os/exec"
)

// resetTerminal resets the terminal to a sane state after a docker attach
// session. It clears any pending input, runs stty sane + stty echo, and
// sends an ANSI terminal reset sequence.
func resetTerminal() {
	tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return
	}
	defer tty.Close()

	tty.Write([]byte{0})

	exec.Command("stty", "sane").Run()
	exec.Command("stty", "echo").Run()

	tty.Write([]byte{0x1b, 'c'})
}
