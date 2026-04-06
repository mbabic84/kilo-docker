# Plan: Add --workspace Flag

## Context
The user wants to add a `--workspace` flag (with `-w` short flag) to kilo-docker, similar to the existing `--volume` / `-v` flag. Currently, the workspace is always the current working directory (`os.Getwd()`). The new flag should allow users to specify a custom workspace folder.

## Current Behavior
- Workspace defaults to `os.Getwd()` (current directory)
- The workspace is mounted as a volume: `-v pwd:pwd`
- The working directory is set to: `-w pwd`
- Container name is derived from the workspace path

## Changes Required

### 1. flags.go - Add workspace field and parsing
**File:** `/home/mbabic/projects/github/kilo-docker/cmd/kilo-docker/flags.go`

Add `workspace` field to config struct (line 20-32):
```go
type config struct {
    // ... existing fields ...
    workspace       string   // Custom workspace directory (defaults to current directory)
    // ...
}
```

Add flag parsing in `parseArgs()` switch statement (after line 51):
```go
case "--workspace", "-w":
    if i+1 < len(args) && !strings.HasPrefix(args[i+1], "--") {
        cfg.workspace = args[i+1]
        i++
    }
```

### 2. args.go - Use workspace in container args
**File:** `/home/mbabic/projects/github/kilo-docker/cmd/kilo-docker/args.go`

Modify `serializeArgs()` to include workspace (after line 36):
```go
if cfg.workspace != "" {
    sessionArgs += "--workspace " + cfg.workspace + " "
}
```

Modify `buildContainerArgs()` signature to accept workspace instead of pwd, or handle workspace logic:
- If `cfg.workspace` is set, use it as the mount point and working directory
- Otherwise, use `pwd` as before

Update lines 51-52:
```go
// Use custom workspace if provided, otherwise use pwd
workspace := cfg.workspace
if workspace == "" {
    workspace = pwd
}
args := []string{
    // ...
    "-v", workspace + ":" + workspace,
    "-w", workspace,
    // ...
}
```

Update line 59 label:
```go
args = append(args, "--label", "kilo.workspace="+workspace)
```

### 3. main.go - Use workspace for container naming
**File:** `/home/mbabic/projects/github/kilo-docker/cmd/kilo-docker/main.go`

Update `runContainer()` function:

Lines 86-87:
```go
pwd, _ := os.Getwd()
workspace := cfg.workspace
if workspace == "" {
    workspace = pwd
}
containerName := deriveContainerName(workspace)
```

Line 175:
```go
containerArgs := buildContainerArgs(cfg, dataVolume, pwd, workspace, containerName, containerState,
    sshAuthSock, hostEnvVars)
```

Update `buildContainerArgs()` signature to accept both `pwd` (for info) and `workspace` (for mounting).

### 4. setup.go - Add help text
**File:** `/home/mbabic/projects/github/kilo-docker/cmd/kilo-docker/setup.go`

Add help line (after line 73):
```go
optLines = append(optLines, fmt.Sprintf("  %-*s %s", w-2, "--workspace, -w <path>", "Set a custom workspace directory (defaults to current directory)"))
```

## Files Modified

| File | Change |
|------|--------|
| `cmd/kilo-docker/flags.go` | Add `workspace` field to config struct; add `--workspace, -w` flag parsing |
| `cmd/kilo-docker/args.go` | Update `serializeArgs()` to include workspace; update `buildContainerArgs()` to use workspace for mount and working dir |
| `cmd/kilo-docker/main.go` | Use workspace for container name derivation; pass workspace to `buildContainerArgs()` |
| `cmd/kilo-docker/setup.go` | Add `--workspace, -w` to help text |

## Verification
- Build: `go build ./cmd/kilo-docker`
- Test: Run `kilo-docker --workspace /path/to/project` and verify container mounts the specified path
- Test: Run `kilo-docker -w /path/to/project` (short flag) and verify same behavior
- Test: Run `kilo-docker` without flag and verify it still uses current directory
- Test: Run `kilo-docker help` and verify `--workspace, -w` appears in options
