package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
			json.NewEncoder(w).Encode(collectionsResponse{
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
			json.NewEncoder(w).Encode(collectionsResponse{
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
		json.NewEncoder(w).Encode(collectionsResponse{
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
			json.NewEncoder(w).Encode(collectionsResponse{Collections: []collection{}})
		case r.Method == "POST" && r.URL.Path == "/collections":
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"collection_id": "new-col-456"})
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
			json.NewEncoder(w).Encode(collectionsResponse{
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
			json.NewEncoder(w).Encode(collectionsResponse{Collections: []collection{}})
		case r.Method == "POST" && r.URL.Path == "/collections":
			// Return response without collection_id
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"name": "kilo-docker"})
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
