# Plan: Manual Volume Mounts for kilo-docker

## Context

Currently, kilo-docker only supports volume mounts through the built-in service system (e.g., `--docker` mounts the Docker socket). Users need the ability to manually specify additional volume mounts for their specific use cases (e.g., mounting host directories, config files, or cache directories).

This feature will allow users to pass custom volume mounts using a `--volume` (or `-v`) flag, similar to Docker's native `-v` flag.

## Changes

### 1. `cmd/kilo-docker/flags.go` — Add volume flag parsing

Add a `volumes` field to the config struct and parse `--volume` / `-v` flags:

```go
// config holds parsed CLI flags for the host binary.
type config struct {
    once            bool
    playwright      bool
    ssh             bool
    yes             bool
    network         string
    networkFlag     bool
    ports           []string // Port mappings in host_port:container_port format
    volumes         []string // Volume mounts in host_path:container_path format
    command         string
    args            []string
    enabledServices []string // Names of enabled services from builtInServices
}
```

Update `parseArgs()` to handle `--volume` / `-v` flags:

```go
case "--volume", "-v":
    if i+1 < len(args) && !strings.HasPrefix(args[i+1], "--") {
        cfg.volumes = append(cfg.volumes, args[i+1])
        i++
    }
```

### 2. `cmd/kilo-docker/args.go` — Include volumes in container args and session label

Update `serializeArgs()` to include volume mounts:

```go
for _, vol := range cfg.volumes {
    sessionArgs += "--volume " + vol + " "
}
```

Update `buildContainerArgs()` to append volume mounts to container args (after service volumes):

```go
// After service volumes (around line 82)
for _, vol := range cfg.volumes {
    args = append(args, "-v", vol)
}
```

### 3. `cmd/kilo-docker/main.go` — Update help text

Add the `--volume` option to the help text:

```go
// In the package comment (around line 11), add:
//
// Flags:
//
//	--once            One-time session (no volume)
//	--port, -p        Map a port (host_port:container_port), repeatable
//	--volume, -v      Mount a volume (host_path:container_path), repeatable
//	--playwright      Start Playwright MCP sidecar
//	--ssh             Enable SSH agent forwarding
//	--network <name>  Connect to a Docker network
//	--yes, -y         Auto-confirm all prompts
```

Update `printHelp()` function to document the new flag:

```go
func printHelp() {
    fmt.Fprintf(os.Stderr, "Usage: kilo-docker [flags] [command] [args...]\n\n")
    fmt.Fprintf(os.Stderr, "Flags:\n")
    fmt.Fprintf(os.Stderr, "  --once            Run without persistence (no named volume)\n")
    fmt.Fprintf(os.Stderr, "  --port, -p        Map a port (host:container), repeatable\n")
    fmt.Fprintf(os.Stderr, "  --volume, -v      Mount a volume (host:container), repeatable\n")
    // ... rest of help text
}
```

### 4. `README.md` — Document the feature

Add a new section for manual volume mounts after the "Options" table (around line 91):

```markdown
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

Note: The current working directory is always mounted at the same path automatically.
```

Update the Options table to include the new flag:

```markdown
| Option | Description |
|--------|-------------|
| `--once` | Run a one-time session without persistence (no volume) |
| `--volume`, `-v` | Mount a volume (host_path:container_path), repeatable |
| `--port`, `-p` | Map a port (host_port:container_port), repeatable |
| `--password`, `-p` | Protect volume with a password (encrypts tokens, derives volume name from password) |
| `--ainstruct` | Authenticate with Ainstruct API (volume from user_id, tokens encrypted, file sync enabled) |
| `--mcp` | Enable MCP servers (prompts for Context7 and Ainstruct API tokens) |
| `--playwright` | Start a Playwright MCP sidecar container for browser automation |
| `--ssh` | Enable SSH agent forwarding into the container |
| `--network <name>` | Attach to a specific Docker network |
| `--yes`, `-y` | Auto-confirm all prompts (useful for piped/non-interactive installs) |
```

## Files Modified

| File | Change |
|------|--------|
| `cmd/kilo-docker/flags.go` | Add `volumes` field to config struct, parse `--volume`/`-v` flags |
| `cmd/kilo-docker/args.go` | Append custom volumes to container args, include in session args label |
| `cmd/kilo-docker/main.go` | Update package comment and `printHelp()` with new flag documentation |
| `README.md` | Add volume mounts section to documentation |

## Verification

1. **Build test**:
   ```bash
   go build ./cmd/kilo-docker
   go build ./cmd/kilo-entrypoint
   ```

2. **Functionality test**:
   ```bash
   # Test single volume mount
   ./kilo-docker --volume /tmp:/container/tmp --once echo "volume mounted"
   
   # Test multiple volume mounts
   ./kilo-docker -v /tmp:/tmp -v ~/.config:/config --once ls /config
   
   # Test that volumes persist in session label
   ./kilo-docker -v /tmp:/test-data
   # Detach, then verify session was created with volume flag
   docker inspect <container> --format '{{index .Config.Labels "kilo.args"}}'
   ```

3. **Help text test**:
   ```bash
   ./kilo-docker help | grep -A1 "volume"
   # Should show: --volume, -v      Mount a volume (host:container), repeatable
   ```

4. **Regression test**:
   ```bash
   # Ensure existing functionality still works
   ./kilo-docker --once -- echo "hello"
   ./kilo-docker --docker --once docker ps
   ./kilo-docker -p 8080:8080 --once
   ```

## Design Decisions

1. **Flag naming**: Using `--volume` and `-v` to match Docker's convention, making it intuitive for users familiar with Docker.

2. **Repeatable**: Like `--port`, the `--volume` flag can be specified multiple times to mount multiple volumes.

3. **No validation**: We don't validate volume paths (same as Docker CLI) — invalid mounts will fail at container runtime.

4. **Session persistence**: Volume mounts are stored in the `kilo.args` label, so recreating a session with `kilo-docker sessions recreate` will preserve the volume mounts.

5. **Order of precedence**: Custom volumes are mounted after service volumes, allowing user-specified mounts to take precedence if there are conflicts (Docker's last-mount-wins behavior).
