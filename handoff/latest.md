# Handoff

**Active state**: `state/session-state.json`
**Milestone log**: `AGENTS.md`

## Project Vision

**FeralOS** is a new Linux distribution based on Alpine Linux, focused on **memory safety** and **security**. It replaces traditional C components with modern alternatives (uutils, micro, vector). For software not in Alpine repos or requiring glibc, **Nix** is used. After the installer is stable, **Dinit** will replace OpenRC, and a **custom repository** will be created for FeralOS services.

## Current Focus

Phase 1: Go installer complete. Both btrfsStandard and btrfsStandardServer pass verification checks with ALL features enabled (latest run 30/30 + `services_status` = 31 checks). Highlights:

- **Swapfile FIXED** — `btrfs filesystem mkswapfile` works: root cause was `btrfs property set compression none` setting the `m` attribute which conflicts with `chattr +C` (NixOS/nixpkgs #384047). `chattr +C` on @swap dir before mount.
- **NoCoW cleanup** — compression property removed from ALL NoCoW subvolumes (NODATACOW implies no compression), verified by `lsattr_nocow` harness check.
- **Swap shutdown FIXED** — OpenRC swap service `before localmount` guaranteed start order only, not stop order. Fix: swap moved to boot runlevel + `sed 's/before localmount/need localmount/' /etc/init.d/swap` in SetupServices. apk `.apk-new` verified: the sed survives openrc upgrades.
- **`log_errors` harness check** — non-regression guard: scans target-serial.log + installer.log for error/failed markers, fails run on any match. Whitelist: counters, `libgpg-error` package name, efibootmgr `error trace:` (QEMU, no EFI vars), smartd `start-stop-daemon: no matching processes found`, fsck `fsck.vfat: No such file or directory` (both QEMU-only).
- **`services_status` harness check** — verifies configured OpenRC services are actually STARTED via `rc-status --all` (not just `[ ok ]` at boot). Excludes QEMU-only: smartd (no SMART hardware — crashed), fsck (fsck.vfat not installed), udev-settle (one-shot). Parser handles supervise-daemon format `[ started 00:00:36 (0) ]`.
- **Harness lint clean** — full golangci-lint cleanup (commit `6b42ff3`): errcheck, nilerr, dead code removed, goconst, shadow, gosec, gocritic, hugeParam pointer refactors. Behavior verified: full run 30/30 PASS after refactor.
- **niri desktop environment** — greetd + regreet + niri + wayvnc + DMS (DankMaterialShell):
  - `nixPackages` field in internalRules (same behavior as apk — data selects install, auto-enables Nix)
  - `chrootPhase/desktops/niri.go`: greetd config, greeter.sh (`niri -- regreet`), regreet.toml, niri configs (greeter + user, keymap), wayvnc config (spawn-at-startup), xdg-su wrapper, OpenRC dms service (waits for wayland socket)
  - DMS + quickshell from **stable nixpkgs channel** (nixos-26.05) — dms-shell is packaged in nixpkgs
  - Nix: `--option sandbox false` chroot-only (trusted-users), TMPDIR on @nix-var-nix-build subvolume, registry pinned to stable
  - Alpine: wayland-libs-server, greeter in greetd group, elogind seat provider, no render group, no turnstile (edge only)
  - Harness: virtio-gpu-pci, checks niri_config_valid/greetd_config/niri_greeter/wayvnc_installed/dms_installed — **37/37 PASS** (run 2026-08-18T20-25-07Z)
- **sudoers fix** — Alpine default `/etc/sudoers` has `@includedir` commented out → `/etc/sudoers.d/wheel` ineffective (wheel sudo denied). Enabled with sed in misc.go + new `sudoers_includedir` harness check
- **nix.conf clean** — `allow-unfree`/`allow-insecure` not recognized by Alpine's nix build (warning noise) — removed; NIXPKGS_ALLOW_* env vars in profile.d/fish/bash carry it permanently
- **Dummy packages working** — APK v3 `apk mkpkg` creates valid packages with symlinks.
- **Ripgrep NOT a drop-in for grep** — rg doesn't support POSIX BRE (`\+`), breaks `efi-mkuki`. GNU grep removed — BusyBox grep used instead.

## What's Working

- Shell script installer (`internal/start.sh`) — full reference implementation
- Go installer: config loading, dynamic rules, host phase, chroot phase
- **btrfsStandardServer**: Dual-disk RAID1 layout with Btrfs subvolumes (21 subvolumes)
- **btrfsStandard**: Single-disk layout with swapfile (25 subvolumes)
- LUKS2 encryption on root and swap partitions
- **LUKS keyfile auto-unlock** — embedded in initramfs via `rd.luks.key=/keyfile:luks-<uuid>`
- booster initramfs (replaces mkinitfs)
- Signed UKI (Unified Kernel Image) for Secure Boot
- Serial console output (`console=ttyS0,115200`)
- efibootmgr guarded for BIOS runners
- APK hooks for kernel fallback and EFI sync
- Interactive wizard and standalone generator
- QEMU-based test harness with feature toggles
- Non-interactive mode for automated testing
- **Clevis TPM2 binding** — empty PCR policy (TPM identity only)
- **Vector logging** — replaces rsyslog/logrotate (Rust, memory-safe)
  - Socket: `/run/vector/syslog.sock` (symlinked from `/dev/log`)
  - Sinks: `/var/log/messages`, `auth.log`, `daemon.log`, `kern.log`
  - Service: `vector-setup` (boot) + `vector` (boot)
  - Check: `apk info | grep -q '^vector$'` (init-system agnostic)
- **btrbk** — btrfs backup/snapshot tool with subvolume `@btrbk`
- **turnstile** — session tracker for desktop environments
- **internalRules.yaml cleaned** — only base OS in defaultRunlevels, services in packageLogic
- **APK v3 dummy packages** — `apk mkpkg` creates valid packages, installed before main `apk add`
- **providePackages** — empty dummies for dependency satisfaction (e.g., `util-linux-misc`)
- **uutils-coreutils-more** — provides `/usr/bin/more`, dummy `util-linux-misc` satisfies `incus-feature-vm`
- **Swapfile working** — `btrfs filesystem mkswapfile --size 8g` with `chattr +C` on @swap dir before mount
  - btrfsStandardServer: no swapfile (dedicated LUKS swap partitions)
- **NoCoW subvolumes clean** — no compression property anywhere (NODATACOW implies no compression)
  - `chattr +C` on dir is the only mechanism; all 7 NoCoW subvolumes verified by `lsattr_nocow` harness check
  - Compressed subvolumes verified to stay CoW (no C flag)

## What's Next

### Phase 1 (Installer)
1. Run full harness to confirm 31/31 (services_status now in check suite)
2. Test on real hardware with 2x NVMe
3. Test with partial features (Secure Boot only, TPM only)
4. Add more unit tests for verify checks
5. Complete CI/CD pipeline

### Phase 2 (Distribution)
1. Dinit init system (replace OpenRC)
2. Custom FeralOS repository
3. Nix integration (glibc + alternatives)
4. Full distribution release

## Architecture Principle

**Standard core, pluggable data.**

- Installer logic never changes
- Storage layouts are Go packages
- Essential packages hardcoded in Go
- Optional packages + rules in YAML
- Parser always same code, only data changes

## Key Files

| File | Purpose |
|------|---------|
| `src/main.go` | Installer entry point |
| `src/utils/config/config.go` | Config types + LoadConfig |
| `src/utils/shell/execute.go` | Unified ExecuteCmd (retry, logging) |
| `src/variables/variables.yaml` | Template with defaults |
| `src/variables/internalRules.yaml` | Dynamic rules (packages, services) |
| `src/hostPhase/` | Host-side operations (orchestrator + internal/) |
| `src/hostPhase/disks/` | Storage layouts (Go packages) |
| `src/chrootPhase/` | Chroot operations (orchestrator + internal/ + features/) |
| `src/firstBoot/` | Standalone first-boot programs (secureboot, tpm) |
| `setup.sh` | Automated setup script |
| `tests/harness/` | QEMU-based test harness |
| `tests/debug-run/` | Debug tool for existing runs |
| `docs/INSTALLER.md` | Installer architecture documentation |
| `docs/USAGE.md` | Usage guide and configuration |
| `CONTEXT.md` | Domain vocabulary |

## Decisions Made

- **Vector socket**: `/run/vector/syslog.sock` (not `/dev/log` — permissions)
- **vector-setup**: boot stage (not sysinit — /var/log mounted by localmount)
- **Vector config**: Written AFTER package install (apk overwrites with demo_logs)
- **Vector check**: `apk info | grep -q '^vector$'` (init-system agnostic)
- **defaultRunlevels**: Only base OS services, rest in packageLogic
- **udev services**: In packageLogic under eudev (Alpine uses mdev by default)
- **modules service**: Removed (booster loads /etc/modules automatically)
- **btrbk snapshot_dir**: `/btrbk/snapshots` (not `/mnt/btrbk/snapshots`)
- **less package**: Solved via dummy package symlink (bat replaces less)
- **booster** instead of mkinitfs for initramfs
- **Signed UKI** instead of GRUB for boot
- **LUKS2** encryption on RAID and Swap partitions
- **Btrfs RAID1** on LUKS (not mdadm)
- **Phase separation**: hostPhase (writes files) vs chrootPhase (runs commands)
- **Ripgrep NOT a drop-in for grep**: rg doesn't support POSIX BRE (`\+`), breaks `efi-mkuki`
- **GNU grep removed**: BusyBox grep provides POSIX BRE, zero reverse dependencies
- **Dummy packages**: Symlink approach (APK content, not wrapper scripts)
- **Swapfile**: `btrfs filesystem mkswapfile` + `chattr +C` on dir before mount (Debian guide)
- **NoCoW**: no compression property — `chattr +C` on dir only (NODATACOW implies no compression, `m` attribute conflicts with C)
- **Swap service**: boot runlevel + `need localmount` (sed replaces `before localmount` — start order only, not stop order). apk preserves modified /etc files as `.apk-new` — the sed survives upgrades
- **Log errors check**: harness `log_errors` scans target-serial.log + installer.log for error/failed markers, fails run on any match (non-regression guard, whitelist for known innocuous lines incl. QEMU-only smartd/fsck)
- **Services active check**: harness `services_status` verifies configured services are actually started via `rc-status --all`; excludes QEMU-only services (smartd, fsck, udev-settle); parses supervise-daemon format
- **niri DE**: data in internalRules (`nixPackages` field, auto-enables Nix) + config logic in `chrootPhase/desktops/niri.go` (kde/gnome stay data-only)
- **Nix stable**: registry pinned to nixos-26.05 (default maps nixpkgs→unstable); sandbox disabled only for chroot install; TMPDIR on nix build subvolume; NIXPKGS_ALLOW_* env vars (not nix.conf — unknown settings)
- **Seat provider**: elogind (not seatd); no render group (Alpine uses video); no turnstile (edge only)
- **sudoers**: `@includedir /etc/sudoers.d` enabled (Alpine default comments it out — wheel sudo was denied)

## Memory-Safe Alternatives

| Traditional (C) | FeralOS Alternative | Language | Drop-in? |
|-----------------|------------------|----------|----------|
| coreutils | uutils-coreutils | Rust | No |
| vim/nano | micro | Go | No |
| sudo | sudo-rs | Rust | No |
| rsyslog/logrotate | vector | Rust | No |
| bash/zsh | fish | Rust | No |
| lxd | incus | Go | No |
| openrc | dinit (future) | C++ | No |
| grep (GNU) | BusyBox grep | POSIX BRE | Yes (built-in) |
| find (GNU) | fd | Rust | No (different syntax) |
| less | bat | Rust | Yes (symlink) |

---

*See `state/session-state.json` for current goals, decisions, risks, and next actions.*
*See `AGENTS.md` for project context and milestone history.*
*See `docs/PROJECT.md` for full distribution vision.*
