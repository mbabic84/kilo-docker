package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func newTestSyncer(serverURL string) *Syncer {
	return &Syncer{
		apiURL:       serverURL,
		accessToken:  "test-token",
		refreshToken: "test-refresh",
		tokenExpiry:  time.Now().Add(1 * time.Hour).Unix(),
		client:       &http.Client{Timeout: 5 * time.Second},
		syncPaths:    defaultSyncPaths,
		saveTokensFn: func() {}, // no-op: prevent writing to real encrypted storage
	}
}

// TestApiRequestSuccess verifies that a normal 200 response is returned
// without triggering retry logic.
func TestApiRequestSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/test" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(200)
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
			return
		}
		w.WriteHeader(404)
	}))
	defer srv.Close()

	s := newTestSyncer(srv.URL)
	data, err := s.apiRequest("GET", "/test", nil)
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}
	var result map[string]string
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if result["status"] != "ok" {
		t.Errorf("expected status=ok, got %s", result["status"])
	}
}

// TestApiRequestNon2xxError verifies that non-2xx responses without
// INVALID_TOKEN return an error immediately.
func TestApiRequestNon2xxError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		_, _ = w.Write([]byte(`{"detail":"internal error"}`))
	}))
	defer srv.Close()

	s := newTestSyncer(srv.URL)
	_, err := s.apiRequest("GET", "/test", nil)
	if err == nil {
		t.Fatal("expected error for 500 response, got nil")
	}
}

// TestApiRequestInvalidTokenRetry verifies that a 401 response with
// INVALID_TOKEN triggers a token refresh and retry.
func TestApiRequestInvalidTokenRetry(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/auth/refresh":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"access_token":  "refreshed-access-token",
				"refresh_token": "new-refresh",
				"expires_in":    3600,
			})
		case "/test":
			callCount++
			if callCount == 1 {
				// First call: return INVALID_TOKEN
				w.WriteHeader(401)
				_, _ = w.Write([]byte(`{"error":"INVALID_TOKEN"}`))
			} else {
				// Retry: return success (token was refreshed)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(200)
				_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
			}
		default:
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()

	s := newTestSyncer(srv.URL)
	data, err := s.apiRequest("GET", "/test", nil)
	if err != nil {
		t.Fatalf("expected retry to succeed, got error: %v", err)
	}
	var result map[string]string
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if result["status"] != "ok" {
		t.Errorf("expected status=ok after retry, got %s", result["status"])
	}
	if s.accessToken != "refreshed-access-token" {
		t.Errorf("expected access token to be refreshed, got %s", s.accessToken)
	}
	if callCount != 2 {
		t.Errorf("expected 2 calls to /test, got %d", callCount)
	}
}

// TestApiRequestInvalidTokenNoRefresh verifies that INVALID_TOKEN with
// no refresh token sets authExpired.
func TestApiRequestInvalidTokenNoRefresh(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		_, _ = w.Write([]byte(`{"error":"INVALID_TOKEN"}`))
	}))
	defer srv.Close()

	s := newTestSyncer(srv.URL)
	s.refreshToken = ""
	_, err := s.apiRequest("GET", "/test", nil)
	if err == nil {
		t.Fatal("expected error when no refresh token available")
	}
	if !s.authExpired {
		t.Error("expected authExpired to be set")
	}
}

// TestApiRequestInvalidTokenAfterRefresh verifies that INVALID_TOKEN
// after refresh sets authExpired.
func TestApiRequestInvalidTokenAfterRefresh(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/auth/refresh":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"access_token":  "new-token",
				"refresh_token": "new-refresh",
				"expires_in":    3600,
			})
		default:
			// Always return INVALID_TOKEN
			w.WriteHeader(401)
			_, _ = w.Write([]byte(`{"error":"INVALID_TOKEN"}`))
		}
	}))
	defer srv.Close()

	s := newTestSyncer(srv.URL)
	_, err := s.apiRequest("GET", "/test", nil)
	if err == nil {
		t.Fatal("expected error when INVALID_TOKEN persists after refresh")
	}
	if !s.authExpired {
		t.Error("expected authExpired to be set")
	}
}

// TestRefreshTokenSkipsWhenNotNeeded verifies that refresh is skipped
// when the token has sufficient remaining lifetime.
func TestRefreshTokenSkipsWhenNotNeeded(t *testing.T) {
	refreshCalled := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/auth/refresh" {
			refreshCalled = true
		}
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	s := newTestSyncer(srv.URL)
	s.tokenExpiry = time.Now().Add(5 * time.Minute).Unix()

	err := s.refreshTokenIfNeeded()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if refreshCalled {
		t.Error("refresh should not have been called")
	}
}

// TestRefreshTokenTriggeredNearExpiry verifies that refresh is triggered
// when the token expires within 60 seconds.
func TestRefreshTokenTriggeredNearExpiry(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/auth/refresh" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"access_token":  "refreshed-token",
				"refresh_token": "new-refresh",
				"expires_in":    3600,
			})
			return
		}
		w.WriteHeader(404)
	}))
	defer srv.Close()

	s := newTestSyncer(srv.URL)
	s.tokenExpiry = time.Now().Add(30 * time.Second).Unix() // < 60s remaining

	err := s.refreshTokenIfNeeded()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.accessToken != "refreshed-token" {
		t.Errorf("expected token to be refreshed, got %s", s.accessToken)
	}
}
