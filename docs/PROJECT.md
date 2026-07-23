# PROJECT — Skea Distribution

**Status**: Active Development
**Stack**: Go 1.22 (installer) + Shell (legacy) + YAML config
**Distribution Base**: Alpine Linux

## Vision

**Skea** is a new Linux distribution based on Alpine Linux, focused on **memory safety** and **security**. It replaces traditional C components with modern, secure alternatives while keeping Alpine's package repositories. For software not available in Alpine or requiring glibc, **Nix** is used as a secondary package manager.

The installer is the first step — once stable, Skea will:
1. Replace OpenRC with **Dinit** as init system
2. Create a **custom repository** for Skea-specific services
3. Ship as a complete, bootable distribution

## Users

- **Primary**: Security-conscious self-hosters and developers
- **Secondary**: Small teams running on-premise infrastructure
- **Not for**: Cloud providers, managed Kubernetes, desktop-only use

## Problem

Existing Linux distributions either:
- Are minimal but lack security focus (Alpine)
- Have security features but heavy footprint (Fedora Silverblue)
- Don't prioritize memory safety (most distros)
- Don't offer modern init systems (OpenRC, SysVinit)

There's no distribution that combines Alpine's minimalism with a systematic memory-safety approach and modern service management.

## Solution

### Phase 1: Installer (current)
Go-based installer that:
1. Generates configuration interactively or unattended
2. Partitions disks with RAID1 + Btrfs
3. Bootstraps Alpine with memory-safe components
4. Configures system via chroot

### Phase 2: Distribution (future)
Complete Skea distribution with:
1. Dinit replacing OpenRC
2. Custom repository for Skea services
3. Nix for glibc/alternative software
4. Full desktop and server capabilities

## Architecture Principle

**Standard core, pluggable data.**

- Installer logic is standard, never changes
- Storage layouts are separate Go packages
- Essential packages are hardcoded in Go
- Optional packages + rules are in YAML
- Parser is always the same code, only data changes

## Package Architecture

### Essential (hardcoded in Go)
Always installed, non-negotiable:
- micro, doas, mdadm, musl-locales, lang, tzdata
- kbd-bkeymaps, grub-efi, efibootmgr, linux-lts
- btrfs-progs, zstd, cronie, chrony, dbus
- network-extras, uutils, uutils-coreutils, grep, findutils

### Optional (YAML-driven)
User selects from `variables.yaml`. Parser applies rules:
- Desktop: kde → plasma, sddm, konsole, dolphin
- Video: amdgpu → xf86-video-amdgpu, mesa, vulkan
- Incus: incus-feature → groups, services, modules
- QEMU: qemu-system-x86_64 → kvm groups, modules
- ZRAM: zram-init → service, modules

### Parser Logic
```
if package in packageLogic:
    inject groups → userGroups
    inject services → runlevels
    inject modules → /etc/modules
    inject grubCmdline → /etc/default/grub
    inject mkinitfsFeatures → /etc/mkinitfs/mkinitfs.conf
```

## Storage Layouts

Go packages under `installer/hostPhase/disks/`:

| Layout | Disks | RAID | Use Case |
|--------|-------|------|----------|
| `btrfsStandard` | 1 | No | Testing, desktop |
| `btrfsStandardServer` | 2 | RAID1 | Production servers |

Each layout handles:
1. Partitioning (fdisk)
2. Formatting (mkfs)
3. RAID creation (mdadm) if applicable
4. Btrfs subvolumes
5. Mounts
6. Fstab generation

## Memory-Safe Alternatives

| Traditional (C) | Skea Alternative | Language |
|-----------------|------------------|----------|
| coreutils | uutils-coreutils | Rust |
| vim/nano | micro | Go |
| sudo | doas | C (minimal, audited) |
| systemd/openrc | dinit (future) | C++ |
| grep (GNU) | ripgrep (optional) | Rust |
| find (GNU) | fd (optional) | Rust |

## Installation Modes

### Interactive (guided)
```
Linux Mint Live
    │
    ▼
setup.sh (downloads + launches)
    │
    ▼
Installer starts
    │
    ▼
Generator (interactive)
    │
    ├── Disk selection
    ├── User/password
    ├── Hostname, timezone, locale
    ├── Desktop environment
    ├── Video driver
    ├── Optional packages
    │
    ▼
variables.yaml generated
    │
    ▼
Installer continues with values
    │
    ▼
Installation complete
```

### Unattended (automated)
```
Linux Mint Live
    │
    ▼
setup.sh (downloads + launches)
    │
    ▼
Generator (standalone or pre-configured variables.yaml)
    │
    ▼
variables.yaml ready
    │
    ▼
Installer -config variables.yaml (no interaction)
    │
    ▼
Installation complete
```

## Metrics

| Metric | Target | How |
|--------|--------|-----|
| Install time | < 10 min | Parallel operations |
| Disk minimum | 2x 8GB | Enforced in layout |
| Boot redundancy | EFI on 2 disks | APK hook |
| Data safety | RAID1 + Btrfs | Subvolume layout |

## Risks

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| NVMe naming varies | Medium | High | Use blockdev for detection |
| APK hooks fail silently | Low | High | Validate files, log output |
| GRUB dual EFI fails | Medium | High | Fallback to single EFI |
| Btrfs mount order matters | Low | Medium | Strict sequence |

## Out of Scope (v1)

- Web UI (v2)
- Disk encryption (v2)
- Secure boot (v2)
- ARM64 (v2)
- Custom repos (Phase 2)
- Dinit (Phase 2)

## Roadmap

| Phase | Milestone | Status |
|-------|-----------|--------|
| Phase 1 | Go installer: btrfsStandardServer | In Progress |
| Phase 1 | Go installer: btrfsStandard | Planned |
| Phase 1 | Generator (interactive + standalone) | Planned |
| Phase 1 | Test suite + CI | Planned |
| Phase 2 | Dinit init system | Planned |
| Phase 2 | Custom Skea repository | Planned |
| Phase 2 | Nix integration | Planned |
| Phase 2 | Full distribution release | Planned |

---

*Updated: 2026-07-23*
