# Goals — FeralOS

Source: `PROJECT.md#Users / #Problem / #Solution / #Metrics / #Out of Scope` + `CONTEXT.md` terminology (double docs — `AGENTS.md` is source of truth, this copy is reference).

## Users

- **Primary**: Security-conscious self-hosters and developers.
- **Secondary**: Small teams running on-premise infrastructure.
- **Not for**: Cloud providers, managed Kubernetes, desktop-only use.

Ref: `PROJECT.md#Users`.

## Problem

No distribution combines Alpine minimalism with systematic memory-safety and modern service management:

- Minimal but lacks security focus (Alpine).
- Security features but heavy footprint (Fedora Silverblue).
- Doesn't prioritize memory safety (most distros).
- Doesn't offer modern init systems (OpenRC, SysVinit).

Ref: `PROJECT.md#Problem` + `CONTEXT.md#Project Vision`.

## Solution

### Phase 1 — Installer (current)

Go-based installer (Go 1.22 + Shell legacy ref + YAML config):

1. Generates config interactively (installer calls Generator) or unattended (standalone Generator / existing `variables.yaml`).
2. Partitions disks with RAID1 + Btrfs (separate Go packages per layout).
3. Bootstraps Alpine with memory-safe components (Essential hardcoded, Optional via `internal_rules.yaml`).
4. Configures system via chroot (booster + UKI, LUKS2, AppArmor).

### Phase 2 — Desktop

niri Wayland compositor + wayvnc VNC server.

### Phase 3 — Distribution (future)

Complete FeralOS: Dinit replaces OpenRC + custom Alpine-style repo for FeralOS services + Nix for glibc software + full desktop/server capabilities.

Ref: `PROJECT.md#Solution` + `ROADMAP.md#Phases` + `AGENTS.md#Roadmap`.

## Out of Scope (v1)

- Web UI (v2)
- Disk encryption beyond LUKS2 root/swap (v2 scope already covered; general encryption extensions v2)
- Secure Boot / ARM64 (v2)
- Custom repos (Phase 3)
- Dinit (Phase 3)

Ref: `PROJECT.md#Out of Scope (v1)`.

## Metrics

| Metric | Target | How |
|--------|--------|-----|
| Install time | < 10 min | Parallel operations |
| Disk minimum | 2× 8GB | Enforced in layout (`getMinDiskSize`) |
| Boot redundancy | EFI on 2 disks | APK hook + EFI Sync (`rsync /boot/efi → /boot/efi2`) |
| Data safety | RAID1 + Btrfs | Btrfs RAID1 on LUKS + subvolume layout (`zstd:3, autodefrag, ssd`) |
| Verification | Fixtures + harness | 14 `tests/fixtures/variables/` templates + QEMU harness |

Ref: `PROJECT.md#Metrics` + `CONTEXT.md#Boot & Redundancy` + `CONTEXT.md#File Locations`.

## References

- `PROJECT.md#Users`, `#Problem`, `#Solution`, `#Metrics`, `#Out of Scope`
- `CONTEXT.md#Project Vision` + `#Core Terms`
- `AGENTS.md#Roadmap` + `AGENTS.md#Architecture Principle`
- `ROADMAP.md`, `docs/PROJECT.md`
