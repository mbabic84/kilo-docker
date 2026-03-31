package main

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// runBackup creates a gzipped tar archive of the user's home directory
// (KILO_HOME) and writes it to outputPath. Used by the host binary via
// `docker exec` for volume backups.
func runBackup(outputPath string) error {
	home := os.Getenv("HOME")
	if home == "" {
		home = "/home/kilo-t8x3m7kp"
	}

	f, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer f.Close()

	gz := gzip.NewWriter(f)
	defer gz.Close()

	tw := tar.NewWriter(gz)
	defer tw.Close()

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
			defer file.Close()
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
	home := os.Getenv("HOME")
	if home == "" {
		home = "/home/kilo-t8x3m7kp"
	}

	f, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()

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
			os.MkdirAll(filepath.Dir(target), 0755)
			file, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(header.Mode))
			if err != nil {
				return err
			}
			if _, err := io.Copy(file, tr); err != nil {
				file.Close()
				return err
			}
			file.Close()
		case tar.TypeSymlink:
			os.Symlink(header.Linkname, target)
		}

		os.Chown(target, currentUID, currentGID)
	}
	return nil
}

// tarFile adds a single file or symlink to the tar archive with the
// given relative path.
func tarFile(tw *tar.Writer, path, relPath string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}

	var link string
	if info.Mode()&os.ModeSymlink != 0 {
		link, _ = os.Readlink(path)
	}

	header, err := tar.FileInfoHeader(info, link)
	if err != nil {
		return err
	}
	header.Name = filepath.ToSlash(relPath)

	if err := tw.WriteHeader(header); err != nil {
		return err
	}

	if info.Mode().IsRegular() {
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = io.Copy(tw, f)
		return err
	}
	return nil
}

// walkDir recursively adds all files under dir to the tar archive.
func walkDir(tw *tar.Writer, dir, relDir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		name := entry.Name()
		fullPath := filepath.Join(dir, name)
		relPath := filepath.Join(relDir, name)

		if entry.IsDir() {
			if err := walkDir(tw, fullPath, relPath); err != nil {
				return err
			}
		} else {
			if err := tarFile(tw, fullPath, relPath); err != nil {
				return err
			}
		}
	}
	return nil
}

// extractTarGz extracts a gzipped tar archive from r into dest,
// setting ownership to the current UID/GID.
func extractTarGz(r io.Reader, dest string) error {
	gzr, err := gzip.NewReader(r)
	if err != nil {
		return err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)
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

		target := filepath.Join(dest, filepath.FromSlash(header.Name))

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
		case tar.TypeReg:
			os.MkdirAll(filepath.Dir(target), 0755)
			file, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(header.Mode))
			if err != nil {
				return err
			}
			if _, err := io.Copy(file, tr); err != nil {
				file.Close()
				return err
			}
			file.Close()
		case tar.TypeSymlink:
			os.Symlink(header.Linkname, target)
		}

		os.Chown(target, currentUID, currentGID)
	}
	return nil
}

// sanitizeArchivePath validates that a resolved path stays within the base
// directory, preventing path traversal attacks in archive extraction.
func sanitizeArchivePath(base, path string) (string, error) {
	clean := filepath.Clean(path)
	if !strings.HasPrefix(clean, base) {
		return "", fmt.Errorf("path %q escapes base %q", path, base)
	}
	return clean, nil
}
