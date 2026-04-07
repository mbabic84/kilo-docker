package utils

import (
	"regexp"
	"strings"
)

var uuidPattern = regexp.MustCompile(`[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`)

var sensitiveKeys = []string{
	"user_id",
	"collection_id",
	"document_id",
	"access_token",
	"refresh_token",
	"userID",
	"collectionID",
	"documentID",
}

func RedactID(id string) string {
	if id == "" {
		return "<empty>"
	}
	if len(id) <= 8 {
		return "***"
	}
	return id[:2] + "..." + id[len(id)-2:]
}

func RedactToken(token string) string {
	if token == "" {
		return "<empty>"
	}
	if len(token) <= 8 {
		return "***"
	}
	return token[:4] + "..." + token[len(token)-4:]
}

func Redact(s string) string {
	result := s

	result = uuidPattern.ReplaceAllStringFunc(result, func(match string) string {
		return RedactID(match)
	})

	for _, key := range sensitiveKeys {
		pattern := regexp.MustCompile(`(?i)` + key + `[=:]["']?([a-zA-Z0-9_-]+)`)
		result = pattern.ReplaceAllStringFunc(result, func(match string) string {
			return key + "=" + RedactID(extractIDFromMatch(match, key))
		})
	}

	return result
}

func extractIDFromMatch(match, key string) string {
	parts := strings.Split(match, "=")
	if len(parts) == 2 {
		val := strings.Trim(parts[1], `"' `)
		val = strings.TrimPrefix(val, "Bearer ")
		return val
	}
	parts = strings.Split(match, ":")
	if len(parts) == 2 {
		return strings.Trim(parts[1], `"' `)
	}
	return match
}
