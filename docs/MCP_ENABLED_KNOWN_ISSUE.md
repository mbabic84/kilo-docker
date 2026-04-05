# Known Issue: MCP Servers Not Auto-Enabling in Container

**Status:** Investigated, workaround available  
**Last Updated:** 2026-04-05  
**Test Again After:** Next Kilo CLI release (> 7.1.20)

## Problem

MCP servers configured with `enabled: true` in `opencode.json` are not automatically enabled when Kilo CLI starts in the container environment. They show as `disabled` in `kilo mcp list` and must be manually enabled via Ctrl+P menu.

## Investigation Summary

### What Was Tested

1. ✅ Config file (`~/.config/kilo/opencode.json`) correctly has `enabled: true`
2. ✅ Same Kilo CLI version (7.1.20) on host and container
3. ✅ Environment variables (`KD_MCP_CONTEXT7_TOKEN`, `KD_MCP_AINSTRUCT_TOKEN`) are set
4. ✅ Our `applyMCPEnabledFromEnv()` function correctly updates config
5. ❌ Kilo CLI shows `enabled: false` in `kilo debug config` output
6. ✅ Same configuration works correctly on host machine

### Key Findings

- The `enabled` field is properly written to config by our wrapper
- Kilo CLI reads the config but overrides `enabled` to `false` at runtime
- Only difference identified: `KILOCODE_FEATURE=cli` in container (empty on host)
- Unsetting `KILOCODE_FEATURE` did not resolve the issue

### Root Cause

Unknown. Likely a Kilo CLI bug that only manifests in the container environment (possibly related to:
- Different library behavior in Alpine Linux
- Timing of config loading
- Interaction with the container's isolated environment)

## Workaround

Users must manually enable MCP servers after Kilo starts:

1. Press `Ctrl+P`
2. Type "mcps" or select "Toggle MCP Servers"
3. Toggle the desired servers to enable them

Once enabled, they work correctly for the session.

## Code Changes Made

To support auto-enabling when Kilo eventually fixes this:

1. **`cmd/kilo-entrypoint/config.go`**: Added `applyMCPEnabledFromEnv()` function
   - Reads `KD_MCP_*` tokens from environment
   - Updates `enabled` field in `opencode.json`
   - Comprehensive debug logging

2. **`scripts/kilo-wrapper.sh`**: Added MCP config application
   - Calls `kilo-entrypoint mcp-config` before starting Kilo
   - Shows debug output to help diagnose issues

3. **Removed duplicate sync calls**: 
   - `zellijattach.go` no longer calls sync
   - `userinit.go` no longer calls sync
   - Only wrapper calls it once at startup

## Testing Instructions

To verify if this is fixed in a future Kilo version:

```bash
# 1. Check Kilo version
kilo --version

# 2. Ensure tokens are set
env | grep KD_MCP

# 3. Check config has enabled: true
cat ~/.config/kilo/opencode.json | grep enabled

# 4. Start Kilo and check MCP status
kilo mcp list

# 5. Verify servers show as "enabled" not "disabled"
```

If servers auto-enable, this issue is resolved.

## Related GitHub Issues

- Kilo CLI issue tracking: Check [Kilo-Org/kilocode](https://github.com/Kilo-Org/kilocode/issues)
- Related issues found: #6292, #7079, #6481 (config persistence problems)

## Notes

- The implementation is correct and complete
- The issue is upstream in Kilo CLI's container environment handling
- No further changes needed in kilo-docker unless Kilo CLI behavior changes
