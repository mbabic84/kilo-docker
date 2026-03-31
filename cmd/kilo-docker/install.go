package main

import (
	"os"
)

// copyFile reads the source file and writes its contents to the destination
// with mode 0755. Used as a fallback when symlink creation fails during install.
func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0755)
}
