# Changelog

All notable changes to FeralOS will be documented in this file.

## [Unreleased]

### Added
- Go installer with btrfsStandardServer layout (2-disk RAID1)
- Dynamic rules engine (internalRules.yaml + logic.go)
- APK hooks for kernel fallback and EFI sync
- GRUB dual EFI installation
- Btrfs subvolume creation with mount options
- Fstab generation per layout
- Interactive command execution with retry/ignore/abort
- Config loading from YAML

### Changed
- Migrated from monolithic shell script to modular Go architecture
- Separated host phase and chroot phase into distinct packages
- Project vision: from installer to full FeralOS distribution

### Planned (Phase 1)
- btrfsStandard layout (single disk)
- Generator (interactive + standalone)
- Test suite + CI

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
