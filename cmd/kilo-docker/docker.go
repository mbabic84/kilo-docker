package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
)

// dockerSubcommands lists recognized docker CLI subcommands. If the first
// argument to dockerRun/dockerRunWithStdin is not in this set, "run --rm" is
// automatically prepended so callers can pass container flags directly.
var dockerSubcommands = map[string]bool{
	"attach": true, "build": true, "commit": true, "cp": true,
	"create": true, "diff": true, "events": true, "exec": true,
	"export": true, "history": true, "images": true, "import": true,
	"info": true, "inspect": true, "kill": true, "load": true,
	"login": true, "logout": true, "logs": true, "network": true,
	"pause": true, "port": true, "ps": true, "pull": true,
	"push": true, "rename": true, "restart": true, "rm": true,
	"rmi": true, "run": true, "save": true, "search": true,
	"start": true, "stats": true, "stop": true, "tag": true,
	"top": true, "unpause": true, "update": true, "version": true,
	"volume": true, "wait": true,
}

// ensureRunArgs prepends "run --rm" when the first argument is not a
// recognized docker subcommand (e.g. caller passed "-e", "-v", etc.
// directly). This makes dockerRun safe to call with just container flags.
func ensureRunArgs(args []string) []string {
	if len(args) == 0 || dockerSubcommands[args[0]] {
		return args
	}
	return append([]string{"run", "--rm"}, args...)
}

// dockerRun executes a docker command, capturing combined output.
// Returns trimmed stdout and an error if the command fails.
// If the first argument is not a recognized docker subcommand, "run --rm"
// is automatically prepended.
func dockerRun(args ...string) (string, error) {
	args = ensureRunArgs(args)
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
// If the first argument is not a recognized docker subcommand, "run --rm"
// is automatically prepended. The -i flag is always added so Docker keeps
// stdin open and the input data reaches the container process.
func dockerRunWithStdin(input string, args ...string) (string, error) {
	args = ensureRunArgs(args)
	// Insert -i after "run" (or "run --rm") so Docker attaches stdin.
	// Without -i, docker run ignores stdin entirely and the container
	// process receives empty input regardless of cmd.Stdin.
	args = append(args[:2], append([]string{"-i"}, args[2:]...)...)
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

// execDockerInteractive replaces the current process with a docker exec
// command for interactive sessions (e.g. attaching to zellij). It uses
// -it for interactive TTY and does not use SysProcAttr since docker exec
// doesn't use TTY-detach semantics.
func execDockerInteractive(container string, user string, args ...string) error {
	execArgs := []string{"exec", "-it"}
	if user != "" {
		execArgs = append(execArgs, "--user", user)
	}
	execArgs = append(execArgs, container)
	execArgs = append(execArgs, args...)
	cmd := exec.Command("docker", execArgs...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
