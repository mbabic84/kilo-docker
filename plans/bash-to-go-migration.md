# Plan: Go Migration with Container Entrypoint Subcommands

## Context

Two separate plans exist:

1. **Move host-side logic to container entrypoint** — extract ~15 inline `docker run` invocations from `scripts/kilo-docker` into entrypoint subcommands (token I/O, API calls, config, backup/restore)
2. **Bash to Go migration** — replace all shell scripts with Go binaries

Rather than refactoring bash (plan 1) then migrating to Go (plan 2), this combined plan builds the Go binaries from scratch with the container-side logic already integrated as subcommands. The old bash scripts remain in the repo as a fallback until the new binaries are verified.

## Current State (validated against HEAD)

```
Host (any Linux/macOS)              Container (Alpine)
────────────────────────             ──────────────────
scripts/kilo-docker (1335L bash)     scripts/entrypoint.sh (84L sh)
  ├── flags + arg parsing              ├── user/group setup (adduser/addgroup)
  ├── docker daemon check              ├── tool download (curl/wget → Docker, Compose, Zellij)
  ├── help / networks                  ├── SSH agent socket chown + known_hosts
  ├── sessions (list/attach/cleanup)   ├── ~/.config/kilo/ 6-subdir creation
  ├── update / update-config           ├── su-exec privilege drop
  ├── volume/password logic            ├── sources setup-kilo-config.sh
  ├── crypto (AES-256-CBC)             ├── Zellij config copy
  ├── ainstruct login (HTTP+JSON)      ├── ainstruct sync launch
  ├── token load/save                  └── exec kilo / sh
  ├── SSH agent forwarding
  ├── playwright sidecar               scripts/setup-kilo-config.sh (36L sh)
  ├── container naming (hash)            └── jq filter to toggle MCP servers
  ├── container state machine
  ├── session labels                   cmd/ainstruct-sync/ (833L Go, already done)
  ├── backup/restore                     ├── inotify watcher + debounce (6 dirs, recursive skills/)
  ├── install/update                     ├── REST API client + JWT refresh + error handling
  ├── terminal reset                     ├── hash tracking + documentType() mapping
  └── network selection                  └── file-based logging

Container packages: libstdc++ git openssh-client ripgrep su-exec sudo jq curl
```

**Problem:** The host script is ~1335 lines, tightly coupled to container internals. It spawns ~15 ephemeral `docker run --rm` containers inline for token management, API auth, config updates, and backup/restore. This makes maintenance difficult and the host script a bottleneck for changes.

**Removable packages:** `jq` (~1 MB, only for setup-kilo-config.sh), `curl` (~1 MB, only for entrypoint.sh downloads), `su-exec` (~50 KB, only for entrypoint.sh privilege drop).

## Design Decisions

### 1. No refactoring of existing bash — build new Go from scratch

The old bash scripts stay untouched in the repo. The Go binaries are developed alongside. This eliminates regression risk in the existing solution and allows gradual rollout.

### 2. Security: host-side encryption/decryption

Docker environment variables are inspectable via `docker inspect`. The original code runs `openssl` on the host to avoid passing `VOLUME_PASSWORD` into containers. The Go host binary preserves this: crypto operations (`crypto/aes` + `crypto/cipher` + PBKDF2) run on the host, not in the container. The container entrypoint handles only plaintext I/O.

### 3. Docker interaction: `os/exec.Command`, not Docker SDK

The host already requires Docker CLI. Using `exec.Command("docker", ...)` keeps the binary small, avoids a large dependency, and matches the existing mental model.

### 4. Host prompts stay on host

TTY-dependent operations (password input, username prompts, confirmations) remain in the host binary using `golang.org/x/term`. The container entrypoint receives credentials via env vars and outputs structured results to stdout.

### 5. Backup/restore: exec-based, no entrypoint subcommand dependency

Backup and restore use `docker exec` on a `tail -f` container rather than entrypoint subcommands. This avoids race conditions (tar finishing before docker cp) and doesn't depend on the entrypoint being present (though it is, since we use the kilo-docker image).

## Target Architecture

```
Host (cmd/kilo-docker/)                Container (cmd/kilo-entrypoint/)
──────────────────────                 ─────────────────────────────────
main.go      — CLI dispatch            main.go      — init + subcommand dispatch
docker.go    — exec.Command wrapper    init.go      — container init (replaces entrypoint.sh)
session.go   — list/attach/cleanup     config.go    — MCP toggling (replaces setup-kilo-config.sh)
terminal.go  — reset_terminal          sync.go      — ainstruct sync (existing)
crypto.go    — AES-256-CBC             api.go       — REST client (existing)
ainstruct.go — host-side login prompts watcher.go   — inotify (existing)
backup.go    — backup/restore logic    hash.go      — (existing)
install.go   — install/update          loadsave.go  — token load/save (replaces inline docker runs)
network.go   — network selection       login.go     — ainstruct HTTP auth (replaces inline docker runs)
playwright.go— sidecar mgmt            updatecfg.go — config template download & merge
volume.go    — volume mgmt             backup.go    — tar czf subcommand
ssh.go       — SSH agent forwarding    restore.go   — tar xzf subcommand

Container packages: libstdc++ git openssh-client ripgrep sudo
                    (removed: jq, curl, su-exec)
```

## Release Artifacts

| Artifact | Platform | Source |
|---|---|---|
| `kilo-docker-linux-amd64` | Host | `cmd/kilo-docker/` |
| `kilo-docker-linux-arm64` | Host | `cmd/kilo-docker/` |
| `kilo-docker-darwin-amd64` | Host | `cmd/kilo-docker/` |
| `kilo-docker-darwin-arm64` | Host | `cmd/kilo-docker/` |
| `ghcr.io/mbabic84/kilo-docker:latest` | Container | Dockerfile (includes `kilo-entrypoint`) |

Host binaries attached to GitHub Releases. Container binary built inside Dockerfile multi-stage build.

---

## Phase 1: Container Binary (`cmd/kilo-entrypoint/`)

**Goal:** Single Go binary that replaces `entrypoint.sh`, `setup-kilo-config.sh`, and all inline container-side operations. Removes `jq`, `curl`, `su-exec` from image.

### Subcommand mapping

| Subcommand | Replaces (current bash) | Description |
|---|---|---|
| `(no args)` | `entrypoint.sh` default | Container init: user setup, tool downloads, privilege drop, config toggle, exec kilo |
| `load-tokens` | `kilo-docker:1061-1064` (cat tokens) | Read plaintext token file, output KEY=VALUE to stdout |
| `save-tokens` | `kilo-docker:158-162` (write tokens) | Read KEY=VALUE from stdin, write to volume with chmod 600 |
| `ainstruct-login` | `kilo-docker:202-290` (curl login+profile) | HTTP login + profile fetch, structured output to stdout |
| `update-config` | `kilo-docker:715-734` (inline sh -c) | Download template, merge with existing config |
| `backup` | `docker cp` approach | `tar czf` of KILO_HOME |
| `restore` | `docker run` approach | `tar xzf` into KILO_HOME + chown |
| `config` | `setup-kilo-config.sh` | Toggle MCP servers based on env vars |
| `sync` | `ainstruct-sync` binary | File watcher + REST sync (existing logic) |

### Files

| File | Replaces | Implementation |
|---|---|---|
| `cmd/kilo-entrypoint/main.go` | — | Subcommand dispatcher: `(no args)` → init, else route to subcommand |
| `cmd/kilo-entrypoint/init.go` | `scripts/entrypoint.sh` | User setup via `os/exec` (adduser/addgroup), tool download via `net/http`, privilege drop via `syscall.Setuid/Setgid`, config toggle, final exec via `syscall.Exec` |
| `cmd/kilo-entrypoint/config.go` | `scripts/setup-kilo-config.sh` | `encoding/json` walk of `mcp` map, set `enabled` based on env vars (~25 lines) |
| `cmd/kilo-entrypoint/loadsave.go` | `kilo-docker:106-128, 153-166` | `load`: read file → stdout; `save`: stdin → file + mkdir + chmod 600 |
| `cmd/kilo-entrypoint/login.go` | `kilo-docker:169-294` | HTTP POST login + GET profile using `net/http`, structured output to stdout |
| `cmd/kilo-entrypoint/updatecfg.go` | `kilo-docker:715-734` | Download template via `net/http`, merge JSON with `encoding/json` |
| `cmd/kilo-entrypoint/backup.go` | backup container logic | `filepath.Walk` + `archive/tar` + `compress/gzip` → write file |
| `cmd/kilo-entrypoint/restore.go` | restore container logic | `archive/tar` + `compress/gzip` → extract + `os.Chown` |

#### Existing files to move (unchanged or minimally adapted)

| From | To |
|---|---|
| `cmd/ainstruct-sync/main.go` (43L) | `cmd/kilo-entrypoint/sync.go` |
| `cmd/ainstruct-sync/api.go` (123L) | `cmd/kilo-entrypoint/api.go` |
| `cmd/ainstruct-sync/watcher.go` (269L) | `cmd/kilo-entrypoint/watcher.go` |
| `cmd/ainstruct-sync/hash.go` (67L) | `cmd/kilo-entrypoint/hash.go` |
| `cmd/ainstruct-sync/sync.go` (331L) | `cmd/kilo-entrypoint/sync_content.go` |

### init.go — container initialization (replaces entrypoint.sh)

Key operations:

| Operation | Bash | Go |
|---|---|---|
| User/group setup | `adduser -D -u $PUID` | `os/exec.Command("adduser", ...)` |
| Docker/Compose download | `curl` / `wget` | `net/http.Get()` + `io.Copy()` + `archive/tar` |
| Docker group | `addgroup -g $DOCKER_GID docker` | `os/exec.Command("addgroup", ...)` |
| SSH agent socket | `chown kilo-t8x3m7kp $SSH_AUTH_SOCK` | `os.Chown(SSH_AUTH_SOCK, uid, gid)` |
| Known hosts | `ssh-keyscan github.com gitlab.com bitbucket.com` | `os/exec.Command("ssh-keyscan", ...)` |
| Config dirs | `mkdir -p` × 6 | `os.MkdirAll()` + `os.Chown()` |
| Privilege drop | `su-exec kilo-t8x3m7kp "$0" "$@"` | `syscall.Setgid()` + `syscall.Setuid()` |
| Config toggle | `. setup-kilo-config.sh` | internal call to `runConfig()` |
| Ainstruct sync | `su-exec ... sh -c 'exec ainstruct-sync &'` | `exec.Command("kilo-entrypoint", "sync")` background |
| Zellij config | `cp /etc/zellij/config.kdl` | `os.CopyFile()` |
| Final exec | `exec "$@"` | `syscall.Exec(binary, args, env)` |
| Default (no args) | `exec sh` | `syscall.Exec("/bin/sh", ["sh"], env)` |

### loadsave.go — token operations

```go
// load: read token file, output to stdout
func runLoadTokens() error {
    home := os.Getenv("HOME")
    tokenFile := filepath.Join(home, ".local/share/kilo/.tokens.env")
    data, err := os.ReadFile(tokenFile)
    if err != nil {
        if os.IsNotExist(err) {
            return nil // no tokens, output nothing
        }
        return err
    }
    os.Stdout.Write(data)
    return nil
}

// save: read from stdin, write to token file
func runSaveTokens() error {
    home := os.Getenv("HOME")
    tokenDir := filepath.Join(home, ".local/share/kilo")
    tokenFile := filepath.Join(tokenDir, ".tokens.env")
    if err := os.MkdirAll(tokenDir, 0700); err != nil {
        return err
    }
    data, err := io.ReadAll(os.Stdin)
    if err != nil {
        return err
    }
    if err := os.WriteFile(tokenFile, data, 0600); err != nil {
        return err
    }
    return nil
}
```

### login.go — ainstruct authentication

Replaces `kilo-docker:169-294`. Uses `net/http` instead of `curl` subprocess.

```go
// Env: USERNAME, PASSWORD, API_URL
// Stdout: STATUS=success\nUSER_ID=...\nACCESS_TOKEN=...\nREFRESH_TOKEN=...\nEXPIRES_IN=...
// On error: STATUS=error\nERROR=...\n  (exit 1)

func runAinstructLogin() error {
    username := os.Getenv("USERNAME")
    password := os.Getenv("PASSWORD")
    apiURL := os.Getenv("API_URL")
    // validate non-empty
    // POST /auth/login → extract tokens
    // GET /auth/profile → extract user_id
    // output structured lines
}
```

HTTP status handling preserves current behavior:
- 200: success
- 401: invalid credentials
- 403: account disabled
- 0: connection failed
- Other: error with detail message

### backup.go / restore.go

```go
// backup: tar czf of KILO_HOME
func runBackup(outputPath string) error {
    home := os.Getenv("HOME")
    f, err := os.Create(outputPath)
    if err != nil { return err }
    defer f.Close()
    gz := gzip.NewWriter(f)
    defer gz.Close()
    tw := tar.NewWriter(gz)
    defer tw.Close()
    return filepath.Walk(home, func(path string, info os.FileInfo, err error) error {
        // create tar headers, write file contents
    })
}

// restore: tar xzf into KILO_HOME + chown
func runRestore(archivePath string) error {
    home := os.Getenv("HOME")
    // extract tar.gz into home
    // chown all files to current uid/gid
}
```

These subcommands are used via `docker exec` on a running container (same pattern as current bash), not via `docker run --entrypoint ""`.

### Dockerfile changes

```diff
 FROM golang:1.26-alpine AS go-builder
 WORKDIR /build
 COPY go.mod go.sum ./
 RUN go mod download
 COPY cmd/ cmd/
-RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /out/ainstruct-sync ./cmd/ainstruct-sync
+RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /out/kilo-entrypoint ./cmd/kilo-entrypoint

 RUN apk add --no-cache \
-    libstdc++ git openssh-client ripgrep su-exec sudo jq curl \
+    libstdc++ git openssh-client ripgrep sudo \
     && adduser -D -u 1000 kilo-t8x3m7kp \
     && echo "kilo-t8x3m7kp ALL=(ALL) NOPASSWD:ALL" >> /etc/sudoers

-COPY scripts/entrypoint.sh /usr/local/bin/docker-entrypoint.sh
-COPY scripts/setup-kilo-config.sh /usr/local/bin/setup-kilo-config.sh
-RUN chmod +x /usr/local/bin/docker-entrypoint.sh /usr/local/bin/setup-kilo-config.sh
 COPY configs/zellij.kdl /etc/zellij/config.kdl
 COPY configs/opencode.json /home/kilo-t8x3m7kp/.config/kilo/opencode.json
 COPY --from=builder /tmp/kilo /usr/local/bin/kilo
-COPY --from=go-builder /out/ainstruct-sync /usr/local/bin/ainstruct-sync
+COPY --from=go-builder /out/kilo-entrypoint /usr/local/bin/kilo-entrypoint
+RUN ln -sf kilo-entrypoint /usr/local/bin/ainstruct-sync

-ENTRYPOINT ["docker-entrypoint.sh"]
+ENTRYPOINT ["kilo-entrypoint"]
```

### Image size impact

```
Removed: jq (~1 MB) + curl (~1 MB) + su-exec (~0.05 MB) = ~2.05 MB
Added: kilo-entrypoint replaces ainstruct-sync (~5 MB → ~7 MB with init+config+login+loadsave+backup/restore)
Net: ~0.5 MB larger, zero shell scripts in image
```

### Verification

1. `docker build -t kilo-docker:test .`
2. `docker run --rm kilo-test ldd /usr/local/bin/kilo-entrypoint` — statically linked
3. `docker run --rm -e PUID=$(id -u) -e PGID=$(id -g) kilo-test kilo --version`
4. `docker run --rm kilo-test which jq` — should fail (jq removed)
5. `docker run --rm kilo-test which curl` — should fail (curl removed)
6. Test each subcommand (see Verification section below)

---

## Phase 2: Host Binary (`cmd/kilo-docker/`)

**Goal:** Go binary replacing `scripts/kilo-docker` (1335 lines bash). Only does orchestration — delegates all container-side work to entrypoint subcommands.

### File structure

| File | Est. Lines | Replaces (Bash) | Key responsibility |
|---|---|---|---|
| `cmd/kilo-docker/main.go` | ~110 | Flag parsing (296-350), command dispatch | CLI entry, flag definitions, subcommand routing |
| `cmd/kilo-docker/docker.go` | ~150 | Docker CLI calls (throughout) | `dockerRun()`, `dockerExec()`, container state detection, naming |
| `cmd/kilo-docker/session.go` | ~120 | Sessions (509-638) | List/attach/cleanup with table display |
| `cmd/kilo-docker/terminal.go` | ~25 | `reset_terminal()` (95-108) | Drain stdin, stty sane, ANSI reset |
| `cmd/kilo-docker/volume.go` | ~80 | Volume/password (667-701) | `deriveVolumeName()`, volume create/inspect/rm |
| `cmd/kilo-docker/crypto.go` | ~80 | Encrypt/decrypt (110-128) | AES-256-CBC with PBKDF2 via `crypto/aes` + `golang.org/x/crypto` |
| `cmd/kilo-docker/ainstruct.go` | ~140 | Login flow (169-294) | Host-side prompts, calls `docker run ... ainstruct-login` |
| `cmd/kilo-docker/tokens.go` | ~80 | Token load/save (1041-1077, 153-166) | Calls `docker run ... load-tokens` / `save-tokens`, parses output |
| `cmd/kilo-docker/backup.go` | ~80 | Backup/restore (797-940) | `docker exec` based backup/restore with volume |
| `cmd/kilo-docker/install.go` | ~80 | Install (944-1017), update (642-665) | Symlink/copy, PATH check, image pull |
| `cmd/kilo-docker/network.go` | ~30 | Network selection (1019-1039) | Interactive network picker |
| `cmd/kilo-docker/playwright.go` | ~90 | Sidecar (1081-1164) | Network create, container start, readiness check, cleanup |
| `cmd/kilo-docker/ssh.go` | ~60 | SSH agent (376-399) | Agent detection, auto-start, key discovery, socket mount |
| `cmd/kilo-docker/config.go` | ~30 | update-config (705-736) | Calls `docker run ... update-config` |
| **Total** | **~1155** | **1335** | |

### Host ↔ container interaction pattern

The host binary calls container subcommands via:

```go
// Token loading (plaintext)
output, err := dockerRun(volumeMount, "--entrypoint", "", image, "container-tokens.sh", "load")

// Token loading (encrypted) — decrypt on host, not in container
encrypted, _ := dockerRun(volumeMount, image, "cat", encryptedTokenFile)
plaintext := decryptAES(encrypted, volumePassword)

// Token saving (plaintext)
dockerRunWithStdin(tokenData, volumeMount, "--entrypoint", "", image, "container-tokens.sh", "save")

// Token saving (encrypted) — encrypt on host, not in container
encrypted := encryptAES(tokenData, volumePassword)
dockerRunWithStdin(encrypted, volumeMount, image, "sh", "-c", "cat > "+encPath)

// Ainstruct login
output, _ := dockerRun("-e", "USERNAME="+user, "-e", "PASSWORD="+pass, "-e", "API_URL="+url,
    "--entrypoint", "", image, "container-ainstruct-login.sh")
result := parseStructuredOutput(output) // STATUS, USER_ID, ACCESS_TOKEN, etc.

// Backup
dockerRunDetached(volumeMountRO, image, "tail", "-f", "/dev/null")
dockerExec(containerName, "tar", "czf", "/tmp/backup.tar.gz", "-C", home, ".")
dockerCopy(containerName+":/tmp/backup.tar.gz", backupFile)

// Restore
dockerRunDetached(volumeMount, image, "tail", "-f", "/dev/null")
dockerCopy(backupFile, containerName+":/tmp/backup.tar.gz")
dockerExec(containerName, "tar", "xzf", "/tmp/backup.tar.gz", "-C", home)
dockerExec(containerName, "chown", "-R", uid+":"+gid, home)
```

When using the new container entrypoint, the host can call subcommands directly:

```go
// Using kilo-entrypoint subcommands (after Phase 1 complete)
output, _ := dockerRun(volumeMount, image, "load-tokens")
output, _ := dockerRunWithStdin(tokenData, volumeMount, image, "save-tokens")
output, _ := dockerRun("-e", "USERNAME="+user, "-e", "PASSWORD="+pass, "-e", "API_URL="+url,
    image, "ainstruct-login")
dockerRun(volumeMount, image, "update-config")
```

This is simpler — no `--entrypoint ""` override needed since the entrypoint dispatcher routes subcommands.

### Host binary design decisions (from bash-to-go plan, confirmed)

1. **Docker interaction:** `os/exec.Command("docker", ...)` — NOT Docker Go SDK
2. **Password input:** `golang.org/x/term.ReadPassword()` — replaces `read -s </dev/tty`
3. **Encryption:** `crypto/aes` + `crypto/cipher` (CBC) + `golang.org/x/crypto/pbkdf2` — replaces `openssl` subprocess
4. **TTY detection:** `golang.org/x/term.IsTerminal()` — replaces `[ -t 0 ]`
5. **Process exec:** `syscall.Exec()` for final `docker run`/`docker attach` — replaces Bash `exec`

### Host binary key functions

```go
// volume.go
func deriveVolumeName(password string) string {
    hash := sha256.Sum256([]byte(password))
    return fmt.Sprintf("kilo-%x", hash[:12])
}

// crypto.go
func encryptAES(plaintext []byte, password string) ([]byte, error) {
    // AES-256-CBC with PBKDF2 — compatible with existing openssl output
    key := pbkdf2.Key([]byte(password), salt, 10000, 32, sha256.New)
    // ... CBC encryption with PKCS7 padding
}

func decryptAES(ciphertext []byte, password string) ([]byte, error) {
    // Reverse of encryptAES
}

// ainstruct.go — host-side login flow
func ainstructLogin(image string) (loginResult, error) {
    // 1. Prompt username (with retry loop if empty)
    // 2. Prompt password (hidden input)
    // 3. Call: docker run -e USERNAME=... -e PASSWORD=... -e API_URL=... image ainstruct-login
    // 4. Parse STATUS/USER_ID/ACCESS_TOKEN/REFRESH_TOKEN/EXPIRES_IN from stdout
    // 5. Set VOLUME_PASSWORD = USER_ID
    // 6. Derive volume name
}

// tokens.go — token load/save
func loadTokens(image, volume string, encrypted bool, password string) (string, string) {
    if encrypted {
        // docker run cat encrypted file → decrypt on host
    } else {
        // docker run image load-tokens → parse stdout
    }
}

func saveTokens(image, volume string, token1, token2 string, encrypted bool, password string) {
    data := fmt.Sprintf("KD_CONTEXT7_TOKEN=%s\nKD_AINSTRUCT_TOKEN=%s\n", token1, token2)
    if encrypted {
        enc := encryptAES([]byte(data), password)
        // docker run write encrypted file
    } else {
        // docker run -i image save-tokens  (pipe data to stdin)
    }
}

// backup.go — backup/restore using docker exec pattern
func backup(image, volume, home, outputFile string) error {
    container := fmt.Sprintf("kilo-backup-temp-%d", os.Getpid())
    exec.Command("docker", "run", "--rm", "-d", "--name", container,
        "-v", volume+":"+home+":ro", image, "tail", "-f", "/dev/null").Run()
    defer exec.Command("docker", "rm", "-f", container).Run()
    exec.Command("docker", "exec", container, "tar", "czf", "/tmp/backup.tar.gz", "-C", home, ".").Run()
    return exec.Command("docker", "cp", container+":/tmp/backup.tar.gz", outputFile).Run()
}
```

### go.mod additions

```
require (
    golang.org/x/crypto v0.x.x   // PBKDF2
    golang.org/x/sys v0.x.x       // syscall (already present)
    golang.org/x/term v0.x.x      // terminal I/O
)
```

### Testing

- Unit tests for crypto (encrypt/decrypt roundtrip with existing openssl format)
- Unit tests for volume name derivation (known hash)
- Unit tests for container name derivation
- Unit tests for structured output parsing (ainstruct-login)
- Integration: `go build -o kilo-docker ./cmd/kilo-docker && ./kilo-docker help`
- Manual: `./kilo-docker --once`, `./kilo-docker sessions`, `./kilo-docker --ssh`

---

## Phase 3: Release Workflow Update

**Goal:** Cross-compile host binary for 4 platforms, attach to GitHub Releases.

### .github/workflows/release.yml additions

Add after semantic-release step, before Docker build:

```yaml
- name: Set up Go
  if: ${{ steps.release.outputs.new_release_published == 'true' }}
  uses: actions/setup-go@v5
  with:
    go-version: '1.26'

- name: Build host binaries
  if: ${{ steps.release.outputs.new_release_published == 'true' }}
  run: |
    set -e
    VERSION="${{ steps.release.outputs.new_release_version }}"
    for OS_ARCH in linux-amd64 linux-arm64 darwin-amd64 darwin-arm64; do
      OS="${OS_ARCH%%-*}"
      ARCH="${OS_ARCH##*-}"
      CGO_ENABLED=0 GOOS=$OS GOARCH=$ARCH go build \
        -ldflags="-s -w -X main.version=${VERSION}" \
        -o "kilo-docker-${OS_ARCH}" \
        ./cmd/kilo-docker
    done

- name: Upload binaries to release
  if: ${{ steps.release.outputs.new_release_published == 'true' }}
  uses: softprops/action-gh-release@v2
  with:
    tag_name: v${{ steps.release.outputs.new_release_version }}
    files: |
      kilo-docker-linux-amd64
      kilo-docker-linux-arm64
      kilo-docker-darwin-amd64
      kilo-docker-darwin-arm64
  env:
    GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

### .github/workflows/ci.yml additions

```yaml
- name: Set up Go
  uses: actions/setup-go@v5
  with:
    go-version: '1.26'

- name: Build host binary
  run: CGO_ENABLED=0 go build -o /dev/null ./cmd/kilo-docker

- name: Build container binary
  run: CGO_ENABLED=0 go build -o /dev/null ./cmd/kilo-entrypoint

- name: Run tests
  run: go test ./cmd/...
```

---

## Phase 4: Documentation & Cleanup

### Files to keep (old bash — backup)

| File | Reason to keep |
|---|---|
| `scripts/kilo-docker` | Backup host script until Go binary is verified in production |
| `scripts/entrypoint.sh` | Backup container entrypoint |
| `scripts/setup-kilo-config.sh` | Backup config setup |

### Files to delete (after verification)

| File | Replaced by |
|---|---|
| `scripts/kilo-docker` | `cmd/kilo-docker/` (Go binary) |
| `scripts/entrypoint.sh` | `cmd/kilo-entrypoint/init.go` |
| `scripts/setup-kilo-config.sh` | `cmd/kilo-entrypoint/config.go` |
| `cmd/ainstruct-sync/` (all files) | `cmd/kilo-entrypoint/` |

### Files to create

| File | Purpose |
|---|---|
| `scripts/install.sh` | Bootstrap script for `curl \| sh` pattern (~15 lines) |

### scripts/install.sh bootstrap

```sh
#!/bin/sh
set -e
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
case "$ARCH" in x86_64) ARCH="amd64";; aarch64|arm64) ARCH="arm64";; esac
INSTALL_DIR="${HOME}/.local/bin"
mkdir -p "$INSTALL_DIR"
curl -fsSL "https://github.com/mbabic84/kilo-docker/releases/latest/download/kilo-docker-${OS}-${ARCH}" \
  -o "${INSTALL_DIR}/kilo-docker"
chmod +x "${INSTALL_DIR}/kilo-docker"
exec "${INSTALL_DIR}/kilo-docker" install "$@"
```

### README.md updates

- Install section: add binary download option alongside current bash install
- Project structure: update to reflect Go-only architecture
- Remove references to `jq`, `curl`, `su-exec`, shell scripts

---

## Execution Order

```
Phase 1 (container binary) ─┐
                             ├─ independent, parallel-safe
Phase 2 (host binary) ──────┘
         │
         ▼
Phase 3 (release workflow) ── depends on Phase 2
         │
         ▼
Phase 4 (cleanup + docs) ──── depends on all
```

**Incremental safety:**
- Phase 1 can be developed and tested independently — `docker build` + `docker run` subcommands
- Phase 2 can be developed alongside Phase 1 — the host binary initially calls container subcommands via the same `docker run` patterns as current bash
- Old bash scripts remain functional throughout Phases 1-3
- Phase 4 (deleting bash) happens only after Go binaries are verified in production

## Dependencies

**Host binary (`cmd/kilo-docker/`):**
```
golang.org/x/crypto  — PBKDF2 key derivation
golang.org/x/sys     — syscall (already in go.mod)
golang.org/x/term    — terminal I/O, password input, TTY detection
```

**Container binary (`cmd/kilo-entrypoint/`):**
```
golang.org/x/sys     — inotify (already in go.mod)
```

No external libraries beyond `golang.org/x`. No Docker SDK.

---

## Implementation Notes

### 1. Subcommand dispatch via `os.Args[0]` basename

The Dockerfile creates a symlink `ln -sf kilo-entrypoint /usr/local/bin/ainstruct-sync`. The `main.go` dispatcher must check `filepath.Base(os.Args[0])` to route correctly:
- `"ainstruct-sync"` → run sync mode directly (backward compat)
- `"kilo-entrypoint"` → route by `os.Args[1]` subcommand

### 2. JSON merge in `updatecfg.go` must replicate `jq -s ".[0] * .[1]"`

Current bash (lines 715-734) uses jq's recursive object merge: `.[0] * .[1]`. This deeply merges nested objects (template defaults ← user customizations). Go's `encoding/json` doesn't have a built-in deep merge — implement a recursive `mergeJSON(dst, src map[string]any) map[string]any` function that:
- Recurses into nested `map[string]any`
- Overwrites scalar values from src into dst
- Preserves dst keys not in src

### 3. `go.mod` additions

Current `go.mod` has only `golang.org/x/sys v0.28.0`. Add:
```
golang.org/x/crypto v0.x.x   — PBKDF2 (host binary crypto.go)
golang.org/x/term v0.x.x     — ReadPassword, IsTerminal (host binary)
```
Run `go get golang.org/x/crypto golang.org/x/term` to resolve versions.

### 4. OpenSSL compatibility for crypto.go

Current bash uses `openssl enc -aes-256-cbc -salt -pbkdf2`. The Go implementation must produce byte-identical output for interoperability:
- OpenSSL prepends `Salted__` + 8-byte salt header to ciphertext
- PBKDF2 iteration count: default 10000 (openssl 1.1.1+)
- Must handle both encrypt (host → container write) and decrypt (container read → host)
- Test roundtrip: encrypt with Go → decrypt with openssl, and vice versa

### 5. Playwright readiness check

Current bash (lines 1134-1139) uses `docker exec ... node -e "net.connect()"` for TCP port check. Go equivalent: use `net.DialTimeout("tcp", "127.0.0.1:8931", 2*time.Second)` inside a `docker exec` call, or poll `docker inspect -f '{{.State.Status}}'` + TCP check via `net.Dial` to the container's IP on the Docker network.

### 6. Container name derivation consistency

Current bash: `printf '%s' "$(pwd)" | sha256sum | cut -c1-12`
Go equivalent: `fmt.Sprintf("kilo-%x", sha256.Sum256([]byte(pwd))[:6])`
Must produce identical output — verify with unit test using known path.

### 7. `exec.Command` output handling

The host binary wraps `docker run`/`docker exec` calls. Key patterns:
- `dockerRun()` — capture stdout, return as string, propagate stderr to user
- `dockerRunWithStdin()` — pipe data to container stdin
- `dockerRunDetached()` — fire-and-forget, return container name
- All should set `cmd.StdoutPipe()` / `cmd.StdinPipe()` as needed, not inherit host TTY for non-interactive calls

### 8. Phase 1 verification before Phase 2 integration

Before Phase 2 (host binary) calls Phase 1 (container binary) subcommands:
1. Build `kilo-entrypoint` binary
2. `docker build -t kilo-docker:test .`
3. Test each subcommand manually:
   - `docker run --rm kilo-docker:test load-tokens` (expect empty, no error)
   - `docker run --rm -i kilo-docker:test save-tokens` (pipe stdin)
   - `docker run --rm -e USERNAME=x -e PASSWORD=y -e API_URL=z kilo-docker:test ainstruct-login`
   - `docker run --rm kilo-docker:test config` (test MCP toggling)
4. Verify `jq`, `curl`, `su-exec` are absent: `docker run --rm kilo-docker:test which jq` should fail

---

## Implementation Notes

### 1. Subcommand dispatch via `os.Args[0]` basename

The Dockerfile creates a symlink `ln -sf kilo-entrypoint /usr/local/bin/ainstruct-sync`. The `main.go` dispatcher must check `filepath.Base(os.Args[0])` to route correctly:
- `"ainstruct-sync"` → run sync mode directly (backward compat)
- `"kilo-entrypoint"` → route by `os.Args[1]` subcommand

### 2. JSON merge in `updatecfg.go` must replicate `jq -s ".[0] * .[1]"`

Current bash (lines 715-734) uses jq's recursive object merge: `.[0] * .[1]`. This deeply merges nested objects (template defaults ← user customizations). Go's `encoding/json` doesn't have a built-in deep merge — implement a recursive `mergeJSON(dst, src map[string]any) map[string]any` function that:
- Recurses into nested `map[string]any`
- Overwrites scalar values from src into dst
- Preserves dst keys not in src

### 3. `go.mod` additions

Current `go.mod` has only `golang.org/x/sys v0.28.0`. Add:
```
golang.org/x/crypto v0.x.x   — PBKDF2 (host binary crypto.go)
golang.org/x/term v0.x.x     — ReadPassword, IsTerminal (host binary)
```
Run `go get golang.org/x/crypto golang.org/x/term` to resolve versions.

### 4. OpenSSL compatibility for crypto.go

Current bash uses `openssl enc -aes-256-cbc -salt -pbkdf2`. The Go implementation must produce byte-identical output for interoperability:
- OpenSSL prepends `Salted__` + 8-byte salt header to ciphertext
- PBKDF2 iteration count: default 10000 (openssl 1.1.1+)
- Must handle both encrypt (host → container write) and decrypt (container read → host)
- Test roundtrip: encrypt with Go → decrypt with openssl, and vice versa

### 5. Playwright readiness check

Current bash (lines 1134-1139) uses `docker exec ... node -e "net.connect()"` for TCP port check. Go equivalent: use `net.DialTimeout("tcp", "127.0.0.1:8931", 2*time.Second)` inside a `docker exec` call, or poll `docker inspect -f '{{.State.Status}}'` + TCP check via `net.Dial` to the container's IP on the Docker network.

### 6. Container name derivation consistency

Current bash: `printf '%s' "$(pwd)" | sha256sum | cut -c1-12`
Go equivalent: `fmt.Sprintf("kilo-%x", sha256.Sum256([]byte(pwd))[:6])`
Must produce identical output — verify with unit test using known path.

### 7. `exec.Command` output handling

The host binary wraps `docker run`/`docker exec` calls. Key patterns:
- `dockerRun()` — capture stdout, return as string, propagate stderr to user
- `dockerRunWithStdin()` — pipe data to container stdin
- `dockerRunDetached()` — fire-and-forget, return container name
- All should set `cmd.StdoutPipe()` / `cmd.StdinPipe()` as needed, not inherit host TTY for non-interactive calls

### 8. Phase 1 verification before Phase 2 integration

Before Phase 2 (host binary) calls Phase 1 (container binary) subcommands:
1. Build `kilo-entrypoint` binary
2. `docker build -t kilo-docker:test .`
3. Test each subcommand manually:
   - `docker run --rm kilo-docker:test load-tokens` (expect empty, no error)
   - `docker run --rm -i kilo-docker:test save-tokens` (pipe stdin)
   - `docker run --rm -e USERNAME=x -e PASSWORD=y -e API_URL=z kilo-docker:test ainstruct-login`
   - `docker run --rm kilo-docker:test config` (test MCP toggling)
4. Verify `jq`, `curl`, `su-exec` are absent: `docker run --rm kilo-docker:test which jq` should fail
