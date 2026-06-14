package main

import (
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func TestParseEnvMapEmpty(t *testing.T) {
	envs := parseEnvMap("")
	if len(envs) != 0 {
		t.Errorf("expected empty map, got %d entries", len(envs))
	}
}

func TestParseEnvMapSingle(t *testing.T) {
	envs := parseEnvMap("MY_TOKEN=abc123\n")
	if len(envs) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(envs))
	}
	if envs["MY_TOKEN"] != "abc123" {
		t.Errorf("expected abc123, got %q", envs["MY_TOKEN"])
	}
}

func TestParseEnvMapMultiple(t *testing.T) {
	data := "API_KEY=sk-12345\nSECRET=mysecret\nDB_URL=postgresql://localhost/db\n"
	envs := parseEnvMap(data)
	if len(envs) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(envs))
	}
	if envs["API_KEY"] != "sk-12345" {
		t.Errorf("API_KEY: got %q, want sk-12345", envs["API_KEY"])
	}
	if envs["SECRET"] != "mysecret" {
		t.Errorf("SECRET: got %q, want mysecret", envs["SECRET"])
	}
	if envs["DB_URL"] != "postgresql://localhost/db" {
		t.Errorf("DB_URL: got %q, want postgresql://localhost/db", envs["DB_URL"])
	}
}

func TestParseEnvMapSkipsEmptyLines(t *testing.T) {
	data := "FOO=bar\n\nBAZ=qux\n\n"
	envs := parseEnvMap(data)
	if len(envs) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(envs))
	}
}

func TestParseEnvMapSkipsWhitespaceLines(t *testing.T) {
	data := "FOO=bar\n  \nBAZ=qux\n"
	envs := parseEnvMap(data)
	if len(envs) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(envs))
	}
}

func TestParseEnvMapSkipsMalformedLines(t *testing.T) {
	data := "FOO=bar\nMALFORMED\nBAZ=qux\n"
	envs := parseEnvMap(data)
	if len(envs) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(envs))
	}
	if _, ok := envs["MALFORMED"]; ok {
		t.Errorf("MALFORMED should not be present")
	}
}

func TestParseEnvMapSkipsEmptyKey(t *testing.T) {
	data := "=value\nFOO=bar\n"
	envs := parseEnvMap(data)
	if len(envs) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(envs))
	}
	if envs["FOO"] != "bar" {
		t.Errorf("FOO: got %q, want bar", envs["FOO"])
	}
}

func TestParseEnvMapValueWithEquals(t *testing.T) {
	data := "URL=http://example.com?a=1&b=2\n"
	envs := parseEnvMap(data)
	if envs["URL"] != "http://example.com?a=1&b=2" {
		t.Errorf("got %q, want http://example.com?a=1&b=2", envs["URL"])
	}
}

func TestSerializeEnvMapEmpty(t *testing.T) {
	data := serializeEnvMap(map[string]string{})
	if data != "" {
		t.Errorf("expected empty string, got %q", data)
	}
}

func TestSerializeEnvMapSingle(t *testing.T) {
	data := serializeEnvMap(map[string]string{"FOO": "bar"})
	if data != "FOO=bar\n" {
		t.Errorf("got %q, want FOO=bar\\n", data)
	}
}

func TestSerializeEnvMapMultiple(t *testing.T) {
	envs := map[string]string{
		"Z_KEY": "zval",
		"A_KEY": "aval",
		"M_KEY": "mval",
	}
	data := serializeEnvMap(envs)
	expected := "A_KEY=aval\nM_KEY=mval\nZ_KEY=zval\n"
	if data != expected {
		t.Errorf("got %q, want %q", data, expected)
	}
}

func TestSerializeEnvMapRoundtrip(t *testing.T) {
	original := map[string]string{
		"API_TOKEN":     "sk-secret-12345",
		"DB_URL":        "postgresql://localhost:5432/mydb",
		"CUSTOM_CONFIG": "value with spaces",
	}
	serialized := serializeEnvMap(original)
	parsed := parseEnvMap(serialized)

	if len(parsed) != len(original) {
		t.Fatalf("roundtrip changed count: got %d, want %d", len(parsed), len(original))
	}
	for k, v := range original {
		if parsed[k] != v {
			t.Errorf("key %q: got %q, want %q", k, parsed[k], v)
		}
	}
}

func TestLoadCustomEnvsFileNotExist(t *testing.T) {
	homeDir := t.TempDir()
	_, err := loadCustomEnvs(homeDir, "test-user-id")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestLoadCustomEnvsCorruptData(t *testing.T) {
	homeDir := t.TempDir()
	storeDir := filepath.Join(homeDir, ".local/share/kilo-docker")
	_ = os.MkdirAll(storeDir, 0o700)
	customPath := filepath.Join(storeDir, ".custom-envs.env.enc")
	_ = os.WriteFile(customPath, []byte("not-encrypted-data"), 0o600)

	_, err := loadCustomEnvs(homeDir, "test-user-id")
	if err == nil {
		t.Error("expected error for corrupt data")
	}
}

func TestSaveAndLoadCustomEnvsRoundtrip(t *testing.T) {
	homeDir := t.TempDir()
	userID := "test-user-id"

	envs := map[string]string{
		"API_KEY":  "sk-abc123",
		"DB_PASS":  "s3cret",
		"ENDPOINT": "https://api.example.com/v2",
	}

	if err := saveCustomEnvs(homeDir, userID, envs); err != nil {
		t.Fatalf("saveCustomEnvs failed: %v", err)
	}

	loaded, err := loadCustomEnvs(homeDir, userID)
	if err != nil {
		t.Fatalf("loadCustomEnvs failed: %v", err)
	}

	if len(loaded) != len(envs) {
		t.Fatalf("roundtrip count mismatch: got %d, want %d", len(loaded), len(envs))
	}
	for k, v := range envs {
		if loaded[k] != v {
			t.Errorf("key %q: got %q, want %q", k, loaded[k], v)
		}
	}
}

func TestSaveCustomEnvsOverwrite(t *testing.T) {
	homeDir := t.TempDir()
	userID := "test-user-id"

	envs1 := map[string]string{"FOO": "bar", "BAZ": "qux"}
	if err := saveCustomEnvs(homeDir, userID, envs1); err != nil {
		t.Fatalf("first save failed: %v", err)
	}

	envs2 := map[string]string{"NEW_KEY": "new_value"}
	if err := saveCustomEnvs(homeDir, userID, envs2); err != nil {
		t.Fatalf("second save failed: %v", err)
	}

	loaded, err := loadCustomEnvs(homeDir, userID)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if len(loaded) != 1 {
		t.Fatalf("expected 1 entry after overwrite, got %d", len(loaded))
	}
	if loaded["NEW_KEY"] != "new_value" {
		t.Errorf("got %q, want new_value", loaded["NEW_KEY"])
	}
}

func TestSaveCustomEnvsFilePermission(t *testing.T) {
	homeDir := t.TempDir()
	userID := "test-user-id"

	envs := map[string]string{"KEY": "value"}
	if err := saveCustomEnvs(homeDir, userID, envs); err != nil {
		t.Fatalf("saveCustomEnvs failed: %v", err)
	}

	customPath := filepath.Join(homeDir, ".local/share/kilo-docker/.custom-envs.env.enc")
	info, err := os.Stat(customPath)
	if err != nil {
		t.Fatalf("stat failed: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("expected 0600 permissions, got %04o", info.Mode().Perm())
	}
}

func TestRunCustomEnvsListNoEnvs(t *testing.T) {
	homeDir := t.TempDir()
	runCustomEnvsList(homeDir, "test-user-id")
}

func TestRunCustomEnvsListWithEnvs(t *testing.T) {
	homeDir := t.TempDir()
	userID := "test-user-id"

	envs := map[string]string{"FOO": "barvalue", "BAZ": "secret123"}
	if err := saveCustomEnvs(homeDir, userID, envs); err != nil {
		t.Fatalf("saveCustomEnvs failed: %v", err)
	}

	runCustomEnvsList(homeDir, userID)
}

func TestRunCustomEnvsGetKeyExists(t *testing.T) {
	homeDir := t.TempDir()
	userID := "test-user-id"

	envs := map[string]string{"MY_VAR": "expected-value"}
	if err := saveCustomEnvs(homeDir, userID, envs); err != nil {
		t.Fatalf("saveCustomEnvs failed: %v", err)
	}

	origStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	runCustomEnvsGet(homeDir, userID, "MY_VAR")

	_ = w.Close()
	os.Stdout = origStdout

	data, _ := io.ReadAll(r)
	got := strings.TrimSpace(string(data))

	if got != "expected-value" {
		t.Errorf("got %q, want expected-value", got)
	}
}

func TestRunCustomEnvsGetKeyMissing(t *testing.T) {
	homeDir := t.TempDir()
	userID := "test-user-id"

	envs := map[string]string{"OTHER_VAR": "value"}
	if err := saveCustomEnvs(homeDir, userID, envs); err != nil {
		t.Fatalf("saveCustomEnvs failed: %v", err)
	}

	origStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	runCustomEnvsGet(homeDir, userID, "MISSING_VAR")

	_ = w.Close()
	os.Stdout = origStdout

	data, _ := io.ReadAll(r)
	got := strings.TrimSpace(string(data))

	if got != "" {
		t.Errorf("expected empty output for missing key, got %q", got)
	}
}

func TestRunCustomEnvsAddAndRemove(t *testing.T) {
	homeDir := t.TempDir()
	userID := "test-user-id"

	runCustomEnvsAdd(homeDir, userID, "NEW_KEY", "new_value")

	loaded, err := loadCustomEnvs(homeDir, userID)
	if err != nil {
		t.Fatalf("loadCustomEnvs failed: %v", err)
	}
	if loaded["NEW_KEY"] != "new_value" {
		t.Errorf("got %q, want new_value", loaded["NEW_KEY"])
	}

	runCustomEnvsRemove(homeDir, userID, "NEW_KEY")

	loaded, err = loadCustomEnvs(homeDir, userID)
	if err != nil && !os.IsNotExist(err) {
		t.Fatalf("loadCustomEnvs failed: %v", err)
	}
	if len(loaded) != 0 {
		t.Errorf("expected empty envs after remove, got %d entries", len(loaded))
	}
}

func TestRunCustomEnvsEdit(t *testing.T) {
	homeDir := t.TempDir()
	userID := "test-user-id"

	runCustomEnvsAdd(homeDir, userID, "MY_KEY", "original_value")
	runCustomEnvsEdit(homeDir, userID, "MY_KEY", "updated_value")

	loaded, err := loadCustomEnvs(homeDir, userID)
	if err != nil {
		t.Fatalf("loadCustomEnvs failed: %v", err)
	}
	if loaded["MY_KEY"] != "updated_value" {
		t.Errorf("got %q, want updated_value", loaded["MY_KEY"])
	}
}

func TestRunCustomEnvsListOrdered(t *testing.T) {
	homeDir := t.TempDir()
	userID := "test-user-id"

	envs := map[string]string{
		"Z_VAR": "last",
		"A_VAR": "first",
		"M_VAR": "middle",
	}
	if err := saveCustomEnvs(homeDir, userID, envs); err != nil {
		t.Fatalf("saveCustomEnvs failed: %v", err)
	}

	loaded, err := loadCustomEnvs(homeDir, userID)
	if err != nil {
		t.Fatalf("loadCustomEnvs failed: %v", err)
	}

	keys := make([]string, 0, len(loaded))
	for k := range loaded {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	if keys[0] != "A_VAR" || keys[1] != "M_VAR" || keys[2] != "Z_VAR" {
		t.Errorf("keys not sorted: %v", keys)
	}
}
