# Package Configs — the `*-config` Migration Inventory

**Rule** (ADR-013): static configs (no `variables.yaml` input, no per-install variation) move to aports `<pkgname>-config` packages; input-dependent/dynamic configs stay installer-written. This file is the LIVING migration inventory — update it per migration.

**Two strategic connections**:
1. **internal_rules-driven configs** (values from the rules engine, not user input) stay for now — but each is a REDESIGN CANDIDATE: make it input-free/generic and it becomes migratable. Going generic simplifies the installer (less config code).
2. **Propedeutic to the Arch-style chroot installation** (spec-distro §7 item 3): the chroot install path has no installer writing configs — every static config must already be an aports package. This migration builds that prerequisite.

## Done

| Config | Package | Notes |
|---|---|---|
| `/etc/vector/vector.yaml` | `vector-config 0.1.0-r0` | `replaces="vector"` demo takeover; GENERIC 1:1 source→file pattern; all sources inert via `ignore_not_found` (no conditional blocks); `conf.d` drop-in dir + README shipped; no API, no feralos.json.log, no security-stack vector sources (native persistent files — shipping later via conf.d per file) |

## Candidates (static — migrate when picked up)

| Installer write | Where | Notes |
|---|---|---|
| `/etc/greetd/config.toml` | niri/niriWayle setup | content fully static (vt 7, greeter.sh, user greeter) — shared by both DEs → `greetd-config` |
| `/etc/greetd/greeter.sh` | niri/niriWayle setup | static (XDG fallback + exec niri -- regreet + session.log redirect) → same `greetd-config` |
| `/etc/fail2ban/jail.d/50-feralos.conf` | `internal/security_stack.go` | static (nftables banaction, sshd jail) → `fail2ban-config` (or merged into a security-stack config package) |
| `/etc/clamav/clamd.conf` | `features/hardening.go` | static → `clamav-config` |
| `/etc/audit/audit.rules` | `internal/security_stack.go` | NEAR-static: existence-filtered watches (sudoers/ssh paths) — migrate only with the paths settled |
| `/etc/aide.conf` + daily cron | `internal/security_stack.go` | static (watch roots, report paths) → with the audit.rules decision |
| modprobe blacklist | `features/hardening.go` | static → `hardening-config`? (groups with the sysctl drop-ins if they ever go static) |
| `/etc/fish/conf.d/pager.fish` | `internal/misc.go` setupPager | static → tiny; batch with another migration |

## Stays installer-written (input-dependent / dynamic)

| Config | Why it stays |
|---|---|
| `/etc/profile.d/20locale.sh` | `locale` from variables (expanded it_IT.UTF-8) |
| `/etc/conf.d/loadkmap` | `keymap` from variables |
| `/etc/greetd/regreet.toml` | `timezone` from variables |
| user niri/wayle configs | keymap + user name + desktop choices from variables |
| `/etc/hostname`, users, groups, sudoers (user rules) | user/hostname from variables |
| UKI/kernel cmdline fragments | hardening + debug overrides from variables |
| fstab, crypttab, initramfs conf | layout + LUKS from variables |
| `/etc/X11/xorg.conf.d/00-keyboard.conf` | `keymap` from variables (locale1 data source — see backlog feralos-localed) |

## Conventions (per ADR-013)

- ONE package per service family: `<svc>-config` (configuration) and `<svc>-dinit` (service) are SEPARATE packages.
- Vendor demo-config takeover: `replaces="<vendor-pkg>"` (klogd-openrc precedent — vendor updates leave our file as `.apk-new`).
- The config is static and complete: absent-file sources are inert (`ignore_not_found`), no conditional blocks, one config valid on every install.
- Boot-time files the service needs: pre-created by the service's OWN tmpfiles, never by the installer.
