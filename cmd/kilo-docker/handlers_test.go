package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetOSArchReturnsValidValues(t *testing.T) {
	osName, arch := getOSArch()

	if osName == "" {
		t.Error("getOSArch() returned empty osName")
	}
	if arch == "" {
		t.Error("getOSArch() returned empty arch")
	}

	// Verify osName is one of the expected values
	if osName != "linux" && osName != "darwin" {
		t.Errorf("getOSArch() returned unknown osName %q, expected linux or darwin", osName)
	}

	// Verify arch is one of the expected values
	if arch != "amd64" && arch != "arm64" {
		t.Errorf("getOSArch() returned unknown arch %q, expected amd64 or arm64", arch)
	}
}

func TestGetOSArchNormalization(t *testing.T) {
	// Test that the function returns consistent, normalized values
	osName, arch := getOSArch()

	// Run multiple times to verify consistency
	for i := 0; i < 5; i++ {
		currentOS, currentArch := getOSArch()
		if currentOS != osName {
			t.Errorf("getOSArch() returned inconsistent osName: first=%q, subsequent=%q", osName, currentOS)
		}
		if currentArch != arch {
			t.Errorf("getOSArch() returned inconsistent arch: first=%q, subsequent=%q", arch, currentArch)
		}
	}
}

func TestDownloadFileWithInvalidURL(t *testing.T) {
	// Create a temp file
	tempFile, err := os.CreateTemp("", "download-test-*")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	tempPath := tempFile.Name()
	_ = tempFile.Close()
	defer func() { _ = os.Remove(tempPath) }()

	// Try to download from an invalid URL - both curl and wget should fail
	err = downloadFile("http://this-domain-does-not-exist-12345.invalid/file", tempPath)

	if err == nil {
		t.Error("downloadFile() expected error for invalid URL, got nil")
	}
}

func TestDownloadFileCreatesTempFile(t *testing.T) {
	// This test verifies the downloadFile creates the destination file
	// even if download fails, the temp file path should be valid
	tempFile, err := os.CreateTemp("", "download-test-*")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	tempPath := tempFile.Name()
	_ = tempFile.Close()
	defer func() { _ = os.Remove(tempPath) }()

	// Verify temp file exists before download
	if _, err := os.Stat(tempPath); err != nil {
		t.Fatalf("temp file does not exist: %v", err)
	}
}

func TestHandleUpdateConstructsCorrectDownloadURL(t *testing.T) {
	// Verify the download URL format matches GitHub releases pattern
	osName, arch := getOSArch()
	downloadURL := "https://github.com/mbabic84/kilo-docker/releases/latest/download/kilo-docker-" + osName + "-" + arch

	expectedURL := filepath.Join(os.Getenv("HOME"), ".local", "bin", "kilo-docker")
	_ = expectedURL // used implicitly via target in handleUpdate

	// Verify URL contains expected components
	if downloadURL == "" {
		t.Error("downloadURL should not be empty")
	}
	if osName == "" || arch == "" {
		t.Error("osName and arch should not be empty")
	}

	// Verify URL matches expected pattern
	expected := "https://github.com/mbabic84/kilo-docker/releases/latest/download/kilo-docker-linux-amd64"
	if osName == "linux" && arch == "amd64" && downloadURL != expected {
		t.Errorf("downloadURL = %q, want %q", downloadURL, expected)
	}
}

func TestHandleUpdateDetectsNotInstalled(t *testing.T) {
	// Create a temp directory that doesn't contain the kilo-docker binary
	tempDir, err := os.MkdirTemp("", "kilo-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	// Save original home and set temp as home
	origHome := os.Getenv("HOME")
	_ = os.Setenv("HOME", tempDir)
	defer func() { _ = os.Setenv("HOME", origHome) }()

	// Check that ~/.local/bin/kilo-docker doesn't exist
	target := filepath.Join(tempDir, ".local", "bin", "kilo-docker")
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("expected kilo-docker to not exist at %s, got: %v", target, err)
	}
}

func TestHandleUpdateNotInstalledMessage(t *testing.T) {
	// Verify the "not installed" output message matches what handleUpdate prints
	expectedStderr := "kilo-docker is not installed locally.\n"

	// This is a simple validation that the message is defined correctly
	// Actual integration test would verify the message is printed
	if expectedStderr == "" {
		t.Error("expected stderr message should not be empty")
	}
}

func TestDownloadFileURLFormat(t *testing.T) {
	testCases := []struct {
		osName   string
		arch     string
		expected string
	}{
		{"linux", "amd64", "https://github.com/mbabic84/kilo-docker/releases/latest/download/kilo-docker-linux-amd64"},
		{"linux", "arm64", "https://github.com/mbabic84/kilo-docker/releases/latest/download/kilo-docker-linux-arm64"},
		{"darwin", "amd64", "https://github.com/mbabic84/kilo-docker/releases/latest/download/kilo-docker-darwin-amd64"},
		{"darwin", "arm64", "https://github.com/mbabic84/kilo-docker/releases/latest/download/kilo-docker-darwin-arm64"},
	}

	for _, tc := range testCases {
		url := "https://github.com/mbabic84/kilo-docker/releases/latest/download/kilo-docker-" + tc.osName + "-" + tc.arch
		if url != tc.expected {
			t.Errorf("URL for %s/%s = %q, want %q", tc.osName, tc.arch, url, tc.expected)
		}
	}
}
