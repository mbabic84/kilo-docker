package main

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"

	"github.com/mbabic84/kilo-docker/pkg/constants"
)

// runBackup creates a gzipped tar archive of the user's home directory
// (KILO_HOME) and writes it to outputPath. Used by the host binary via
// `docker exec` for volume backups.
func runBackup(outputPath string) error {
	home := constants.GetHomeDir()

	if err := ensurePathUnderBase(filepath.Dir(outputPath), filepath.Dir(outputPath)); err != nil {
		return err
	}

	f, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	gz := gzip.NewWriter(f)
	defer func() { _ = gz.Close() }()

	tw := tar.NewWriter(gz)
	defer func() { _ = tw.Close() }()

	return writeBackupTree(home, home, tw)
}

// runRestore extracts a gzipped tar archive into the user's home directory,
// setting file ownership to the current UID/GID. Used by the host binary
// via `docker exec` for volume restores.
func runRestore(archivePath string) error {
	home := constants.GetHomeDir()

	f, err := os.Open(archivePath)
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
	currentUID := os.Getuid()
	currentGID := os.Getgid()

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		target, err := safeRestoreTarget(home, header.Name)
		if err != nil {
			return err
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o700); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
				return err
			}
			mode, err := safeTarFileMode(header.Mode)
			if err != nil {
				return err
			}
			file, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
			if err != nil {
				return err
			}
			if _, err := io.Copy(file, tr); err != nil {
				_ = file.Close()
				return err
			}
			_ = file.Close()
		case tar.TypeSymlink:
			_ = os.Symlink(header.Linkname, target)
		}

		_ = os.Chown(target, currentUID, currentGID)
	}
	return nil
}

func writeBackupTree(root, path string, tw *tar.Writer) error {
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
		if err := writeBackupEntry(tw, fullPath, relPath, info); err != nil {
			return err
		}
		if info.IsDir() {
			if err := writeBackupTree(root, fullPath, tw); err != nil {
				return err
			}
		}
	}
	return nil
}

func writeBackupEntry(tw *tar.Writer, fullPath, relPath string, info os.FileInfo) error {
	header, err := tar.FileInfoHeader(info, "")
	if err != nil {
		return err
	}
	header.Name = filepath.ToSlash(relPath)
	if info.IsDir() && !strings.HasSuffix(header.Name, "/") {
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

func safeTarFileMode(mode int64) (os.FileMode, error) {
	if mode < 0 || mode > math.MaxUint32 {
		return 0, fmt.Errorf("invalid tar mode %d", mode)
	}
	return os.FileMode(uint32(mode)), nil
}

func safeRestoreTarget(home, headerName string) (string, error) {
	cleanName := filepath.Clean(filepath.FromSlash(headerName))
	if cleanName == "." || cleanName == "" {
		return "", fmt.Errorf("invalid archive entry %q", headerName)
	}
	if filepath.IsAbs(cleanName) || cleanName == ".." || strings.HasPrefix(cleanName, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("archive entry escapes home directory: %q", headerName)
	}

	target := filepath.Join(home, cleanName)
	if err := ensurePathUnderBase(target, home); err != nil {
		return "", err
	}
	return target, nil
}

func ensurePathUnderBase(path, base string) error {
	cleanPath := filepath.Clean(path)
	cleanBase := filepath.Clean(base)
	rel, err := filepath.Rel(cleanBase, cleanPath)
	if err != nil {
		return err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return fmt.Errorf("path %q escapes base directory %q", path, base)
	}
	return nil
}
