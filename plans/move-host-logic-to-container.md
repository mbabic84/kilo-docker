# Plan: Move Host-Side Logic into Container Entrypoint

**Superseded by:** [bash-to-go-migration.md](bash-to-go-migration.md)

This plan has been merged into the combined Go migration plan. The container-side subcommands (token I/O, ainstruct-login, update-config, backup, restore) are now part of Phase 1: Container Binary (`cmd/kilo-entrypoint/`). The host binary (Phase 2) delegates to these subcommands instead of inline `docker run` calls.

Key design decisions from this plan that were carried forward:
- Security: host-side encryption/decryption (VOLUME_PASSWORD never enters containers)
- Backup/restore: `docker exec` pattern to avoid race conditions
- Username validation: host-side retry loop preserved
- `init` stays as `docker volume rm` on host (no subcommand needed)
