package main

import (
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCleanupStaleSocketPathRemovesDirectory(t *testing.T) {
	// Docker creates a directory when the bind mount source doesn't exist.
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "agent.sock")

	if err := os.Mkdir(socketPath, 0755); err != nil {
		t.Fatalf("failed to create test directory: %v", err)
	}

	cleanupStaleSocketPath(socketPath)

	if _, err := os.Stat(socketPath); !os.IsNotExist(err) {
		t.Errorf("expected directory at %s to be removed, but it still exists", socketPath)
	}
}

func TestCleanupStaleSocketPathNoopWhenPathMissing(t *testing.T) {
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "agent.sock")

	// Path doesn't exist — should be a no-op.
	cleanupStaleSocketPath(socketPath)

	if _, err := os.Stat(socketPath); !os.IsNotExist(err) {
		t.Errorf("expected path %s to remain non-existent", socketPath)
	}
}

func TestCleanupStaleSocketPathSkipsRegularFile(t *testing.T) {
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "agent.sock")

	if err := os.WriteFile(socketPath, []byte("not a socket"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	cleanupStaleSocketPath(socketPath)

	// Regular files should NOT be removed — only directories and stale sockets.
	if _, err := os.Stat(socketPath); err != nil {
		t.Errorf("expected regular file at %s to be preserved, got error: %v", socketPath, err)
	}
}

func TestCleanupStaleSocketPathRemovesStaleSocket(t *testing.T) {
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "agent.sock")

	// Create a Unix socket with no listener.
	ln, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatalf("failed to create test socket: %v", err)
	}
	// Close the listener immediately — socket file remains but has no listener.
	_ = ln.Close()

	cleanupStaleSocketPath(socketPath)

	// The stale socket should be removed since ssh-add -l will fail
	// (no agent is listening).
	if _, err := os.Stat(socketPath); !os.IsNotExist(err) {
		t.Errorf("expected stale socket at %s to be removed, but it still exists", socketPath)
	}
}

func TestCleanupStaleSocketPathPreservesActiveSocket(t *testing.T) {
	// Start a real ssh-agent to get an active socket.
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "agent.sock")

	authSock, agentPid, started := startTestSSHAgent(t, socketPath)
	if !started {
		t.Skip("ssh-agent not available, skipping active socket test")
	}
	defer stopTestSSHAgent(t, agentPid)

	// Verify the socket is active.
	if authSock == "" || authSock != socketPath {
		t.Fatalf("expected SSH_AUTH_SOCK=%s, got %s", socketPath, authSock)
	}

	cleanupStaleSocketPath(socketPath)

	// Active socket should be preserved.
	if _, err := os.Stat(socketPath); err != nil {
		t.Errorf("expected active socket at %s to be preserved, got error: %v", socketPath, err)
	}
}

func TestSetupSSHRemovesStaleDirectory(t *testing.T) {
	if _, err := lookPathSSHAgent(); err != nil {
		t.Skip("ssh-agent not available, skipping setupSSH test")
	}

	tmpDir := t.TempDir()
	sshDir := filepath.Join(tmpDir, ".ssh")
	socketDir := filepath.Join(sshDir, "kilo")
	socketPath := filepath.Join(socketDir, "agent.sock")

	// Create the directory structure with agent.sock as a directory
	// (simulating what Docker does when the socket doesn't exist).
	if err := os.MkdirAll(socketDir, 0700); err != nil {
		t.Fatalf("failed to create socket dir: %v", err)
	}
	if err := os.Mkdir(socketPath, 0755); err != nil {
		t.Fatalf("failed to create agent.sock directory: %v", err)
	}

	// Override HOME so setupSSH finds our test paths.
	origHome := os.Getenv("HOME")
	_ = os.Setenv("HOME", tmpDir)
	defer func() { _ = os.Setenv("HOME", origHome) }()

	// Clear SSH_AUTH_SOCK so setupSSH doesn't try to reuse a host agent.
	origAuthSock := os.Getenv("SSH_AUTH_SOCK")
	_ = os.Unsetenv("SSH_AUTH_SOCK")
	defer func() { _ = os.Setenv("SSH_AUTH_SOCK", origAuthSock) }()

	sock, _, started := setupSSH()
	if !started {
		t.Fatal("setupSSH failed to start ssh-agent")
	}
	defer cleanupSSH(os.Getenv("SSH_AGENT_PID"))

	// The returned path should be a socket, not a directory.
	info, err := os.Stat(sock)
	if err != nil {
		t.Fatalf("socket path %s does not exist: %v", sock, err)
	}
	if info.IsDir() {
		t.Errorf("expected %s to be a socket, but it is a directory", sock)
	}
	if info.Mode()&os.ModeSocket == 0 {
		t.Errorf("expected %s to be a socket, got mode %v", sock, info.Mode())
	}
}

func TestSetupSSHCreatesSocketWhenPathMissing(t *testing.T) {
	if _, err := lookPathSSHAgent(); err != nil {
		t.Skip("ssh-agent not available, skipping setupSSH test")
	}

	tmpDir := t.TempDir()
	sshDir := filepath.Join(tmpDir, ".ssh")

	// Only create the .ssh directory, not the kilo subdirectory.
	if err := os.MkdirAll(sshDir, 0700); err != nil {
		t.Fatalf("failed to create ssh dir: %v", err)
	}

	origHome := os.Getenv("HOME")
	_ = os.Setenv("HOME", tmpDir)
	defer func() { _ = os.Setenv("HOME", origHome) }()

	origAuthSock := os.Getenv("SSH_AUTH_SOCK")
	_ = os.Unsetenv("SSH_AUTH_SOCK")
	defer func() { _ = os.Setenv("SSH_AUTH_SOCK", origAuthSock) }()

	sock, _, started := setupSSH()
	if !started {
		t.Fatal("setupSSH failed to start ssh-agent")
	}
	defer cleanupSSH(os.Getenv("SSH_AGENT_PID"))

	info, err := os.Stat(sock)
	if err != nil {
		t.Fatalf("socket path %s does not exist: %v", sock, err)
	}
	if info.IsDir() {
		t.Errorf("expected %s to be a socket, but it is a directory", sock)
	}
	if info.Mode()&os.ModeSocket == 0 {
		t.Errorf("expected %s to be a socket, got mode %v", sock, info.Mode())
	}
}

func TestSetupSSHReusesActiveSocket(t *testing.T) {
	if _, err := lookPathSSHAgent(); err != nil {
		t.Skip("ssh-agent not available, skipping setupSSH test")
	}

	tmpDir := t.TempDir()
	socketDir := filepath.Join(tmpDir, ".ssh", "kilo")
	socketPath := filepath.Join(socketDir, "agent.sock")

	// Pre-create an ssh-agent at the expected path.
	_, agentPid, ok := startTestSSHAgent(t, socketPath)
	if !ok {
		t.Fatal("failed to start test ssh-agent")
	}
	defer stopTestSSHAgent(t, agentPid)

	origHome := os.Getenv("HOME")
	_ = os.Setenv("HOME", tmpDir)
	defer func() { _ = os.Setenv("HOME", origHome) }()

	origAuthSock := os.Getenv("SSH_AUTH_SOCK")
	_ = os.Unsetenv("SSH_AUTH_SOCK")
	defer func() { _ = os.Setenv("SSH_AUTH_SOCK", origAuthSock) }()

	// setupSSH should detect the active socket and reuse it.
	sock, running, startedByUs := setupSSH()
	if !running {
		t.Fatal("expected running=true for active socket")
	}
	if startedByUs {
		t.Error("expected startedByUs=false when reusing existing agent")
	}
	if sock != socketPath {
		t.Errorf("expected sock=%s, got %s", socketPath, sock)
	}
}

// --- helpers ---

// startTestSSHAgent starts an ssh-agent with a fixed socket path for testing.
// Returns (socketPath, pid, true) on success.
func startTestSSHAgent(t *testing.T, socketPath string) (string, string, bool) {
	t.Helper()
	// Create parent directory if needed.
	if err := os.MkdirAll(filepath.Dir(socketPath), 0700); err != nil {
		t.Fatalf("failed to create socket dir: %v", err)
	}
	output, err := exec.Command("ssh-agent", "-a", socketPath).CombinedOutput()
	if err != nil {
		t.Logf("ssh-agent failed: %v, output: %s", err, output)
		return "", "", false
	}
	// Parse SSH_AGENT_PID from output.
	var pid string
	for _, seg := range strings.Split(string(output), ";") {
		seg = strings.TrimSpace(seg)
		if strings.HasPrefix(seg, "SSH_AGENT_PID=") {
			pid = strings.TrimPrefix(seg, "SSH_AGENT_PID=")
			break
		}
	}
	return socketPath, pid, true
}

func stopTestSSHAgent(t *testing.T, pid string) {
	t.Helper()
	if pid != "" {
		_ = os.Setenv("SSH_AGENT_PID", pid)
		_ = exec.Command("ssh-agent", "-k").Run()
		_ = os.Unsetenv("SSH_AGENT_PID")
	}
}

func lookPathSSHAgent() (string, error) {
	return exec.LookPath("ssh-agent")
}
