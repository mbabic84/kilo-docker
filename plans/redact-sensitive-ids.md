# Plan: Redact Sensitive IDs from Logs

## Context
Sensitive IDs like `user_id`, `collection_id`, `document_id`, and token prefixes are being logged in plaintext. This exposes internal identifiers that could be used to correlate data across systems or infer user activity patterns.

## Changes

### 1. Create redaction utility (`pkg/utils/redact.go`)
Create a utility package with:
- `RedactID(id string) string` - masks IDs showing first 2 + last 2 chars
- `RedactToken(token string) string` - existing `maskToken` logic (first 4 + last 4)
- `Redact(s string) string` - redacts all known sensitive patterns from a string

Sensitive patterns to redact:
- UUIDs/IDs: `user_id`, `collection_id`, `document_id`, `access_token`, `refresh_token`
- UUIDs matching patterns like `xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx`

### 2. Update `cmd/kilo-entrypoint/sync_content.go`
Replace direct ID logging with redacted versions:
- Line 217, 236: `s.collectionID` → `RedactID(s.collectionID)`
- Line 395: `s.collectionID` → `RedactID(s.collectionID)`
- Line 412: `doc.DocumentID` → `RedactID(doc.DocumentID)`
- Line 484: `doc.DocumentID` → `RedactID(doc.DocumentID)`

### 3. Update `cmd/kilo-entrypoint/api.go`
Replace sensitive data in error responses:
- Line 121, 127: Log response bodies without exposing tokens

### 4. Update `cmd/kilo-entrypoint/zellijattach.go`
- Line 148: `userID` → `RedactID(userID)`

## Files Modified
| File | Change |
|------|--------|
| `pkg/utils/redact.go` | New file - redaction utilities |
| `cmd/kilo-entrypoint/sync_content.go` | Redact collection/document IDs |
| `cmd/kilo-entrypoint/api.go` | Redact error response bodies |
| `cmd/kilo-entrypoint/zellijattach.go` | Redact userID |

## Verification
- `go build ./...` - compiles successfully
- `go test ./...` - all tests pass