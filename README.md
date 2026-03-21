# Kilo Docker

Docker environment for [Kilo CLI](https://kilo.ai/docs/code-with-ai/platforms/cli) - enabling portable, zero-install execution on remote hosts.

## Features

- **Node.js LTS** - Latest stable Node.js via Alpine Linux
- **Non-root user** - Runs as `node` user for security
- **Pre-installed tools** - `git`, `curl`, `bash` for common workflows
- **Zero config** - Kilo config is ephemeral with `--rm` flag

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
