# Kilo Docker

Docker environment for [Kilo CLI](https://kilo.ai/docs/code-with-ai/platforms/cli) - enabling portable, zero-install execution on remote hosts.

## Image Variants

| Image | Base | Size | Description |
|-------|------|------|-------------|
| `ghcr.io/mbabic84/kilo-docker:latest` | Alpine | ~192 MB | Lightweight base with `git`, `openssh-client`, `ripgrep`, and `libstdc++` |

## Features

- **Non-root user** - Runs as `kilo` user with dynamic `PUID`/`PGID` mapping to match host user
- **Persistent database** - SQLite database and auth state survive container restarts via named volume
- **Token persistence** - MCP server tokens are prompted once and saved in the volume
- **Volume encryption** - `--password` flag encrypts tokens and derives a non-discoverable volume name
- **Ainstruct auth** - `--ainstruct` flag authenticates with the Ainstruct API to derive volume name from user_id
- **Ainstruct file sync** - `--ainstruct` flag enables automatic push/pull sync of config files, commands, agents, and instructions via the Ainstruct API
- **One-time sessions** - `--once` flag for ephemeral runs without persistence
- **Browser automation** - `--playwright` flag starts a Playwright MCP sidecar for screenshots, navigation, and web interaction
- **Docker access** - `--docker` flag mounts the host Docker socket for container management from within Kilo
- **Zellij sessions** - `--zellij` flag starts a Zellij terminal multiplexer session

## Quick Start

Install `kilo-docker` as a global command:

```bash
# curl
bash <(curl -fsSL https://raw.githubusercontent.com/mbabic84/kilo-docker/main/scripts/kilo-docker) install

# wget
bash <(wget -qO- https://raw.githubusercontent.com/mbabic84/kilo-docker/main/scripts/kilo-docker) install
```

Then run from any directory:

```bash
kilo-docker
```

No need to clone the repository. Run directly with `curl` or `wget`:

```bash
# Interactive mode (base image)
bash <(curl -fsSL https://raw.githubusercontent.com/mbabic84/kilo-docker/main/scripts/kilo-docker)

# Autonomous mode
bash <(curl -fsSL https://raw.githubusercontent.com/mbabic84/kilo-docker/main/scripts/kilo-docker) run "your prompt here"

# Show all commands
bash <(curl -fsSL https://raw.githubusercontent.com/mbabic84/kilo-docker/main/scripts/kilo-docker) help
```

> **Note:** Use `bash <(...)` instead of `curl | bash` to preserve stdin for interactive input. Piping consumes stdin, which breaks interactive prompts and `docker run -it`.

Alternatively, download the script once and run it locally:

```bash
curl -fsSL -o kilo-docker https://raw.githubusercontent.com/mbabic84/kilo-docker/main/scripts/kilo-docker
chmod +x kilo-docker
./kilo-docker
```

Or install it as a global command (symlinked to `~/.local/bin/kilo-docker`):

```bash
bash kilo-docker install
```

After installing, `kilo-docker` is available from any directory. The install command also pulls the Docker image so it is ready to use immediately.

On first run, the script prompts for MCP server tokens and saves them to a named Docker volume. Subsequent runs reuse the saved tokens.

## Script Commands

| Command | Description |
|---------|-------------|
| *(none)* | Start Kilo in interactive mode |
| `run "prompt"` | Run Kilo in autonomous mode |
| `backup [-f]` | Create backup of volume to tar.gz |
| `restore <file>` | Restore volume from backup |
| `init` | Reset configuration (remove volume, re-enter tokens) |
| `cleanup` | Remove volume, containers, image, and installed script |
| `install` | Install as a global command (`~/.local/bin/kilo-docker`) |
| `update` | Update the installed script and pull the latest Docker image |
| `help` | Show help message |

### Options

| Option | Description |
|--------|-------------|
| `--once` | Run a one-time session without persistence (no volume) |
| `--password`, `-p` | Protect volume with a password (encrypts tokens, derives volume name from password) |
| `--ainstruct` | Authenticate with Ainstruct API (volume from user_id, tokens encrypted, file sync enabled) |
| `--playwright` | Start a Playwright MCP sidecar container for browser automation |
| `--docker` | Mount Docker socket for container management from within Kilo |
| `--zellij` | Start a Zellij terminal multiplexer session |
| `--network <name>` | Attach to a specific Docker network |

## One-Time Sessions

Use `--once` to run without creating or mounting a named volume. No data persists after the container exits:

```bash
kilo-docker --once

# Autonomous one-shot
kilo-docker --once run "fix build errors"
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

## Ainstruct Authentication & File Sync

Use `--ainstruct` to authenticate with the Ainstruct API. This enables two features:

### Volume naming & encryption

The username and password are used to obtain the user's `user_id`, which is then used for volume naming and token encryption:

```bash
kilo-docker --ainstruct
```

On first run, you are prompted for your Ainstruct username and password. The script authenticates via the API, obtains the `user_id`, and derives a non-discoverable volume name and encryption key from it. MCP server tokens are then prompted and stored encrypted in the volume.

### File sync

When `--ainstruct` is used, configuration files are automatically synced to and from the Ainstruct API:

**Synced files:**
- `~/.config/kilo/opencode.json` — Kilo configuration
- `~/.config/kilo/rules/*.md` — Instruction files
- `~/.kilo/command/*.md` — Custom slash commands
- `~/.kilo/agent/*.md` — Custom agent definitions

**Push (local → API):** A Go-based file watcher (`ainstruct-sync`) detects local changes via inotify with a 5-second quiet-period debounce (rapid events are coalesced into a single sync) and pushes updates to the Ainstruct API.

**Pull (API → local):** On container startup, the sync state file (`~/.config/kilo/.ainstruct-hashes`) is compared against the API's `content_hash` values. Only changed or new files are downloaded. Unchanged files are skipped without any API calls.

**Token refresh:** JWT access tokens (30 min lifetime) are refreshed automatically before API calls when within 60 seconds of expiry.

## Browser Automation

The `--playwright` flag starts a [Playwright MCP](https://github.com/microsoft/playwright-mcp) sidecar container alongside Kilo, enabling browser automation (screenshots, navigation, form filling, etc.):

```bash
# Interactive with browser
kilo-docker --playwright

# Autonomous with browser
kilo-docker --once --playwright run "take a screenshot of example.com"
```

The sidecar runs headless Chromium in HTTP mode on port 8931 inside a dedicated Docker network (`kilo-playwright-<username>`). Both the sidecar container and network are automatically cleaned up when Kilo exits.

Screenshots and other output files are saved to `.playwright-mcp/` in the workspace directory.

## Docker Socket Access

The `--docker` flag mounts the host Docker socket into the container, allowing Kilo to manage containers, images, and networks from within the session:

```bash
# Interactive with Docker access
kilo-docker --docker

# Autonomous with Docker access
kilo-docker --once --docker run "list running containers and check their logs"
```

The Docker CLI and Compose plugin are installed at runtime inside the container. The socket's group ownership is dynamically matched so `docker` commands work without `sudo`.

> **Security:** Mounting the Docker socket grants full Docker API access inside the container, which is equivalent to root access on the host. Only use `--docker` in trusted environments.

## Zellij Sessions

The `--zellij` flag starts a [Zellij](https://zellij.dev/) terminal multiplexer session:

```bash
# Interactive with Zellij
kilo-docker --zellij
```

Run `kilo` inside the session to start Kilo. Zellij is installed at runtime from the latest GitHub release. Pane frames, startup tips, and release notes are hidden by default.

### Key Bindings

| Action | Keys |
|--------|------|
| Enter session mode | `Ctrl+P` (pane), `Ctrl+T` (tab), `Ctrl+N` (resize) |
| Quit | `Ctrl+Q` |

Zellij configuration is stored in `configs/zellij.kdl` and copied to the container at runtime.

## Data Persistence

The script uses a named Docker volume mounted at `/home/kilo-t8x3m7kp`. This stores:

- SQLite database, auth state, logs
- Configuration (`opencode.json` — model selection, provider connections, MCP settings)
- Custom commands (`.kilo/command/*.md`) and agents (`.kilo/agent/*.md`)
- Instruction files (`.config/kilo/rules/*.md`)
- Session state and snapshots
- Cache
- Ainstruct sync state (`.config/kilo/.ainstruct-hashes`) — when using `--ainstruct`

**Default mode** — Volume name: `kilo-data-<username>`. Tokens stored in plaintext.

**Encrypted mode** (`--password`) — Volume name: `kilo-<hash>` (derived from password). Tokens stored as AES-256-CBC ciphertext.

**Ainstruct mode** (`--ainstruct`) — Volume name: `kilo-<hash>` (derived from Ainstruct user_id). Tokens stored as AES-256-CBC ciphertext.

The volume persists across container restarts. Use `kilo-docker init` to reset tokens, or `kilo-docker cleanup` to remove all state (volume, containers, image, and installed script).

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
kilo-docker backup -f ~/my-kilo-backup.tar.gz

# Restore from backup
kilo-docker restore ~/my-kilo-backup.tar.gz
```

Backups are portable tar.gz archives containing all volume data. The restore command validates the archive and preserves file ownership (UID 1000).

> **Note:** Encrypted volumes (`--password`) require the same password for backup and restore. Backups from encrypted volumes are standard tar.gz files (the encryption applies only to tokens at rest, not the backup archive itself).

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
# Run directly on a remote host (no clone needed)
ssh remote-host 'bash <(curl -fsSL https://raw.githubusercontent.com/mbabic84/kilo-docker/main/scripts/kilo-docker)'
```

> Tokens are prompted interactively on first run via the TTY.

### Shared Hosts

On shared hosts where other users have Docker access, use `--password` to protect your volume and tokens:

```bash
ssh remote-host 'bash <(curl -fsSL https://raw.githubusercontent.com/mbabic84/kilo-docker/main/scripts/kilo-docker)' -- --password
```

This ensures other users cannot discover your volume or read your API tokens.

### SSH Alias for Convenience

Add to your `~/.ssh/config`:

```
Host remote
    HostName remote.example.com
    User username
    RequestTTY yes
    RemoteCommand bash <(curl -fsSL https://raw.githubusercontent.com/mbabic84/kilo-docker/main/scripts/kilo-docker)
```

## Image Tags

| Tag | Description |
|-----|-------------|
| `latest` | Most recent stable release (base) |
| `v{version}` | Exact semantic version (e.g., `v1.2.3`) |
| `v{major}.{minor}` | Minor track (e.g., `v1.2`) |

## Building Locally

```bash
# Build image
docker build -t kilo-docker .

# Test
docker run --rm kilo-docker --version

# Interactive
docker run -it --rm -v $(pwd):/workspace -e PUID=$(id -u) -e PGID=$(id -g) kilo-docker
```

## Project Structure

```
├── Dockerfile                  # Multi-stage build (Go builder + Alpine runtime)
├── go.mod                      # Go module definition
├── go.sum                      # Go dependency checksums
├── cmd/
│   └── ainstruct-sync/
│       └── main.go             # File sync watcher (inotify, JWT, REST API)
├── configs/
│   ├── opencode.json           # Kilo config for base image
│   └── zellij.kdl              # Zellij config (keybinds, pane settings)
└── scripts/
    ├── kilo-docker             # CLI wrapper script
    ├── entrypoint.sh           # Entrypoint for Alpine (base image)
    └── setup-kilo-config.sh    # Shared config logic (sourced by entrypoint)
```

## License

See [LICENSE](LICENSE) for details.
