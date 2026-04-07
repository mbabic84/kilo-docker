package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mbabic84/kilo-docker/pkg/constants"
	"github.com/mbabic84/kilo-docker/pkg/utils"
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

// defaultSyncPaths lists paths (relative to the kilo config dir) that are
// whitelisted for sync. Only files matching these paths are watched, pulled,
// and pushed. Directories are synced recursively. Everything else is ignored.
var defaultSyncPaths = []string{
	"opencode.json",
	"rules",
	"commands",
	"agents",
	"plugins",
	"skills",
	"tools",
}

// Syncer manages bidirectional sync between local config files and the
// Ainstruct REST API. It tracks content hashes to avoid redundant uploads,
// handles JWT token refresh, and manages the collection lifecycle.
type Syncer struct {
	apiURL         string
	accessToken    string
	refreshToken   string
	tokenExpiry    int64
	homeDir        string
	kiloConfigDir  string
	hashFile       string
	hashMu         sync.Mutex
	collectionID   string
	authExpired    bool
	client         *http.Client
	syncPaths      []string // whitelist of paths (relative to kilo config dir) to sync
}

// NewSyncer creates a Syncer configured from environment variables.
// Reads API URL, tokens, and token expiry from KD_AINSTRUCT_* env vars.
func NewSyncer() *Syncer {
	home := constants.GetHomeDir()
	kiloConfigDir := constants.GetKiloConfigDir()
	baseURL := os.Getenv("KD_AINSTRUCT_BASE_URL")
	if baseURL == "" {
		baseURL = constants.AinstructBaseURL
	}
	apiURL := baseURL + "/api/v1"
	var expiry int64
	if v := os.Getenv("KD_AINSTRUCT_SYNC_TOKEN_EXPIRY"); v != "" {
		expiry, _ = strconv.ParseInt(v, 10, 64)
	}
	return &Syncer{
		apiURL:        apiURL,
		accessToken:   os.Getenv("KD_AINSTRUCT_SYNC_TOKEN"),
		refreshToken:  os.Getenv("KD_AINSTRUCT_SYNC_REFRESH_TOKEN"),
		tokenExpiry:   expiry,
		homeDir:       home,
		kiloConfigDir: kiloConfigDir,
		hashFile:      filepath.Join(kiloConfigDir, ".ainstruct-hashes"),
		client:        &http.Client{Timeout: 30 * time.Second},
		syncPaths:     defaultSyncPaths,
	}
}

// isSyncedPath checks whether a relative path (relative to the kilo config
// directory, e.g. "rules/bash.md") is whitelisted for sync.
func (s *Syncer) isSyncedPath(relPath string) bool {
	for _, sp := range s.syncPaths {
		if relPath == sp {
			return true
		}
		if strings.HasPrefix(relPath, sp+"/") {
			return true
		}
	}
	return false
}

// syncedAbsDirs returns the absolute paths of all whitelisted sync directories
// that exist on disk. Used by the watcher to know which directories to monitor.
// Note: Does NOT include the root config dir to avoid watching log files, etc.
func (s *Syncer) syncedAbsDirs() []string {
	var dirs []string
	for _, sp := range s.syncPaths {
		abs := filepath.Join(s.kiloConfigDir, sp)
		if info, err := os.Stat(abs); err == nil && info.IsDir() {
			dirs = append(dirs, abs)
		} else if err != nil {
			utils.Log("[ainstruct-sync] Directory not found: %s (err=%v)\n", abs, err, utils.WithOutput())
		}
	}
	return dirs
}

// syncedAbsFiles returns the absolute paths of all whitelisted sync files
// (not directories) that exist on disk. Used by the watcher to monitor
// individual files like opencode.json.
func (s *Syncer) syncedAbsFiles() []string {
	var files []string
	for _, sp := range s.syncPaths {
		abs := filepath.Join(s.kiloConfigDir, sp)
		if info, err := os.Stat(abs); err == nil && !info.IsDir() {
			files = append(files, abs)
		}
	}
	return files
}

// pushAll walks whitelisted directories and pushes every existing file
// to the API. Used by the resync command after clearing the remote collection.
func (s *Syncer) pushAll() {
	for _, sp := range s.syncPaths {
		abs := filepath.Join(s.kiloConfigDir, sp)
		info, err := os.Stat(abs)
		if err != nil {
			continue
		}
		if !info.IsDir() {
			// Single file (e.g. opencode.json)
			if err := s.syncFile(abs); err != nil && !s.authExpired {
				utils.LogError("[ainstruct-sync] Initial push error for %s: %v\n", sp, err)
			}
			continue
		}
		// Directory — walk recursively
		_ = filepath.Walk(abs, func(path string, fi os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if fi.IsDir() {
				return nil
			}
			if err := s.syncFile(path); err != nil && !s.authExpired {
				utils.LogError("[ainstruct-sync] Initial push error for %s: %v\n", path, err)
			}
			return nil
		})
	}
}

// pushUnsynced walks whitelisted directories and pushes only files that
// have never been synced (no hash entry). This is called on startup to
// catch files that were created while sync was not running.
func (s *Syncer) pushUnsynced() {
	utils.Log("[ainstruct-sync] Checking for unsynced files...\n", utils.WithOutput())
	var syncCount int
	for _, sp := range s.syncPaths {
		abs := filepath.Join(s.kiloConfigDir, sp)
		info, err := os.Stat(abs)
		if err != nil {
			continue
		}
		if !info.IsDir() {
			// Single file (e.g. opencode.json)
		if s.isUnsynced(abs) {
			if err := s.syncFile(abs); err != nil && !s.authExpired {
				utils.LogError("[ainstruct-sync] Initial push error for %s: %v\n", sp, err)
			} else {
				syncCount++
			}
		}
		continue
	}
	// Directory — walk recursively
	_ = filepath.Walk(abs, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if fi.IsDir() {
			return nil
		}
		if s.isUnsynced(path) {
			if err := s.syncFile(path); err != nil && !s.authExpired {
				utils.LogError("[ainstruct-sync] Initial push error for %s: %v\n", path, err)
			} else {
				syncCount++
			}
		}
		return nil
	})
	}
	if syncCount > 0 {
		utils.Log("[ainstruct-sync] Synced %d unsynced file(s)\n", syncCount, utils.WithOutput())
	}
}

// isUnsynced returns true if the file has never been synced (no hash entry).
func (s *Syncer) isUnsynced(absPath string) bool {
	relPath := strings.TrimPrefix(absPath, s.kiloConfigDir+"/")
	return s.hashGet(relPath) == ""
}

type collection struct {
	CollectionID string `json:"collection_id"`
	Name         string `json:"name"`
}

type collectionsResponse struct {
	Collections []collection `json:"collections"`
}

// findCollection looks up the sync collection by name without creating it.
// Returns true if found (s.collectionID is set), false if not.
func (s *Syncer) findCollection() (bool, error) {
	if s.collectionID != "" {
		return true, nil
	}
	data, err := s.apiRequest("GET", "/collections", nil)
	if err != nil {
		return false, fmt.Errorf("listing collections: %w", err)
	}
	var cr collectionsResponse
	if err := json.Unmarshal(data, &cr); err != nil {
		return false, fmt.Errorf("parsing collections response: %w (body: %s)", err, string(data))
	}
	for _, c := range cr.Collections {
		if c.Name == collectionName {
			s.collectionID = c.CollectionID
			return true, nil
		}
	}
	return false, nil
}

// ensureCollection creates or retrieves the sync collection from the API.
// On first run, it creates a new collection named "kilo-docker"; subsequent
// runs reuse the existing one by ID.
func (s *Syncer) ensureCollection() error {
	if s.collectionID != "" {
		return nil
	}
	found, err := s.findCollection()
	if err != nil {
		return err
	}
	if found {
		utils.Log("[ainstruct-sync] Collection ready: %s\n", utils.RedactID(s.collectionID))
		return nil
	}
	body := map[string]string{"name": collectionName}
	data, err := s.apiRequest("POST", "/collections", body)
	if err != nil {
		return fmt.Errorf("creating collection: %w", err)
	}
	utils.Log("[ainstruct-sync] POST /collections response: %s\n", utils.Redact(string(data)))
	var created struct {
		CollectionID string `json:"collection_id"`
	}
	if err := json.Unmarshal(data, &created); err != nil {
		return fmt.Errorf("parsing create collection response: %w (body: %s)", err, string(data))
	}
	s.collectionID = created.CollectionID
	if s.collectionID == "" {
		return fmt.Errorf("failed to initialize collection — no collection_id in response")
	}
	utils.Log("[ainstruct-sync] Collection ready: %s\n", utils.RedactID(s.collectionID))
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
	info, err := os.Stat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if info.IsDir() {
		return nil
	}
	if err := s.ensureCollection(); err != nil {
		return err
	}
	relPath := strings.TrimPrefix(absPath, s.kiloConfigDir+"/")
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
			if err := s.hashSet(relPath, result.ContentHash); err != nil {
				utils.LogWarn("[ainstruct-sync] Warning: hash update failed for %s: %v\n", relPath, err)
			}
		}
		utils.Log("[ainstruct-sync] Updated: %s\n", relPath)
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
			if err := s.hashSet(relPath, result.ContentHash); err != nil {
				utils.LogWarn("[ainstruct-sync] Warning: hash update failed for %s: %v\n", relPath, err)
			}
		}
		utils.Log("[ainstruct-sync] Created: %s\n", relPath)
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
		if err := s.hashDelete(relPath); err != nil {
			utils.LogWarn("[ainstruct-sync] Warning: hash delete failed for %s: %v\n", relPath, err)
		}
		utils.Log("[ainstruct-sync] Deleted: %s\n", relPath)
	}
	return nil
}

// pullCollection downloads all documents from the remote collection and
// writes them to local paths, skipping files whose hash matches the remote.
// On first run (no collection), it returns nil with no action.
func (s *Syncer) pullCollection() error {
	found, err := s.findCollection()
	if err != nil {
		return err
	}
	if !found {
		utils.Log("[ainstruct-sync] No existing collection — nothing to pull\n")
		return nil
	}
	utils.Log("[ainstruct-sync] Pulling documents from collection %s\n", utils.RedactID(s.collectionID))
	data, err := s.apiRequest("GET", "/documents?collection_id="+s.collectionID, nil)
	if err != nil {
		return fmt.Errorf("listing documents: %w", err)
	}
	utils.Log("[ainstruct-sync] Pull: GET /documents response: %s\n", utils.Redact(string(data)))
	var dr documentsResponse
	if err := json.Unmarshal(data, &dr); err != nil {
		return fmt.Errorf("parsing documents response: %w (body: %s)", err, string(data))
	}
	if len(dr.Documents) == 0 {
		utils.Log("[ainstruct-sync] Collection is empty — nothing to pull\n")
		return nil
	}
	utils.Log("[ainstruct-sync] Pull: processing %d documents\n", len(dr.Documents))
	for i, doc := range dr.Documents {
		relPath := doc.Metadata.LocalPath
		utils.Log("[ainstruct-sync] Pull: doc[%d] id=%s relPath=%q contentHash=%s\n", i, utils.RedactID(doc.DocumentID), relPath, doc.ContentHash)
		if relPath == "" {
			utils.Log("[ainstruct-sync] Pull: doc[%d] skipped — empty relPath\n", i)
			continue
		}
		if !s.isSyncedPath(relPath) {
			utils.Log("[ainstruct-sync] Pull: doc[%d] %s skipped — not a synced path\n", i, relPath)
			continue
		}
		apiHash := doc.ContentHash
		storedHash := s.hashGet(relPath)
		if storedHash == apiHash {
			utils.Log("[ainstruct-sync] Pull: doc[%d] %s skipped — hash match (local=%q api=%q)\n", i, relPath, storedHash, apiHash)
			continue
		}
		utils.Log("[ainstruct-sync] Pull: doc[%d] %s fetching (localHash=%q apiHash=%q authExpired=%v)\n", i, relPath, storedHash, apiHash, s.authExpired)
		docData, err := s.apiRequest("GET", "/documents/"+doc.DocumentID, nil)
		if err != nil {
			utils.LogError("[ainstruct-sync] Failed to pull %s: %v\n", relPath, err)
			continue
		}
		var fullDoc struct {
			Content string `json:"content"`
		}
		if err := json.Unmarshal(docData, &fullDoc); err != nil {
			utils.LogError("[ainstruct-sync] Failed to parse %s: %v (body: %s)\n", relPath, err, string(docData))
			continue
		}
		if fullDoc.Content == "" {
			utils.Log("[ainstruct-sync] Pull: doc[%d] %s skipped — empty content after fetch\n", i, relPath)
			continue
		}
		absPath := filepath.Join(s.kiloConfigDir, relPath)
		_ = os.MkdirAll(filepath.Dir(absPath), 0o755)
		if err := os.WriteFile(absPath, []byte(fullDoc.Content), 0o644); err != nil {
			utils.LogError("[ainstruct-sync] Failed to write %s: %v\n", relPath, err)
			continue
		}
		if err := s.hashSet(relPath, apiHash); err != nil {
			utils.LogWarn("[ainstruct-sync] Warning: hash update failed for %s: %v\n", relPath, err)
		}
		utils.Log("[ainstruct-sync] Pulled: %s\n", relPath)
	}
	return nil
}

// deleteAllDocuments removes every document in the collection.
// Used by the reset-sync subcommand to clear stale paths.
func (s *Syncer) deleteAllDocuments() error {
	found, err := s.findCollection()
	if err != nil {
		return err
	}
	if !found {
		utils.Log("[ainstruct-sync] No collection found — nothing to delete\n", utils.WithOutput())
		return nil
	}
	data, err := s.apiRequest("GET", "/documents?collection_id="+s.collectionID, nil)
	if err != nil {
		return fmt.Errorf("listing documents: %w", err)
	}
	var dr documentsResponse
	if err := json.Unmarshal(data, &dr); err != nil {
		return fmt.Errorf("parsing documents response: %w", err)
	}
	if len(dr.Documents) == 0 {
		utils.Log("[ainstruct-sync] Collection is empty — nothing to delete\n", utils.WithOutput())
		return nil
	}
	utils.Log("[ainstruct-sync] Deleting %d documents from collection %s...\n", len(dr.Documents), utils.RedactID(s.collectionID), utils.WithOutput())
	for _, doc := range dr.Documents {
		if _, err := s.apiRequest("DELETE", "/documents/"+doc.DocumentID, nil); err != nil {
			utils.LogError("[ainstruct-sync] Failed to delete %s (%s): %v\n", doc.Metadata.LocalPath, utils.RedactID(doc.DocumentID), err)
			continue
		}
		utils.Log("[ainstruct-sync] Deleted: %s\n", doc.Metadata.LocalPath, utils.WithOutput())
	}
	utils.Log("[ainstruct-sync] Done. Restart the container to re-sync with correct paths.\n", utils.WithOutput())
	return nil
}
