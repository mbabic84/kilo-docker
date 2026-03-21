# Kilo Docker

Docker environment for [Kilo CLI](https://kilo.ai/docs/code-with-ai/platforms/cli) - enabling portable, zero-install execution on remote hosts.

## Features

- **Alpine Linux** - Lightweight image with `git`, `ca-certificates`, and `openssh-client`
- **Non-root user** - Runs as `node` user (UID 1000) for security
- **Persistent database** - SQLite database and auth state survive container restarts via named volume
- **Token persistence** - MCP server tokens are saved in the volume on first run
- **One-time sessions** - `--once` flag for ephemeral runs without persistence
- **Host user mapping** - Runs with your UID/GID so files are owned by you

## Quick Start

No need to clone the repository. Run directly with `curl` or `wget`:

```bash
# Interactive mode (curl)
curl -fsSL https://raw.githubusercontent.com/mbabic84/kilo-docker/main/scripts/kilo-docker | bash

# Interactive mode (wget)
wget -qO- https://raw.githubusercontent.com/mbabic84/kilo-docker/main/scripts/kilo-docker | bash

# Autonomous mode
curl -fsSL https://raw.githubusercontent.com/mbabic84/kilo-docker/main/scripts/kilo-docker | bash -s -- run "your prompt here"

# Show all commands
curl -fsSL https://raw.githubusercontent.com/mbabic84/kilo-docker/main/scripts/kilo-docker | bash -s -- help
```

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

## One-Time Sessions

Use `--once` to run without creating or mounting a named volume. No data persists after the container exits. Tokens must be provided via environment variables:

```bash
CONTEXT7_TOKEN=xxx AINSTRUCT_TOKEN=xxx kilo-docker --once

# Autonomous one-shot
CONTEXT7_TOKEN=xxx kilo-docker --once run "fix build errors"
```

This is useful for CI pipelines, ephemeral environments, or when you don't want to leave any state on the host.

## Data Persistence

The script uses a named Docker volume (`kilo-data-<username>`) mounted at `/home/user/.local/share/kilo`. This stores:

- SQLite database
- MCP server tokens
- Auth state, logs, and snapshots

The volume persists across container restarts. Use `kilo-docker init` to reset tokens, or `kilo-docker cleanup` to remove all state (volume, containers, and image).

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
# Run directly on a remote host (no clone needed)
ssh remote-host 'curl -fsSL https://raw.githubusercontent.com/mbabic84/kilo-docker/main/scripts/kilo-docker | bash'

# With API key from environment
ssh remote-host 'curl -fsSL https://raw.githubusercontent.com/mbabic84/kilo-docker/main/scripts/kilo-docker | CONTEXT7_TOKEN="$CONTEXT7_TOKEN" bash'

# Pipe script via stdin
ssh remote-host "bash -s" < <(curl -fsSL https://raw.githubusercontent.com/mbabic84/kilo-docker/main/scripts/kilo-docker)
```

### SSH Alias for Convenience

Add to your `~/.ssh/config`:

```
Host remote
    HostName remote.example.com
    User username
    RequestTTY yes
    RemoteCommand bash -c 'curl -fsSL https://raw.githubusercontent.com/mbabic84/kilo-docker/main/scripts/kilo-docker | bash'
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

To use the local image, set the `IMAGE` environment variable:

```bash
IMAGE="kilo-docker" ./kilo-docker
```
