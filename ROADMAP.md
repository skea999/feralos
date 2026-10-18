# Roadmap

**Status**: Active Development
**Distribution**: FeralOS (based on Alpine Linux)
**Updated**: 2026-10-18

## Phases

### Phase 1 — Installer ✅
Standard, hackable Go installer for Alpine Linux with RAID + Btrfs. All four
layouts Working; harness-verified. Refining for real hardware continues.

### Phase 2 — Desktop ✅
Desktop environments Working: **niriWayle** (flagship — niri + Wayle shell +
Veila + Raffi + oo7 secrets), niri, KDE Plasma, COSMIC. Remote desktop via
wayvnc (loopback). Refining continues alongside real-hardware testing.

### Phase 3 — Distribution 🔨 IN PROGRESS
Transform the installer output into a complete FeralOS distribution with Dinit
and custom repositories.

---

## Phase 1 Milestones

| Ver | Target | Status | What |
|-----|--------|--------|------|
| v0.1 | — | Done | Shell script installer working |
| v0.2 | — | Done | Go installer: btrfsStandardServer (2-disk RAID1) |
| v0.3 | — | Done | Go installer: btrfsStandard (single disk) |
| v0.4 | — | Done | Generator (interactive + standalone) |
| v0.5 | — | Done | Test suite: harness (QEMU) + debug_run |
| v0.6 | — | Done | Nix integration |
| v0.7 | — | Done | Go installer: btrfsRaid0 (2-disk RAID0) |
| v0.8 | — | Done | Go installer: btrfsStandardUnencrypted (VM layout, no LUKS) |
| v0.9 | — | Done | swapSize refactor, secureboot/TPM firstboot binaries, podman/webmin |
| v1.0 | — | In progress | Refining: real-hardware rough edges |

## Phase 2 Milestones

| Ver | Target | Status | What |
|-----|--------|--------|------|
| v2.0 | — | Done | niri Wayland compositor (+ DMS native apk, firewalld, wayvnc) |
| v2.1 | — | Done | KDE Plasma |
| v2.2 | — | Done | COSMIC (seatd-based) |
| v2.3 | — | Done | English-only + anonymization policy; content audit procedure |
| v2.4 | — | Done | niriWayle flagship desktop (Wayle shell + Veila + Raffi + oo7 secrets — login auto-unlock; harness 100/100) |

## Phase 3 Milestones

| Ver | Target | Status | What |
|-----|--------|--------|------|
| v3.1 | — | In progress | Custom FeralOS repository — **LIVE**: [feralos-aports](https://github.com/skea999/feralos-aports) (signed repo on GitHub Pages, CI client-verified; rule 0: Alpine packages first) |
| v3.0 | — | In progress | Dinit init system — boot suite (`dinit-chimera`) + full `-dinit` service suite packaged; installer `enableDinit` toggle shipped (OpenRC co-installed as fallback) |
| v3.2 | — | Planned | Full FeralOS distribution release (Dinit default) |

See `docs/dinit/` (installer-side) and [feralos-aports](https://github.com/skea999/feralos-aports) `PLAN.md`/`STATUS.md` for the step-by-step Dinit plan.

---

## Phase 1 Pipeline

| Feature | Prio | Status | Notes |
|---------|------|--------|-------|
| Go installer — btrfsStandardServer | P0 | Done | 2-disk RAID1 layout |
| Go installer — btrfsStandard | P0 | Done | Single disk, no RAID |
| Go installer — btrfsRaid0 | P0 | Done | 2-disk RAID0 layout (LUKS swap partitions) |
| Go installer — btrfsStandardUnencrypted | P0 | Done | Single disk, no LUKS (for VMs) |
| Secure Boot (signed UKI) | P0 | Done | efi-mkuki + firstboot binary |
| TPM auto-unlock | P0 | Done | clevis + firstboot binary |
| swap (swapfile / LUKS partitions) + zswap | P1 | Done | swapSize auto = 2×RAM |
| Nix integration | P1 | Done | nix.go, subvolumes, daemon config |
| Podman + Webmin | P1 | Done | btrfs driver, web admin UI (firstboot binary) |
| Incus | P1 | Done | incus-feature chain (containers + VMs) |
| Test suite | P2 | Done | harness (QEMU) up to 100 checks/fixture + debug_run |
| Real-hardware refining | P2 | In progress | rough edges blocking bare-metal installs |

## Phase 2 Pipeline

| Feature | Prio | Status | Notes |
|---------|------|--------|-------|
| niri Wayland compositor | P0 | Done | greetd + regreet + DMS (native apk) |
| KDE Plasma | P0 | Done | sddm |
| COSMIC | P0 | Done | seatd (not elogind) + cosmic-greeter |
| niriWayle flagship desktop | P0 | Done | Wayle shell + Veila lock + Raffi + oo7 secrets (login auto-unlock; harness 100/100) |
| wayvnc remote desktop | P1 | Done | wayvnc-user-dinit, loopback 127.0.0.1:5900 (RustDesk removed) |
| Unicode rc.conf for all DEs | P1 | Done | desktops.go (musl default off) |

## Phase 3 Pipeline

| Feature | Prio | Status | Notes |
|---------|------|--------|-------|
| Custom FeralOS repository | P0 | **Done** | feralos-aports: abuild → signed APKINDEX → GitHub Pages; rule 0 |
| dinit-chimera package | P0 | **Done** | 0.99.24-r0 published (52 services + compiled helpers) |
| dinit hooks (devd/cryptdisks) + getty | P0 | Done | getty-dinit + busybox-loadkmap-dinit + dinit-chimera boot-log shipped; all fixtures boot green |
| Boot test QEMU | P0 | Done | booster; all layouts/fixtures boot validated |
| System services (`*-dinit`) | P1 | Done | 48 packages (dbus, sshd, chronyd, cronie, …) aligned to Artix/Chimera (ADR-009) |
| Desktop under Dinit | P1 | Done | niri/niriWayle/kde/cosmic matrices green; turnstile user sessions |
| Installer `enableDinit` toggle | P1 | Done | repo key + packages + cmdline init= (template default) |
| Dinit as default init | P2 | Pending | only after full matrix green |

---

## Timeline

*Schedule TBD — driven by contribution velocity.*

---

**Next review**: before the v3.2 full-release / Dinit-as-default decision.
