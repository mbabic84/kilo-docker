package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestLoginWithRetries_InvalidCredentialsReprompts(t *testing.T) {
	prompts := 0
	prompt := func() (string, string, error) {
		prompts++
		return "user", "pass", nil
	}
	auth := func(_, _ string) (loginResult, error) {
		return loginResult{}, &loginError{Kind: loginErrInvalidCredentials, Err: errors.New("invalid credentials")}
	}

	_, err := loginWithRetries(prompt, auth)
	if err == nil {
		t.Fatal("expected error after exhausting all attempts")
	}
	if prompts != maxLoginAttempts {
		t.Errorf("prompts = %d, want %d", prompts, maxLoginAttempts)
	}
	if !strings.Contains(err.Error(), "after 3 attempts") {
		t.Errorf("error %q should mention attempt exhaustion", err.Error())
	}
}

func TestLoginWithRetries_AccountDisabledAborts(t *testing.T) {
	prompts := 0
	prompt := func() (string, string, error) {
		prompts++
		return "user", "pass", nil
	}
	auth := func(_, _ string) (loginResult, error) {
		return loginResult{}, &loginError{Kind: loginErrAccountDisabled, Err: errors.New("account disabled")}
	}

	_, err := loginWithRetries(prompt, auth)
	if err == nil {
		t.Fatal("expected account-disabled error")
	}
	if prompts != 1 {
		t.Errorf("prompts = %d, want 1 (abort immediately)", prompts)
	}
	var le *loginError
	if !errors.As(err, &le) || le.Kind != loginErrAccountDisabled {
		t.Errorf("error should preserve account-disabled classification, got %v", err)
	}
}

func TestLoginWithRetries_TransientThenSuccess(t *testing.T) {
	attempts := 0
	prompt := func() (string, string, error) {
		return "user", "pass", nil
	}
	auth := func(_, _ string) (loginResult, error) {
		attempts++
		if attempts < maxLoginAttempts {
			return loginResult{}, &loginError{Kind: loginErrTransient, Err: errors.New("connection failed")}
		}
		return loginResult{UserID: "user-1", AccessToken: "access"}, nil
	}

	res, err := loginWithRetries(prompt, auth)
	if err != nil {
		t.Fatalf("expected success after transient retries, got %v", err)
	}
	if res.UserID != "user-1" {
		t.Errorf("UserID = %q, want %q", res.UserID, "user-1")
	}
	if attempts != maxLoginAttempts {
		t.Errorf("attempts = %d, want %d", attempts, maxLoginAttempts)
	}
}

func TestLoginWithRetries_CancellationAborts(t *testing.T) {
	prompts := 0
	prompt := func() (string, string, error) {
		prompts++
		return "", "", errLoginCancelled
	}
	auth := func(_, _ string) (loginResult, error) {
		t.Fatal("auth must not be called after cancellation")
		return loginResult{}, nil
	}

	_, err := loginWithRetries(prompt, auth)
	if !errors.Is(err, errLoginCancelled) {
		t.Errorf("expected errLoginCancelled, got %v", err)
	}
	if prompts != 1 {
		t.Errorf("prompts = %d, want 1", prompts)
	}
}

// writeStoredTokens encrypts a token set into a temp home so resolveCredentials
// can read it back.
func writeStoredTokens(t *testing.T, home, userID, ainstruct, access, refresh, syncExpiry, patExpiry string) {
	t.Helper()
	dir := filepath.Join(home, ".local/share/kilo-docker")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	store := TokenStore{}
	store.Set(tokenKeyAinstruct, ainstruct)
	store.Set(tokenKeySync, access)
	store.Set(tokenKeySyncRefresh, refresh)
	store.Set(tokenKeySyncExpiry, syncExpiry)
	store.Set(tokenKeyPATExtiry, patExpiry)
	if err := saveTokenStore(home, userID, store); err != nil {
		t.Fatal(err)
	}
}

// patListHandler serves GET /auth/pat with a single PAT matching label.
func patListHandler(t *testing.T, label string, expiresAt time.Time) http.HandlerFunc {
	t.Helper()
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/auth/pat" && r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"tokens": []map[string]any{{
					"pat_id":     "pat-1",
					"label":      label,
					"expires_at": expiresAt.Format("2006-01-02T15:04:05"),
				}},
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}
}

func TestResolveCredentials_ReusesUnexpiredToken(t *testing.T) {
	t.Setenv("KD_USERNAME", "tester")
	t.Setenv("KD_HOSTNAME", "host")
	label := buildAinstructPATLabel()

	srv := httptest.NewServer(patListHandler(t, label, time.Now().Add(72*time.Hour)))
	defer srv.Close()

	home := t.TempDir()
	userID := "user-1"
	writeStoredTokens(t, home, userID, "pat-123", "access-123", "refresh-123",
		strconv.FormatInt(time.Now().Add(time.Hour).Unix(), 10),
		strconv.FormatInt(time.Now().Add(72*time.Hour).Unix(), 10))

	res, ok := resolveCredentials(home, userID, srv.URL)
	if !ok {
		t.Fatal("expected to reuse unexpired stored credentials")
	}
	if res.AccessToken != "access-123" {
		t.Errorf("AccessToken = %q, want %q", res.AccessToken, "access-123")
	}
	if res.AinstructPAT != "pat-123" {
		t.Errorf("AinstructPAT = %q, want stored PAT %q", res.AinstructPAT, "pat-123")
	}
	if res.UserID != userID {
		t.Errorf("UserID = %q, want %q", res.UserID, userID)
	}
}

func TestResolveCredentials_RefreshesExpiredToken(t *testing.T) {
	t.Setenv("KD_USERNAME", "tester")
	t.Setenv("KD_HOSTNAME", "host")
	label := buildAinstructPATLabel()

	refreshCalled := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/auth/refresh":
			refreshCalled = true
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"access_token":  "new-access",
				"refresh_token": "new-refresh",
				"expires_in":    3600,
			})
		case r.URL.Path == "/auth/pat" && r.Method == http.MethodGet:
			patListHandler(t, label, time.Now().Add(72*time.Hour))(w, r)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	home := t.TempDir()
	userID := "user-1"
	writeStoredTokens(t, home, userID, "pat-123", "old-access", "refresh-123",
		strconv.FormatInt(time.Now().Add(-time.Hour).Unix(), 10),
		strconv.FormatInt(time.Now().Add(72*time.Hour).Unix(), 10))

	res, ok := resolveCredentials(home, userID, srv.URL)
	if !ok {
		t.Fatal("expected silent refresh to produce usable credentials")
	}
	if !refreshCalled {
		t.Error("expected /auth/refresh to be called for an expired token")
	}
	if res.AccessToken != "new-access" {
		t.Errorf("AccessToken = %q, want %q", res.AccessToken, "new-access")
	}
}

func TestResolveCredentials_NoTokens(t *testing.T) {
	home := t.TempDir()
	res, ok := resolveCredentials(home, "user-1", "http://127.0.0.1:0")
	if ok {
		t.Errorf("expected ok=false with no stored tokens, got %+v", res)
	}
}

func TestResolveCredentials_AuthRejectionPrompts(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	home := t.TempDir()
	userID := "user-1"
	writeStoredTokens(t, home, userID, "pat-123", "access-123", "refresh-123",
		strconv.FormatInt(time.Now().Add(time.Hour).Unix(), 10),
		strconv.FormatInt(time.Now().Add(72*time.Hour).Unix(), 10))

	if _, ok := resolveCredentials(home, userID, srv.URL); ok {
		t.Error("expected 401 from the live call to fall back to the prompt")
	}
}

func TestResolveCredentials_RefreshFailurePrompts(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	home := t.TempDir()
	userID := "user-1"
	writeStoredTokens(t, home, userID, "pat-123", "old-access", "refresh-123",
		strconv.FormatInt(time.Now().Add(-time.Hour).Unix(), 10),
		strconv.FormatInt(time.Now().Add(72*time.Hour).Unix(), 10))

	if _, ok := resolveCredentials(home, userID, srv.URL); ok {
		t.Error("expected a failed silent refresh to fall back to the prompt")
	}
}

func TestResolveCredentials_TransientPATFailureDegrades(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	home := t.TempDir()
	userID := "user-1"
	writeStoredTokens(t, home, userID, "pat-keep", "access-123", "refresh-123",
		strconv.FormatInt(time.Now().Add(time.Hour).Unix(), 10),
		strconv.FormatInt(time.Now().Add(72*time.Hour).Unix(), 10))

	res, ok := resolveCredentials(home, userID, srv.URL)
	if !ok {
		t.Fatal("expected transient PAT failure to degrade, not prompt")
	}
	if res.AccessToken != "access-123" {
		t.Errorf("AccessToken = %q, want %q", res.AccessToken, "access-123")
	}
	if res.AinstructPAT != "pat-keep" {
		t.Errorf("AinstructPAT = %q, want stored PAT retained (%q)", res.AinstructPAT, "pat-keep")
	}
}
