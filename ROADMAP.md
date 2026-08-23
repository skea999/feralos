# Roadmap

**Status**: Active Development
**Distribution**: FeralOS (based on Alpine Linux)

## Phases

### Phase 1 — Installer
Build a standard, hackable Go installer for Alpine Linux with RAID + Btrfs.

### Phase 2 — Desktop
Install and configure niri (Wayland compositor) + wayvnc (VNC server).

### Phase 3 — Distribution
Transform the installer output into a complete FeralOS distribution with Dinit, custom repos.

---

## Phase 1 Milestones

| Ver | Target | Status | What |
|-----|--------|--------|------|
| v0.1 | — | Done | Shell script installer working |
| v0.2 | — | Done | Go installer: btrfsStandardServer (2-disk RAID1) |
| v0.3 | — | Done | Go installer: btrfsStandard (single disk) |
| v0.4 | — | Done | Generator (interactive + standalone) |
| v0.5 | — | Done | Test suite: harness + debug-run |
| v0.6 | — | Done | Nix integration |
| v0.7 | — | Planned | Go installer: btrfsRaid0 (2-disk RAID0) |
| v0.8 | — | Planned | Go installer: btrfsStandardUnencrypted (VM layout, no LUKS) |
| v0.8 | — | Planned | Refining: fix errors that block on real hardware (harness ignores them) |
| v1.0 | — | Planned | Stable installer release |

## Phase 2 Milestones

| Ver | Target | Status | What |
|-----|--------|--------|------|
| v2.0 | — | Planned | niri Wayland compositor |
| v2.1 | — | Planned | wayvnc VNC server |

## Phase 3 Milestones

| Ver | Target | Status | What |
|-----|--------|--------|------|
| v3.0 | — | Planned | Dinit init system (replace OpenRC) |
| v3.1 | — | Planned | Custom FeralOS repository |
| v3.2 | — | Planned | Full FeralOS distribution release |

---

## Phase 1 Pipeline

| Feature | Prio | Status | Notes |
|---------|------|--------|-------|
| Go installer — btrfsStandardServer | P0 | Done | 2-disk RAID1 layout |
| Go installer — btrfsStandard | P0 | Done | Single disk, no RAID |
| Go installer — btrfsRaid0 | P0 | Planned | 2-disk RAID0 layout (like btrfsStandardServer but RAID0) |
| Go installer — btrfsStandardUnencrypted | P0 | Planned | Single disk, no LUKS (for VMs) |
| Essential packages (hardcoded) | P0 | Done | micro, doas, mdadm, grub, etc. |
| Dynamic rules engine | P0 | Done | internal_rules.yaml + logic.go |
| Generator (interactive) | P1 | Done | Interactive questions, guided defaults |
| Generator (standalone) | P1 | Done | Pre-configured variables.yaml |
| APK hooks (kernel + EFI) | P1 | Done | hooks.go |
| GRUB dual EFI | P1 | Done | configurator.go |
| Fstab generation | P1 | Done | fstab.go per layout |
| Nix integration | P1 | Done | nix.go, subvolumes, daemon config |
| Test suite | P2 | Done | harness (QEMU) + debug-run |

## Phase 2 Pipeline

| Feature | Prio | Status | Notes |
|---------|------|--------|-------|
| niri Wayland compositor | P0 | Planned | Install + config |
| wayvnc VNC server | P0 | Planned | Install + config |

## Phase 3 Pipeline

| Feature | Prio | Status | Notes |
|---------|------|--------|-------|
| Dinit init system | P0 | Planned | Replace OpenRC |
| Custom FeralOS repository | P0 | Planned | Alpine-style repo for FeralOS services |
| Full desktop environment | P2 | Planned | KDE/other DEs |
| Server services | P2 | Planned | Web, DB, container runtimes |

---

## Timeline

*Schedule TBD — driven by contribution velocity.*

---

**Updated**: 2026-08-10
**Next review**: TBD
