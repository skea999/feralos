# FeralOS Installer — Domain Vocabulary

Consistent terminology for code, docs, and conversations.

## Project Vision

**FeralOS** is a new Linux distribution based on Alpine Linux, focused on **memory safety** and **security**. It uses modern, secure components replacing traditional C software, while keeping Alpine's package repositories. For software not available in Alpine repos or requiring glibc, **Nix** is used as a secondary package manager.

## Core Terms

**Installer**
Standard Go program that partitions disks, bootstraps Alpine Linux, and configures a complete hypervisor system. The installer logic never changes — only data changes.
_Avoid_: script, setup, deploy

**Generator**
Go program that generates `variables.yaml` interactively. Can be called by the installer (interactive mode) or run standalone (unattended mode). Starts with default values from template.
_Avoid_: configurator, wizard

**Essential Packages**
Hardcoded packages in Go code that are always installed. Non-negotiable core system.
_Examples_: micro, doas, mdadm, grub-efi, btrfs-progs, linux-lts
_Avoid_: base packages, core packages

**Optional Packages**
User-selectable packages defined in `variables.yaml`. Injected via APK. Their rules (groups, services, modules) are defined in `internalRules.yaml`.
_Avoid_: extra packages, addons

**Dynamic Rules**
Declarative logic in `internalRules.yaml` that maps optional packages to groups, services, modules, GRUB options, and mkinitfs features.
_Avoid_: auto-config, implicit logic

**Parser**
Standard code in `logic.go` that processes dynamic rules. Always the same code — only the data in `internalRules.yaml` changes. If a package has no rules, nothing happens.
_Avoid_: rule engine, logic processor

## Distribution Architecture

**FeralOS Distribution**
New Linux distribution based on Alpine Linux. Not minimal — includes full desktop and server capabilities. Uses Alpine's package repositories with additional custom repository.
_Avoid_: Alpine fork, custom Alpine

**Memory Safety Focus**
Core design principle: replace C components with memory-safe alternatives where possible.
_Examples_: uutils (Rust) instead of coreutils, micro instead of vim/nano
_Avoid_: secure by default, hardened

**Nix Package Manager**
Secondary package manager for software not in Alpine repos or requiring glibc. Allows accessing packages from Nixpkgs without polluting the base system.
_Avoid_: nixos, nix packages

**Dinit**
Future init system replacement for OpenRC. Service manager with better dependency handling and parallel startup. Will replace OpenRC after base installer is stable.
_Avoid_: init, service manager (in generic context)

**Custom Repository**
New Alpine-style repository created for FeralOS-specific services and packages not upstream in Alpine. Added to APK sources alongside Alpine repos.
_Avoid_: local repo, private repo

## Disk Layout

**Disk Layout**
Go package that implements a specific partitioning scheme. Selected by user in `variables.yaml`. Each layout is a separate package under `installer/hostPhase/disks/`.
_Avoid_: scheme, partitioning config

**Layout — btrfsStandard**
Single-disk layout: EFI (512M) + SWAP (8G) + ROOT/Btrfs (remainder). No RAID.
_Use when_: testing, single-disk servers, desktops

**Layout — btrfsStandardServer**
Dual-disk RAID1 layout: EFI (512M each) + RAID1 partition + SWAP (each disk) + Data (each disk). Btrfs root on RAID1 with subvolumes.
_Use when_: production servers, high-availability hypervisors

**Subvolume**
Btrfs subdivision of root filesystem, mounted at specific path with tailored options (compression, NoCoW, security flags).
_Avoid_: partition, slice

## Installation Modes

**Interactive Mode**
User runs installer → installer calls generator → generator asks questions → generates variables.yaml → installer continues with user-provided values.

**Unattended Mode**
User runs generator standalone (or uses existing variables.yaml) → generator saves variables.yaml → user runs installer → installer reads variables and proceeds without interaction.

**Default Values**
Values from template `variables.yaml`. Generator starts with these and lets user override. Allows saving specific installation types.

## Phases

**Host Phase**
Installation stage on live system: disk partitioning, RAID creation, bootstrap, bind mounts, config injection.
_Avoid_: pre-install, setup phase

**Chroot Phase**
Installation stage inside target chroot: package installation, user creation, service setup, bootloader.
_Avoid_: post-install, config phase

**Bootstrap**
Download `apk-tools-static` and initialize base Alpine rootfs into target partition.
_Avoid_: init, base install

## Boot & Redundancy

**RAID1**
Mirror across two disks. Applied to `/boot` (Btrfs RAID1) and EFI (rsync via hook).
_Avoid_: mirror, redundancy

**EFI Sync**
APK post-commit hook: rsync `/boot/efi` → `/boot/efi2` after kernel upgrades.
_Avoid_: efi-copy, boot-sync

**Kernel Fallback**
APK pre-commit hook: saves current kernel before upgrade for rollback.
_Avoid_: kernel-backup, rollback hook

## System Components

**Runlevel**
OpenRC service startup stage: `sysinit`, `boot`, `default`, `shutdown`. Services are added dynamically based on packages.
_Avoid_: init-level, startup stage

**APK Hook**
Alpine Package Keeper commit hook that executes scripts before/after package upgrades. Used for kernel backup and EFI sync.
_Avoid_: package hook, apk script

**Incus**
Container and VM manager (LXD successor). Installed but not started by default. Recommended to mount on separate RAID disks at `/var/lib/incus`.
_Avoid_: lxd, container runtime (in hypervisor context)

**ZRAM**
Compressed swap in RAM via `zram-init`. For low-memory self-hosted environments.
_Avoid_: compressed swap, ram-swap

## Memory-Safe Alternatives

| Traditional (C) | FeralOS Alternative | Language |
|-----------------|------------------|----------|
| coreutils | uutils-coreutils | Rust |
| vim/nano | micro | Go |
| sudo | doas | C (minimal, audited) |
| systemd/openrc | dinit (future) | C++ |
| bash | ash (Alpine default) | C |
| grep (GNU) | ripgrep (optional) | Rust |
| find (GNU) | fd (optional) | Rust |
| curl | curl (keeping) | C |

_Note: Some C components remain when no memory-safe alternative exists or when the component is minimal/audited._

## File Locations

| Path | Purpose |
|------|---------|
| `src/main.go` | Installer entry point |
| `src/generator/main.go` | Value generator |
| `src/variables.yaml` | User config (generated) |
| `src/variables/variables.yaml` | Template with defaults |
| `src/variables/internalRules.yaml` | Dynamic rules |
| `src/utils/` | Config, exec, files, logic |
| `src/installer/hostPhase/` | Host operations |
| `src/installer/hostPhase/disks/` | Storage layouts (Go packages) |
| `src/installer/chrootPhase/` | Chroot configuration |
| `internal/` | Legacy shell scripts (reference) |

## Conventions

- **Language**: English only in code, configs, docs
- **Go module**: `alpineInstaller`
- **Config format**: YAML
- **Mount point**: `/mnt` (default chrootDir)
- **Disk labels**: `EFI<DISK>`, `swap-<DISK>`, `data-<DISK>`, `boot`, `root`
- **Btrfs compression**: `zstd:3` with `autodefrag,ssd`
- **NoCoW paths**: /swap, /srv, /var/log, /var/spool, /var/tmp, /var/lib/ex
