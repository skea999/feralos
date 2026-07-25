# Handoff

**Active state**: `state/session-state.json`
**Milestone log**: `AGENTS.md`

## Project Vision

**FeralOS** is a new Linux distribution based on Alpine Linux, focused on **memory safety** and **security**. It replaces traditional C components with modern alternatives (uutils, micro, doas). For software not in Alpine repos or requiring glibc, **Nix** is used. After the installer is stable, **Dinit** will replace OpenRC, and a **custom repository** will be created for FeralOS services.

## Current Focus

Phase 1: Go installer for Alpine Linux with RAID1 + Btrfs. Primary layout (`btrfsStandardServer`) is in progress.

## What's Working

- Shell script installer (`internal/start.sh`) — full reference implementation
- Go installer: config loading, dynamic rules, host phase (partitioning, bootstrap, hooks), chroot phase
- Dual-disk RAID1 layout with Btrfs subvolumes
- APK hooks for kernel fallback and EFI sync

## What's Next

### Phase 1 (Installer)
1. Verify Go `btrfsStandardServer` produces identical fstab to shell script
2. Test on real hardware with 2x NVMe
3. Implement `btrfsStandard` (single disk) layout
4. Build generator (interactive + standalone)
5. Add test suite

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
| `src/generator/main.go` | Value generator (planned) |
| `src/variables.yaml` | User config (runtime) |
| `src/variables/variables.yaml` | Template with defaults |
| `src/variables/internalRules.yaml` | Dynamic rules |
| `src/installer/hostPhase/disks/` | Storage layouts (Go packages) |
| `internal/start.sh` | Reference shell script |
| `docs/PROJECT.md` | Distribution vision, architecture |
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

## Memory-Safe Alternatives

| Traditional (C) | FeralOS Alternative | Language |
|-----------------|------------------|----------|
| coreutils | uutils-coreutils | Rust |
| vim/nano | micro | Go |
| sudo | doas | C (minimal) |
| openrc | dinit (future) | C++ |

---

*See `state/session-state.json` for current goals, decisions, risks, and next actions.*
*See `AGENTS.md` for project context and milestone history.*
*See `docs/PROJECT.md` for full distribution vision.*
