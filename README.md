# Kilo Docker

Docker environment for [Kilo CLI](https://kilo.ai/docs/code-with-ai/platforms/cli) - enabling portable, zero-install execution on remote hosts.

## Features

- **Non-root user** - Runs as `kilo` user with dynamic `PUID`/`PGID` mapping to match host user
- **Persistent database** - SQLite database and auth state survive container restarts via named volume
- **Token persistence** - MCP server tokens are prompted once and saved in encrypted volume storage
- **Custom environment variables** - Store your own API keys and config values in encrypted storage, available in all sessions
- **One-time sessions** - `--once` flag for ephemeral runs without persistence
- **Browser automation** - `--playwright` flag starts a Playwright MCP sidecar for screenshots, navigation, and web interaction
- **Built-in services** - Extensible service system with `--docker`, `--go`, `--nvm`, and more (see [Services](#services))

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

## Commands

| Command | Description |
|---------|-------------|
| `sessions [name\|index]` | List sessions or attach to one by name or index |
| `sessions stop <name\|index>` | Stop a running session, freeing its ports |
| `sessions cleanup [-y] [name\|index]` | Remove a session (interactive if no name given) |
| `sessions cleanup -y -a` | Remove all exited sessions |
| `sessions recreate <name\|index>` | Recreate a session with the same flags (preserves volume) |
| `networks` | List available Docker networks |
| `playwright` | Recreate the Playwright MCP sidecar container |
| `profile save <name> <flags>` | Save current flags as a profile (flags passed after name) |
| `profile list` | List all saved profiles (default marked with `*`) |
| `profile show <name>` | Display full profile JSON |
| `profile edit <name>` | Open profile in $EDITOR |
| `profile delete <name>` | Remove a profile |
| `profile import <file>` | Load a profile from a JSON file |
| `profile export <name>` | Print profile JSON to stdout |
| `profile set-default <name>` | Set a profile as the default |
| `profile unset-default` | Remove the default profile |
| `profile show-default` | Print the current default profile name |
| `backup [-f] [--legacy-volume]` | Create backup of volume to tar.gz |
| `restore <file> [-f] [-v\|--volume <name>] [--legacy-volume]` | Restore volume from backup |
| `init` | Reset configuration (remove volume, re-enter tokens) |
| `cleanup` | Remove volume, containers, image, and installed binary |
| `update [config]` | Pull latest Docker image and update binary, or merge config template |
| `version` | Show kilo-docker and kilo versions |
| `help` | Show help message |

### Options

| Option | Description |
|--------|-------------|
| `--help`, `-h` | Show help message |
| `--once` | Run a one-time session without persistence (no volume) |
| `--volume`, `-v` | Mount a volume (host_path:container_path), repeatable |
| `--workspace`, `-w` | Set custom workspace path (default: current directory) |
| `--port`, `-p` | Map port (host_port:container_port), repeatable |
| `--playwright` | Start a Playwright MCP sidecar container for browser automation |
| `--ssh` | Enable SSH agent forwarding into the container |
| `--network <name>` | Attach to a Docker network (repeatable, `kilo-shared` included by default). Use `host` to share the host network stack |
| `--profile <name>` | Load a named flag profile from `~/.config/kilo-docker/profiles/` |
| `--yes`, `-y` | Auto-confirm all prompts (useful for piped/non-interactive installs) |

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
| `--build` | Install build essentials (gcc, g++, make) for compiling native extensions |
| `--gh` | Install GitHub CLI for interacting with GitHub |
| `--uv` | Install uv for fast Python package management |
| `--nvm` | Install NVM (Node Version Manager) for managing Node.js versions |
| `--rclone` | Install rclone, a universal CLI for S3 and 40+ cloud storage backends |
| `--gitnexus` | Install GitNexus for codebase knowledge graph indexing and MCP-based code intelligence |

### Config Profiles

Named profiles let you save reusable flag combinations and apply them with a single `--profile` flag. Profiles are stored as JSON files under `~/.config/kilo-docker/profiles/`.

```bash
# Save current flags as a profile
kilo-docker --go --ssh --docker --workspace /path profile save fullstack

# List all profiles (default marked with *)
kilo-docker profile list

# Show full profile JSON
kilo-docker profile show fullstack

# Edit a profile in your editor
kilo-docker profile edit fullstack

# Use a profile
kilo-docker --profile fullstack
```

#### Default Profile

Set a default profile and it auto-loads whenever you run `kilo-docker` with no flags:

```bash
kilo-docker profile set-default fullstack
kilo-docker                        # auto-loads --go --ssh --docker
kilo-docker --profile other       # explicit profile overrides default
kilo-docker --ssh                  # CLI flags override profile flags
kilo-docker profile unset-default  # stop auto-loading
```

**Merge precedence:** CLI flags always win. Services from the profile are additive — a service already enabled by CLI won't be disabled. SSH only enables from the profile if not already set on the CLI. Ports, volumes, and networks are always additive. When `--network host` is used, all other networks are discarded (Docker does not allow combining host with other network modes).

#### Import/Export

Share profiles between machines:

```bash
kilo-docker profile export fullstack > fullstack.json
scp fullstack.json remote-host:
ssh remote-host kilo-docker profile import fullstack.json
```

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

The sidecar runs headless Chromium in HTTP mode on port 8931 inside the shared Docker network (`kilo-shared`). The Playwright container is shared across sessions — if already running, it's reused rather than recreated.

Screenshots and output files are saved to a shared volume (`kilo-playwright-output`) mounted at `/mnt/playwright-output` inside the Kilo container.

## SSH Agent Forwarding

Use the `--ssh` flag to enable SSH agent forwarding. The host binary detects whether an SSH agent is running on the host:

- **Agent running** — Uses the existing agent via `$SSH_AUTH_SOCK`
- **No agent** — Starts one automatically, loads all private keys from `~/.ssh/`, and cleans up on exit

The container mounts the host's SSH agent socket, allowing `git`, `ssh`, and `scp` to use your host SSH keys without copying private keys into the container.

```bash
kilo-docker --ssh
```

> **Security:** Private keys never enter the container. The container communicates with the host's SSH agent via a Unix socket.

## Networking

All Kilo containers are automatically attached to a shared Docker network (`kilo-shared`), enabling container-to-container communication by name across sessions:

```bash
# Kilo containers can resolve each other by container name
kilo-docker
# Inside container: ping <other-container-name>
```

You can attach to additional networks using the `--network` flag (repeatable):

```bash
kilo-docker --network my-network
kilo-docker --network net1 --network net2
```

The `kilo-shared` network is always included implicitly — it doesn't need to be specified and won't trigger flag mismatch detection when comparing stored vs current args.

### Host Network Mode

Use `--network host` to share the host's network stack. The container's `localhost` becomes the host's `localhost`, giving direct access to any services or other containers running on the host:

```bash
kilo-docker --network host
```

When `host` is specified, all other networks (including `kilo-shared`) are ignored, and a warning is printed if additional `--network` flags were passed. This matches Docker's restriction that host networking cannot be combined with other networks.

Host networking also bypasses port mapping — all host ports are directly accessible inside the container without needing `--port` / `-p`.

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

The host binary uses a per-user named Docker volume mounted at `/home`. Each user gets their own volume (named by SHA-256 hash of the username, e.g. `kilo-a1b2c3d4e5f6-data`), providing data isolation between users. All sessions for the same user share this volume. Inside the container, the user home directory is dynamically generated as `/home/kd-<hash>`. This stores:

- SQLite database, auth state, logs
- Configuration (`kilo.jsonc` — model selection, provider connections, MCP settings)
- Custom commands (`.config/kilo/commands/*.md`) and agents (`.config/kilo/agents/*.md`)
- Plugins (`.config/kilo/plugins/*.{js,ts}`), skills (`.config/kilo/skills/*/SKILL.md`), tools (`.config/kilo/tools/*.{js,ts}`)
- Instruction files (`.config/kilo/rules/*.md`)
- Session state and snapshots
- Cache

The volume persists across container restarts. Use `kilo-docker init` to reset tokens, or `kilo-docker cleanup` to remove all state (volume, containers, image, and installed binary).

### Automatic migration from shared volume

If you previously used a shared `kilo-docker-data` volume, `kilo-docker` will automatically copy your data to a new per-user volume on first run. The legacy volume is left intact so you can verify the migration succeeded. Remove it manually once confirmed:

```bash
docker volume rm kilo-docker-data
```

### Updating config from template

When a new Kilo Docker image adds MCP servers or config changes, run:

```bash
kilo-docker update config
```

This downloads the latest `kilo.jsonc` template from the repository and merges it with your existing config. New servers are added, existing customizations are preserved.

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

During the transition from the shared volume, use `--legacy-volume` to back up or restore the old shared volume:

```bash
# Backup the legacy shared volume
kilo-docker backup --legacy-volume

# Restore to the legacy shared volume
kilo-docker restore ~/my-kilo-backup.tar.gz --legacy-volume
```

Backups are portable tar.gz archives containing all volume data. The restore command validates the archive and preserves file ownership.

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

# Stop a running session (frees ports, preserves container)
kilo-docker sessions stop <name-or-index>

# Recreate a session with the same flags (preserves volume)
kilo-docker sessions recreate <name-or-index>
```

When attaching to a session, `kilo-docker` detects the container state: if running it attaches directly, if stopped it starts the container then attaches.

## MCP Servers

### Base Image

| Server | Description | Auth |
|--------|-------------|------|
| `context7` | Library documentation lookup | Bearer token |
| `ainstruct` | Document storage and semantic search | Bearer token (auto-created) |
| `playwright` | Browser automation (screenshots, navigation) | None (local sidecar) |
| `gitnexus` | Codebase knowledge graph indexing and MCP-based code intelligence | None (local) |

`context7` requires a Bearer token. Use `kilo-entrypoint mcp-tokens` to manage MCP tokens interactively. Use `kilo-entrypoint custom-envs` to manage your own environment variables (see [Custom Environment Variables](#custom-environment-variables)).

`playwright` is only available when using the `--playwright` flag. It runs as a separate container on a shared Docker network with no authentication required.

## Custom Environment Variables

Store your own API keys, config values, or any environment variables in AES-256 encrypted storage. Values are loaded automatically into every Kilo session alongside the built-in MCP tokens.

```bash
# List all custom envs (values are masked)
kilo-entrypoint custom-envs list

# Add a new variable
kilo-entrypoint custom-envs add MY_API_KEY "sk-abc123"

# Edit an existing variable
kilo-entrypoint custom-envs edit MY_API_KEY "sk-new-value"

# Get raw value (for scripting)
kilo-entrypoint custom-envs get MY_API_KEY

# Remove a variable
kilo-entrypoint custom-envs remove MY_API_KEY
```

Custom envs use the same encryption as MCP tokens (`AES-256-CBC`, keyed by your user ID) and are stored at `.local/share/kilo/.custom-envs.env.enc`.

To use custom envs from a subshell or script inside a Kilo session:

```bash
eval $(kilo-entrypoint print-env 2>/dev/null)
echo $MY_API_KEY
```

## Usage on Remote Hosts

```bash
# Run directly on a remote host (binary download)
ssh remote-host 'curl -fsSL -o ~/.local/bin/kilo-docker https://github.com/mbabic84/kilo-docker/releases/latest/download/kilo-docker-linux-amd64 && chmod +x ~/.local/bin/kilo-docker && ~/.local/bin/kilo-docker'
```

> Use `kilo-entrypoint mcp-tokens` to manage MCP server tokens and `kilo-entrypoint custom-envs` for custom environment variables.

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
| `v{major}` | Major track (e.g., `v1`) |

## Building Locally

```bash
# Build Go binaries and Docker image
scripts/build.sh all

# Build Docker image only
scripts/build.sh docker

# Test
docker run --rm kilo-docker version

# Interactive
docker run -it --rm -v $(pwd):/workspace -e PUID=$(id -u) -e PGID=$(id -g) kilo-docker
```

The build uses a multi-stage Dockerfile: a `golang:1.26-bookworm` builder compiles the `kilo-entrypoint` binary as a static binary, then the runtime stage copies it into the final Debian Bookworm image. No Go toolchain is needed on the host.

## License

See [LICENSE](LICENSE) for details.
