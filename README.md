# Kilo Docker

Docker environment for [Kilo CLI](https://kilo.ai/docs/code-with-ai/platforms/cli) - enabling portable, zero-install execution on remote hosts.

## Features

- **Alpine Linux** - Lightweight image with `git`, `ca-certificates`, and `openssh-client`
- **Non-root user** - Runs as `node` user (UID 1000) for security
- **Persistent database** - SQLite database and auth state survive container restarts via named volume
- **Token persistence** - MCP server tokens are saved in the volume on first run
- **Host user mapping** - Runs with your UID/GID so files are owned by you

## Quick Start

```bash
# Interactive mode
./scripts/kilo-docker

# Autonomous mode
./scripts/kilo-docker run "your prompt here"

# Show all commands
./scripts/kilo-docker help
```

On first run, the script prompts for MCP server tokens and saves them to a named Docker volume. Subsequent runs reuse the saved tokens.

## Script Commands

| Command | Description |
|---------|-------------|
| *(none)* | Start Kilo in interactive mode |
| `run "prompt"` | Run Kilo in autonomous mode |
| `init` | Reset configuration (remove volume, re-enter tokens) |
| `update` | Pull the latest Docker image |
| `help` | Show help message |

## Data Persistence

The script uses a named Docker volume (`kilo-data-<username>`) mounted at `/home/user/.local/share/kilo`. This stores:

- SQLite database
- MCP server tokens
- Auth state, logs, and snapshots

The volume persists across container restarts. Use `kilo-docker init` to reset.

## MCP Servers

The image ships with two remote MCP servers pre-configured:

| Server | Description |
|--------|-------------|
| `context7` | Library documentation lookup |
| `ainstruct` | Document storage and semantic search |

Both require Bearer token authentication. Tokens are prompted on first run and stored in the named volume for subsequent runs.

### Passing tokens via environment

If `CONTEXT7_TOKEN` or `AINSTRUCT_TOKEN` are already set in your environment, the script detects them and offers to use them during first-time setup.

## Usage on Remote Hosts

```bash
# SSH and run
ssh remote-host './scripts/kilo-docker'

# With API key from environment
ssh remote-host 'CONTEXT7_TOKEN="$CONTEXT7_TOKEN" ./scripts/kilo-docker'

# One-liner (copy script to remote host first)
ssh remote-host "bash -s" < scripts/kilo-docker
```

### SSH Alias for Convenience

Add to your `~/.ssh/config`:

```
Host remote
    HostName remote.example.com
    User username
    RequestTTY yes
    RemoteCommand ./scripts/kilo-docker
```

## Image Tags

| Tag | Description |
|-----|-------------|
| `latest` | Most recent stable release |
| `v{version}` | Exact semantic version (e.g., `v1.2.3`) |
| `v{major}.{minor}` | Minor track (e.g., `v1.2`) |

## Building Locally

```bash
# Build
docker build -t kilo-docker .

# Test
docker run -it --rm -v $(pwd):/workspace -w /workspace kilo-docker --version
```

To use the local image, update `IMAGE` in `scripts/kilo-docker`:

```bash
IMAGE="kilo-docker"
```
