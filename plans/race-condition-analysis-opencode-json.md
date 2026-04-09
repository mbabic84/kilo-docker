# Race Condition Analysis: Concurrent opencode.json Modifications

## Problem Summary

When multiple kilo-docker instances run simultaneously, they independently modify their local `opencode.json` files and sync changes to the ainstruct collection. Due to the **last-write-wins** behavior, changes from one instance overwrite changes from another, resulting in **lost updates**.

### Specific Scenario

```
Time ──────────────────────────────────────────────────────────►

Instance A                    Instance B                    ainstruct Collection
──────────                    ──────────                    ────────────────────
                              
Read local opencode.json      Read local opencode.json      
(perms: {tool1: allow})       (perms: {tool1: allow})       
                              
Add permission:               Add permission:               
perms.tool2 = "allow"         perms.tool3 = "allow"         
                              
                              
Sync to collection ───────────────────────────────────────────► 
(perms: {tool1, tool2})                                     Document: {tool1, tool2}
                              
                              Sync to collection ───────────►
                              (perms: {tool1, tool3})       Overwrites! 
                                                            Document: {tool1, tool3}
                                                            ❌ tool2 permission LOST
```

## Root Cause Analysis

### 5-7 Possible Sources of the Problem

1. **No Distributed Locking**: Multiple instances can write to the same collection simultaneously without coordination
2. **Last-Write-Wins Semantics**: The PATCH operation replaces the entire document content rather than merging
3. **No Version/ETag Checking**: No optimistic concurrency control to detect conflicts
4. **No Change Notification**: Instances don't get notified when other instances make changes
5. **Static Sync Paths**: The entire `opencode.json` is synced as a single document, making fine-grained changes impossible
6. **Pull-Before-Push Gap**: Even with `pullCollection()` at startup, concurrent modifications during runtime aren't handled
7. **No Merge Strategy**: When conflicts occur, there's no defined strategy for merging permissions

### Most Likely Sources (Top 2)

1. **Last-Write-Wins at Collection Level** (Primary): The ainstruct collection stores `opencode.json` as a single document. When Instance B syncs after Instance A, it replaces the entire document, discarding A's permission additions.

2. **No Optimistic Concurrency Control** (Secondary): The sync process reads the current state, modifies it locally, and writes it back without verifying that the remote state hasn't changed during the read-modify-write cycle.

## Proposed Solutions

### Solution 1: Optimistic Locking with ETags/Versioning (Recommended for Short Term)

**Approach**: Add version checking to the ainstruct API and sync logic using HTTP conditional requests.

**Implementation**:
1. ainstruct API returns an `ETag` or `version` field with each document
2. Syncer includes `If-Match` header with the version when PATCHing
3. If version mismatch (412 Precondition Failed), the syncer:
   - Pulls the latest version from the collection
   - Merges changes (for permissions: union of keys)
   - Retries the PATCH with the new version

**Pros**:
- Simple to implement
- Works with existing architecture
- Fail-fast on conflicts

**Cons**:
- Requires API changes to support versioning
- Requires retry logic with merge capability
- Still has race window between pull and push

**Go Pattern**:
```go
// API response includes version
type document struct {
    DocumentID  string `json:"document_id"`
    Content     string `json:"content"`
    Version     int64  `json:"version"`  // NEW
}

// PATCH with If-Match
req.Header.Set("If-Match", fmt.Sprintf("%d", doc.Version))

// On 412 conflict, merge and retry
if resp.StatusCode == 412 {
    return s.mergeAndRetry(absPath, relPath)
}
```

### Solution 2: Operation-Based Delta Sync (Recommended for Long Term)

**Approach**: Instead of syncing the entire file, sync individual operations (add/remove permission) as separate documents or events.

**Implementation**:
1. Store permissions as separate documents: `permissions/tool1.json`, `permissions/tool2.json`
2. Each permission is a standalone document in the collection
3. Sync only adds/removes specific permission documents
4. Local opencode.json is reconstructed from the collection on pull

**Pros**:
- Natural conflict resolution (different permissions = different files)
- No merge logic needed for independent permissions
- Fine-grained change tracking

**Cons**:
- Requires architectural change to how opencode.json is managed
- Need permission reconstruction logic on pull
- More complex document structure

**File Structure**:
```
kilo-docker collection/
├── opencode.json              (base config without permissions)
├── permissions/
│   ├── tool1.json            (content: "allow")
│   ├── tool2.json            (content: "ask")
│   └── tool3.json            (content: "deny")
```

### Solution 3: CRDT-Based JSON (Advanced)

**Approach**: Use a Conflict-free Replicated Data Type for the JSON document, specifically for the `permission` object.

**Implementation**:
1. Adopt a library like [automerge](https://automerge.org/) or implement a simple OR-Set CRDT
2. Each permission is treated as an element in a set
3. Concurrent additions are automatically merged (set union)
4. The sync protocol exchanges CRDT deltas rather than full documents

**Pros**:
- Automatic conflict resolution
- No locking needed
- Works offline and syncs later
- Mathematically proven consistency

**Cons**:
- Complex to implement from scratch
- Adds dependency on CRDT library
- Overkill for simple permission management

**CRDT Library Options**:
- Go: `github.com/losfair/crdt` or custom implementation
- Reference: [A Conflict-Free Replicated JSON Datatype](https://arxiv.org/abs/1608.03960) by Kleppmann

### Solution 4: Leader Election / Single Writer

**Approach**: Ensure only one kilo-docker instance at a time can write to the collection.

**Implementation**:
1. Use a distributed lock (e.g., based on ainstruct collection metadata)
2. Or use filesystem-level locking on a shared volume
3. The first instance to start becomes the "writer"
4. Other instances operate read-only or queue changes through the leader

**Pros**:
- Simple conflict model (no conflicts possible)
- No merge logic needed

**Cons**:
- Requires leader election mechanism
- Single point of contention
- Failure handling complexity (what if leader dies?)

### Solution 5: Event Sourcing with Permission Log

**Approach**: Treat permission changes as an append-only log rather than state updates.

**Implementation**:
1. Create a `permission-log.jsonl` document that appends each permission change
2. Log entry format: `{"op": "add", "tool": "tool2", "permission": "allow", "timestamp": "..."}`
3. On sync, append new entries (append is naturally commutative)
4. Local state is computed by replaying the log

**Pros**:
- Append-only = no conflicts
- Full audit trail of changes
- Easy to replay/reconstruct state

**Cons**:
- Log can grow large
- Need log truncation/compaction strategy
- Requires rebuild of opencode.json from log

## ainstruct-mcp Specific Solutions

Based on analysis of [ainstruct-mcp](https://github.com/mbabic84/ainstruct-mcp), here are solutions that leverage its existing architecture:

### Solution 6: JSON Patch (RFC 6902) with Content-Hash Optimistic Locking

**Leverages**: ainstruct-mcp already computes `content_hash` for deduplication in `repository.py:232`

**Approach**:
1. Extend the PATCH endpoint to accept JSON Patch operations (RFC 6902)
2. Use the existing `content_hash` as an ETag for optimistic locking
3. Clients send `If-Match: <content_hash>` header
4. Server rejects with 412 if hash doesn't match current content

**Implementation in ainstruct-mcp**:
```python
# documents.py - extend update_document()
@router.patch(
    "/{document_id}",
    response_model=DocumentUpdateResponse,
    responses={
        404: {"model": ErrorResponse, "description": "Document not found"},
        412: {"model": ErrorResponse, "description": "Precondition failed - content changed"},
    },
)
async def update_document(
    document_id: str,
    body: DocumentUpdate,
    db: DbDep,
    user: UserDep,
    if_match: str | None = Header(None, alias="If-Match"),  # NEW
):
    # ... existing validation ...
    
    # NEW: Optimistic locking check
    if if_match and document.content_hash != if_match:
        raise HTTPException(
            status_code=status.HTTP_412_PRECONDITION_FAILED,
            detail={"code": "CONCURRENT_MODIFICATION", 
                    "message": "Document was modified by another client"},
        )
    
    # NEW: Support JSON Patch operations for partial updates
    if body.patch_operations:
        content = apply_json_patch(document.content, body.patch_operations)
    else:
        content = body.content
    
    # ... rest of update logic
```

**Client-side (kilo-docker)**:
```go
func (s *Syncer) syncFileWithPatch(absPath string) error {
    // Read current remote state
    existing, err := s.getDocumentByPath(relPath)
    
    // Compute local content hash
    localContent, _ := os.ReadFile(absPath)
    localHash := computeHash(localContent)
    
    // Generate JSON Patch diff
    patchOps := generateJSONPatch(existing.Content, string(localContent))
    
    // Send PATCH with If-Match
    body := map[string]any{
        "patch_operations": patchOps,
    }
    
    req, _ := http.NewRequest("PATCH", url, body)
    req.Header.Set("If-Match", existing.ContentHash)
    
    resp, err := s.client.Do(req)
    if resp.StatusCode == 412 {
        // Conflict - pull latest and retry with merge
        return s.handleConflictAndRetry(absPath, relPath)
    }
}
```

**Pros**:
- Uses existing `content_hash` field (no DB migration needed)
- JSON Patch is standardized (RFC 6902)
- Fine-grained updates reduce conflict probability
- Works with ainstruct-mcp's current deduplication logic

**Cons**:
- Requires ainstruct-mcp API changes
- More complex client implementation

### Solution 7: Permission-Aware Merge at API Level

**Leverages**: ainstruct-mcp stores `doc_metadata` as JSONB which can store structured data

**Approach**:
1. Store opencode.json permissions as a structured object in `doc_metadata`
2. Add a special endpoint: `POST /documents/{id}/merge-permissions`
3. Server-side merge with configurable conflict resolution

**Implementation**:
```python
# New endpoint in documents.py
@router.post(
    "/{document_id}/merge-metadata",
    response_model=DocumentUpdateResponse,
)
async def merge_document_metadata(
    document_id: str,
    body: MetadataMergeRequest,  # { "path": "permission", "merge": { ... } }
    db: DbDep,
    user: UserDep,
):
    document = await doc_repo.get_by_id(document_id)
    
    # Server-side merge logic
    current_metadata = document.doc_metadata or {}
    
    if body.strategy == "union":
        # Union merge for permissions
        current_perms = current_metadata.get(body.path, {})
        merged_perms = {**current_perms, **body.merge}
        current_metadata[body.path] = merged_perms
    elif body.strategy == "deep_merge":
        current_metadata = deep_merge(current_metadata, body.merge)
    
    updated = await doc_repo.update(
        document_id, 
        doc_metadata=current_metadata
    )
    return updated
```

**Pros**:
- Server handles merge complexity
- Can implement domain-specific merge logic (e.g., permission inheritance)
- Single request, no retry loop needed

**Cons**:
- Requires new API endpoint
- ainstruct-mcp needs to understand opencode.json structure

### Solution 8: Document-Level Concurrency with Update Token

**Leverages**: ainstruct-mcp already has `updated_at` timestamps

**Approach**:
1. Return `update_token` (hash of content + updated_at) in GET responses
2. Require `update_token` in PATCH requests
3. Server validates token matches current state before updating

**Implementation**:
```python
# In repository.py - update() method
async def update(
    self,
    doc_id: str,
    title: str | None = None,
    content: str | None = None,
    document_type: str | None = None,
    doc_metadata: dict | None = None,
    expected_update_token: str | None = None,  # NEW
) -> DocumentResponse | None:
    async with self.async_session() as session:
        db_doc = await session.get(DocumentModel, doc_id)
        
        if expected_update_token:
            current_token = compute_update_token(db_doc)
            if current_token != expected_update_token:
                raise ConcurrentModificationError()
        
        # ... apply updates ...
```

**Pros**:
- Simple to implement
- No additional DB fields needed
- Clear error semantics

**Cons**:
- Still requires client-side retry logic
- Token generation adds overhead

## Recommended Implementation Path

### Phase 1: Quick Fix (Content-Hash Optimistic Locking)

Use ainstruct-mcp's existing `content_hash` field:

1. **Modify ainstruct-mcp REST API** (if possible):
   - Add `If-Match` header support to PATCH endpoint
   - Return 412 on hash mismatch
   - Include current content in 412 response for client merge

2. **Update kilo-docker syncer**:
   ```go
   func (s *Syncer) syncFileWithConflictHandling(absPath string) error {
       // ... get existing document ...
       
       // Try optimistic update
       body := map[string]string{"content": newContent}
       req, _ := http.NewRequest("PATCH", url, body)
       req.Header.Set("If-Match", existing.ContentHash)
       
       resp, err := s.client.Do(req)
       if resp.StatusCode == 412 {
           // Conflict! Pull latest and merge
           latest := s.pullDocument(existing.DocumentID)
           mergedContent := mergeOpencodeJSON(newContent, latest.Content)
           
           // Retry with merged content
           return s.retryUpdate(existing.DocumentID, mergedContent, latest.ContentHash)
       }
   }
   ```

3. **Implement domain-specific merge** for opencode.json:
   ```go
   func mergeOpencodeJSON(local, remote string) string {
       localConfig := parseJSON(local)
       remoteConfig := parseJSON(remote)
       
       // Union merge for permissions
       for tool, perm := range localConfig.Permission {
           remoteConfig.Permission[tool] = perm  // Local wins on conflict
       }
       
       // Merge MCP server configs (more complex)
       // ...
       
       return marshalJSON(remoteConfig)
   }
   ```

### Phase 2: JSON Patch Support (If ainstruct-mcp can be modified)

1. **Add JSON Patch endpoint** to ainstruct-mcp
2. **Update kilo-docker** to generate and send patch operations
3. **Reduces conflict probability** by only sending changed fields

### Phase 3: Permission-Aware Sync (Architectural Change)

If ainstruct-mcp cannot be modified, implement client-side coordination:

1. **Use a coordination document** in the collection:
   ```json
   {
     "title": "sync-lock",
     "content": "{\"locked_by\": \"instance-a\", \"expires\": \"2025-01-01T00:00:00Z\"}"
   }
   ```

2. **Implement acquire/release lock** using atomic POST/DELETE

3. **Falls back to last-write-wins** if lock cannot be acquired

## Verification Plan

1. **Unit Tests**:
   - Simulate concurrent permission additions
   - Verify merge logic correctness
   - Test retry behavior on conflicts

2. **Integration Tests**:
   - Start two kilo-docker instances
   - Add different permissions from each
   - Verify both permissions exist in final state

3. **Chaos Testing**:
   - Rapid concurrent additions/removals
   - Network interruption during sync
   - Verify eventual consistency

## Verification Plan

1. **Unit Tests**:
   - Simulate concurrent permission additions
   - Verify merge logic correctness
   - Test retry behavior on conflicts

2. **Integration Tests**:
   - Start two kilo-docker instances
   - Add different permissions from each
   - Verify both permissions exist in final state

3. **Chaos Testing**:
   - Rapid concurrent additions/removals
   - Network interruption during sync
   - Verify eventual consistency

## References

- [Optimistic Locking Made Easy: The Power of ETags](https://martincarstenbach.com/2026/02/03/optimistic-locking-made-easy-the-power-of-etags-in-action/)
- [HTTP Conditional Requests - MDN](https://developer.mozilla.org/en-US/docs/Web/HTTP/Guides/Conditional_requests)
- [A Conflict-Free Replicated JSON Datatype](https://arxiv.org/abs/1608.03960) (Kleppmann et al.)
- [Merge Rules | Automerge CRDT](https://automerge.org/docs/reference/under-the-hood/merge_rules)
- [Cross Platform File Locking with Go](https://www.chronohq.com/blog/cross-platform-file-locking-with-go)
- [Optimistic Locking in a REST API](https://sookocheff.com/post/api/optimistic-locking-in-a-rest-api/)
