package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
)

// dockerRun executes a docker command, capturing combined output.
// Returns trimmed stdout and an error if the command fails.
func dockerRun(args ...string) (string, error) {
	cmd := exec.Command("docker", args...)
	cmd.Stdin = os.Stdin
	output, err := cmd.CombinedOutput()
	if err != nil {
		return strings.TrimSpace(string(output)), fmt.Errorf("docker %s: %w\n%s", strings.Join(args, " "), err, string(output))
	}
	return strings.TrimSpace(string(output)), nil
}

// dockerRunWithStdin executes a docker command with the given input piped to
// its stdin. Returns trimmed combined output and an error if the command fails.
func dockerRunWithStdin(input string, args ...string) (string, error) {
	cmd := exec.Command("docker", args...)
	cmd.Stdin = strings.NewReader(input)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return strings.TrimSpace(string(output)), fmt.Errorf("docker %s: %w\n%s", strings.Join(args, " "), err, string(output))
	}
	return strings.TrimSpace(string(output)), nil
}

// dockerRunDetached executes a detached docker command (e.g. docker run -d).
// Returns the container ID on success.
func dockerRunDetached(args ...string) (string, error) {
	cmd := exec.Command("docker", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("docker %s: %w\n%s", strings.Join(args, " "), err, string(output))
	}
	return strings.TrimSpace(string(output)), nil
}

// dockerExec executes a command inside a running container via docker exec.
// Returns trimmed combined output.
func dockerExec(container string, args ...string) (string, error) {
	execArgs := append([]string{"exec", container}, args...)
	cmd := exec.Command("docker", execArgs...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return strings.TrimSpace(string(output)), fmt.Errorf("docker exec %s %s: %w\n%s", container, strings.Join(args, " "), err, string(output))
	}
	return strings.TrimSpace(string(output)), nil
}

// dockerInspect returns the output of `docker inspect -f format container`.
func dockerInspect(container, format string) (string, error) {
	return dockerRun("inspect", "-f", format, container)
}

// dockerState returns the container status string (e.g. "running", "exited")
// or "not_found" if the container doesn't exist.
func dockerState(container string) string {
	state, _ := dockerInspect(container, "{{.State.Status}}")
	return state
}

// containerExists reports whether a container with the given name exists.
func containerExists(container string) bool {
	_, err := dockerRun("inspect", container)
	return err == nil
}

// execDocker replaces the current process with a docker command, inheriting
// stdin, stdout, and stderr from the calling process. Used for interactive
// docker run sessions.
func execDocker(args ...string) error {
	cmd := exec.Command("docker", args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// execDockerAttach replaces the current process with a docker attach/start
// command, inheriting stdin, stdout, and stderr. The SysProcAttr ensures
// proper signal handling for TTY-detach (Ctrl+P Ctrl+Q).
func execDockerAttach(args ...string) error {
	cmd := exec.Command("docker", args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{}
	return cmd.Run()
}
