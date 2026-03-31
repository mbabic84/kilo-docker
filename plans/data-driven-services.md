# Plan: Data-Driven Services Configuration

## Context

Currently, services like `--docker` and `--zellij` are hardcoded with boolean flags in `flags.go`, switch-case handling in `args.go`, and installation logic in `kilo-entrypoint/init.go`. This makes adding new services require code changes in multiple places.

The goal is to create a data-driven `builtInServices` structure that centralizes service definitions and generates flag handling, argument building, and installation logic automatically.

## Current Implementation

### Host Side (kilo-docker)

| File | Current Behavior |
|------|-----------------|
| `flags.go:18-32` | `config` struct has `docker bool`, `zellij bool`, `playwright bool` |
| `flags.go:49-52` | Switch cases: `case "--docker": cfg.docker = true` |
| `args.go:39-47` | Session args appended per service: `if cfg.zellij { sessionArgs += "--zellij " }` |
| `args.go:74-84` | Env vars/volumes appended per service: `if cfg.docker { args = append(args, "-v", "/var/run/docker.sock:...") }` |
| `main.go:97-107` | Docker socket GID extraction: `dockerGID = strconv.FormatUint(uint64(info.Sys().(*syscall.Stat_t).Gid), 10)` |

### Container Side (kilo-entrypoint)

| File | Current Behavior |
|------|-----------------|
| `init.go:55-62` | `DOCKER_ENABLED=1` → calls `downloadDockerClient()`, `downloadDockerCompose()`, `setupDockerGroup()` |
| `init.go:64-68` | `ZELLIJ_ENABLED=1` → calls `downloadZellij()` |
| `init.go:150-165` | Copies zellij config from `/etc/zellij/config.kdl` to `~/.config/zellij/config.kdl` |

## Service Definition

### `Flag` vs `SessionFlag` (Removed)

Previously the design had separate `Flag` and `SessionFlag`. This has been simplified: only `Flag` exists, used for both CLI parsing and session args.

```
kilo-docker --docker --zellij
              ^^^^^^  ^^^^^^
              Flag    Flag  (same for CLI and session args)
```

### Service Struct

```go
type Service struct {
    Name        string            // Internal name: "docker", "zellij"
    Flag        string            // CLI flag: "--docker", "--zellij"
    Description string            // Help text
    Install     []string          // Commands to run inside container at startup
    EnvVars     map[string]string // Env vars to set ("": dynamic, set at runtime)
    Volumes     []string          // Volumes to mount from host
}
```

### builtInServices

```go
var builtInServices = []Service{
    {
        Name:        "docker",
        Flag:        "--docker",
        Description: "Mount Docker socket for container management from within Kilo",
        Install: []string{
            "command -v docker >/dev/null || (curl -fsSL https://download.docker.com/linux/static/stable/x86_64/docker-*.tgz -o /tmp/docker.tgz && tar xzf /tmp/docker.tgz -C /tmp && mv /tmp/docker/docker /usr/local/bin/docker && chmod +x /usr/local/bin/docker && rm -rf /tmp/docker*)",
            "command -v docker-compose >/dev/null || (curl -fsSL https://github.com/docker/compose/releases/latest/download/docker-compose-linux-x86_64 -o /usr/local/bin/docker-compose && chmod +x /usr/local/bin/docker-compose && mkdir -p /usr/libexec/docker/cli-plugins && ln -sf /usr/local/bin/docker-compose /usr/libexec/docker/cli-plugins/docker-compose)",
        },
        EnvVars: map[string]string{
            "DOCKER_ENABLED": "1",
            "DOCKER_GID":     "", // Set dynamically from host
        },
        Volumes: []string{"/var/run/docker.sock:/var/run/docker.sock"},
    },
    {
        Name:        "zellij",
        Flag:        "--zellij",
        Description: "Start with Zellij multiplexer (detach: Ctrl+P Ctrl+Q, reattach: kilo-docker sessions)",
        Install: []string{
            "command -v zellij >/dev/null || (curl -fsSL https://github.com/zellij-org/zellij/releases/latest/download/zellij-x86_64-unknown-linux-musl.tar.gz -o /tmp/zellij.tar.gz && tar xzf /tmp/zellij.tar.gz -C /usr/local/bin && rm -rf /tmp/zellij.tar.gz)",
        },
        EnvVars: map[string]string{
            "ZELLIJ_ENABLED": "1",
        },
        Volumes: []string{},
    },
}
```

## Installation Execution Flow

```
1. USER RUNS: kilo-docker --docker
                    ↓
2. HOST: parseArgs() matches "--docker" → cfg.enabledServices = ["docker"]
                    ↓
3. HOST: buildDockerArgs() builds:
   - env vars: KD_SERVICES=docker
   - session args: --label kilo.args="--docker "
   - volumes: /var/run/docker.sock:/var/run/docker.sock
   - env: DOCKER_ENABLED=1, DOCKER_GID=<gid>
                    ↓
4. HOST: docker run ... -e KD_SERVICES=docker ghcr.io/.../kilo-docker:latest
                    ↓
5. CONTAINER: kilo-entrypoint starts as PID 1
                    ↓
6. CONTAINER: read KD_SERVICES="docker" from environment
                    ↓
7. CONTAINER: look up "docker" in builtInServices
                    ↓
8. CONTAINER: for each Install command, run via shell:
   sh -c "command -v docker >/dev/null || (curl -fsSL https://... -o /tmp/docker.tgz && ...)"
                    ↓
9. CONTAINER: set env vars, copy configs, drop privileges, exec shell
```

## Changes

### 1. Create `cmd/kilo-docker/services.go` (NEW)

```go
package main

type Service struct {
    Name        string
    Flag        string
    Description string
    Install     []string
    EnvVars     map[string]string
    Volumes     []string
}

var builtInServices = []Service{
    // ... see above ...
}
```

### 2. Modify `cmd/kilo-docker/flags.go`

Replace hardcoded boolean fields with service-based approach:

```go
type config struct {
    once            bool
    encrypted       bool
    ainstruct       bool
    playwright      bool
    docker          bool      // Keep for host-side Docker socket check
    zellij          bool
    ssh             bool
    mcp             bool
    yes             bool
    network         string
    networkFlag     bool
    command         string
    args            []string
    enabledServices []string  // Replaces: docker, zellij booleans
}

func parseArgs(args []string) config {
    var cfg config

    for i := 0; i < len(args); i++ {
        switch args[i] {
        // ... keep existing non-service flags (--once, --password, etc.) ...

        default:
            matched := false
            for _, svc := range builtInServices {
                if args[i] == svc.Flag {
                    cfg.enabledServices = append(cfg.enabledServices, svc.Name)
                    matched = true
                    break
                }
            }
            if !matched {
                if cfg.command == "" {
                    cfg.command = args[i]
                } else {
                    cfg.args = append(cfg.args, args[i])
                }
            }
        }
    }

    return cfg
}
```

### 3. Modify `cmd/kilo-docker/args.go`

Replace hardcoded service handling with service lookup:

```go
func buildDockerArgs(cfg config, volume, pwd, containerName, containerState,
    sshAuthSock, dockerGID string, kdContext7Token, kdAinstructToken,
    ainstructSyncToken, ainstructSyncRefreshToken string, ainstructSyncTokenExpiry int64) []string {

    // ... existing base args (lines 16-33) ...

    // Build session args from enabled services
    sessionArgs := ""
    if cfg.once {
        sessionArgs += "--once "
    }
    for _, svcName := range cfg.enabledServices {
        svc := getService(svcName)
        if svc != nil && svc.Flag != "" {
            sessionArgs += svc.Flag + " "
        }
    }
    // ... ssh-agent, encrypted, ainstruct, mcp, network appends ...
    if len(cfg.args) > 0 {
        sessionArgs += strings.Join(cfg.args, " ") + " "
    }
    args = append(args, "--label", "kilo.args="+strings.TrimSpace(sessionArgs))

    // Build env vars and volumes from enabled services
    for _, svcName := range cfg.enabledServices {
        svc := getService(svcName)
        if svc == nil {
            continue
        }

        // Env vars
        for key, value := range svc.EnvVars {
            if value == "" && key == "DOCKER_GID" {
                args = append(args, "-e", "DOCKER_GID="+dockerGID)
            } else if value != "" {
                args = append(args, "-e", key+"="+value)
            }
        }

        // Volumes
        for _, vol := range svc.Volumes {
            args = append(args, "-v", vol)
        }
    }

    // ... rest unchanged (lines 89-126) ...
}

func getService(name string) *Service {
    for i := range builtInServices {
        if builtInServices[i].Name == name {
            return &builtInServices[i]
        }
    }
    return nil
}
```

### 4. Modify `cmd/kilo-entrypoint/init.go`

Replace hardcoded env-var checks with service-based installation:

```go
// In runInit(), replace lines 55-68 (DOCKER_ENABLED/ZELLIJ_ENABLED checks)
// with:

func installServices() error {
    servicesEnv := os.Getenv("KD_SERVICES")
    if servicesEnv == "" {
        return nil
    }

    enabledServices := strings.Split(servicesEnv, ",")
    for _, svcName := range enabledServices {
        svc := getService(svcName)
        if svc == nil {
            continue
        }

        for _, installCmd := range svc.Install {
            if installCmd == "" {
                continue
            }
            fmt.Fprintf(os.Stderr, "[kilo-docker] Installing %s...\n", svc.Name)
            cmd := exec.Command("sh", "-c", installCmd)
            cmd.Stdout = os.Stderr
            cmd.Stderr = os.Stderr
            if err := cmd.Run(); err != nil {
                fmt.Fprintf(os.Stderr, "[kilo-docker] Warning: failed to install %s: %v\n", svc.Name, err)
            }
        }
    }
    return nil
}

func getService(name string) *Service {
    for i := range builtInServices {
        if builtInServices[i].Name == name {
            return &builtInServices[i]
        }
    }
    return nil
}
```

### 5. Create `cmd/kilo-entrypoint/services.go` (NEW)

Copy the `Service` struct and `builtInServices` from `cmd/kilo-docker/services.go`.

### 6. Pass Enabled Services to Container

In `cmd/kilo-docker/args.go`, add `KD_SERVICES` env var to container:

```go
// After building env vars from services (around line 85)
if len(cfg.enabledServices) > 0 {
    args = append(args, "-e", "KD_SERVICES="+strings.Join(cfg.enabledServices, ","))
}
```

## Files Modified

| File | Change |
|------|--------|
| `cmd/kilo-docker/services.go` | **NEW** - Service struct and builtInServices |
| `cmd/kilo-docker/flags.go` | Remove hardcoded service booleans, use enabledServices slice |
| `cmd/kilo-docker/args.go` | Replace hardcoded service handling with service lookup, add KD_SERVICES |
| `cmd/kilo-entrypoint/services.go` | **NEW** - Copy of builtInServices |
| `cmd/kilo-entrypoint/init.go` | Replace env-var checks with service-based installation |

## Verification

1. `go build ./cmd/kilo-docker` - compiles
2. `go build ./cmd/kilo-entrypoint` - compiles
3. `kilo-docker --docker` - Docker socket mounted, DOCKER_ENABLED=1 set, install runs
4. `kilo-docker --zellij` - ZELLIJ_ENABLED=1 set, install runs
5. `kilo-docker --docker --zellij` - Both work together
6. Container starts and installs services from builtInServices definitions

## Backwards Compatibility

- CLI flags remain the same: `--docker`, `--zellij`
- No config file changes required
- Existing sessions work unchanged
