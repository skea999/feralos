# Roadmap

**Status**: Active Development
**Distribution**: Skea (based on Alpine Linux)

## Phases

### Phase 1 — Installer
Build a standard, hackable Go installer for Alpine Linux with RAID1 + Btrfs.

### Phase 2 — Distribution
Transform the installer output into a complete Skea distribution with Dinit, custom repos, and Nix integration.

---

## Phase 1 Milestones

| Ver | Target | Status | What |
|-----|--------|--------|------|
| v0.1 | — | Done | Shell script installer working |
| v0.2 | — | In Progress | Go installer: btrfsStandardServer (2-disk RAID1) |
| v0.3 | — | Planned | Go installer: btrfsStandard (single disk) |
| v0.4 | — | Planned | Generator (interactive + standalone) |
| v0.5 | — | Planned | Test suite + CI pipeline |
| v1.0 | — | Planned | Stable installer release |

## Phase 2 Milestones

| Ver | Target | Status | What |
|-----|--------|--------|------|
| v2.0 | — | Planned | Dinit init system (replace OpenRC) |
| v2.1 | — | Planned | Custom Skea repository |
| v2.2 | — | Planned | Nix integration (glibc + alternatives) |
| v2.3 | — | Planned | Full Skea distribution release |

---

## Phase 1 Pipeline

| Feature | Prio | Status | Notes |
|---------|------|--------|-------|
| Go installer — btrfsStandardServer | P0 | In Progress | 2-disk RAID1 layout |
| Go installer — btrfsStandard | P0 | Planned | Single disk, no RAID |
| Essential packages (hardcoded) | P0 | Done | micro, doas, mdadm, grub, etc. |
| Dynamic rules engine | P0 | Done | internalRules.yaml + logic.go |
| Generator (interactive) | P1 | Planned | Interactive questions, guided defaults |
| Generator (standalone) | P1 | Planned | Pre-configured variables.yaml |
| APK hooks (kernel + EFI) | P1 | Done | hooks.go |
| GRUB dual EFI | P1 | Done | configurator.go |
| Fstab generation | P1 | Done | fstab.go per layout |
| Test suite | P2 | Planned | Unit + integration tests |
| CI pipeline | P2 | Planned | GitHub Actions |

## Phase 2 Pipeline

| Feature | Prio | Status | Notes |
|---------|------|--------|-------|
| Dinit init system | P0 | Planned | Replace OpenRC |
| Custom Skea repository | P0 | Planned | Alpine-style repo for Skea services |
| Nix package manager | P1 | Planned | glibc + alternative software |
| Memory-safe components | P1 | Planned | uutils, micro, doas (already in Phase 1) |
| Full desktop environment | P2 | Planned | KDE/other DEs |
| Server services | P2 | Planned | Web, DB, container runtimes |

---

## Timeline

*Schedule TBD — driven by contribution velocity.*

---

**Updated**: 2026-07-23
**Next review**: TBD
