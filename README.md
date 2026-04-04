# Kilo Docker

Docker environment for [Kilo CLI](https://kilo.ai/docs/code-with-ai/platforms/cli) - enabling portable, zero-install execution on remote hosts.

## Image Variants

| Image | Base | Size | Description |
|-------|------|------|-------------|
| `ghcr.io/mbabic84/kilo-docker:latest` | Alpine | ~202 MB | Lightweight base with `git`, `openssh-client`, `ripgrep`, and `libstdc++` |

## Features

- **Non-root user** - Runs as `kilo` user with dynamic `PUID`/`PGID` mapping to match host user
- **Persistent database** - SQLite database and auth state survive container restarts via named volume
- **Token persistence** - MCP server tokens are prompted once and saved in the volume
- **Volume encryption** - `--password` flag encrypts tokens and derives a non-discoverable volume name
- **Ainstruct auth** - `--ainstruct` flag authenticates with the Ainstruct API to derive volume name from user_id
- **Ainstruct file sync** - `--ainstruct` flag enables automatic push/pull sync of config files, commands, agents, and instructions via the Ainstruct API
- **One-time sessions** - `--once` flag for ephemeral runs without persistence
- **Browser automation** - `--playwright` flag starts a Playwright MCP sidecar for screenshots, navigation, and web interaction
- **Built-in services** - Extensible service system with `--docker` and more (see [Services](#services))

## Quick Start

Download the host binary from GitHub Releases:

```bash
# Linux amd64
curl -fsSL -o ~/.local/bin/kilo-docker https://github.com/mbabic84/kilo-docker/releases/latest/download/kilo-docker-linux-amd64
chmod +x ~/.local/bin/kilo-docker

# macOS arm64
curl -fsSL -o ~/.local/bin/kilo-docker https://github.com/mbabic84/kilo-docker/releases/latest/download/kilo-docker-darwin-arm64
chmod +x ~/.local/bin/kilo-docker
```

Or use the bootstrap installer:

```bash
curl -fsSL https://raw.githubusercontent.com/mbabic84/kilo-docker/main/scripts/install.sh | sh
```

Then run from any directory:

```bash
kilo-docker
```

On first run, the binary prompts for MCP server tokens and saves them to a named Docker volume. Subsequent runs reuse the saved tokens.

## Commands

| Command | Description |
|---------|-------------|
| `sessions [name\|index]` | List sessions or attach to one by name or index |
| `sessions cleanup [-y] [name\|index]` | Remove a session (interactive if no name given) |
| `sessions cleanup -y -a` | Remove all exited sessions |
| `sessions recreate <name\|index>` | Recreate a session with the same flags (preserves volume) |
| `networks` | List available Docker networks |
| `backup [-f]` | Create backup of volume to tar.gz |
| `restore <file> [-f] [-v\|--volume <name>]` | Restore volume from backup |
| `init` | Reset configuration (remove volume, re-enter tokens) |
| `cleanup` | Remove volume, containers, image, and installed binary |
| `update` | Pull the latest Docker image and update the binary |
| `update-config` | Download latest opencode.json template and merge with existing config |
| `version` | Show kilo-docker and kilo versions |
| `help` | Show help message |

### Options

| Option | Description |
|--------|-------------|
| `--once` | Run a one-time session without persistence (no volume) |
| `--password`, `-p` | Protect volume with a password (encrypts tokens, derives volume name from password) |
| `--ainstruct` | Authenticate with Ainstruct API (volume from user_id, tokens encrypted, file sync enabled) |
| `--mcp` | Enable MCP servers (prompts for Context7 and Ainstruct API tokens) |
| `--playwright` | Start a Playwright MCP sidecar container for browser automation |
| `--ssh` | Enable SSH agent forwarding into the container |
| `--network <name>` | Attach to a specific Docker network |
| `--yes`, `-y` | Auto-confirm all prompts (useful for piped/non-interactive installs) |

### Services

| Service | Description |
|---------|-------------|
| `--docker` | Mount Docker socket for container management from within Kilo |
| `--go` | Install Go 1.26.1 (latest stable) for development |
| `--node` | Install Node.js LTS for development |
| `--gh` | Install GitHub CLI for interacting with GitHub |
| `--uv` | Install uv for fast Python package management |

## One-Time Sessions

Use `--once` to run without creating or mounting a named volume. No data persists after the container exits:

```bash
kilo-docker --once
```

This is useful for CI pipelines, ephemeral environments, or when you don't want to leave any state on the host.

## Volume Encryption

Use `--password` (or `-p`) to protect your volume on shared hosts. This applies two layers of protection:

1. **Non-discoverable volume name** — The volume name is derived from the SHA-256 hash of your password (e.g., `kilo-a3f2b1c9d4e5`). Other users on the host cannot find or target your volume via `docker volume ls`.

2. **Encrypted tokens** — API tokens are encrypted with AES-256-CBC (PBKDF2 key derivation) before being stored in the volume. Plaintext tokens never touch the disk.

```bash
# Start with encryption
kilo-docker --password

# Reset encrypted volume
kilo-docker --password init
```

On first run, you are prompted for a volume password and then for API tokens. Subsequent runs only ask for the volume password.

Without `--password`, the volume name is `kilo-data-<username>` and tokens are stored in plaintext (original behavior).

> **Note:** `--once` and `--password` are mutually exclusive. `--once` creates no volume, so there is nothing to encrypt.

## Ainstruct

[Ainstruct](https://ainstruct-dev.kralicinora.cz) provides document storage, semantic search, and configuration sync for Kilo. The web UI lets you manage collections, documents, and API keys.

Use `--ainstruct` to authenticate and enable integration:

```bash
kilo-docker --ainstruct
```

On first run, you are prompted for your Ainstruct username and password. The binary authenticates via the API and obtains your `user_id`.

### Volume encryption

The `user_id` is used to derive a non-discoverable volume name and encryption key. MCP server tokens are stored encrypted (AES-256-CBC with PBKDF2) in the volume — plaintext tokens never touch the disk.

### File sync

Configuration files are automatically synced to and from the Ainstruct API:

**Synced files:**
- `~/.config/kilo/opencode.json` — Kilo configuration
- `~/.config/kilo/rules/*.md` — Instruction files
- `~/.config/kilo/commands/*.md` — Custom slash commands (markdown with YAML frontmatter)
- `~/.config/kilo/agents/*.md` — Custom agent definitions (markdown with YAML frontmatter)
- `~/.config/kilo/plugins/*.{js,ts}` — Plugins (JavaScript/TypeScript hook modules)
- `~/.config/kilo/skills/*/SKILL.md` — Agent skills (per-skill directories with optional `scripts/`, `references/`, `assets/`)
- `~/.config/kilo/tools/*.{js,ts}` — Custom tools (JavaScript/TypeScript tool definitions)

**Push (local → API):** A Go-based file watcher detects local changes via inotify with a per-file 5-second debounce. Each file has an independent timer that resets on every change — the file is synced only after 5 seconds with no further modifications.

**Pull (API → local):** On container startup, the sync state file (`~/.config/kilo/.ainstruct-hashes`) is compared against the API's `content_hash` values. Only changed or new files are downloaded. Unchanged files are skipped without any API calls.

**Token refresh:** JWT access tokens are refreshed automatically before API calls when within 60 seconds of expiry.

The sync engine runs as a subcommand of `kilo-entrypoint` inside the container — no `bash`, `inotify-tools`, `curl`, or `jq` runtime dependencies. It uses native Linux inotify via `golang.org/x/sys/unix` and communicates with the Ainstruct API using Go's `net/http` stdlib.

## Browser Automation

The `--playwright` flag starts a [Playwright MCP](https://github.com/microsoft/playwright-mcp) sidecar container alongside Kilo, enabling browser automation (screenshots, navigation, form filling, etc.):

```bash
kilo-docker --playwright
```

The sidecar runs headless Chromium in HTTP mode on port 8931 inside a dedicated Docker network (`kilo-playwright-<username>`). Both the sidecar container and network are automatically cleaned up when Kilo exits.

Screenshots and other output files are saved to `.playwright-mcp/` in the workspace directory.

## SSH Agent Forwarding

Use the `--ssh` flag to enable SSH agent forwarding. The host binary detects whether an SSH agent is running on the host:

- **Agent running** — Uses the existing agent via `$SSH_AUTH_SOCK`
- **No agent** — Starts one automatically, loads all private keys from `~/.ssh/`, and cleans up on exit

The container mounts the host's SSH agent socket, allowing `git`, `ssh`, and `scp` to use your host SSH keys without copying private keys into the container.

```bash
kilo-docker --ssh
```

> **Security:** Private keys never enter the container. The container communicates with the host's SSH agent via a Unix socket.

## Services

Kilo Docker uses a data-driven service architecture. Services are defined as structured data, making it easy to add new capabilities without modifying core logic.

### How Services Work

Each service can specify:

- **CLI flag** — enabling the service
- **Installation commands** — shell commands run inside the container at startup
- **Environment variables** — passed to the container
- **Host environment variables** — values sourced from the host
- **Volumes** — filesystem paths mounted from the host
- **Required socket** — optional host socket path for validation
- **Config files** — optional files copied from container to user home

When you pass a service flag, the host binary validates required host resources, collects socket GIDs, builds the `KD_SERVICES` env var, and mounts required volumes.

Inside the container, the entrypoint reads `KD_SERVICES`, runs installation commands, copies config files, and sets environment variables.

## Data Persistence

The host binary uses a named Docker volume mounted at `/home`. Inside the container, the user home directory is dynamically generated as `/home/kd-<hash>`. This stores:

- SQLite database, auth state, logs
- Configuration (`opencode.json` — model selection, provider connections, MCP settings)
- Custom commands (`.config/kilo/commands/*.md`) and agents (`.config/kilo/agents/*.md`)
- Plugins (`.config/kilo/plugins/*.{js,ts}`), skills (`.config/kilo/skills/*/SKILL.md`), tools (`.config/kilo/tools/*.{js,ts}`)
- Instruction files (`.config/kilo/rules/*.md`)
- Session state and snapshots
- Cache
- Ainstruct sync state (`.config/kilo/.ainstruct-hashes`) — when using `--ainstruct`

**Default mode** — Volume name: `kilo-data-<username>`. Tokens stored in plaintext.

**Encrypted mode** (`--password`) — Volume name: `kilo-<hash>` (derived from password). Tokens stored as AES-256-CBC ciphertext.

**Ainstruct mode** (`--ainstruct`) — Volume name: `kilo-<hash>` (derived from Ainstruct user_id). Tokens stored as AES-256-CBC ciphertext.

The volume persists across container restarts. Use `kilo-docker init` to reset tokens, or `kilo-docker cleanup` to remove all state (volume, containers, image, and installed binary).

### Updating config from template

When a new Kilo Docker image adds MCP servers or config changes, run:

```bash
kilo-docker update-config
```

This downloads the latest `opencode.json` template from the repository and merges it with your existing config. New servers are added, existing customizations are preserved. Run `kilo-docker --password update-config` for encrypted volumes.

## Backup and Restore

Create a backup of your volume to transfer data between hosts or protect against data loss:

```bash
# Create backup with auto-generated filename
kilo-docker backup

# Create backup with custom filename
kilo-docker backup ~/my-kilo-backup.tar.gz

# Restore from backup
kilo-docker restore ~/my-kilo-backup.tar.gz
```

Backups are portable tar.gz archives containing all volume data. The restore command validates the archive and preserves file ownership (UID 1000).

> **Note:** Encrypted volumes (`--password`) require the same password for backup and restore. Backups from encrypted volumes are standard tar.gz files (the encryption applies only to tokens at rest, not the backup archive itself).

## Session Management

Kilo-docker tracks sessions by directory. Each working directory gets its own container (named by SHA-256 hash of the path).

```bash
# List all sessions
kilo-docker sessions

# Attach to a session by name or index
kilo-docker sessions <name-or-index>

# Remove a session (interactive selection)
kilo-docker sessions cleanup

# Remove a specific session
kilo-docker sessions cleanup <name-or-index>

# Remove all exited sessions
kilo-docker sessions cleanup -a

# Recreate a session with the same flags (preserves volume)
kilo-docker sessions recreate <name-or-index>
```

When attaching to a session, `kilo-docker` detects the container state: if running it attaches directly, if stopped it starts the container then attaches.

## MCP Servers

### Base Image

| Server | Description | Auth |
|--------|-------------|------|
| `context7` | Library documentation lookup | Bearer token |
| `ainstruct` | Document storage and semantic search | Bearer token |
| `playwright` | Browser automation (screenshots, navigation) | None (local sidecar) |

`context7` and `ainstruct` require Bearer token authentication. Tokens are prompted on first run and stored in the named volume for subsequent runs.

`playwright` is only available when using the `--playwright` flag. It runs as a separate container on a shared Docker network with no authentication required.

## Usage on Remote Hosts

```bash
# Run directly on a remote host (binary download)
ssh remote-host 'curl -fsSL -o ~/.local/bin/kilo-docker https://github.com/mbabic84/kilo-docker/releases/latest/download/kilo-docker-linux-amd64 && chmod +x ~/.local/bin/kilo-docker && ~/.local/bin/kilo-docker'
```

> Tokens are prompted interactively on first run via the TTY.

### Shared Hosts

On shared hosts where other users have Docker access, use `--password` to protect your volume and tokens:

```bash
ssh remote-host 'kilo-docker --password'
```

This ensures other users cannot discover your volume or read your API tokens.

### SSH Alias for Convenience

Add to your `~/.ssh/config`:

```
Host remote
    HostName remote.example.com
    User username
    RequestTTY yes
    RemoteCommand ~/.local/bin/kilo-docker
```

## Image Tags

| Tag | Description |
|-----|-------------|
| `latest` | Most recent stable release (base) |
| `v{version}` | Exact semantic version (e.g., `v1.2.3`) |
| `v{major}.{minor}` | Minor track (e.g., `v1.2`) |

## Building Locally

```bash
# Build Go binaries and Docker image
scripts/build.sh all

# Build Docker image only
scripts/build.sh docker

# Test
docker run --rm kilo-docker --version

# Interactive
docker run -it --rm -v $(pwd):/workspace -e PUID=$(id -u) -e PGID=$(id -g) kilo-docker
```

The build uses a multi-stage Dockerfile: a `golang:1.26-alpine` builder compiles the `kilo-entrypoint` binary as a static binary, then the runtime stage copies it into the final Alpine image. No Go toolchain is needed on the host.

## License

See [LICENSE](LICENSE) for details.
