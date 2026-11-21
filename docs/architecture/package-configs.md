# Package Configs — the `*-config` Migration Inventory

**Rule** (ADR-013): static configs (no `variables.yaml` input, no per-install variation) move to aports `<pkgname>-config` packages; input-dependent/dynamic configs stay installer-written. This file is the LIVING migration inventory — update it per migration.

**Two strategic connections**:
1. **internal_rules-driven configs** (values from the rules engine, not user input) stay for now — but each is a REDESIGN CANDIDATE: make it input-free/generic and it becomes migratable. Going generic simplifies the installer (less config code).
2. **Propedeutic to the Arch-style chroot installation** (spec-distro §7 item 3): the chroot install path has no installer writing configs — every static config must already be an aports package. This migration builds that prerequisite.

## Done

| Config | Package | Notes |
|---|---|---|
| `/etc/vector/vector.yaml` | `vector-config 0.1.0-r2` | `replaces="vector"` demo takeover; GENERIC 1:1 source→file pattern; all sources inert via `ignore_not_found` (no conditional blocks); `conf.d` drop-in dir + README shipped; no API, no feralos.json.log, no security-stack vector sources (native persistent files — shipping later via conf.d per file); **the sinks it names ship here too** (ADR-013 R1): `post-install` materializes `/var/log/{messages,auth.log,daemon.log,kern.log,sudo.log,firewalld.log,user-session.log}` as `vector:vector 0644` (dynamic uid → install-time chown), idempotent + repairs a wrong owner |
| `/etc/logrotate.d/feralos` | `logrotate-config 0.1.0-r0` | GENERIC rule `/var/log/*.log` + `/var/log/*/*.log` (daily, rotate 14, compress, copytruncate); `depends="logrotate"` (execution via `/etc/periodic/daily/logrotate`, cronie — init-agnostic); also takes over the vendor `/etc/logrotate.d/fail2ban` (`replaces="fail2ban"`) with a comment-only stub — the generic rule matches the same file and a second entry makes every run fail with `duplicate log entry` |
| `/etc/fail2ban/jail.d/50-feralos.conf` | `fail2ban-config 0.1.0-r0` | static jail (nftables banaction, sshd on vector's `/var/log/auth.log`, polling backend); sorts after the vendor `alpine-ssh.conf` so its `[DEFAULT]`/`[sshd]` values win; installer `writeFail2banJail` DELETED |
| `/etc/clamav/clamd.conf` | `clamav-config 0.1.0-r0` | static (LogFile/PidFile/LocalSocket/DB dir/User, Foreground off); `replaces="clamav-daemon"` takes over the sample copy Alpine installs; hardening-gated via `HardeningPackages`; installer `writeClamdConfig` DELETED (the runtime dirs come from the service units' own checkpath/wrapper) |
| `/etc/sysctl.d/{93-ipv4Configs,95-randomizeAddresses,96-disableSuidDumps,97-disableCoreDumps,bridging}.conf` + `/etc/modprobe.d/disabledModules.conf` | `hardening-config 0.1.0-r0` | static hardened defaults; hardening-gated (`HardeningPackages`), so non-hardening installs are untouched. SPLIT: `94-limitLogs` (printk), `98-hardening` (io_uring/memfd), `99-cmdline-hardening` (dmesg_restrict) and `zswap.conf` (min_free_kbytes) STAY — values come from `enableDebug`/RAM |
| `/etc/profile.d/{editor.sh,console.sh,pager.sh}` + `/etc/fish/conf.d/pager.fish` | `profile-config 0.1.0-r0` | static shell defaults; the old `sed` on the vendor `/etc/profile` (PAGER=less→moor) is replaced by the `pager.sh` drop-in |
| `/etc/smartd.conf` | `smartd-config 0.1.0-r0` | static DEVICESCAN + self-test schedule; no collision (Alpine's smartmontools ships `/etc/conf.d/smartd`) |
| `/etc/polkit-1/rules.d/49-feralos-power.rules` | `polkit-power-config 0.1.0-r0` | static wheel rule (login1 power + PPD switch/hold); `depends=""` on purpose — inert without polkit, which preserves the old behaviour (file always present, polkit only where a DE pulls it) |

## Candidates (static — migrate when picked up)

| Installer write | Where | Notes |
|---|---|---|
| `/etc/greetd/config.toml` | niri/niriWayle setup | content fully static (vt 7, greeter.sh, user greeter) — shared by both DEs → `greetd-config` |
| `/etc/greetd/greeter.sh` | niri/niriWayle setup | static (XDG fallback + exec niri -- regreet + session.log redirect) → same `greetd-config` |
| `/etc/audit/audit.rules` | `internal/security_stack.go` | NEAR-static: existence-filtered watches (sudoers/ssh paths) — migrate only with the paths settled |
| `/etc/aide.conf` + daily cron | `internal/security_stack.go` | static (watch roots, report paths) → with the audit.rules decision |

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
- Boot-time files the service must FIND are configuration: shipped by `<svc>-config` (package content, or its `post-install` when the owner is a dynamically allocated system user), never created by the service wrapper, never by the installer. Files that ARE the service's own state (runtime dir, socket path, pidfile) stay with the service package, declared in tmpfiles — no imperative code (ADR-013 R1).
