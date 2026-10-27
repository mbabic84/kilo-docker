package main

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func tokenTestHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, ".local/share/kilo-docker"), 0o700); err != nil {
		t.Fatal(err)
	}
	return home
}

func TestParseTokenEnvPreservesUnknownKeys(t *testing.T) {
	data := []byte(strings.Join([]string{
		"KD_MCP_CONTEXT7_TOKEN=ctx7",
		"KD_MCP_GITEA_TOKEN=gitea-token",
		"UNKNOWN_KEY=unknown-value",
		"VALUE_WITH_EQUALS=a=b=c",
		"EMPTY_VALUE=",
		"malformed-line-without-equals",
		"",
	}, "\n"))

	store := parseTokenEnv(data)

	if got := store.Get(tokenKeyContext7); got != "ctx7" {
		t.Errorf("context7 = %q, want %q", got, "ctx7")
	}
	if got := store.Get(tokenKeyGiteaToken); got != "gitea-token" {
		t.Errorf("gitea token = %q, want %q", got, "gitea-token")
	}
	if got := store.Get("UNKNOWN_KEY"); got != "unknown-value" {
		t.Errorf("unknown key = %q, want %q", got, "unknown-value")
	}
	if got := store.Get("VALUE_WITH_EQUALS"); got != "a=b=c" {
		t.Errorf("value with equals = %q, want %q", got, "a=b=c")
	}
	if !store.Has("EMPTY_VALUE") {
		t.Error("expected EMPTY_VALUE key to be preserved")
	}
	if store.Has("malformed-line-without-equals") {
		t.Error("malformed line must not produce a key")
	}
}

func TestTokenStoreRoundTrip(t *testing.T) {
	home := tokenTestHome(t)
	userID := "user-1"

	store := TokenStore{}
	store.Set(tokenKeyContext7, "ctx7-value")
	store.Set(tokenKeyAinstruct, "ainstruct-value")
	store.Set(tokenKeySync, "sync-access")
	store.Set(tokenKeySyncRefresh, "sync-refresh")
	store.Set(tokenKeySyncExpiry, "1234567890")
	store.Set(tokenKeyPATExtiry, "9876543210")
	store.Set(tokenKeyGiteaToken, "gitea-token")
	store.Set(tokenKeyGiteaHost, "https://gitea.example.com")
	store.Set(tokenKeyGiteaInsecure, "true")
	store.Set("FUTURE_KEY", "future-value")

	if err := saveTokenStore(home, userID, store); err != nil {
		t.Fatalf("saveTokenStore() error = %v", err)
	}

	loaded, err := loadTokenStore(home, userID)
	if err != nil {
		t.Fatalf("loadTokenStore() error = %v", err)
	}
	if !reflect.DeepEqual(store, loaded) {
		t.Errorf("round trip mismatch:\n got %v\nwant %v", loaded, store)
	}
}

func TestTokenStoreWritesSortedKeysAnd0600(t *testing.T) {
	home := tokenTestHome(t)
	userID := "user-1"

	store := TokenStore{
		"B_KEY":       "2",
		"A_KEY":       "1",
		"UNKNOWN_KEY": "x",
	}
	if err := saveTokenStore(home, userID, store); err != nil {
		t.Fatalf("saveTokenStore() error = %v", err)
	}

	encData, err := os.ReadFile(tokenFilePath(home))
	if err != nil {
		t.Fatalf("reading token file: %v", err)
	}
	decrypted, err := decryptAES(encData, userID)
	if err != nil {
		t.Fatalf("decrypting token file: %v", err)
	}
	want := "A_KEY=1\nB_KEY=2\nUNKNOWN_KEY=x\n"
	if string(decrypted) != want {
		t.Errorf("token file content = %q, want %q", string(decrypted), want)
	}

	info, err := os.Stat(tokenFilePath(home))
	if err != nil {
		t.Fatalf("stat token file: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("token file permission = %o, want 600", perm)
	}
}

func TestTokenStorePreservesEmptyValues(t *testing.T) {
	home := tokenTestHome(t)
	userID := "user-1"

	store := TokenStore{}
	store.Set(tokenKeyGiteaInsecure, "")
	if err := saveTokenStore(home, userID, store); err != nil {
		t.Fatalf("saveTokenStore() error = %v", err)
	}

	loaded, err := loadTokenStore(home, userID)
	if err != nil {
		t.Fatalf("loadTokenStore() error = %v", err)
	}
	if !loaded.Has(tokenKeyGiteaInsecure) {
		t.Error("expected empty Gitea insecure key to survive round trip")
	}
	if got := loaded.Get(tokenKeyGiteaInsecure); got != "" {
		t.Errorf("Gitea insecure = %q, want empty", got)
	}
}

func TestLoadTokenStoreMissingFileReturnsEmptyStore(t *testing.T) {
	home := tokenTestHome(t)

	store, err := loadTokenStore(home, "user-1")
	if err == nil {
		t.Fatal("expected error for missing token file")
	}
	if store == nil {
		t.Fatal("expected a non-nil empty store")
	}
	if len(store) != 0 {
		t.Errorf("expected empty store, got %v", store)
	}
}

func TestSyncTokenHelpersDoNotOverwriteUnreadableStore(t *testing.T) {
	home := tokenTestHome(t)
	encPath := tokenFilePath(home)
	corrupt := []byte("not-encrypted")
	if err := os.WriteFile(encPath, corrupt, 0o600); err != nil {
		t.Fatal(err)
	}

	if err := saveSyncTokensToEncrypted(home, "user-1", "a", "b", "c"); err == nil {
		t.Error("saveSyncTokensToEncrypted() = nil, want an error for an unreadable store")
	}
	if err := clearSyncTokensFromEncrypted(home, "user-1"); err == nil {
		t.Error("clearSyncTokensFromEncrypted() = nil, want an error for an unreadable store")
	}

	after, err := os.ReadFile(encPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(after, corrupt) {
		t.Error("unreadable token store must be left untouched")
	}
}

func TestSaveSyncTokensPreservesOtherKeys(t *testing.T) {
	home := tokenTestHome(t)
	userID := "user-1"

	initial := TokenStore{}
	initial.Set(tokenKeyContext7, "ctx7")
	initial.Set(tokenKeyAinstruct, "ainstruct")
	initial.Set(tokenKeySync, "old-access")
	initial.Set(tokenKeySyncRefresh, "old-refresh")
	initial.Set(tokenKeySyncExpiry, "111")
	initial.Set(tokenKeyPATExtiry, "222")
	initial.Set(tokenKeyGiteaToken, "gitea")
	initial.Set(tokenKeyGiteaHost, "https://gitea.example.com")
	initial.Set("FUTURE_KEY", "future")
	if err := saveTokenStore(home, userID, initial); err != nil {
		t.Fatal(err)
	}

	if err := saveSyncTokensToEncrypted(home, userID, "new-access", "new-refresh", "333"); err != nil {
		t.Fatalf("saveSyncTokensToEncrypted() error = %v", err)
	}

	store, err := loadTokenStore(home, userID)
	if err != nil {
		t.Fatal(err)
	}
	if got := store.Get(tokenKeySync); got != "new-access" {
		t.Errorf("sync token = %q, want %q", got, "new-access")
	}
	if got := store.Get(tokenKeySyncRefresh); got != "new-refresh" {
		t.Errorf("sync refresh = %q, want %q", got, "new-refresh")
	}
	if got := store.Get(tokenKeySyncExpiry); got != "333" {
		t.Errorf("sync expiry = %q, want %q", got, "333")
	}
	for key, want := range map[string]string{
		tokenKeyContext7:   "ctx7",
		tokenKeyAinstruct:  "ainstruct",
		tokenKeyPATExtiry:  "222",
		tokenKeyGiteaToken: "gitea",
		tokenKeyGiteaHost:  "https://gitea.example.com",
		"FUTURE_KEY":       "future",
	} {
		if got := store.Get(key); got != want {
			t.Errorf("%s = %q, want %q (must be preserved)", key, got, want)
		}
	}
}

func TestClearSyncTokensPreservesOtherKeys(t *testing.T) {
	home := tokenTestHome(t)
	userID := "user-1"

	initial := TokenStore{}
	initial.Set(tokenKeyContext7, "ctx7")
	initial.Set(tokenKeySync, "access")
	initial.Set(tokenKeySyncRefresh, "refresh")
	initial.Set(tokenKeySyncExpiry, "111")
	initial.Set(tokenKeyGiteaToken, "gitea")
	initial.Set(tokenKeyGiteaInsecure, "true")
	initial.Set("FUTURE_KEY", "future")
	if err := saveTokenStore(home, userID, initial); err != nil {
		t.Fatal(err)
	}

	if err := clearSyncTokensFromEncrypted(home, userID); err != nil {
		t.Fatalf("clearSyncTokensFromEncrypted() error = %v", err)
	}

	store, err := loadTokenStore(home, userID)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{tokenKeySync, tokenKeySyncRefresh, tokenKeySyncExpiry} {
		if got := store.Get(key); got != "" {
			t.Errorf("%s = %q, want empty after clear", key, got)
		}
	}
	for key, want := range map[string]string{
		tokenKeyContext7:      "ctx7",
		tokenKeyGiteaToken:    "gitea",
		tokenKeyGiteaInsecure: "true",
		"FUTURE_KEY":          "future",
	} {
		if got := store.Get(key); got != want {
			t.Errorf("%s = %q, want %q (must be preserved)", key, got, want)
		}
	}
}
