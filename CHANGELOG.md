# Changelog

All notable changes to FeralOS will be documented in this file.

## [Unreleased]

### Added
- Go installer with btrfsStandardServer layout (2-disk RAID1)
- Go installer with btrfsStandard layout (single disk, swapfile)
- Dynamic rules engine (internalRules.yaml + logic.go)
- APK hooks for kernel fallback and EFI sync
- Btrfs subvolume creation with mount options
- Fstab generation per layout
- Interactive command execution with retry/ignore/abort
- Config loading from YAML
- Generator (interactive + standalone)
- QEMU-based test harness with runner + target VMs
- Non-interactive mode for automated testing
- LUKS keyfile auto-unlock for headless boot (rd.luks.key)
- Improved logging with module context and timestamps

### Fixed
- Pointer copy bug in configurator.go, hardening.go, zswap.go (u := conf.User → u := &conf.User)
- zswap cmdline flow: now appends to KernelCmdline slice, not overwrite file
- dmcrypt skips for btrfsStandard (uses swapfile, no swap partition)
- Harness hardcoded feature toggles: added EnableSecureboot/EnableHardening/EnableZswap/EnableNix to RunConfig
- efibootmgr guarded with EFI availability check (no error on BIOS runner)
- Harness disk layout changed to btrfsStandardServer (matches RAID1 verification)
- Unreachable code in harness service.go (shift key sleep)
- booster per-device keyfile args: rd.luks.key=/keyfile:luks-<uuid> for multi-device LUKS

### Changed
- Migrated from monolithic shell script to modular Go architecture
- Separated host phase and chroot phase into distinct packages
- Console output via console=ttyS0,115200 in kernel cmdline
- Keyfile generation added as Step 6 in main.go (between bootstrap and chroot prep)

### Planned (Phase 2)
- Dinit init system (replace OpenRC)
- Custom FeralOS repository
- Nix package manager integration
- Full distribution release

## [0.1.0] — Shell Script

### Added
- Working shell script installer (internal/start.sh)
- Dual-disk RAID1 partitioning
- Btrfs subvolume layout (20+ subvolumes)
- Alpine bootstrap via apk-tools-static
- Basic fstab and GRUB configuration

---

*Format based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/)*
