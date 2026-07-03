package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
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
	"kilo.json",
	"kilo.jsonc",
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
	apiURL            string
	accessToken       string
	refreshToken      string
	tokenExpiry       int64
	homeDir           string
	kiloConfigDir     string
	kiloDockerDataDir string
	hashFile          string
	localHashFile     string
	hashMu            sync.Mutex
	lockFile          string
	collectionID      string
	authExpired       bool
	client            *http.Client
	syncPaths         []string
	saveTokensFn      func()
	refreshMu         sync.Mutex
}

// NewSyncer creates a Syncer configured from encrypted token storage.
// Reads tokens from <home>/.local/share/kilo-docker/.tokens.env.enc and falls back
// to environment variables for backward compatibility.
func NewSyncer() *Syncer {
	home := constants.GetHomeDir()
	kiloConfigDir := constants.GetKiloConfigDir()
	kiloDockerDataDir := constants.GetKiloDockerConfigDir()
	baseURL := os.Getenv("KD_AINSTRUCT_BASE_URL")
	if baseURL == "" {
		baseURL = constants.AinstructBaseURL
	}
	apiURL := baseURL + "/api/v1"

	var accessToken, refreshToken string
	var expiry int64

	homeDir, _, _, userID := loadUserConfig()
	if homeDir != "" && userID != "" {
		encPath := filepath.Join(homeDir, ".local/share/kilo-docker/.tokens.env.enc")
		if encData, err := os.ReadFile(encPath); err == nil {
			if decrypted, err := decryptAES(encData, userID); err == nil {
				var expiryStr string
				_, _, accessToken, refreshToken, expiryStr, _, _ = parseTokenEnv(string(decrypted))
				if expiryStr != "" {
					expiry, _ = strconv.ParseInt(expiryStr, 10, 64)
				}
				utils.Log("[ainstruct-sync] Loaded tokens from encrypted storage\n")
			}
		}
	}

	if accessToken == "" {
		utils.LogError("[ainstruct-sync] No sync tokens found — please run login first\n")
	}

	return &Syncer{
		apiURL:            apiURL,
		accessToken:       accessToken,
		refreshToken:      refreshToken,
		tokenExpiry:       expiry,
		homeDir:           home,
		kiloConfigDir:     kiloConfigDir,
		kiloDockerDataDir: kiloDockerDataDir,
		hashFile:          filepath.Join(kiloDockerDataDir, ".ainstruct-hashes"),
		localHashFile:     filepath.Join(kiloDockerDataDir, ".ainstruct-local-hashes"),
		lockFile:          filepath.Join(kiloDockerDataDir, ".sync.lock"),
		client:            &http.Client{Timeout: 30 * time.Second},
		syncPaths:         defaultSyncPaths,
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
			utils.Log("[ainstruct-sync] Directory not found: %s (err=%v)\n", abs, err)
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
	utils.Log("[ainstruct-sync] Checking for unsynced files...\n")
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
		utils.Log("[ainstruct-sync] Synced %d unsynced file(s)\n", syncCount)
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
	DocumentID  string            `json:"document_id"`
	Title       string            `json:"title"`
	ContentHash string            `json:"content_hash"`
	CreatedAt   flexTime          `json:"created_at"`
	Metadata    map[string]string `json:"metadata"`
}

type documentsResponse struct {
	Documents []document `json:"documents"`
	Total     int        `json:"total"`
	Limit     int        `json:"limit"`
	Offset    int        `json:"offset"`
}

// getDocumentByPath looks up a document in the sync collection by its
// local file path metadata. Returns nil if not found.
func (s *Syncer) getDocumentByPath(relPath string) (*document, error) {
	docs, err := s.listDocuments()
	if err != nil {
		return nil, err
	}
	return findDocumentByPath(docs, relPath), nil
}

// findDocumentByPath returns the first document in docs whose local_path
// matches relPath, or nil if not found.
func findDocumentByPath(docs []document, relPath string) *document {
	for i := range docs {
		if docs[i].Metadata["local_path"] == relPath {
			return &docs[i]
		}
	}
	return nil
}

// listDocuments returns all documents in the sync collection.
func (s *Syncer) listDocuments() ([]document, error) {
	data, err := s.apiRequest("GET", "/documents?collection_id="+s.collectionID, nil)
	if err != nil {
		return nil, err
	}
	var dr documentsResponse
	if err := json.Unmarshal(data, &dr); err != nil {
		return nil, fmt.Errorf("parsing documents response: %w (body: %s)", err, string(data))
	}
	return dr.Documents, nil
}

// deduplicateByPath removes extra documents that share the same local_path,
// keeping only the most recently created one. This is a safety net for
// race conditions where multiple sync processes create documents simultaneously.
// If allDocs is provided, it is used instead of fetching from the API.
// Safe: skips deletion when timestamps are equal or zero, preventing
// accidental deletion of the wrong copy.
func (s *Syncer) deduplicateByPath(relPath string, allDocs []document) error {
	if allDocs == nil {
		data, err := s.apiRequest("GET", "/documents?collection_id="+s.collectionID, nil)
		if err != nil {
			return err
		}
		var dr documentsResponse
		if err := json.Unmarshal(data, &dr); err != nil {
			return err
		}
		allDocs = dr.Documents
	}
	var matches []document
	for _, d := range allDocs {
		if d.Metadata["local_path"] == relPath {
			matches = append(matches, d)
		}
	}
	if len(matches) <= 1 {
		return nil
	}
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].CreatedAt.Before(matches[j].CreatedAt.Time)
	})
	if matches[0].CreatedAt.Equal(matches[len(matches)-1].CreatedAt.Time) {
		allSameHash := true
		for i := 1; i < len(matches); i++ {
			if matches[i].ContentHash != matches[0].ContentHash {
				allSameHash = false
				break
			}
		}
		if !allSameHash {
			utils.Log("[ainstruct-sync] %d documents for %s share same created_at but differ in content, skipping dedup\n", len(matches), relPath)
			return nil
		}
		utils.Log("[ainstruct-sync] %d documents for %s share same created_at and content, cleaning up\n", len(matches), relPath)
		for i := 1; i < len(matches); i++ {
			if _, err := s.apiRequest("DELETE", "/documents/"+matches[i].DocumentID, nil); err != nil {
				utils.LogWarn("[ainstruct-sync] Failed to delete duplicate %s: %v\n", matches[i].DocumentID, err)
			}
		}
		return nil
	}
	utils.Log("[ainstruct-sync] Found %d duplicates for %s, cleaning up\n", len(matches), relPath)
	for i := 0; i < len(matches)-1; i++ {
		if _, err := s.apiRequest("DELETE", "/documents/"+matches[i].DocumentID, nil); err != nil {
			utils.LogWarn("[ainstruct-sync] Failed to delete duplicate %s: %v\n", matches[i].DocumentID, err)
		}
	}
	return nil
}

// cleanupDuplicates removes duplicate documents that share the same local_path,
// keeping only the most recently created one per path.
func (s *Syncer) cleanupDuplicates() error {
	lock, err := utils.Acquire(s.lockFile, true)
	if err != nil {
		return fmt.Errorf("acquiring sync lock for cleanup: %w", err)
	}
	defer lock.Release() //nolint:errcheck // best-effort release

	data, err := s.apiRequest("GET", "/documents?collection_id="+s.collectionID, nil)
	if err != nil {
		return err
	}
	var dr documentsResponse
	if err := json.Unmarshal(data, &dr); err != nil {
		return err
	}
	byPath := make(map[string][]document)
	for _, d := range dr.Documents {
		if d.Metadata["local_path"] == "" {
			continue
		}
		byPath[d.Metadata["local_path"]] = append(byPath[d.Metadata["local_path"]], d)
	}
	var deleted int
	for path, docs := range byPath {
		if len(docs) <= 1 {
			continue
		}
		sort.Slice(docs, func(i, j int) bool {
			return docs[i].CreatedAt.Before(docs[j].CreatedAt.Time)
		})
		if docs[0].CreatedAt.Equal(docs[len(docs)-1].CreatedAt.Time) {
			allSameHash := true
			for i := 1; i < len(docs); i++ {
				if docs[i].ContentHash != docs[0].ContentHash {
					allSameHash = false
					break
				}
			}
			if !allSameHash {
				utils.Log("[ainstruct-sync] %d documents for %s share same created_at but differ in content, skipping\n", len(docs), path)
				continue
			}
			utils.Log("[ainstruct-sync] %d documents for %s share same created_at and content, cleaning up\n", len(docs), path)
			for i := 1; i < len(docs); i++ {
				if _, err := s.apiRequest("DELETE", "/documents/"+docs[i].DocumentID, nil); err != nil {
					utils.LogWarn("[ainstruct-sync] Failed to delete duplicate %s: %v\n", docs[i].DocumentID, err)
				} else {
					deleted++
				}
			}
			continue
		}
		utils.Log("[ainstruct-sync] Cleaning %d duplicates for %s\n", len(docs), path)
		for i := 0; i < len(docs)-1; i++ {
			if _, err := s.apiRequest("DELETE", "/documents/"+docs[i].DocumentID, nil); err != nil {
				utils.LogWarn("[ainstruct-sync] Failed to delete duplicate %s: %v\n", docs[i].DocumentID, err)
			} else {
				deleted++
			}
		}
	}
	if deleted > 0 {
		utils.Log("[ainstruct-sync] Removed %d duplicate document(s)\n", deleted)
	}
	return nil
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

	lock, err := utils.Acquire(s.lockFile, true)
	if err != nil {
		return fmt.Errorf("acquiring sync lock: %w", err)
	}
	defer lock.Release() //nolint:errcheck // best-effort release

	return s.syncFileLocked(absPath, relPath)
}

// syncFileLocked performs the actual sync after the flock is held.
func (s *Syncer) syncFileLocked(absPath, relPath string) error {
	title := filepath.Base(absPath)
	content, err := os.ReadFile(absPath)
	if err != nil {
		return fmt.Errorf("reading %s: %w", absPath, err)
	}
	localHash := computeLocalHash(content)
	if s.localHashGet(relPath) == localHash {
		utils.Log("[ainstruct-sync] Skipped (unchanged): %s\n", relPath)
		return nil
	}
	allDocs, err := s.listDocuments()
	if err != nil {
		return err
	}
	existing := findDocumentByPath(allDocs, relPath)
	if s.authExpired {
		return fmt.Errorf("auth expired")
	}
	if existing != nil {
		if err := s.patchDocument(existing, relPath, content); err != nil {
			return err
		}
	} else {
		if err := s.createDocument(absPath, relPath, title, content, allDocs); err != nil {
			return err
		}
	}
	if err := s.localHashSet(relPath, localHash); err != nil {
		utils.LogWarn("[ainstruct-sync] Warning: local hash update failed for %s: %v\n", relPath, err)
	}
	return nil
}

func (s *Syncer) patchDocument(existing *document, relPath string, content []byte) error {
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
	return nil
}

func (s *Syncer) createDocument(absPath, relPath, title string, content []byte, allDocs []document) error {
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
	if err := s.deduplicateByPath(relPath, allDocs); err != nil {
		utils.LogWarn("[ainstruct-sync] Dedup warning for %s: %v\n", relPath, err)
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
		if err := s.localHashDelete(relPath); err != nil {
			utils.LogWarn("[ainstruct-sync] Warning: local hash delete failed for %s: %v\n", relPath, err)
		}
		utils.Log("[ainstruct-sync] Deleted: %s\n", relPath)
	}
	return nil
}

// listSyncFiles lists all ainstruct sync files with their metadata
func (s *Syncer) listSyncFiles(humanReadable bool) error {
	if err := s.ensureCollection(); err != nil {
		return err
	}

	data, err := s.apiRequest("GET", "/documents?collection_id="+s.collectionID, nil)
	if err != nil {
		return fmt.Errorf("listing documents: %w", err)
	}

	var dr documentsResponse
	if err := json.Unmarshal(data, &dr); err != nil {
		return fmt.Errorf("parsing documents response: %w (body: %s)", err, string(data))
	}

	if len(dr.Documents) == 0 {
		logSyncOutput("No sync files found\n")
		return nil
	}

	logSyncOutput("%-50s %-12s %-20s\n", "FILE", "SIZE", "MODIFIED")
	logSyncOutput("%-50s %-12s %-20s\n", "----", "----", "--------")

	for _, doc := range dr.Documents {
		if doc.Metadata["local_path"] == "" {
			continue
		}
		if !s.isSyncedPath(doc.Metadata["local_path"]) {
			continue
		}
		
		// Try to get file info for size and modification time
		size := "-"
		modTime := "-"
		
		absPath := filepath.Join(s.kiloConfigDir, doc.Metadata["local_path"])
		if info, err := os.Stat(absPath); err == nil {
			if humanReadable {
				size = formatFileSize(info.Size())
			} else {
				size = fmt.Sprintf("%d B", info.Size())
			}
			modTime = info.ModTime().Format("2006-01-02 15:04")
		}
		
		logSyncOutput("%-50s %-12s %-20s\n", doc.Metadata["local_path"], size, modTime)
	}

	return nil
}

// formatFileSize converts a size in bytes to a human-readable format
func formatFileSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func logSyncOutput(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	utils.Log("[ainstruct-sync] "+msg, utils.WithOutput())
}

// removeSyncFile removes a specific sync file (both local and remote copies)
func (s *Syncer) removeSyncFile(filePath string) error {
	if filePath == "" {
		return fmt.Errorf("file path cannot be empty")
	}
	
	// Make path relative to kilo config dir if it's absolute
	relPath := filePath
	if filepath.IsAbs(filePath) {
		relPath = strings.TrimPrefix(filePath, s.kiloConfigDir+"/")
		if relPath == filePath {
			// Not actually under kilo config dir
			return fmt.Errorf("file %s is not under kilo config directory", filePath)
		}
	}
	
	// Check if file is in sync paths
	if !s.isSyncedPath(relPath) {
		return fmt.Errorf("file %s is not a synced path", relPath)
	}
	
	// Prompt for confirmation
	logSyncOutput("Are you sure you want to remove '%s'? This will delete both local and remote copies. [y/N] ", relPath)
	var response string
	if _, err := fmt.Scanln(&response); err != nil {
		return fmt.Errorf("failed to read input: %w", err)
	}
	if strings.ToLower(response) != "y" && strings.ToLower(response) != "yes" {
		logSyncOutput("Removal cancelled\n")
		return nil
	}
	
	// Remove local file
	absPath := filepath.Join(s.kiloConfigDir, relPath)
	if err := os.Remove(absPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove local file: %w", err)
	}
	
	// Remove remote document
	if err := s.deleteByPath(relPath); err != nil {
		return fmt.Errorf("failed to remove remote file: %w", err)
	}
	
	// Remove hash entry
	if err := s.hashDelete(relPath); err != nil {
		utils.LogWarn("[ainstruct-sync] Warning: hash delete failed for %s: %v\n", relPath, err)
	}
	
	logSyncOutput("Removed '%s'\n", relPath)
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
		relPath := doc.Metadata["local_path"]
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
		_ = os.MkdirAll(filepath.Dir(absPath), 0o700)
		if err := os.WriteFile(absPath, []byte(fullDoc.Content), 0o600); err != nil {
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
		utils.Log("[ainstruct-sync] No collection found — nothing to delete\n")
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
		utils.Log("[ainstruct-sync] Collection is empty — nothing to delete\n")
		return nil
	}
	utils.Log("[ainstruct-sync] Deleting %d documents from collection %s...\n", len(dr.Documents), utils.RedactID(s.collectionID))
	for _, doc := range dr.Documents {
		if _, err := s.apiRequest("DELETE", "/documents/"+doc.DocumentID, nil); err != nil {
			utils.LogError("[ainstruct-sync] Failed to delete %s (%s): %v\n", doc.Metadata["local_path"], utils.RedactID(doc.DocumentID), err)
			continue
		}
		utils.Log("[ainstruct-sync] Deleted: %s\n", doc.Metadata["local_path"])
	}
	utils.Log("[ainstruct-sync] Done. Restart the container to re-sync with correct paths.\n")
	return nil
}
