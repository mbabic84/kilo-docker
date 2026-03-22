# Kilo Docker

Docker environment for [Kilo CLI](https://kilo.ai/docs/code-with-ai/platforms/cli) - enabling portable, zero-install execution on remote hosts.

## Features

- **Alpine Linux** - Lightweight image with `git`, `openssh-client`, `ripgrep`, and `libstdc++`
- **Non-root user** - Runs as `kilo` user with dynamic `PUID`/`PGID` mapping to match host user
- **Persistent database** - SQLite database and auth state survive container restarts via named volume
- **Token persistence** - MCP server tokens are prompted once and saved in the volume
- **One-time sessions** - `--once` flag for ephemeral runs without persistence

## Quick Start

No need to clone the repository. Run directly with `curl` or `wget`:

```bash
# Interactive mode (curl)
bash <(curl -fsSL https://raw.githubusercontent.com/mbabic84/kilo-docker/main/scripts/kilo-docker)

# Interactive mode (wget)
bash <(wget -qO- https://raw.githubusercontent.com/mbabic84/kilo-docker/main/scripts/kilo-docker)

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

## One-Time Sessions

Use `--once` to run without creating or mounting a named volume. No data persists after the container exits:

```bash
kilo-docker --once

# Autonomous one-shot
kilo-docker --once run "fix build errors"
```

This is useful for CI pipelines, ephemeral environments, or when you don't want to leave any state on the host.

## Data Persistence

The script uses a named Docker volume (`kilo-data-<username>`) mounted at `/home/kilo/.local/share/kilo`. This stores:

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

## Usage on Remote Hosts

```bash
# Run directly on a remote host (no clone needed)
ssh remote-host 'bash <(curl -fsSL https://raw.githubusercontent.com/mbabic84/kilo-docker/main/scripts/kilo-docker)'
```

> Tokens are prompted interactively on first run via the TTY.

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
| `latest` | Most recent stable release |
| `v{version}` | Exact semantic version (e.g., `v1.2.3`) |
| `v{major}.{minor}` | Minor track (e.g., `v1.2`) |

## Building Locally

```bash
# Build
docker build -t kilo-docker .

# Test
docker run --rm kilo-docker --version

# Interactive
docker run -it --rm -v $(pwd):/workspace -e PUID=$(id -u) -e PGID=$(id -g) kilo-docker
```
