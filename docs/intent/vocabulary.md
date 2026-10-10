# Vocabulary — Domain Glossary

Source: `CONTEXT.md#Core Terms` (double docs — `AGENTS.md` is source of truth, this copy is reference). Copy of domain vocabulary for code/docs/conversations. Keep `CONTEXT.md` as canonical.

## Core Terms

**Installer**
Standard Go program that partitions disks, bootstraps Alpine, configures hypervisor system. Logic never changes — only data changes. *Avoid*: script, setup, deploy. Ref: `src/main.go`.

**Generator**
Go program that generates `variables.yaml` interactively. Called by installer (interactive) or standalone (unattended). Starts from template defaults. *Avoid*: configurator, wizard. Ref: `src/utils/createConfig/`.

**Essential Packages**
Hardcoded in Go, always installed. Non-negotiable core. *Examples*: micro, doas, mdadm, grub-efi, btrfs-progs, linux-lts. *Avoid*: base/core packages.

**Optional Packages**
User-selectable in `variables.yaml`. Injected via APK. Rules in `internal_rules.yaml`. *Avoid*: extra/addons.

**Dynamic Rules**
Declarative logic in `internal_rules.yaml` mapping optional packages → groups, services, modules, GRUB options, mkinitfs features. *Avoid*: auto-config, implicit logic.

**Parser**
Standard code in `logic.go` that processes dynamic rules. Always same code — only data changes. No rules → nothing happens. *Avoid*: rule engine, logic processor. Ref: `src/utils/logic/`.

## Distribution Architecture

**FeralOS Distribution**
New Linux distro based on Alpine. Not minimal — full desktop + server via Alpine repos + custom repo. *Avoid*: Alpine fork.

**Memory Safety Focus**
Replace C components with memory-safe alternatives. *Examples*: uutils (Rust) instead of coreutils, micro instead of vim/nano. *Avoid*: secure by default, hardened.

**Nix Package Manager**
Secondary manager for software not in Alpine or requiring glibc. Access Nixpkgs without polluting base. *Avoid*: nixos, nix packages.

**Dinit**
Future init replacement for OpenRC. Better dependency handling, parallel startup. *Avoid*: init, service manager (generic).

**Custom Repository**
New Alpine-style repo for FeralOS-specific services/packages not upstream. Added to APK sources alongside Alpine. *Avoid*: local/private repo.

## Disk Layout

**Disk Layout**
Go package implementing specific partitioning scheme. Selected via `variables.yaml`. Each layout under `src/hostPhase/disks/`. *Avoid*: scheme, partitioning config.

**Layout — btrfsStandard**
Single disk: EFI (512M) + SWAP (8G) + ROOT/Btrfs (remainder). No RAID. *Use*: testing, single-disk servers, desktops.

**Layout — btrfsStandardServer**
Dual-disk RAID1: EFI (512M each) + RAID1 partition + SWAP (each) + Data (each). Btrfs root on RAID1 with subvolumes. *Use*: production servers, hypervisors.

**Layout — btrfsRaid0**
Dual-disk RAID0: EFI (512M each) + 2× LUKS swap p2 (`swapSize/2` gated `EnableSwap`) + root p3 remainder. Pure RAID0. Ref: `src/hostPhase/disks/btrfsRaid0/`.

**Layout — btrfsStandardUnencrypted**
Single disk, no LUKS (VMs). Planned. Ref: `ROADMAP.md`.

**Subvolume**
Btrfs subdivision of root filesystem, mounted at specific path with tailored options (compression, NoCoW, security flags). *Avoid*: partition, slice.

## Installation Modes

**Interactive Mode**
Installer → calls Generator → asks questions → generates variables.yaml → continues with values.

**Unattended Mode**
Generator standalone (or existing variables.yaml) → saves variables.yaml → installer reads and proceeds without interaction.

**Default Values**
Values from template `variables.yaml`. Generator starts with these, lets user override. Allows saving installation types. Ref: `src/variables/variables.yaml`.

## Phases

**Host Phase**
Live system stage: disk partitioning, RAID creation, bootstrap, bind mounts, config injection. *Avoid*: pre-install, setup phase. Ref: `src/hostPhase/`.

**Chroot Phase**
Target chroot stage: package install, user creation, service setup, bootloader. *Avoid*: post-install, config phase. Ref: `src/chrootPhase/`.

**Bootstrap**
Download `apk-tools-static` and initialize base Alpine rootfs into target partition. *Avoid*: init, base install. Ref: `src/hostPhase/internal/bootstrap.go`.

## Boot & Redundancy

**RAID1**
Mirror across two disks. Applied to `/boot` (Btrfs RAID1) and EFI (rsync via hook). *Avoid*: mirror, redundancy.

**EFI Sync**
APK post-commit hook: `rsync /boot/efi → /boot/efi2` after kernel upgrades. *Avoid*: efi-copy, boot-sync.

**Kernel Fallback**
APK pre-commit hook: saves current kernel before upgrade for rollback. *Avoid*: kernel-backup, rollback hook.

## System Components

**Runlevel**
OpenRC startup stage: `sysinit`, `boot`, `default`, `shutdown`. Services added dynamically per packages. *Avoid*: init-level, startup stage.

**APK Hook**
Alpine Package Keeper commit hook executing scripts before/after upgrades. Used for kernel backup + EFI sync. *Avoid*: package hook, apk script.

**Incus**
Container/VM manager (LXD successor). Installed but not started by default. Recommend mount on separate RAID disks at `/var/lib/incus`. *Avoid*: lxd, container runtime.

**ZRAM**
Compressed swap in RAM via `zram-init`. For low-memory self-hosted envs. *Avoid*: compressed swap, ram-swap.

## Memory-Safe Alternatives

| Traditional (C) | FeralOS Alternative | Language |
|-----------------|---------------------|----------|
| coreutils | uutils-coreutils | Rust |
| vim/nano | micro | Go |
| sudo | sudo-rs | Rust |
| rsyslog/logrotate | vector | Rust |
| bash/zsh | fish | Rust |
| lxd | incus | Go |
| systemd/openrc | dinit (future) | C++ |
| grep (GNU) | ripgrep (optional) | Rust |
| find (GNU) | fd (optional) | Rust |
| curl | curl (keeping) | C |

> Some C components remain when no memory-safe alternative exists or component minimal/audited.

## File Locations

| Path | Purpose |
|------|---------|
| `src/main.go` | Installer entry point |
| `src/generator/main.go` | Value generator |
| `src/variables.yaml` | User config (generated) |
| `src/variables/variables.yaml` | Template with defaults |
| `src/variables/internal_rules.yaml` | Dynamic rules |
| `src/utils/` | Config, exec, files, logic |
| `src/hostPhase/` | Host operations |
| `src/hostPhase/disks/` | Storage layouts (Go packages) |
| `src/chrootPhase/` | Chroot configuration |
| `internal/` | Legacy shell scripts (reference) |

## Conventions

- **Language**: English only in code, configs, docs
- **Go module**: `feralos/src`
- **Config format**: YAML
- **Mount point**: `/mnt` (default chrootDir)
- **Disk labels**: `EFI<DISK>`, `swap-<DISK>`, `data-<DISK>`, `boot`, `root`
- **Btrfs compression**: `zstd:3` with `autodefrag,ssd`
- **NoCoW paths**: `/swap`, `/srv`, `/var/log`, `/var/spool`, `/var/tmp`, `/var/lib/ex`

## References

- `CONTEXT.md` (canonical)
- `PROJECT.md#Package Architecture` + `#Storage Layouts`
- `AGENTS.md#Architecture Principle` + `#Naming Conventions`
- `docs/architecture/overview.md`
