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
- **One-time sessions** - `--once` flag for ephemeral runs without persistence
- **Browser automation** - `--playwright` flag starts a Playwright MCP sidecar for screenshots, navigation, and web interaction

## Quick Start

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

On first run, the script prompts for MCP server tokens and saves them to a named Docker volume. Subsequent runs reuse the saved tokens.

## Script Commands

| Command | Description |
|---------|-------------|
| *(none)* | Start Kilo in interactive mode |
| `run "prompt"` | Run Kilo in autonomous mode |
| `init` | Reset configuration (remove volume, re-enter tokens) |
| `cleanup` | Remove volume, containers, and image |
| `update` | Pull the latest Docker image |
| `help` | Show help message |

### Options

| Option | Description |
|--------|-------------|
| `--once` | Run a one-time session without persistence (no volume) |
| `--password`, `-p` | Protect volume with a password (encrypts tokens, derives volume name from password) |
| `--playwright` | Start a Playwright MCP sidecar container for browser automation |
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

## Data Persistence

The script uses a named Docker volume mounted at `/home/kilo/.local/share/kilo`. This stores:

- SQLite database
- MCP server tokens
- Auth state, logs, and snapshots

**Default mode** — Volume name: `kilo-data-<username>`. Tokens stored in plaintext.

**Encrypted mode** (`--password`) — Volume name: `kilo-<hash>` (derived from password). Tokens stored as AES-256-CBC ciphertext.

The volume persists across container restarts. Use `kilo-docker init` to reset tokens, or `kilo-docker cleanup` to remove all state (volume, containers, and image).

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
├── Dockerfile                  # Base image (Alpine, musl kilo binary)
├── configs/
│   └── opencode.json           # Kilo config for base image
└── scripts/
    ├── kilo-docker             # CLI wrapper script
    ├── entrypoint.sh           # Entrypoint for Alpine (base image)
    └── setup-kilo-config.sh    # Shared config logic (sourced by entrypoint)
```

## License

See [LICENSE](LICENSE) for details.
