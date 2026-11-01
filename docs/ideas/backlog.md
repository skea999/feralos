# Backlog — Ideas & Roadmap

Source: `ROADMAP.md` Phases + Pipeline (double docs — `AGENTS.md#Roadmap` is source of truth, this copy is reference). Status reflects `ROADMAP.md` as of 2026-08-10 + `AGENTS.md#Roadmap` (Completed for btrfsRaid0/Standard/Server, Generator, Harness, Nix).

## Phase 1 — Installer

Build standard, hackable Go installer for Alpine with RAID + Btrfs.

| Ver | Feature | Prio | Status | Notes |
|-----|---------|------|--------|-------|
| v0.1 | Shell script installer working | P0 | Done | Legacy `internal/` reference |
| v0.2 | Go installer — btrfsStandardServer (2-disk RAID1) | P0 | Done | Dual EFI, LUKS swap, Btrfs RAID1, 21 subvolumes |
| v0.3 | Go installer — btrfsStandard (single disk) | P0 | Done | LUKS root, swapfile in @swap, 25 subvolumes |
| v0.4 | Generator (interactive + standalone) | P1 | Done | `src/utils/createConfig/` + `feralos_create_config` |
| v0.5 | Test suite: harness + debug_run | P2 | Done | QEMU harness, 14 fixtures, 22/22 checks |
| v0.6 | Nix integration | P1 | Done | `chrootPhase/features/nix.go`, subvolumes, daemon |
| v0.7 | **btrfsRaid0** (2-disk RAID0) | P0 | Done | Pure RAID0, 2 LUKS swap p2 (`swapSize/2` gated `EnableSwap`), per `AGENTS.md` Completed |
| v0.8 | **btrfsStandardUnencrypted** (VM, no LUKS) | P0 | Planned | Single disk, no encryption, swapfile |
| v0.8 | **Refining**: fix real-hardware errors (harness ignores) | P0 | Planned | QEMU-only `smartd`/`fsck` whitelisted → fix remaining |
| v1.0 | Stable installer release | P0 | Planned | Milestone |

### Phase 1 Pipeline (detailed)

| Feature | Prio | Status | Notes |
|---------|------|--------|-------|
| Go installer — btrfsStandardServer | P0 | Done | 2-disk RAID1 layout |
| Go installer — btrfsStandard | P0 | Done | Single disk, no RAID |
| Go installer — btrfsRaid0 | P0 | Done | 2-disk RAID0 (ROADMAP says Planned, AGENTS marks Completed) |
| Go installer — btrfsStandardUnencrypted | P0 | Planned | Single disk, no LUKS (VMs) |
| Essential packages (hardcoded) | P0 | Done | micro, doas, mdadm, grub, etc. |
| Dynamic rules engine | P0 | Done | `internal_rules.yaml` + `logic.go` |
| Generator (interactive) | P1 | Done | Interactive questions, guided defaults |
| Generator (standalone) | P1 | Done | Pre-configured `variables.yaml` |
| APK hooks (kernel + EFI) | P1 | Done | `hooks.go` |
| GRUB dual EFI → UKI | P1 | Done | `configurator.go` → `uki.go` (booster + signed UKI) |
| Fstab generation | P1 | Done | `fstab.go` per layout |
| Nix integration | P1 | Done | `nix.go`, subvolumes, daemon config |
| Test suite | P2 | Done | harness (QEMU) + debug_run, fixtures 14 templates |

## Phase 2 — Desktop

Install and configure niri + wayvnc.

| Ver | Feature | Prio | Status | Notes |
|-----|---------|------|--------|-------|
| v2.0 | **niri** Wayland compositor | P0 | Planned | Install + config |
| v2.1 | **wayvnc** VNC server | P0 | Planned | Install + config |

| Feature | Prio | Status | Notes |
|---------|------|--------|-------|
| niri Wayland compositor | P0 | Planned | Install + config |
| wayvnc VNC server | P0 | Planned | Install + config |

## Phase 3 — Distribution

Transform installer output into complete FeralOS distribution.

| Ver | Feature | Prio | Status | Notes |
|-----|---------|------|--------|-------|
| v3.0 | **Dinit** init system (replace OpenRC) | P0 | Planned | Service manager, parallel startup |
| v3.1 | **Custom FeralOS repository** | P0 | Planned | Alpine-style repo for FeralOS services |
| v3.2 | **Full FeralOS distribution release** | P0 | Planned | Milestone |
| — | Full desktop environment (KDE/other DEs) | P2 | Planned | Beyond niri |
| — | Server services (Web, DB, container runtimes) | P2 | Planned | Incus, etc. |

| Feature | Prio | Status | Notes |
|---------|------|--------|-------|
| Dinit init system | P0 | Planned | Replace OpenRC |
| Custom FeralOS repository | P0 | Planned | Alpine-style repo |
| Full desktop environment | P2 | Planned | KDE/other DEs |
| Server services | P2 | Planned | Web, DB, container runtimes |

## Priorities

- **P0**: Must — blocks release (layouts, core installer, Dinit, repo).
- **P1**: Should — important but not blocking (Generator, hooks, GRUB/EFI, Nix, fstab).
- **P2**: Nice — enhancements (test suite, extra DEs, server services).

## References

- `ROADMAP.md` (source — Phases, Milestones, Pipelines, Timeline)
- `AGENTS.md#Roadmap` (compressed roadmap, status Completed vs Planned)
- `CONTEXT.md#Memory-Safe Alternatives` (alternatives table)
- `docs/PROJECT.md#Roadmap`
- `CHANGELOG.md` (history)
