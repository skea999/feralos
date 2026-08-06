# Handoff

**Active state**: `state/session-state.json`
**Milestone log**: `AGENTS.md`

## Project Vision

**FeralOS** is a new Linux distribution based on Alpine Linux, focused on **memory safety** and **security**. It replaces traditional C components with modern alternatives (uutils, micro, doas). For software not in Alpine repos or requiring glibc, **Nix** is used. After the installer is stable, **Dinit** will replace OpenRC, and a **custom repository** will be created for FeralOS services.

## Current Focus

Phase 1: Go installer complete. Both btrfsStandard and btrfsStandardServer pass 17/17 verification checks with ALL features enabled (Secure Boot + TPM + Nix + Zswap + Hardening). Test artifacts cleaned, harness README updated.

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
- Improved logging with module context
- **Clevis TPM2 binding** — empty PCR policy (TPM identity only)
- **Auto-clean old runs** — keeps last 2, saves disk space
- **Test artifacts** — 14 .gitkeep files preserve directory structure

## What's Next

### Phase 1 (Installer)
1. Test on real hardware with 2x NVMe
2. Test with partial features (Secure Boot only, TPM only)
3. Add more unit tests for verify checks
4. Complete CI/CD pipeline

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

## Memory-Safe Alternatives

| Traditional (C) | FeralOS Alternative | Language |
|-----------------|------------------|----------|
| coreutils | uutils-coreutils | Rust |
| vim/nano | micro | Go |
| sudo | doas | C (minimal) |
| openrc | dinit (future) | C++ |
| grep (GNU) | ripgrep (optional) | Rust |
| find (GNU) | fd (optional) | Rust |

---

*See `state/session-state.json` for current goals, decisions, risks, and next actions.*
*See `AGENTS.md` for project context and milestone history.*
*See `docs/PROJECT.md` for full distribution vision.*
