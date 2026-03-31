package main

import (
	"fmt"
	"os"
	"os/exec"
	"time"
)

// backupVolume creates a gzipped tar backup of the volume by running
// docker cp on a detached tail -f container. The container is cleaned up
// via defer.
func backupVolume(image, volume, home, outputFile string) error {
	container := fmt.Sprintf("kilo-backup-temp-%d", os.Getpid())

	_, err := dockerRunDetached("run", "-d", "--name", container,
		"-v", volume+":"+home+":ro", image, "tail", "-f", "/dev/null")
	if err != nil {
		return err
	}
	defer exec.Command("docker", "rm", "-f", container).Run()

	time.Sleep(500 * time.Millisecond)

	_, err = dockerExec(container, "tar", "czf", "/tmp/backup.tar.gz", "-C", home, ".")
	if err != nil {
		return err
	}

	_, err = dockerRun("cp", container+":/tmp/backup.tar.gz", outputFile)
	return err
}

// restoreVolume extracts a tar.gz backup into the volume by running docker
// exec on a detached tail -f container. File ownership is set to 1000:1000.
func restoreVolume(image, volume, home, backupFile string) error {
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

	_, err = dockerExec(container, "tar", "xzf", "/tmp/backup.tar.gz", "-C", home)
	if err != nil {
		return err
	}

	_, _ = dockerExec(container, "chown", "-R", "1000:1000", home)
	return nil
}
