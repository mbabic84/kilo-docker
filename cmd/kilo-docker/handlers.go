package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// handleUpdate pulls the latest Docker image and optionally updates the
// installed host script from the repository.
func handleUpdate() {
	home, _ := os.UserHomeDir()
	target := filepath.Join(home, ".local", "bin", "kilo-docker")

	if _, err := os.Stat(target); err == nil {
		fmt.Fprintf(os.Stderr, "Updating kilo-docker script...\n")
		tempFile, _ := os.CreateTemp("", "kilo-docker-*")
		tempFile.Close()

		cmd := exec.Command("curl", "-fsSL", githubRawURL, "-o", tempFile.Name())
		if cmd.Run() == nil {
			os.Chmod(tempFile.Name(), 0755)
			os.Rename(tempFile.Name(), target)
			fmt.Fprintf(os.Stderr, "Script updated: %s\n", target)
		} else {
			os.Remove(tempFile.Name())
			fmt.Fprintf(os.Stderr, "Warning: Failed to download script update.\n")
		}
	} else {
		fmt.Fprintf(os.Stderr, "kilo-docker script is not installed locally.\n")
		fmt.Fprintf(os.Stderr, "Run 'kilo-docker install' to install it first.\n")
	}

	fmt.Fprintf(os.Stderr, "\nPulling Docker image...\n")
	dockerRun("pull", repoURL+":latest")
	fmt.Fprintf(os.Stderr, "\nUpdate complete.\n")
}

// handleInstall installs kilo-docker as a global command by creating a
// symlink (or copy) in ~/.local/bin and pulling the Docker image.
func handleInstall() {
	home, _ := os.UserHomeDir()
	target := filepath.Join(home, ".local", "bin", "kilo-docker")

	sourceScript, _ := os.Executable()
	os.MkdirAll(filepath.Dir(target), 0755)

	if info, err := os.Lstat(target); err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			fmt.Fprintf(os.Stderr, "kilo-docker is already installed at %s\n", target)
			fmt.Fprintf(os.Stderr, "Updating symlink...\n")
			os.Remove(target)
		} else {
			fmt.Fprintf(os.Stderr, "kilo-docker already exists at %s\n", target)
			if !promptConfirm("Replace it? [y/N]: ") {
				fmt.Fprintf(os.Stderr, "Aborted.\n")
				return
			}
			os.Remove(target)
		}
	}

	if err := os.Symlink(sourceScript, target); err != nil {
		copyFile(sourceScript, target)
	}
	os.Chmod(target, 0755)

	path := os.Getenv("PATH")
	if !strings.Contains(path, filepath.Join(home, ".local", "bin")) {
		fmt.Fprintf(os.Stderr, "\nWarning: %s is not on your PATH.\n", filepath.Join(home, ".local", "bin"))
		fmt.Fprintf(os.Stderr, "Add the following line to your ~/.bashrc (or ~/.zshrc):\n")
		fmt.Fprintf(os.Stderr, "  export PATH=\"$HOME/.local/bin:$PATH\"\n")
	}

	fmt.Fprintf(os.Stderr, "\nPulling Docker image...\n")
	dockerRun("pull", repoURL+":latest")
	fmt.Fprintf(os.Stderr, "\nInstalled successfully: %s -> %s\n", target, sourceScript)
	fmt.Fprintf(os.Stderr, "Run 'kilo-docker' from any directory.\n")
}

// handleCleanup removes all kilo-docker artifacts: containers, volumes,
// Docker images, and the installed script.
func handleCleanup() {
	if !promptConfirm("Remove volume, containers, and images for kilo-docker? [y/N]: ") {
		fmt.Fprintf(os.Stderr, "Aborted.\n")
		return
	}

	output, _ := dockerRun("ps", "-a", "--filter", "ancestor="+repoURL, "-q")
	if output != "" {
		for _, id := range strings.Split(output, "\n") {
			if id != "" {
				dockerRun("rm", "-f", id)
			}
		}
	}

	home, _ := os.UserHomeDir()
	user := filepath.Base(home)
	dataVolume := "kilo-data-" + user
	if volumeExists(dataVolume) {
		removeVolume(dataVolume)
	}

	dockerRun("rmi", repoURL+":latest")

	target := filepath.Join(home, ".local", "bin", "kilo-docker")
	os.Remove(target)

	fmt.Fprintf(os.Stderr, "Cleanup complete.\n")
}

// handleInit resets the data volume, prompting for confirmation.
func handleInit(cfg config) {
	dataVolume := resolveVolume(cfg)
	if dataVolume == "" {
		fmt.Fprintf(os.Stderr, "Nothing to reset in --once mode.\n")
		os.Exit(0)
	}

	if volumeExists(dataVolume) {
		if promptConfirm("Remove volume '" + dataVolume + "' and reset all configuration? [y/N]: ") {
			removeVolume(dataVolume)
			fmt.Fprintf(os.Stderr, "Volume removed. You will be prompted for tokens on next run.\n")
		}
	} else {
		fmt.Fprintf(os.Stderr, "No existing volume found.\n")
	}
}

// handleUpdateConfig downloads the latest opencode.json template and
// merges it with the existing config in the volume.
func handleUpdateConfig(cfg config) {
	dataVolume := resolveVolume(cfg)
	if dataVolume == "" {
		fmt.Fprintf(os.Stderr, "Nothing to update in --once mode.\n")
		os.Exit(0)
	}

	if !volumeExists(dataVolume) {
		fmt.Fprintf(os.Stderr, "No existing volume found. Run kilo-docker first to create one.\n")
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "Updating opencode.json from repository template...\n")

	_, err := dockerRun(
		"-v", dataVolume+":"+kiloHome,
		repoURL+":latest",
		"update-config",
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
