#!/bin/sh
# Shared kilo config setup — sourced by entrypoint scripts.
# Disables MCP servers whose required token env vars are not set.
# Enables/disables Playwright based on PLAYWRIGHT_ENABLED env var.
#
# Kilo reads config with CWD taking priority over user config, so both
# the user config and any workspace config must be patched.

JQ_FILTER='
  {"ainstruct":"AINSTRUCT_TOKEN","context7":"CONTEXT7_TOKEN"} as $mapping |
  .mcp |= with_entries(
    if .key == "playwright" then
      if (env["PLAYWRIGHT_ENABLED"] // "") == "1" then
        .value.enabled = true
      else
        .value.enabled = false
      end
    elif $mapping[.key] and ((env[$mapping[.key]] // "") == "") then
      .value.enabled = false
    else
      .
    end
  )
'

disable_mcp() {
    if [ -f "$1" ]; then
        jq "$JQ_FILTER" "$1" > "$1.tmp" && mv "$1.tmp" "$1"
    fi
}

disable_mcp "/home/kilo/.config/kilo/opencode.json"
disable_mcp "${PWD}/configs/opencode.json"
