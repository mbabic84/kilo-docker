package main

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// handleBackup creates a gzipped tar backup of the data volume.
func handleBackup(cfg config) {
	if cfg.help {
		printCommandHelp("backup")
		return
	}
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

	tempDir, err := os.MkdirTemp("", "kilo-backup-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to create temp directory: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	tempContainer := fmt.Sprintf("kilo-backup-temp-%d", os.Getpid())
	_, _ = dockerRun("run", "--rm", "-d", "--name", tempContainer, "-v", dataVolume+":/src:ro", "debian:bookworm-slim", "tail", "-f", "/dev/null")
	time.Sleep(500 * time.Millisecond)
	_, _ = dockerRun("cp", tempContainer+":/src/.", tempDir+"/src")
	_, _ = dockerRun("rm", "-f", tempContainer)

	_ = os.MkdirAll(filepath.Dir(backupFile), 0o700)
	if err := archiveDirectory(filepath.Join(tempDir, "src"), backupFile); err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to create backup archive: %v\n", err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "Backup created: %s\n", backupFile)
}

func archiveDirectory(srcDir, backupFile string) error {
	f, err := os.Create(backupFile)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	gz := gzip.NewWriter(f)
	defer func() { _ = gz.Close() }()

	tw := tar.NewWriter(gz)
	defer func() { _ = tw.Close() }()

	return walkArchiveTree(srcDir, srcDir, tw)
}

func walkArchiveTree(root, path string, tw *tar.Writer) error {
	entries, err := os.ReadDir(path)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		fullPath := filepath.Join(path, entry.Name())
		info, err := entry.Info()
		if err != nil {
			return err
		}
		relPath, err := filepath.Rel(root, fullPath)
		if err != nil {
			return err
		}
		if err := writeArchiveEntry(tw, fullPath, relPath, info); err != nil {
			return err
		}
		if info.IsDir() {
			if err := walkArchiveTree(root, fullPath, tw); err != nil {
				return err
			}
		}
	}
	return nil
}

func writeArchiveEntry(tw *tar.Writer, fullPath, relPath string, info os.FileInfo) error {
	header, err := tar.FileInfoHeader(info, "")
	if err != nil {
		return err
	}
	header.Name = filepath.ToSlash(relPath)
	if info.IsDir() && header.Name[len(header.Name)-1] != '/' {
		header.Name += "/"
	}
	if err := tw.WriteHeader(header); err != nil {
		return err
	}
	if info.IsDir() {
		return nil
	}

	file, err := os.Open(fullPath)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()
	_, err = io.Copy(tw, file)
	return err
}

// handleRestore restores a tar.gz backup into the data volume.
func handleRestore(cfg config) {
	if cfg.help {
		printCommandHelp("restore")
		return
	}
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

	if err := validateArchiveFile(backupFile); err != nil {
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
		_, _ = dockerRun("volume", "rm", targetVolume)
	}

	fmt.Fprintf(os.Stderr, "Restoring backup '%s' to volume '%s'...\n", backupFile, targetVolume)

	_, _ = dockerRun("volume", "create", targetVolume)

	_, err := dockerRun("run", "--rm", "-v", targetVolume+":/dest", "-v", filepath.Dir(backupFile)+":/backup:ro", "debian:bookworm-slim",
		"tar", "xzf", "/backup/"+filepath.Base(backupFile), "-C", "/dest")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: restore had issues: %v\n", err)
	}

	_, _ = dockerRun("run", "--rm", "-v", targetVolume+":/dest", "debian:bookworm-slim", "chown", "-R", fmt.Sprintf("%d:%d", os.Getuid(), os.Getgid()), "/dest")

	fmt.Fprintf(os.Stderr, "Restore complete.\n")
}

func validateArchiveFile(backupFile string) error {
	f, err := os.Open(backupFile)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer func() { _ = gz.Close() }()

	tr := tar.NewReader(gz)
	for {
		_, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
	}
}
