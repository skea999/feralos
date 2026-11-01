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
| [008](ADR-008-dinit-config-files.md) | dinit Config Files — conf.d-equivalent via Artix Wrapper Pattern | 2026-10-01 | Accepted | All `-dinit` services: service → `/usr/lib/dinit/<svc>` wrapper → sources `/etc/dinit.d/config/<svc>.conf` → exec daemon; user-editable, apk-protected (OpenRC `/etc/conf.d` convention restored). |
| [009](ADR-009-dinit-service-conventions.md) | dinit Service Conventions — Artix/Chimera Alignment | 2026-10-02 | Accepted | No `restart` on daemons (smooth-recovery only; getty exception), `before = login.target` convention, network ordering chain (pre-network before network, firewalls before pre-network), dbus `smooth-recovery=no`, bus-name readiness via `dbus-wait-for`, one-shots scripted; ADR-008 config pattern retained as FeralOS extension. |
| [010](ADR-010-dbus-activation-dinit.md) | D-Bus Activation Routes Through dinit Services | 2026-10-18 | Accepted | Every D-Bus-activatable daemon is a dinit service (system bus → system unit, session bus → user unit); unit name = canonical well-known bus name (+`.user`); effective `.service` Exec = `/usr/libexec/feralos/dbus-activate <unit> -- <upstream exec>`; overrides ship in `/usr/share/feralos/dbus-system-services` + `/usr/share/feralos/dbus-services` and are materialized into `/usr/local/share/dbus-1/system-services` (boot, via `dbus-services-setup`) + `$XDG_RUNTIME_DIR/dbus-1/services` (login) — `/etc/dbus-1/system-services` is daemon-enumerated but not helper-activatable; activation env forwarded via one `dinitctl setenv`; OpenRC system-bus falls back to upstream exec. |

References: `AGENTS.md`, `CONTEXT.md`, `docs/PROJECT.md`, `docs/INSTALLER.md`, `docs/architecture/*`.
