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
