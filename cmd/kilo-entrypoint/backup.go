package main

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"

	"github.com/mbabic84/kilo-docker/pkg/constants"
)

// runBackup creates a gzipped tar archive of the user's home directory
// (KILO_HOME) and writes it to outputPath. Used by the host binary via
// `docker exec` for volume backups.
func runBackup(outputPath string) error {
	home := constants.GetHomeDir()

	f, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	gz := gzip.NewWriter(f)
	defer func() { _ = gz.Close() }()

	tw := tar.NewWriter(gz)
	defer func() { _ = tw.Close() }()

	return filepath.Walk(home, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, _ := filepath.Rel(home, path)
		if relPath == "." {
			return nil
		}

		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		header.Name = filepath.ToSlash(relPath)

		if err := tw.WriteHeader(header); err != nil {
			return err
		}

		if !info.IsDir() {
			file, err := os.Open(path)
			if err != nil {
				return err
			}
			defer func() { _ = file.Close() }()
			if _, err := io.Copy(tw, file); err != nil {
				return err
			}
		}
		return nil
	})
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

		target := filepath.Join(home, filepath.FromSlash(header.Name))

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
		case tar.TypeReg:
			_ = os.MkdirAll(filepath.Dir(target), 0755)
			file, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(header.Mode))
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


