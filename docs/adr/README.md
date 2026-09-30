# ADR Index — FeralOS Installer

| ADR | Title | Date | Status | Decision |
|-----|-------|------|--------|----------|
| [001](ADR-001-go-installer.md) | Go 1.22 Installer + Shell Legacy Ref + YAML Config | 2026-08-26 | Accepted | Use Go 1.22 for installer, keep Shell as legacy reference, YAML for config. |
| [002](ADR-002-host-chroot-isolation.md) | hostPhase vs chrootPhase Separation | 2026-08-26 | Accepted | main delegates only to `hostPhase.Run` / `chrootPhase.Run`; clear host vs chroot boundary. |
| [003](ADR-003-internal-package-rule.md) | hostPhase/internal Import Restriction | 2026-08-26 | Accepted | `hostPhase/internal/*` importable only by `hostPhase`; main reaches it via wrapper. |
| [004](ADR-004-executecmd-logging.md) | Mandatory shell.ExecuteCmd Logging | 2026-08-26 | Accepted | All hostPhase/chrootPhase commands via `shell.ExecuteCmd`; `os.*` banned. |
| [005](ADR-005-luks-btrfs-booster-uki.md) | LUKS2 + Btrfs RAID1 + booster + UKI | 2026-08-26 | Accepted | LUKS2 on RAID/Swap, Btrfs RAID1, booster initramfs, UKI replaces GRUB. |
| [006](ADR-006-no-backward-compat.md) | No Backward Compatibility | 2026-08-26 | Accepted | Breaking changes allowed without fallback; installer runs once, always fresh. |
| [007](ADR-007-dinit-shutdown-ordering.md) | dinit Shutdown Ordering — Session Trackers Before Bus/elogind | 2026-09-30 | Accepted | `dbus-dinit`/`elogind-dinit` `stop-command` guards wait for `turnstiled` exit before stopping; `stop-timeout = 30` failsafe (no-op without turnstile). |

References: `AGENTS.md`, `CONTEXT.md`, `docs/PROJECT.md`, `docs/INSTALLER.md`, `docs/architecture/*`.
