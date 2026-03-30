package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

type apiError struct {
	Error string `json:"error"`
}

func (s *Syncer) refreshTokenIfNeeded() error {
	if s.tokenExpiry == 0 || s.refreshToken == "" {
		return nil
	}
	remaining := s.tokenExpiry - time.Now().Unix()
	if remaining >= 60 {
		return nil
	}
	reqBody, _ := json.Marshal(map[string]string{"refresh_token": s.refreshToken})
	resp, err := s.client.Post(s.apiURL+"/auth/refresh", "application/json", bytes.NewReader(reqBody))
	if err != nil {
		return fmt.Errorf("token refresh request failed: %w", err)
	}
	defer resp.Body.Close()
	var result struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int64  `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("token refresh decode failed: %w", err)
	}
	if result.AccessToken == "" {
		return fmt.Errorf("token refresh returned empty access token")
	}
	s.accessToken = result.AccessToken
	if result.RefreshToken != "" {
		s.refreshToken = result.RefreshToken
	}
	if result.ExpiresIn > 0 {
		s.tokenExpiry = time.Now().Unix() + result.ExpiresIn
	}
	log.Println("[ainstruct-sync] Token refreshed")
	return nil
}

func (s *Syncer) apiRequest(method, path string, body any) ([]byte, error) {
	if err := s.refreshTokenIfNeeded(); err != nil {
		s.authExpired = true
		return nil, err
	}
	resp, err := s.doAPIRequest(method, path, body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}
	log.Printf("[ainstruct-sync] API %s %s => %d", method, path, resp.StatusCode)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		log.Printf("[ainstruct-sync] API error response body: %s", string(respBody))
		return nil, fmt.Errorf("API %s %s returned %d: %s", method, path, resp.StatusCode, string(respBody))
	}
	var apiErr apiError
	if json.Unmarshal(respBody, &apiErr) == nil && apiErr.Error == "INVALID_TOKEN" {
		if s.refreshToken == "" {
			log.Println("[ainstruct-sync] Token invalid — stopping watcher")
			s.authExpired = true
			return nil, fmt.Errorf("INVALID_TOKEN and no refresh token")
		}
		if err := s.refreshTokenIfNeeded(); err != nil {
			log.Println("[ainstruct-sync] Token refresh failed — stopping watcher")
			s.authExpired = true
			return nil, err
		}
		retryResp, retryErr := s.doAPIRequest(method, path, body)
		if retryErr != nil {
			return nil, retryErr
		}
		defer retryResp.Body.Close()
		respBody, err = io.ReadAll(retryResp.Body)
		if err != nil {
			return nil, fmt.Errorf("reading retry response: %w", err)
		}
		log.Printf("[ainstruct-sync] API %s %s (retry) => %d", method, path, retryResp.StatusCode)
		if retryResp.StatusCode < 200 || retryResp.StatusCode >= 300 {
			log.Printf("[ainstruct-sync] API retry error response body: %s", string(respBody))
			return nil, fmt.Errorf("API %s %s (retry) returned %d: %s", method, path, retryResp.StatusCode, string(respBody))
		}
		if json.Unmarshal(respBody, &apiErr) == nil && apiErr.Error == "INVALID_TOKEN" {
			log.Println("[ainstruct-sync] Token invalid after refresh — stopping watcher")
			s.authExpired = true
			return nil, fmt.Errorf("INVALID_TOKEN after refresh")
		}
	}
	return respBody, nil
}

func (s *Syncer) doAPIRequest(method, path string, body any) (*http.Response, error) {
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reader = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, s.apiURL+path, reader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+s.accessToken)
	req.Header.Set("Content-Type", "application/json")
	return s.client.Do(req)
}
