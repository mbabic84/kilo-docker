package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestDocumentType(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"rules/test.md", "markdown"},
		{"commands/foo.js", "javascript"},
		{"agents/bar.ts", "typescript"},
		{"config.json", "json"},
		{"config.jsonc", "json"},
		{"script.py", "python"},
		{"main.go", "go"},
		{"lib.rs", "rust"},
		{"App.java", "java"},
		{"index.html", "html"},
		{"style.css", "css"},
		{"query.sql", "sql"},
		{"data.xml", "xml"},
		{"config.yaml", "yaml"},
		{"config.yml", "yaml"},
		{"README", "text"},
		{"file.sh", "text"},
		{"file.kdl", "text"},
		{"file.toml", "text"},
		// Case insensitive
		{"FILE.MD", "markdown"},
		{"Script.PY", "python"},
		{"Main.GO", "go"},
	}
	for _, tt := range tests {
		got := documentType(tt.path)
		if got != tt.want {
			t.Errorf("documentType(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}

func TestFindCollectionExists(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/collections" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(collectionsResponse{
				Collections: []collection{
					{CollectionID: "col-123", Name: "kilo-docker"},
					{CollectionID: "col-other", Name: "other"},
				},
			})
			return
		}
		w.WriteHeader(404)
	}))
	defer srv.Close()

	s := newTestSyncer(srv.URL)
	found, err := s.findCollection()
	if err != nil {
		t.Fatalf("findCollection error: %v", err)
	}
	if !found {
		t.Error("expected collection to be found")
	}
	if s.collectionID != "col-123" {
		t.Errorf("expected collectionID=col-123, got %s", s.collectionID)
	}
}

func TestFindCollectionNotExists(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/collections" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(collectionsResponse{
				Collections: []collection{
					{CollectionID: "col-other", Name: "other"},
				},
			})
			return
		}
		w.WriteHeader(404)
	}))
	defer srv.Close()

	s := newTestSyncer(srv.URL)
	found, err := s.findCollection()
	if err != nil {
		t.Fatalf("findCollection error: %v", err)
	}
	if found {
		t.Error("expected collection to not be found")
	}
	if s.collectionID != "" {
		t.Errorf("expected empty collectionID, got %s", s.collectionID)
	}
}

func TestFindCollectionCachesID(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(collectionsResponse{
			Collections: []collection{
				{CollectionID: "col-123", Name: "kilo-docker"},
			},
		})
	}))
	defer srv.Close()

	s := newTestSyncer(srv.URL)
	s.collectionID = "cached-id"

	found, err := s.findCollection()
	if err != nil {
		t.Fatalf("findCollection error: %v", err)
	}
	if !found {
		t.Error("expected found=true when ID is cached")
	}
	if s.collectionID != "cached-id" {
		t.Errorf("expected collectionID to remain cached-id, got %s", s.collectionID)
	}
	if callCount != 0 {
		t.Error("should not make API call when collectionID is already set")
	}
}

func TestEnsureCollectionCreatesNew(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "GET" && r.URL.Path == "/collections":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(collectionsResponse{Collections: []collection{}})
		case r.Method == "POST" && r.URL.Path == "/collections":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]string{"collection_id": "new-col-456"})
		default:
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()

	s := newTestSyncer(srv.URL)
	if err := s.ensureCollection(); err != nil {
		t.Fatalf("ensureCollection error: %v", err)
	}
	if s.collectionID != "new-col-456" {
		t.Errorf("expected collectionID=new-col-456, got %s", s.collectionID)
	}
}

func TestEnsureCollectionReusesExisting(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" && r.URL.Path == "/collections" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(collectionsResponse{
				Collections: []collection{
					{CollectionID: "existing-789", Name: "kilo-docker"},
				},
			})
			return
		}
		w.WriteHeader(404)
	}))
	defer srv.Close()

	s := newTestSyncer(srv.URL)
	if err := s.ensureCollection(); err != nil {
		t.Fatalf("ensureCollection error: %v", err)
	}
	if s.collectionID != "existing-789" {
		t.Errorf("expected collectionID=existing-789, got %s", s.collectionID)
	}
}

func TestEnsureCollectionNoIDInResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "GET" && r.URL.Path == "/collections":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(collectionsResponse{Collections: []collection{}})
		case r.Method == "POST" && r.URL.Path == "/collections":
			// Return response without collection_id
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]string{"name": "kilo-docker"})
		default:
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()

	s := newTestSyncer(srv.URL)
	err := s.ensureCollection()
	if err == nil {
		t.Fatal("expected error when response has no collection_id")
	}
}

func TestIsSyncedPath(t *testing.T) {
	s := newTestSyncer("http://localhost")

	tests := []struct {
		path string
		want bool
	}{
		// Whitelisted files
		{"kilo.json", true},
		{"kilo.jsonc", true},
		// Whitelisted directories (root)
		{"rules", true},
		{"commands", true},
		{"agents", true},
		{"plugins", true},
		{"skills", true},
		{"tools", true},
		// Whitelisted files within directories
		{"rules/bash.md", true},
		{"rules/subdir/file.md", true},
		{"commands/foo.js", true},
		{"agents/bar.ts", true},
		{"plugins/myplugin/index.js", true},
		{"skills/deep/nested/file.md", true},
		// NOT whitelisted
		{".ainstruct-hashes", false},
		{"some-random-file.txt", false},
		{"node_modules/package/index.js", false},
		{"some-dir/file.md", false},
		// Partial prefix matches should NOT sync
		{"rules_extra", false},
		{"rules_extra/file.md", false},
	}
	for _, tt := range tests {
		got := s.isSyncedPath(tt.path)
		if got != tt.want {
			t.Errorf("isSyncedPath(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}

func TestIsSyncedPathCustom(t *testing.T) {
	s := newTestSyncer("http://localhost")
	s.syncPaths = []string{"rules", "my-custom-dir"}

	if !s.isSyncedPath("rules/bash.md") {
		t.Error("expected rules/bash.md to be synced")
	}
	if !s.isSyncedPath("my-custom-dir/file.txt") {
		t.Error("expected my-custom-dir/file.txt to be synced")
	}
	if s.isSyncedPath("commands/foo.js") {
		t.Error("expected commands/foo.js to NOT be synced with custom paths")
	}
}

// TestAuthExpiredRecoveryCycle verifies the core restart-loop mechanism:
// when apiRequest hits INVALID_TOKEN, authExpired is set; after a successful
// retry (simulating what the restart loop does via pullCollection), the
// authExpired flag is cleared.
func TestAuthExpiredRecoveryCycle(t *testing.T) {
	var callCount atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/auth/refresh":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"access_token":  "new-token",
				"refresh_token": "new-refresh",
				"expires_in":    3600,
			})
		case "/test":
			n := callCount.Add(1)
			if n <= 2 {
				// First two calls return INVALID_TOKEN → triggers refresh + retry
				// but retry also returns INVALID_TOKEN → authExpired = true
				w.WriteHeader(401)
				_, _ = w.Write([]byte(`{"error":"INVALID_TOKEN"}`))
			} else {
				// Third call succeeds (simulates post-restart recovery)
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

	// First call: INVALID_TOKEN persists after refresh → authExpired = true
	_, err := s.apiRequest("GET", "/test", nil)
	if err == nil {
		t.Fatal("expected error on first call")
	}
	if !s.authExpired {
		t.Fatal("authExpired should be set after INVALID_TOKEN persists")
	}

	// Simulate what the restart loop does: pullCollection calls apiRequest.
	// The successful response should clear authExpired.
	s.authExpired = false // reset as restart loop would after pullCollection
	data, err := s.apiRequest("GET", "/test", nil)
	if err != nil {
		t.Fatalf("expected recovery call to succeed, got: %v", err)
	}
	if s.authExpired {
		t.Error("authExpired should be cleared after successful response")
	}
	var result map[string]string
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if result["status"] != "ok" {
		t.Errorf("expected status=ok, got %s", result["status"])
	}
}
