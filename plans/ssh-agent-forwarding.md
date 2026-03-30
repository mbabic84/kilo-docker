# Plan: SSH Agent Forwarding for kilo-docker

## Context

Users need to use `git clone git@...`, `ssh`, and other SSH-based tools inside the container with the same identity as on the host. The recommended approach is **SSH agent forwarding** — the container accesses the host's SSH agent via a mounted Unix socket, so private keys never enter the container.

## Changes

### 1. `scripts/kilo-docker` — Detect agent and mount socket

After the `DOCKER_ENABLED` block (~line 362) and before `DOCKER_ARGS` construction (~line 1163), add SSH agent detection:

```bash
# --- SSH agent forwarding --------------------------------------------------

SSH_AGENT_FORWARDING=0
if [ -n "${SSH_AUTH_SOCK:-}" ] && [ -S "${SSH_AUTH_SOCK}" ]; then
    SSH_AGENT_FORWARDING=1
fi
```

In the `DOCKER_ARGS` construction block (~line 1210, after ZELLIJ_ENABLED), add:

```bash
if [ "$SSH_AGENT_FORWARDING" -eq 1 ]; then
    DOCKER_ARGS+=(-v "${SSH_AUTH_SOCK}:/ssh-agent.sock")
    DOCKER_ARGS+=(-e "SSH_AUTH_SOCK=/ssh-agent.sock")
fi
```

In the help text (~line 374), add a note about SSH agent forwarding being automatic:

```
SSH Agent:
  SSH agent forwarding is automatic when $SSH_AUTH_SOCK is set and the
  agent socket exists. The host SSH agent is mounted into the container,
  so git and ssh use your host keys without copying private keys.
  Ensure your key is loaded: ssh-add -l
```

In the session args label (~line 1183), add:

```bash
[ "$SSH_AGENT_FORWARDING" -eq 1 ] && KD_SESSION_ARGS+="ssh-agent "
```

### 2. `scripts/entrypoint.sh` — Fix socket ownership and known_hosts

After the user creation block (line 14-15) and before `exec su-exec` (line 59), add:

```sh
# Fix SSH agent socket ownership for the non-root user
if [ -n "${SSH_AUTH_SOCK:-}" ] && [ -S "${SSH_AUTH_SOCK}" ]; then
    chown kilo-t8x3m7kp "${SSH_AUTH_SOCK}" 2>/dev/null || true
fi

# Pre-populate known_hosts to avoid interactive prompts
mkdir -p /home/kilo-t8x3m7kp/.ssh
chmod 700 /home/kilo-t8x3m7kp/.ssh
ssh-keyscan -H github.com gitlab.com bitbucket.com >> /home/kilo-t8x3m7kp/.ssh/known_hosts 2>/dev/null || true
chown -R kilo-t8x3m7kp:kilo-t8x3m7kp /home/kilo-t8x3m7kp/.ssh
```

### 3. `README.md` — Document the feature

Add a new section after "Docker Socket Access" (after line 198):

```markdown
## SSH Agent Forwarding

SSH agent forwarding is enabled automatically when the host has an SSH agent running (`SSH_AUTH_SOCK` is set and the socket exists). The container mounts the host's SSH agent socket, allowing `git`, `ssh`, and `scp` to use your host SSH keys without copying private keys into the container.

```bash
# Ensure your key is loaded
ssh-add -l

# Start kilo-docker — SSH works automatically
kilo-docker
```

> **Security:** Private keys never enter the container. The container communicates with the host's SSH agent via a Unix socket. If the SSH agent is not running, SSH operations inside the container will prompt for passwords or fail for key-only authentication.

Common host-side setup (if not already done):

```bash
eval "$(ssh-agent -s)"
ssh-add ~/.ssh/id_ed25519
```
```

## Files Modified

| File | Change |
|------|--------|
| `scripts/kilo-docker` | Detect `SSH_AUTH_SOCK`, mount socket into container, pass env var, add help text |
| `scripts/entrypoint.sh` | Fix socket ownership for non-root user, pre-populate `known_hosts` |
| `README.md` | Add SSH Agent Forwarding documentation section |

## Verification

1. **Manual test**: On a host with `ssh-agent` running and a key loaded:
   ```bash
   ssh-add -l  # confirm key is loaded
   docker build -t kilo-docker .
   docker run -it --rm -v "$(pwd):/workspace" \
     -v "$SSH_AUTH_SOCK:/ssh-agent.sock" \
     -e SSH_AUTH_SOCK=/ssh-agent.sock \
     -e PUID=$(id -u) -e PGID=$(id -g) \
     kilo-docker ssh -T git@github.com
   ```
   Expected: `Hi <username>! You've successfully authenticated...`

2. **Negative test**: Without `SSH_AUTH_SOCK` set — container should start normally, SSH operations fail gracefully (no crash).

3. **Shellcheck**: `shellcheck scripts/kilo-docker scripts/entrypoint.sh`

4. **Existing tests**: Run any existing test suite to confirm no regressions.
