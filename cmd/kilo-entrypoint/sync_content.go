package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

const collectionName = "kilo-docker"

// documentType returns the ainstruct document type string for a given file
// path based on its extension. Falls back to "text" for unknown extensions.
func documentType(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".md":
		return "markdown"
	case ".js":
		return "javascript"
	case ".ts":
		return "typescript"
	case ".json", ".jsonc":
		return "json"
	case ".py":
		return "python"
	case ".go":
		return "go"
	case ".rs":
		return "rust"
	case ".java":
		return "java"
	case ".html":
		return "html"
	case ".css":
		return "css"
	case ".sql":
		return "sql"
	case ".xml":
		return "xml"
	case ".yaml", ".yml":
		return "yaml"
	default:
		return "text"
	}
}

// Syncer manages bidirectional sync between local config files and the
// Ainstruct REST API. It tracks content hashes to avoid redundant uploads,
// handles JWT token refresh, and manages the collection lifecycle.
type Syncer struct {
	apiURL       string
	accessToken  string
	refreshToken string
	tokenExpiry  int64
	homeDir      string
	hashFile     string
	hashMu       sync.Mutex
	collectionID string
	authExpired  bool
	client       *http.Client
}

// NewSyncer creates a Syncer configured from environment variables.
// Reads API URL, tokens, and token expiry from KD_AINSTRUCT_* env vars.
func NewSyncer() *Syncer {
	home := os.Getenv("HOME")
	apiURL := os.Getenv("KD_AINSTRUCT_API_URL")
	if apiURL == "" {
		apiURL = "https://ainstruct-dev.kralicinora.cz/api/v1"
	}
	var expiry int64
	if v := os.Getenv("KD_AINSTRUCT_SYNC_TOKEN_EXPIRY"); v != "" {
		expiry, _ = strconv.ParseInt(v, 10, 64)
	}
	return &Syncer{
		apiURL:       apiURL,
		accessToken:  os.Getenv("KD_AINSTRUCT_SYNC_TOKEN"),
		refreshToken: os.Getenv("KD_AINSTRUCT_SYNC_REFRESH_TOKEN"),
		tokenExpiry:  expiry,
		homeDir:      home,
		hashFile:     filepath.Join(home, ".config", "kilo", ".ainstruct-hashes"),
		client:       &http.Client{Timeout: 30 * time.Second},
	}
}

type collection struct {
	CollectionID string `json:"collection_id"`
	Name         string `json:"name"`
}

type collectionsResponse struct {
	Collections []collection `json:"collections"`
}

// ensureCollection creates or retrieves the sync collection from the API.
// On first run, it creates a new collection named "kilo-docker"; subsequent
// runs reuse the existing one by ID.
func (s *Syncer) ensureCollection() error {
	if s.collectionID != "" {
		return nil
	}
	data, err := s.apiRequest("GET", "/collections", nil)
	if err != nil {
		return fmt.Errorf("listing collections: %w", err)
	}
	log.Printf("[ainstruct-sync] GET /collections response: %s", string(data))
	var cr collectionsResponse
	if err := json.Unmarshal(data, &cr); err != nil {
		return fmt.Errorf("parsing collections response: %w (body: %s)", err, string(data))
	}
	for _, c := range cr.Collections {
		if c.Name == collectionName {
			s.collectionID = c.CollectionID
			break
		}
	}
	if s.collectionID == "" {
		body := map[string]string{"name": collectionName}
		data, err = s.apiRequest("POST", "/collections", body)
		if err != nil {
			return fmt.Errorf("creating collection: %w", err)
		}
		log.Printf("[ainstruct-sync] POST /collections response: %s", string(data))
		var created struct {
			CollectionID string `json:"collection_id"`
		}
		if err := json.Unmarshal(data, &created); err != nil {
			return fmt.Errorf("parsing create collection response: %w (body: %s)", err, string(data))
		}
		s.collectionID = created.CollectionID
	}
	if s.collectionID == "" {
		return fmt.Errorf("failed to initialize collection — no collection_id in response")
	}
	log.Printf("[ainstruct-sync] Collection ready: %s", s.collectionID)
	return nil
}

type document struct {
	DocumentID  string `json:"document_id"`
	Content     string `json:"content"`
	ContentHash string `json:"content_hash"`
	Metadata    struct {
		LocalPath string `json:"local_path"`
	} `json:"metadata"`
}

type documentsResponse struct {
	Documents []document `json:"documents"`
}

// getDocumentByPath looks up a document in the sync collection by its
// local file path metadata. Returns nil if not found.
func (s *Syncer) getDocumentByPath(relPath string) (*document, error) {
	data, err := s.apiRequest("GET", "/documents?collection_id="+s.collectionID, nil)
	if err != nil {
		return nil, err
	}
	var dr documentsResponse
	if err := json.Unmarshal(data, &dr); err != nil {
		return nil, fmt.Errorf("parsing documents response: %w (body: %s)", err, string(data))
	}
	for _, d := range dr.Documents {
		if d.Metadata.LocalPath == relPath {
			return &d, nil
		}
	}
	return nil, nil
}

// syncFile uploads or updates a local file in the Ainstruct collection.
// Creates a new document if it doesn't exist, patches the existing one
// if it does. Updates the local hash cache on success.
func (s *Syncer) syncFile(absPath string) error {
	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		return nil
	}
	if err := s.ensureCollection(); err != nil {
		return err
	}
	relPath := strings.TrimPrefix(absPath, s.homeDir+"/")
	title := filepath.Base(absPath)
	content, err := os.ReadFile(absPath)
	if err != nil {
		return fmt.Errorf("reading %s: %w", absPath, err)
	}
	existing, err := s.getDocumentByPath(relPath)
	if err != nil {
		return err
	}
	if s.authExpired {
		return fmt.Errorf("auth expired")
	}
	if existing != nil {
		body := map[string]string{"content": string(content)}
		data, err := s.apiRequest("PATCH", "/documents/"+existing.DocumentID, body)
		if err != nil {
			return err
		}
		if s.authExpired {
			return fmt.Errorf("auth expired")
		}
		var result struct {
			ContentHash string `json:"content_hash"`
		}
		if err := json.Unmarshal(data, &result); err != nil {
			return fmt.Errorf("parsing PATCH response: %w (body: %s)", err, string(data))
		}
		if result.ContentHash != "" {
			s.hashSet(relPath, result.ContentHash)
		}
		log.Printf("[ainstruct-sync] Updated: %s", relPath)
	} else {
		body := map[string]any{
			"title":         title,
			"content":       string(content),
			"document_type": documentType(absPath),
			"collection_id": s.collectionID,
			"metadata":      map[string]string{"local_path": relPath},
		}
		data, err := s.apiRequest("POST", "/documents", body)
		if err != nil {
			return err
		}
		if s.authExpired {
			return fmt.Errorf("auth expired")
		}
		var result struct {
			ContentHash string `json:"content_hash"`
		}
		if err := json.Unmarshal(data, &result); err != nil {
			return fmt.Errorf("parsing POST response: %w (body: %s)", err, string(data))
		}
		if result.ContentHash != "" {
			s.hashSet(relPath, result.ContentHash)
		}
		log.Printf("[ainstruct-sync] Created: %s", relPath)
	}
	return nil
}

// deleteByPath removes a document from the Ainstruct collection by its
// local file path metadata. Removes the hash cache entry on success.
func (s *Syncer) deleteByPath(relPath string) error {
	if err := s.ensureCollection(); err != nil {
		return err
	}
	existing, err := s.getDocumentByPath(relPath)
	if err != nil {
		return err
	}
	if s.authExpired {
		return fmt.Errorf("auth expired")
	}
	if existing != nil {
		_, err := s.apiRequest("DELETE", "/documents/"+existing.DocumentID, nil)
		if err != nil {
			return err
		}
		if s.authExpired {
			return fmt.Errorf("auth expired")
		}
		s.hashDelete(relPath)
		log.Printf("[ainstruct-sync] Deleted: %s", relPath)
	}
	return nil
}

// pullCollection downloads all documents from the remote collection and
// writes them to local paths, skipping files whose hash matches the remote.
// On first run (no collection), it returns nil with no action.
func (s *Syncer) pullCollection() error {
	data, err := s.apiRequest("GET", "/collections", nil)
	if err != nil {
		return fmt.Errorf("listing collections: %w", err)
	}
	log.Printf("[ainstruct-sync] Pull: GET /collections response: %s", string(data))
	var cr collectionsResponse
	if err := json.Unmarshal(data, &cr); err != nil {
		return fmt.Errorf("parsing collections response: %w (body: %s)", err, string(data))
	}
	for _, c := range cr.Collections {
		if c.Name == collectionName {
			s.collectionID = c.CollectionID
			break
		}
	}
	if s.collectionID == "" {
		log.Println("[ainstruct-sync] No existing collection — nothing to pull")
		return nil
	}
	log.Printf("[ainstruct-sync] Pulling documents from collection %s", s.collectionID)
	data, err = s.apiRequest("GET", "/documents?collection_id="+s.collectionID, nil)
	if err != nil {
		return fmt.Errorf("listing documents: %w", err)
	}
	log.Printf("[ainstruct-sync] Pull: GET /documents response: %s", string(data))
	var dr documentsResponse
	if err := json.Unmarshal(data, &dr); err != nil {
		return fmt.Errorf("parsing documents response: %w (body: %s)", err, string(data))
	}
	if len(dr.Documents) == 0 {
		log.Println("[ainstruct-sync] Collection is empty — nothing to pull")
		return nil
	}
	for _, doc := range dr.Documents {
		relPath := doc.Metadata.LocalPath
		if relPath == "" {
			continue
		}
		apiHash := doc.ContentHash
		storedHash := s.hashGet(relPath)
		if storedHash == apiHash {
			continue
		}
		docData, err := s.apiRequest("GET", "/documents/"+doc.DocumentID, nil)
		if err != nil {
			log.Printf("[ainstruct-sync] Failed to pull %s: %v", relPath, err)
			continue
		}
		var fullDoc struct {
			Content string `json:"content"`
		}
		if err := json.Unmarshal(docData, &fullDoc); err != nil {
			log.Printf("[ainstruct-sync] Failed to parse %s: %v (body: %s)", relPath, err, string(docData))
			continue
		}
		if fullDoc.Content == "" {
			continue
		}
		absPath := filepath.Join(s.homeDir, relPath)
		os.MkdirAll(filepath.Dir(absPath), 0o755)
		if err := os.WriteFile(absPath, []byte(fullDoc.Content), 0o644); err != nil {
			log.Printf("[ainstruct-sync] Failed to write %s: %v", relPath, err)
			continue
		}
		s.hashSet(relPath, apiHash)
		log.Printf("[ainstruct-sync] Pulled: %s", relPath)
	}
	return nil
}
