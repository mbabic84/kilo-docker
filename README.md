# Kilo Docker

Docker environment for [Kilo CLI](https://kilo.ai/docs/code-with-ai/platforms/cli) - enabling portable, zero-install execution on remote hosts.

## Features

- **Non-root user** - Runs as `kilo` user with dynamic `PUID`/`PGID` mapping to match host user
- **Persistent database** - SQLite database and auth state survive container restarts via named volume
- **Token persistence** - MCP server tokens are prompted once and saved in the volume
- **One-time sessions** - `--once` flag for ephemeral runs without persistence
- **Browser automation** - `--playwright` flag starts a Playwright MCP sidecar for screenshots, navigation, and web interaction
- **Built-in services** - Extensible service system with `--docker`, `--go`, `--node`, and more (see [Services](#services))

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
| `--volume`, `-v` | Mount a volume (host_path:container_path), repeatable |
| `--workspace`, `-w` | Set custom workspace path (default: current directory) |
| `--port`, `-p` | Map port (host_port:container_port), repeatable |
| `--playwright` | Start a Playwright MCP sidecar container for browser automation |
| `--ssh` | Enable SSH agent forwarding into the container |
| `--network <name>` | Attach to a specific Docker network |
| `--yes`, `-y` | Auto-confirm all prompts (useful for piped/non-interactive installs) |
| `--version` | Print kilo-docker version |

### Volume Mounts

Use `--volume` (or `-v`) to mount additional host directories or files into the container:

```bash
# Mount a single directory
kilo-docker --volume /host/data:/container/data

# Mount multiple volumes
kilo-docker -v /host/cache:/cache -v ~/.config:/home/kd-xxx/.config

# Mount with read-only access
kilo-docker --volume /host/readonly:/container/readonly:ro
```

Volume mounts follow the same format as Docker's `-v` flag:
- `host_path:container_path` — Standard mount
- `host_path:container_path:ro` — Read-only mount
- Named volumes: `volume_name:container_path`

The current working directory is always mounted at the same path automatically.

### Services

| Service | Description |
|--------|-------------|
| `--docker` | Mount Docker socket for container management from within Kilo |
| `--go` | Install Go (latest stable) for development |
| `--node` | Install Node.js LTS for development |
| `--gh` | Install GitHub CLI for interacting with GitHub |
| `--uv` | Install uv for fast Python package management |
| `--nvm` | Install NVM (Node Version Manager) for managing Node.js versions |
| `--python` | Install Python 3 for general purpose use |

## One-Time Sessions

Use `--once` to run without creating or mounting a named volume. No data persists after the container exits:

```bash
kilo-docker --once
```

This is useful for CI pipelines, ephemeral environments, or when you don't want to leave any state on the host.

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

**Default mode** — Volume name: `kilo-data-<username>`. Tokens stored in plaintext.

The volume persists across container restarts. Use `kilo-docker init` to reset tokens, or `kilo-docker cleanup` to remove all state (volume, containers, image, and installed binary).

### Updating config from template

When a new Kilo Docker image adds MCP servers or config changes, run:

```bash
kilo-docker update-config
```

This downloads the latest `opencode.json` template from the repository and merges it with your existing config. New servers are added, existing customizations are preserved.

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
