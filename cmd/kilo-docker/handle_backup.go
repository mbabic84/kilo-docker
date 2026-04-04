package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// handleBackup creates a gzipped tar backup of the data volume.
func handleBackup(cfg config) {
	args := cfg.args
	backupFile := ""
	forceBackup := false

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-f", "--force":
			forceBackup = true
		default:
			backupFile = args[i]
		}
	}

	dataVolume := resolveVolume(cfg)
	if dataVolume == "" {
		fmt.Fprintf(os.Stderr, "Error: backup is not available in --once mode.\n")
		os.Exit(1)
	}

	if !volumeExists(dataVolume) {
		fmt.Fprintf(os.Stderr, "Error: volume '%s' does not exist.\n", dataVolume)
		os.Exit(1)
	}

	if backupFile == "" {
		timestamp := time.Now().Format("20060102-150405")
		backupFile = fmt.Sprintf("kilo-backup-%s.tar.gz", timestamp)
	}

	if _, err := os.Stat(backupFile); err == nil && !forceBackup {
		if !promptConfirm("Backup file '" + backupFile + "' exists. Overwrite? [y/N]: ", cfg.yes) {
			fmt.Fprintf(os.Stderr, "Aborted.\n")
			os.Exit(0)
		}
	}

	fmt.Fprintf(os.Stderr, "Creating backup of volume '%s' to '%s'...\n", dataVolume, backupFile)

	tempDir, _ := os.MkdirTemp("", "kilo-backup-*")
	defer os.RemoveAll(tempDir)

	tempContainer := fmt.Sprintf("kilo-backup-temp-%d", os.Getpid())
	dockerRun("run", "--rm", "-d", "--name", tempContainer, "-v", dataVolume+":/src:ro", "alpine:latest", "tail", "-f", "/dev/null")
	time.Sleep(500 * time.Millisecond)
	dockerRun("cp", tempContainer+":/src/.", tempDir+"/src")
	dockerRun("rm", "-f", tempContainer)

	os.MkdirAll(filepath.Dir(backupFile), 0755)
	exec.Command("tar", "czf", backupFile, "-C", tempDir+"/src", ".").Run()

	fmt.Fprintf(os.Stderr, "Backup created: %s\n", backupFile)
}

// handleRestore restores a tar.gz backup into the data volume.
func handleRestore(cfg config) {
	args := cfg.args
	backupFile := ""
	forceRestore := false
	targetVolume := ""

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-f", "--force":
			forceRestore = true
		case "-v", "--volume":
			if i+1 < len(args) {
				targetVolume = args[i+1]
				i++
			}
		default:
			backupFile = args[i]
		}
	}

	dataVolume := resolveVolume(cfg)
	if targetVolume == "" {
		if dataVolume == "" {
			fmt.Fprintf(os.Stderr, "Error: no volume specified. Use --once mode or provide --volume.\n")
			os.Exit(1)
		}
		targetVolume = dataVolume
	}

	if backupFile == "" {
		fmt.Fprintf(os.Stderr, "Error: backup file path required.\n")
		fmt.Fprintf(os.Stderr, "Usage: kilo-docker restore <backup-file> [-f] [--volume <name>]\n")
		os.Exit(1)
	}

	if _, err := os.Stat(backupFile); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Error: backup file '%s' not found.\n", backupFile)
		os.Exit(1)
	}

	if _, err := exec.Command("tar", "-tzf", backupFile).CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: invalid or corrupted backup file.\n")
		os.Exit(1)
	}

	if volumeExists(targetVolume) {
		if !forceRestore {
			if !promptConfirm("Volume '" + targetVolume + "' exists and will be overwritten. Continue? [y/N]: ", cfg.yes) {
				fmt.Fprintf(os.Stderr, "Aborted.\n")
				os.Exit(0)
			}
		}
		dockerRun("volume", "rm", targetVolume)
	}

	fmt.Fprintf(os.Stderr, "Restoring backup '%s' to volume '%s'...\n", backupFile, targetVolume)

	dockerRun("volume", "create", targetVolume)

	_, err := dockerRun("run", "--rm", "-v", targetVolume+":/dest", "-v", filepath.Dir(backupFile)+":/backup:ro", "alpine",
		"tar", "xzf", "/backup/"+filepath.Base(backupFile), "-C", "/dest")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: restore had issues: %v\n", err)
	}

	dockerRun("run", "--rm", "-v", targetVolume+":/dest", "alpine", "chown", "-R", fmt.Sprintf("%d:%d", os.Getuid(), os.Getgid()), "/dest")

	fmt.Fprintf(os.Stderr, "Restore complete.\n")
}
