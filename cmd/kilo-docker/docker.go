package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
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
// Returns trimmed combined output. Runs as root — privilege drop is handled
// by kilo-entrypoint.
func dockerExec(container string, args ...string) (string, error) {
	execArgs := []string{"exec", container}
	execArgs = append(execArgs, args...)
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

// getContainerLabel retrieves a label value from a container.
func getContainerLabel(container, label string) string {
	val, _ := dockerInspect(container, "{{index .Config.Labels \""+label+"\"}}")
	return strings.TrimSpace(val)
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

// execDockerInteractive replaces the current process with a docker exec
// command for interactive sessions (e.g. attaching to zellij). It uses
// -it for interactive TTY and runs as root — privilege drop is handled
// by kilo-entrypoint.
func execDockerInteractive(container string, args ...string) error {
	execArgs := []string{"exec", "-it", container}
	execArgs = append(execArgs, args...)
	cmd := exec.Command("docker", execArgs...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
