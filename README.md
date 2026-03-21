# Kilo Docker

Docker environment for [Kilo CLI](https://kilo.ai/docs/code-with-ai/platforms/cli) - enabling portable, zero-install execution on remote hosts.

## Features

- **Node.js LTS** - Latest stable Node.js via Alpine Linux
- **Non-root user** - Runs as `node` user for security
- **Pre-installed tools** - `git`, `curl`, `bash` for common workflows
- **Zero config** - Kilo config is ephemeral with `--rm` flag
- **Persistent database** - Database and auth state survive container restarts via automatic volume

## Quick Start

```bash
# Interactive mode (current directory)
docker run -it --rm \
  -v $(pwd):/workspace \
  -w /workspace \
  ghcr.io/mbabic84/kilo-docker:latest

# Autonomous mode
docker run --rm \
  -v $(pwd):/workspace \
  -w /workspace \
  ghcr.io/mbabic84/kilo-docker:latest run "your prompt here"
```

## Usage on Remote Hosts

```bash
# SSH and run interactively
ssh remote-host 'docker run -it --rm \
  -v $(pwd):/workspace \
  -w /workspace \
  ghcr.io/mbabic84/kilo-docker:latest'

# With API key from environment
ssh remote-host 'docker run -it --rm \
  -v $(pwd):/workspace \
  -w /workspace \
  -e OPENAI_API_KEY="$OPENAI_API_KEY" \
  ghcr.io/mbabic84/kilo-docker:latest'
```

## Image Tags

| Tag | Description |
|-----|-------------|
| `latest` | Most recent stable release |
| `v{version}` | Exact semantic version (e.g., `v1.2.3`) |
| `v{major}.{minor}` | Minor track (e.g., `v1.2`) |

## Configuration

Kilo CLI writes config to `~/.config/kilo`. By default this is inside the container and ephemeral. To persist config:

```bash
# Mount a config directory
docker run -it --rm \
  -v $(pwd):/workspace \
  -v $HOME/.config/kilo:/home/node/.config/kilo \
  -w /workspace \
  ghcr.io/mbabic84/kilo-docker:latest
```

Or use environment variable override:

```bash
docker run -it --rm \
  -v $(pwd):/workspace \
  -e KILO_CONFIG_CONTENT='{"model":"anthropic/claude-sonnet-4"}' \
  ghcr.io/mbabic84/kilo-docker:latest
```

## Data Persistence

The image declares a `VOLUME` at `/home/user/.local/share/kilo` where Kilo stores its SQLite database, auth state, logs, and snapshots. Docker automatically creates a persistent anonymous volume on first run, so the database migration only executes once.

### Using a named volume (recommended)

For more control, mount a named volume explicitly:

```bash
docker run -it --rm \
  -v $(pwd):/workspace \
  -v kilo-data:/home/user/.local/share/kilo \
  -w /workspace \
  ghcr.io/mbabic84/kilo-docker:latest
```

Named volumes survive `docker system prune` and can be inspected with `docker volume inspect kilo-data`.

### Running as host user with volumes

When using `--user $(id -u):$(id -g)`, the container user may not have write permissions to the volume. Grant access by running the container once without `--user` to initialize the volume, or use a named volume created with correct ownership:

```bash
docker volume create kilo-data
docker run --rm -v kilo-data:/home/user/.local/share/kilo ghcr.io/mbabic84/kilo-docker:latest chown -R $(id -u):$(id -g) /home/user/.local/share/kilo
```

## Default MCP Servers

The image ships with two MCP servers pre-configured:

| Server | Type | Description |
|--------|------|-------------|
| `context7` | Remote | Library documentation lookup via `https://mcp.context7.com/mcp` |
| `ainstruct` | Remote | Document storage and semantic search via `https://ainstruct-dev.kralicinora.cz/mcp` |

Both servers require Bearer token authentication via environment variables:

| Environment Variable | Description |
|---------------------|-------------|
| `CONTEXT7_TOKEN` | API token for Context7 MCP server |
| `AINSTRUCT_TOKEN` | API token for ainstruct MCP server |

### Passing tokens at runtime

```bash
# Interactive
docker run -it --rm \
  -v $(pwd):/workspace \
  -e CONTEXT7_TOKEN="your-token" \
  -e AINSTRUCT_TOKEN="your-token" \
  ghcr.io/mbabic84/kilo-docker:latest

# Via docker-compose (.env file)
echo 'CONTEXT7_TOKEN=your-token' > .env
echo 'AINSTRUCT_TOKEN=your-token' >> .env
docker-compose run kilo
```

Servers are only activated when their respective environment variables are set. Without the token, the server is configured but authentication will fail.

## Running as Host User

By default, the container runs as a non-root user (`node`, UID 1000). This may cause permission issues when creating files. To run with the same UID/GID as your host user:

```bash
# Interactive mode
docker run -it --rm \
  -v $(pwd):/workspace \
  -w /workspace \
  --user $(id -u):$(id -g) \
  ghcr.io/mbabic84/kilo-docker:latest

# SSH one-liner
ssh remote-host "docker run -it --rm -v \$(pwd):/workspace -w /workspace --user \$(id -u):\$(id -g) ghcr.io/mbabic84/kilo-docker:latest"
```

### SSH Alias for Convenience

Add to your `~/.ssh/config`:

```
Host remote
    HostName remote.example.com
    User username
    RequestTTY yes
    RemoteCommand docker run -it --rm -v $(pwd):/workspace -w /workspace --user $(id -u):$(id -g) ghcr.io/mbabic84/kilo-docker:latest
```

When running as your host user, your Git credentials and SSH keys work automatically.

## Building Locally

```bash
# Build
docker build -t kilo-docker .

# Test
docker run -it --rm -v $(pwd):/workspace -w /workspace kilo-docker --version
```

## Versioning

This project uses [semantic-release](https://semantic-release.git/) with [Conventional Commits](https://www.conventionalcommits.org/). Version bumps:

- `feat:` → Minor
- `fix:` → Patch
- `feat!:` / `BREAKING CHANGE:` → Major
- `docs:`, `chore:`, `ci:` → No version change
