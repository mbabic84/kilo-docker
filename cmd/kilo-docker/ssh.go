package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// setupSSH detects or starts an SSH agent for key forwarding. Returns
// (authSock, agentRunning, agentStartedByUs). If SSH_AUTH_SOCK is already
// set and points to a valid socket, it reuses it. Otherwise, it starts a new
// ssh-agent, discovers private keys in ~/.ssh, and adds them.
func setupSSH() (string, bool, bool) {
	sshDir := os.Getenv("HOME") + "/.ssh"

	sshAuthSock := os.Getenv("SSH_AUTH_SOCK")
	if sshAuthSock != "" {
		if info, err := os.Stat(sshAuthSock); err == nil {
			if info.Mode()&os.ModeSocket != 0 {
				fmt.Fprintf(os.Stderr, "[kilo-docker] Reusing existing SSH agent: %s\n", sshAuthSock)
				loadSSHKeys(sshDir)
				return sshAuthSock, true, false
			}
		}
		fmt.Fprintf(os.Stderr, "[kilo-docker] Warning: SSH_AUTH_SOCK=%s is not a valid socket\n", sshAuthSock)
	}

	output, err := exec.Command("ssh-agent", "-s").CombinedOutput()
	if err != nil {
		fmt.Fprintf(os.Stderr, "[kilo-docker] Warning: failed to start ssh-agent: %v\n", err)
		return "", false, false
	}

	var newSock, newPid string
	for _, segment := range strings.Split(string(output), ";") {
		segment = strings.TrimSpace(segment)
		segment = strings.TrimPrefix(segment, "export ")
		if strings.HasPrefix(segment, "SSH_AUTH_SOCK=") {
			parts := strings.SplitN(segment, "=", 2)
			if len(parts) == 2 {
				newSock = strings.TrimSpace(parts[1])
			}
		} else if strings.HasPrefix(segment, "SSH_AGENT_PID=") {
			parts := strings.SplitN(segment, "=", 2)
			if len(parts) == 2 {
				newPid = strings.TrimSpace(parts[1])
			}
		}
	}

	if newSock != "" {
		os.Setenv("SSH_AUTH_SOCK", newSock)
		os.Setenv("SSH_AGENT_PID", newPid)
		fmt.Fprintf(os.Stderr, "[kilo-docker] SSH agent started (pid=%s)\n", newPid)
		loadSSHKeys(sshDir)
		return newSock, true, true
	}

	return "", false, false
}

// loadSSHKeys reads private keys from the given ssh directory and adds them
// to the running SSH agent. It skips directories and .pub files, then checks
// file content for "PRIVATE KEY" to avoid loading non-key files like config
// or known_hosts.
func loadSSHKeys(sshDir string) {
	entries, err := os.ReadDir(sshDir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if entry.IsDir() || strings.HasSuffix(entry.Name(), ".pub") {
			continue
		}
		path := sshDir + "/" + entry.Name()
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		if strings.Contains(string(data), "PRIVATE KEY") {
			if out, err := exec.Command("ssh-add", path).CombinedOutput(); err != nil {
				fmt.Fprintf(os.Stderr, "[kilo-docker] Warning: failed to add key %s: %v\n", entry.Name(), strings.TrimSpace(string(out)))
			} else {
				fmt.Fprintf(os.Stderr, "[kilo-docker] Added SSH key: %s\n", entry.Name())
			}
		}
	}
}

// cleanupSSH kills a previously started ssh-agent. It uses ssh-agent -k to
// ensure the socket file is properly removed.
func cleanupSSH(pid string) {
	if pid != "" {
		exec.Command("ssh-agent", "-k").Run()
	}
}
