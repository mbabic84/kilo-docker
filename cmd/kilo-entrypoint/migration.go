package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/mbabic84/kilo-docker/pkg/constants"
	"github.com/mbabic84/kilo-docker/pkg/utils"
)

// checkRemoteHasConfig queries the ainstruct API to determine whether a
// kilo.jsonc config file already exists in the remote collection. Used during
// first-time init to decide whether to push the local config template.
func checkRemoteHasConfig(homeDir, userID string) (bool, error) {
	utils.Log("[userinit] checkRemote: Checking remote for kilo.jsonc (first time init)\n")

	encPath := filepath.Join(homeDir, ".local/share/kilo-docker/.tokens.env.enc")
	encData, err := os.ReadFile(encPath)
	if err != nil {
		utils.LogWarn("[userinit] checkRemote: no encrypted tokens found: %v\n", err)
		return false, fmt.Errorf("no encrypted tokens found: %w", err)
	}

	decrypted, err := decryptAES(encData, userID)
	if err != nil {
		utils.LogWarn("[userinit] checkRemote: failed to decrypt tokens: %v\n", err)
		return false, fmt.Errorf("failed to decrypt tokens: %w", err)
	}

	_, _, syncToken, _, _, _, _ := parseTokenEnv(string(decrypted))
	if syncToken == "" {
		utils.LogWarn("[userinit] checkRemote: no sync token available\n")
		return false, fmt.Errorf("no sync token available")
	}

	baseURL := os.Getenv("KD_AINSTRUCT_BASE_URL")
	if baseURL == "" {
		baseURL = constants.AinstructBaseURL
	}

	collectionsURL := baseURL + "/api/v1/collections"
	req, err := http.NewRequest("GET", collectionsURL, nil)
	if err != nil {
		return false, err
	}
	req.Header.Set("Authorization", "Bearer "+syncToken)

	client := &http.Client{Timeout: 10 * time.Second}
	utils.Log("[userinit] checkRemote: GET /collections\n")
	resp, err := client.Do(req)
	if err != nil {
		utils.LogWarn("[userinit] checkRemote: collections request failed: %v\n", err)
		return false, err
	}

	if resp.StatusCode != 200 {
		_ = resp.Body.Close()
		utils.LogWarn("[userinit] checkRemote: collections API returned %d\n", resp.StatusCode)
		return false, fmt.Errorf("collections API returned %d", resp.StatusCode)
	}

	var result struct {
		Collections []struct {
			CollectionID string `json:"collection_id"`
			Name         string `json:"name"`
		} `json:"collections"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		_ = resp.Body.Close()
		utils.LogWarn("[userinit] checkRemote: failed to decode collections response: %v\n", err)
		return false, err
	}
	_ = resp.Body.Close()

	var collectionID string
	for _, c := range result.Collections {
		if c.Name == "kilo-docker" {
			collectionID = c.CollectionID
			utils.Log("[userinit] checkRemote: found collection %s\n", utils.RedactID(collectionID))
			break
		}
	}

	if collectionID == "" {
		utils.Log("[userinit] checkRemote: no kilo-docker collection found\n")
		return false, nil
	}

	docsURL := baseURL + "/api/v1/documents?collection_id=" + collectionID
	req, err = http.NewRequest("GET", docsURL, nil)
	if err != nil {
		return false, err
	}
	req.Header.Set("Authorization", "Bearer "+syncToken)

	utils.Log("[userinit] checkRemote: GET /documents for collection %s\n", utils.RedactID(collectionID))
	resp, err = client.Do(req)
	if err != nil {
		utils.LogWarn("[userinit] checkRemote: documents request failed: %v\n", err)
		return false, err
	}

	if resp.StatusCode != 200 {
		_ = resp.Body.Close()
		utils.LogWarn("[userinit] checkRemote: documents API returned %d\n", resp.StatusCode)
		return false, fmt.Errorf("documents API returned %d", resp.StatusCode)
	}

	var docsResult struct {
		Documents []struct {
			Metadata struct {
				LocalPath string `json:"local_path"`
			} `json:"metadata"`
		} `json:"documents"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&docsResult); err != nil {
		_ = resp.Body.Close()
		utils.LogWarn("[userinit] checkRemote: failed to decode documents response: %v\n", err)
		return false, err
	}
	_ = resp.Body.Close()

	for _, d := range docsResult.Documents {
		if d.Metadata.LocalPath == "kilo.jsonc" || d.Metadata.LocalPath == "kilo.json" {
			utils.Log("[userinit] checkRemote: found %s in remote collection\n", d.Metadata.LocalPath)
			return true, nil
		}
	}

	utils.Log("[userinit] checkRemote: no kilo.jsonc/kilo.json in remote collection\n")
	return false, nil
}

// deleteRemoteOpencode deletes opencode.json from the remote ainstruct collection.
// Called after successful migration to kilo.jsonc to clean up old config.
func deleteRemoteOpencode(homeDir, userID string) error {
	utils.Log("[userinit] deleteRemote: Checking for old opencode.json in remote\n")

	encPath := filepath.Join(homeDir, ".local/share/kilo-docker/.tokens.env.enc")
	encData, err := os.ReadFile(encPath)
	if err != nil {
		utils.LogWarn("[userinit] deleteRemote: no encrypted tokens: %v\n", err)
		return err
	}

	decrypted, err := decryptAES(encData, userID)
	if err != nil {
		utils.LogWarn("[userinit] deleteRemote: decrypt failed: %v\n", err)
		return err
	}

	_, _, syncToken, _, _, _, _ := parseTokenEnv(string(decrypted))
	if syncToken == "" {
		return fmt.Errorf("no sync token")
	}

	baseURL := os.Getenv("KD_AINSTRUCT_BASE_URL")
	if baseURL == "" {
		baseURL = constants.AinstructBaseURL
	}

	collectionsURL := baseURL + "/api/v1/collections"
	req, err := http.NewRequest("GET", collectionsURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+syncToken)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != 200 {
		return fmt.Errorf("collections API returned %d", resp.StatusCode)
	}

	var result struct {
		Collections []struct {
			CollectionID string `json:"collection_id"`
			Name         string `json:"name"`
		} `json:"collections"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}

	var collectionID string
	for _, c := range result.Collections {
		if c.Name == "kilo-docker" {
			collectionID = c.CollectionID
			break
		}
	}

	if collectionID == "" {
		return nil
	}

	docsURL := baseURL + "/api/v1/documents?collection_id=" + collectionID
	req, err = http.NewRequest("GET", docsURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+syncToken)

	resp, err = client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != 200 {
		return fmt.Errorf("documents API returned %d", resp.StatusCode)
	}

	var docsResult struct {
		Documents []struct {
			DocumentID string `json:"document_id"`
			Metadata   struct {
				LocalPath string `json:"local_path"`
			} `json:"metadata"`
		} `json:"documents"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&docsResult); err != nil {
		return err
	}

	for _, d := range docsResult.Documents {
		if d.Metadata.LocalPath == "opencode.json" {
			utils.Log("[userinit] deleteRemote: deleting opencode.json from remote\n")
			deleteURL := baseURL + "/api/v1/documents/" + d.DocumentID
			req, err = http.NewRequest("DELETE", deleteURL, nil)
			if err != nil {
				return err
			}
			req.Header.Set("Authorization", "Bearer "+syncToken)

			resp, err = client.Do(req)
			if err != nil {
				utils.LogWarn("[userinit] deleteRemote: delete failed: %v\n", err)
				return err
			}
			_ = resp.Body.Close()
			utils.Log("[userinit] deleteRemote: deleted opencode.json from remote\n")
			return nil
		}
	}

	utils.Log("[userinit] deleteRemote: no opencode.json found in remote\n")
	return nil
}
