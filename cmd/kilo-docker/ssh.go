package main

import (
	"os"
	"os/exec"
	"strings"
)

// setupSSH detects or starts an SSH agent for key forwarding. Returns
// (authSock, agentRunning, agentStartedByUs). If SSH_AUTH_SOCK is already
// set and points to a valid socket, it reuses it. Otherwise, it starts a new
// ssh-agent, discovers private keys in ~/.ssh, and adds them.
func setupSSH() (string, bool, bool) {
	sshAuthSock := os.Getenv("SSH_AUTH_SOCK")
	if sshAuthSock != "" {
		if _, err := os.Stat(sshAuthSock); err == nil {
			if info, _ := os.Stat(sshAuthSock); info.Mode()&os.ModeSocket != 0 {
				return sshAuthSock, true, false
			}
		}
	}

	output, err := exec.Command("ssh-agent", "-s").CombinedOutput()
	if err != nil {
		return "", false, false
	}

	lines := strings.Split(string(output), "\n")
	var newSock, newPid string
	for _, line := range lines {
		if strings.HasPrefix(line, "SSH_AUTH_SOCK=") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				newSock = strings.TrimSuffix(parts[1], ";")
			}
		}
		if strings.HasPrefix(line, "SSH_AGENT_PID=") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				newPid = strings.TrimSuffix(parts[1], ";")
			}
		}
	}

	if newSock != "" {
		os.Setenv("SSH_AUTH_SOCK", newSock)
		os.Setenv("SSH_AGENT_PID", newPid)

		sshDir := os.Getenv("HOME") + "/.ssh"
		if entries, err := os.ReadDir(sshDir); err == nil {
			for _, entry := range entries {
				if entry.IsDir() {
					continue
				}
				if strings.HasSuffix(entry.Name(), ".pub") {
					continue
				}
				data, _ := os.ReadFile(sshDir + "/" + entry.Name())
				if strings.Contains(string(data), "PRIVATE KEY") {
					exec.Command("ssh-add", sshDir+"/"+entry.Name()).Run()
				}
			}
		}

		return newSock, true, true
	}

	return "", false, false
}

// cleanupSSH kills a previously started ssh-agent by its PID.
func cleanupSSH(pid string) {
	if pid != "" {
		exec.Command("kill", pid).Run()
	}
}
