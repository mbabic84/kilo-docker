package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUpdateBashrcManaged_NVMOnly(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("KD_SERVICES", "nvm")

	updateBashrcManaged(tmpDir)

	data, err := os.ReadFile(filepath.Join(tmpDir, ".bashrc"))
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if !strings.Contains(content, "# >>> kilo-managed >>>") {
		t.Error("missing start marker")
	}
	if !strings.Contains(content, "# <<< kilo-managed <<<") {
		t.Error("missing end marker")
	}
	if !strings.Contains(content, "NVM_DIR") {
		t.Error("missing NVM block")
	}
	if strings.Contains(content, "uv run python") {
		t.Error("unexpected uv block")
	}
}

func TestUpdateBashrcManaged_UVOnly(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("KD_SERVICES", "uv")

	updateBashrcManaged(tmpDir)

	data, _ := os.ReadFile(filepath.Join(tmpDir, ".bashrc"))
	content := string(data)
	if !strings.Contains(content, "uv run python") {
		t.Error("missing uv block")
	}
	if strings.Contains(content, "NVM_DIR") {
		t.Error("unexpected NVM block")
	}
}

func TestUpdateBashrcManaged_PreservesExistingNVM(t *testing.T) {
	tmpDir := t.TempDir()

	// Session A: installs NVM (simulates nvm.sh on disk)
	t.Setenv("KD_SERVICES", "nvm")
	updateBashrcManaged(tmpDir)
	_ = os.MkdirAll(filepath.Join(tmpDir, ".nvm"), 0o700)
	_ = os.WriteFile(filepath.Join(tmpDir, ".nvm", "nvm.sh"), []byte("# nvm"), 0o600)

	// Session B: no services — NVM is on disk so the block should be preserved
	t.Setenv("KD_SERVICES", "")
	updateBashrcManaged(tmpDir)

	data, _ := os.ReadFile(filepath.Join(tmpDir, ".bashrc"))
	content := string(data)
	if !strings.Contains(content, "NVM_DIR") {
		t.Error("NVM block was removed by a session without --nvm (NVM still on disk)")
	}
}

func TestUpdateBashrcManaged_PreservesExistingUV(t *testing.T) {
	tmpDir := t.TempDir()

	// Session A: installs UV (simulates uv on disk)
	t.Setenv("KD_SERVICES", "uv")
	updateBashrcManaged(tmpDir)
	_ = os.MkdirAll(filepath.Join(tmpDir, ".local", "bin"), 0o700)
	_ = os.WriteFile(filepath.Join(tmpDir, ".local", "bin", "uv"), []byte("# uv"), 0o700)

	// Session B: no services — UV is on disk so the block should be preserved
	t.Setenv("KD_SERVICES", "")
	updateBashrcManaged(tmpDir)

	data, _ := os.ReadFile(filepath.Join(tmpDir, ".bashrc"))
	content := string(data)
	if !strings.Contains(content, "uv run python") {
		t.Error("UV block was removed by a session without --uv (UV still on disk)")
	}
}

func TestUpdateBashrcManaged_CleansUpRemovedNVM(t *testing.T) {
	tmpDir := t.TempDir()

	// Session A: installs NVM and writes nvm.sh to disk
	t.Setenv("KD_SERVICES", "nvm")
	updateBashrcManaged(tmpDir)
	_ = os.MkdirAll(filepath.Join(tmpDir, ".nvm"), 0o700)
	_ = os.WriteFile(filepath.Join(tmpDir, ".nvm", "nvm.sh"), []byte("# nvm"), 0o600)

	// Verify NVM block is present
	data, _ := os.ReadFile(filepath.Join(tmpDir, ".bashrc"))
	if !strings.Contains(string(data), "NVM_DIR") {
		t.Fatal("NVM block should be present after install")
	}

	// NVM is removed from disk (e.g. user deleted it, volume reset, etc.)
	_ = os.RemoveAll(filepath.Join(tmpDir, ".nvm"))

	// Session B: no services — NVM is gone from disk, block should be cleaned up
	t.Setenv("KD_SERVICES", "")
	updateBashrcManaged(tmpDir)

	data, _ = os.ReadFile(filepath.Join(tmpDir, ".bashrc"))
	content := string(data)
	if strings.Contains(content, "NVM_DIR") {
		t.Error("NVM block should have been removed (NVM no longer on disk)")
	}
	if !strings.Contains(content, "# >>> kilo-managed >>>") {
		t.Error("managed section markers should still be present")
	}
}

func TestUpdateBashrcManaged_CleansUpRemovedUV(t *testing.T) {
	tmpDir := t.TempDir()

	// Session A: installs UV
	t.Setenv("KD_SERVICES", "uv")
	updateBashrcManaged(tmpDir)
	_ = os.MkdirAll(filepath.Join(tmpDir, ".local", "bin"), 0o700)
	_ = os.WriteFile(filepath.Join(tmpDir, ".local", "bin", "uv"), []byte("# uv"), 0o700)

	// UV is removed from disk
	_ = os.RemoveAll(filepath.Join(tmpDir, ".local"))

	// Session B: no services — UV is gone from disk, block should be cleaned up
	t.Setenv("KD_SERVICES", "")
	updateBashrcManaged(tmpDir)

	data, _ := os.ReadFile(filepath.Join(tmpDir, ".bashrc"))
	content := string(data)
	if strings.Contains(content, "uv run python") {
		t.Error("UV block should have been removed (UV no longer on disk)")
	}
}

func TestUpdateBashrcManaged_MergesNVMAndUV(t *testing.T) {
	tmpDir := t.TempDir()

	// Session A: installs NVM (with files on disk)
	t.Setenv("KD_SERVICES", "nvm")
	updateBashrcManaged(tmpDir)
	_ = os.MkdirAll(filepath.Join(tmpDir, ".nvm"), 0o700)
	_ = os.WriteFile(filepath.Join(tmpDir, ".nvm", "nvm.sh"), []byte("# nvm"), 0o600)

	// Session B: installs UV — should keep NVM (on disk) and add UV
	t.Setenv("KD_SERVICES", "uv")
	updateBashrcManaged(tmpDir)

	data, _ := os.ReadFile(filepath.Join(tmpDir, ".bashrc"))
	content := string(data)
	if !strings.Contains(content, "NVM_DIR") {
		t.Error("NVM block was lost after UV session")
	}
	if !strings.Contains(content, "uv run python") {
		t.Error("UV block was not added")
	}
}

func TestUpdateBashrcManaged_DoesNotDuplicate(t *testing.T) {
	tmpDir := t.TempDir()

	// Session A: installs NVM
	t.Setenv("KD_SERVICES", "nvm")
	updateBashrcManaged(tmpDir)
	_ = os.MkdirAll(filepath.Join(tmpDir, ".nvm"), 0o700)
	_ = os.WriteFile(filepath.Join(tmpDir, ".nvm", "nvm.sh"), []byte("# nvm"), 0o600)

	// Session B: also installs NVM — should not duplicate
	t.Setenv("KD_SERVICES", "nvm")
	updateBashrcManaged(tmpDir)

	data, _ := os.ReadFile(filepath.Join(tmpDir, ".bashrc"))
	content := string(data)
	count := strings.Count(content, "NVM_DIR")
	if count != 1 {
		t.Errorf("expected 1 NVM block, got %d", count)
	}
}

func TestUpdateBashrcManaged_PreservesUserContent(t *testing.T) {
	tmpDir := t.TempDir()
	bashrc := filepath.Join(tmpDir, ".bashrc")

	// Write custom user content
	custom := "# My custom aliases\nalias ll='ls -la'\n"
	_ = os.WriteFile(bashrc, []byte(custom), 0o600)

	// Session A: installs NVM
	t.Setenv("KD_SERVICES", "nvm")
	updateBashrcManaged(tmpDir)

	data, _ := os.ReadFile(bashrc)
	content := string(data)
	if !strings.Contains(content, "alias ll=") {
		t.Error("user content was lost")
	}
	if !strings.Contains(content, "NVM_DIR") {
		t.Error("NVM block missing")
	}
}
