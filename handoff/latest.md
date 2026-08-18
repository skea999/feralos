# Handoff

**Active state**: `state/session-state.json`
**Milestone log**: `AGENTS.md`

## Project Vision

**FeralOS** is a new Linux distribution based on Alpine Linux, focused on **memory safety** and **security**. It replaces traditional C components with modern alternatives (uutils, micro, vector). For software not in Alpine repos or requiring glibc, **Nix** is used. After the installer is stable, **Dinit** will replace OpenRC, and a **custom repository** will be created for FeralOS services.

## Current Focus

Phase 1: Go installer complete. Both btrfsStandard and btrfsStandardServer pass 28/28 verification checks with ALL features enabled. **Swapfile FIXED** — `btrfs filesystem mkswapfile` works: root cause was `btrfs property set compression none` setting the `m` attribute which conflicts with `chattr +C` (NixOS/nixpkgs #384047). @swap now skips the compression property, `chattr +C` on the dir before mount (Debian guide), `swapon [ok]` at boot. **Dummy packages working** — APK v3 `apk mkpkg` creates valid packages with symlinks. **Ripgrep NOT a drop-in for grep** — rg doesn't support POSIX BRE (`\+`), breaks `efi-mkuki`. GNU grep removed — BusyBox grep used instead.

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
  - Root cause: `btrfs property set compression none` sets `m` attribute → conflicts with `chattr +C` (NixOS/nixpkgs #384047)
  - Fix: `NoCompressionProp` field on @swap subvolume skips the property
  - btrfsStandardServer: no swapfile (dedicated LUKS swap partitions)

## What's Next

### Phase 1 (Installer)
1. Test Secure Boot error fix (efi-mkkeys after package install)
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
| `src/utils/shell/shell.go` | RunCommand, RunChrootCommand |
| `src/variables/variables.yaml` | Template with defaults |
| `src/variables/internalRules.yaml` | Dynamic rules (packages, services) |
| `src/hostPhase/` | Host-side operations |
| `src/hostPhase/disks/` | Storage layouts (Go packages) |
| `src/chrootPhase/` | Chroot operations |
| `src/chrootPhase/configurator.go` | Main orchestrator |
| `setup.sh` | Automated setup script |
| `tests/harness/` | QEMU-based test harness |
| `tests/debug-run/` | Debug tool for existing runs |

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
- **@swap compression property skipped**: `m` attribute conflicts with `chattr +C` (NixOS/nixpkgs #384047)

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

## What's Next

### Phase 1 (Installer)
1. **Fix Vector VRL syntax error** — critical for logging to work
2. Test vector logging end-to-end
3. Test on real hardware with 2x NVMe
4. Test with partial features (Secure Boot only, TPM only)
5. Add more unit tests for verify checks
6. Complete CI/CD pipeline

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
| `src/utils/shell/shell.go` | RunCommand, RunChrootCommand, RunCommandStrict |
| `src/variables/variables.yaml` | Template with defaults |
| `src/variables/internalRules.yaml` | Dynamic rules |
| `src/hostPhase/` | Host-side operations |
| `src/hostPhase/keyfile.go` | LUKS keyfile generation + embedding |
| `src/hostPhase/booster.go` | booster.yaml + kernel cmdline |
| `src/chrootPhase/` | Chroot operations |
| `src/hostPhase/disks/` | Storage layouts (Go packages) |
| `setup.sh` | Automated setup script |
| `tests/harness/` | QEMU-based test harness |
| `docs/INSTALLER.md` | Installer architecture documentation |
| `docs/USAGE.md` | Usage guide and configuration |
| `CONTEXT.md` | Domain vocabulary |

## Decisions Made

- **Go** chosen over shell for maintainability
- **Shell scripts** kept as reference in `internal/`
- **YAML** for both user config and internal rules
- **Storage layouts** as Go packages (not YAML)
- **Essential packages** hardcoded in Go
- **Generator** can run standalone or integrated
- **Nix** for glibc and alternative software
- **Dinit** will replace OpenRC in Phase 2
- **Custom repository** for FeralOS-specific services
- **booster** instead of mkinitfs for initramfs
- **Signed UKI** instead of GRUB for boot
- **LUKS2** encryption on RAID and Swap partitions
- **Btrfs RAID1** on LUKS (not mdadm)
- **Phase separation**: hostPhase (writes files) vs chrootPhase (runs commands)
- **Module isolation**: Each functionality is a separate module
- **LUKS keyfile** embedded in initramfs for headless auto-unlock
- **Per-device keyfile args** required for multi-LUKS layouts (booster requirement)
- **EFI guarded** — efibootmgr skipped on BIOS runner, BOOTX64.EFI fallback handles boot
- **Clevis TPM2 bind policy** — empty `{}` (TPM identity only, no PCR policy)
- **setup.sh** — launcher only, no configuration logic
- **Harness writes variables.yaml** — direct file generation, no stdin
- **btrfsStandard** — only 1 install disk (no ZFS or btrfs-data)
- **Auto-clean old runs** — keeps last 2, saves disk space
- **Test artifacts** — `tests/artifacts/<tool-name>/<timestamp>/` pattern
- **rsyslog → vector** — memory-safe logging pipeline (Rust)
- **zram-init → zswap** — kernel compressed swap cache with swap file
- **btrbk** — btrfs backup/snapshot tool for data protection
- **turnstile** — session tracker for desktop environments

## Memory-Safe Alternatives

| Traditional (C) | FeralOS Alternative | Language |
|-----------------|------------------|----------|
| coreutils | uutils-coreutils | Rust |
| vim/nano | micro | Go |
| sudo | sudo-rs | Rust |
| rsyslog/logrotate | vector | Rust |
| bash/zsh | fish | Rust |
| lxd | incus | Go |
| openrc | dinit (future) | C++ |
| grep (GNU) | ripgrep (optional) | Rust |
| find (GNU) | fd (optional) | Rust |

---

*See `state/session-state.json` for current goals, decisions, risks, and next actions.*
*See `AGENTS.md` for project context and milestone history.*
*See `docs/PROJECT.md` for full distribution vision.*
