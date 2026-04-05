# Plan: Remove --mcp Flag, Auto-Enable MCP Based on Token Presence

## Context

Currently, MCP servers are only enabled when the `--mcp` flag is explicitly passed. This flag:
1. Sets `cfg.mcp = true` in the host binary
2. Passes `KD_MCP_ENABLED=1` as an environment variable to the container
3. Inside the container, `KD_MCP_ENABLED` triggers token prompting and enables MCP servers in opencode.json

The user wants to remove the `--mcp` flag entirely and instead have MCP servers activate automatically based on the presence of MCP auth token environment variables (`KD_CONTEXT7_TOKEN`, `KD_AINSTRUCT_TOKEN`).

## Intended Flow

### Token Storage States

Tokens can be in three states:
- **Not set**: Never configured (prompt user on init)
- **Empty string**: User explicitly disabled (store `KD_CONTEXT7_TOKEN=""`)
- **Valid token**: User configured the service (store `KD_CONTEXT7_TOKEN="xxx"`)

### Container Init (First Time)

```
1. User runs: kilo-docker
2. Container starts
3. runUserInit() executes
   a. Ainstruct login (gets PAT) → KD_AINSTRUCT_TOKEN always set
   b. Load stored tokens from encrypted volume
   c. Check if KD_CONTEXT7_TOKEN exists in storage:
      - NOT SET (missing): Prompt user
        ├─ Enter token → Store it
        └─ Press Enter (empty) → Store empty string, disable server
      - EXISTS (empty or valid): Use stored value, skip prompt
   d. Set token env vars
   e. Call syncMCPConfig() → updates opencode.json based on token presence
   f. Start zellij
```

### Container Re-attach

```
1. User runs: kilo-docker sessions <name>
2. Attach to existing container
3. runZellijAttach() executes
   a. Load tokens from encrypted storage
   b. Set token env vars
   c. Call syncMCPConfig() → refreshes opencode.json
   d. Start zellij
```

### Manual Token Management

```
kilo-entrypoint mcp-tokens    # Interactive: set/update MCP tokens
   ├─ Context7 token: [show current/prompt]
   │   ├─ Enter new token → Update
   │   ├─ Press Enter → Keep current
   │   └─ Type "clear" → Store empty, disable server
   └─ Ainstruct: auto from login (not prompted)

kilo-entrypoint mcp-config    # Re-apply config without changing tokens
```

**Server Enablement Logic:**
- **Regular MCP servers** (ainstruct, context7): enabled if auth token env var is non-empty
- **Specific MCP servers** (playwright): enabled if `PLAYWRIGHT_ENABLED=1` is set

## Changes

### Host Binary (cmd/kilo-docker/)

#### flags.go
- Remove `mcp bool` field from `config` struct (line 24)
- Remove `--mcp` case from `parseArgs()` switch (lines 51-52)

#### args.go
- Remove `cfg.mcp` check from `serializeArgs()` (lines 28-30)
- Remove `KD_MCP_ENABLED` environment variable from `buildContainerArgs()` (lines 64-66)

#### setup.go
- Remove `--mcp` from help text (line 73)
- Remove `--mcp` example (line 82)

#### main.go
- Remove `--mcp` from package doc comment (line 16)

#### flags_test.go
- Update `TestParseFlagsYesWithOtherFlags()` - remove `--mcp` from test args (line 74)
- Update `TestParseFlagsPortWithOtherFlags()` - remove `--mcp` from test args (line 280)
- Remove `cfg.mcp` assertions from both tests

#### args_test.go
- Update `TestSerializeArgsCombined()` - remove `mcp` from test config and expected output (lines 174-180)

### Container Entrypoint (cmd/kilo-entrypoint/)

#### config.go
- Rename `runConfig()` to `syncMCPConfig()` (more specific name)
- Change `applyConfigFilter()` signature:
  - Remove `mcpEnabled bool` parameter (no longer needed)
  - Keep `playwrightEnabled bool` for specific MCP servers
- Update logic to distinguish two types of MCP servers:
  - **Regular MCP servers** (ainstruct, context7): enabled if auth token env var is present (`KD_AINSTRUCT_TOKEN`, `KD_CONTEXT7_TOKEN`)
  - **Specific MCP servers** (playwright): enabled if specific `_ENABLED=1` env var is set (`PLAYWRIGHT_ENABLED=1`)

#### userinit.go
- Remove `KD_MCP_ENABLED == "1"` check that gates MCP initialization (line 114)
- `syncMCPConfig()` will execute **unconditionally during user initialization**
- Update token prompting logic:
  - Check if Context7 token exists in encrypted storage (distinguish "not set" vs "empty" vs "valid")
  - **Only prompt if NOT SET** (never configured)
  - Allow storing empty string to explicitly disable the server
  - If EXISTS (empty or valid), use stored value without prompting
- Update `initTokens()` to support storing empty tokens

#### zellijattach.go
- After loading tokens from encrypted storage (lines 126-156), call `syncMCPConfig()` to refresh MCP config before starting zellij
- This ensures MCP config is updated on every attach, picking up any token changes
- The tokens are already being re-loaded on attach anyway, so we should also refresh the config

#### main.go (kilo-entrypoint)
- Add new subcommand `mcp-config` that calls `syncMCPConfig()` manually
  - Re-applies MCP configuration based on current env vars
  - Should be callable via: `kilo-entrypoint mcp-config`
- Add new subcommand `mcp-tokens` for interactive token management
  - Prompt for Context7 token with current value shown (masked)
  - Options: enter new token (update), press Enter (keep current), type "clear" (disable)
  - Ainstruct token comes from login, not prompted here
  - After token settings complete (user exits), automatically calls `syncMCPConfig()` to apply changes

## Files Modified

| File | Change |
|------|--------|
| cmd/kilo-docker/flags.go | Remove `mcp` field and `--mcp` case |
| cmd/kilo-docker/args.go | Remove `KD_MCP_ENABLED` env var and serialization |
| cmd/kilo-docker/setup.go | Remove `--mcp` from help/docs |
| cmd/kilo-docker/main.go | Remove `--mcp` from doc comment |
| cmd/kilo-docker/flags_test.go | Remove `--mcp` from tests |
| cmd/kilo-docker/args_test.go | Remove `mcp` from tests |
| cmd/kilo-entrypoint/config.go | Always enable MCP servers with tokens present |
| cmd/kilo-entrypoint/userinit.go | Always run MCP token initialization |
| cmd/kilo-entrypoint/main.go | Add `mcp-config` and `mcp-tokens` subcommands |
| README.md | Update documentation to remove `--mcp` |

## Verification

1. Build the binary: `go build ./cmd/kilo-docker`
2. Run tests: `go test ./cmd/kilo-docker/...`
3. Verify `kilo-docker --help` no longer shows `--mcp`
4. Verify regular MCP servers (ainstruct, context7) are enabled when `KD_CONTEXT7_TOKEN` or `KD_AINSTRUCT_TOKEN` env vars are set
5. Verify specific MCP servers (playwright) are enabled when `PLAYWRIGHT_ENABLED=1` is set
6. Verify `kilo-entrypoint mcp-config` manually re-applies MCP configuration
7. Verify `kilo-entrypoint mcp-tokens` allows interactive token management
8. Verify empty token storage disables server without re-prompting on next init
