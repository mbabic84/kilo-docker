package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// backup creates a gzipped tar archive of the volume by running a detached
// container, tar inside it, and docker cp to extract the archive to the host.
func backup(image, volume, home, outputFile string) error {
	container := fmt.Sprintf("kilo-backup-temp-%d", os.Getpid())

	_, err := dockerRunDetached("run", "-d", "--name", container,
		"-v", volume+":"+home+":ro", image, "tail", "-f", "/dev/null")
	if err != nil {
		return err
	}
	defer exec.Command("docker", "rm", "-f", container).Run()

	time.Sleep(500 * time.Millisecond)

	_, err = dockerExec(container, "", "tar", "czf", "/tmp/backup.tar.gz", "-C", home, ".")
	if err != nil {
		return err
	}

	_, err = dockerRun("cp", container+":/tmp/backup.tar.gz", outputFile)
	return err
}

// restore extracts a tar.gz backup into the volume, setting ownership to the
// host user's UID:GID.
func restore(image, volume, home, backupFile string) error {
	container := fmt.Sprintf("kilo-restore-temp-%d", os.Getpid())

	_, err := dockerRunDetached("run", "-d", "--name", container,
		"-v", volume+":"+home, image, "tail", "-f", "/dev/null")
	if err != nil {
		return err
	}
	defer exec.Command("docker", "rm", "-f", container).Run()

	time.Sleep(500 * time.Millisecond)

	_, err = dockerRun("cp", backupFile, container+":/tmp/backup.tar.gz")
	if err != nil {
		return err
	}

	_, err = dockerExec(container, "", "tar", "xzf", "/tmp/backup.tar.gz", "-C", home)
	if err != nil {
		return err
	}

	_, _ = dockerExec(container, "", "chown", "-R", fmt.Sprintf("%d:%d", os.Getuid(), os.Getgid()), home)
	return nil
}

// session represents a running or stopped kilo-docker container with its metadata.
type session struct {
	Name      string
	Status    string
	Workspace string
	Args      string
}

// getSessions queries Docker for all containers labeled with kilo.workspace.
func getSessions() ([]session, error) {
	output, err := dockerRun("ps", "-a", "--filter", "label=kilo.workspace", "--format", "{{.Names}}\t{{.Status}}\t{{.Labels}}")
	if err != nil {
		return nil, err
	}

	var sessions []session
	for _, line := range strings.Split(output, "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 3)
		if len(parts) < 3 {
			continue
		}
		s := session{
			Name:   parts[0],
			Status: parts[1],
		}
		for _, label := range strings.Split(parts[2], ",") {
			if strings.HasPrefix(label, "kilo.workspace=") {
				s.Workspace = strings.TrimPrefix(label, "kilo.workspace=")
			}
			if strings.HasPrefix(label, "kilo.args=") {
				s.Args = strings.TrimPrefix(label, "kilo.args=")
			}
		}
		sessions = append(sessions, s)
	}
	return sessions, nil
}

// showSessions prints a formatted table of sessions to stderr.
func showSessions(sessions []session) {
	fmt.Fprintf(os.Stderr, "%-4s %-22s %-32s %-52s %s\n", "#", "NAME", "STATUS", "WORKSPACE", "ARGS")
	fmt.Fprintf(os.Stderr, "%-4s %-22s %-32s %-52s %s\n", "---", "----------------------", "--------------------------------", "----------------------------------------------------", "-----")
	for i, s := range sessions {
		fmt.Fprintf(os.Stderr, "%-4d %-22s %-32s %-52s %s\n", i+1, s.Name, s.Status, s.Workspace, s.Args)
	}
}

// resolveTarget converts a target (numeric index or container name) to a
// container name. Returns an error if the container doesn't exist or the
// index is out of range.
func resolveTarget(target string) (string, error) {
	for _, c := range target {
		if c < '0' || c > '9' {
			if containerExists(target) {
				return target, nil
			}
			return "", fmt.Errorf("container '%s' not found", target)
		}
	}

	sessions, err := getSessions()
	if err != nil {
		return "", err
	}

	idx := 0
	fmt.Sscanf(target, "%d", &idx)
	if idx < 1 || idx > len(sessions) {
		return "", fmt.Errorf("no session at index %s", target)
	}
	return sessions[idx-1].Name, nil
}
