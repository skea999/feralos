# Overview

## Project

FeralOS — Alpine-based distribution. Goals: memory safety + security. Nix for glibc packages. Dinit replaces OpenRC later. Custom repo. Installer Phase 1.

## Stack

- Go 1.22 (installer)
- Shell (legacy reference)
- YAML config (`variables.yaml` + `internal_rules.yaml`)

## Architecture Principle

Standard core, pluggable data — installer logic never changes, only data changes. Storage layouts are Go packages. Essential packages hardcoded. Optional + rules in YAML. Parser same code, only data changes. Booster replaces mkinitfs. UKI replaces GRUB. LUKS2 on RAID/Swap. Btrfs RAID1 on LUKS. No backward compatibility — installer runs once, always fresh.

Ref: `AGENTS.md#Architecture Principle`.

## Project Structure

Brief tree — full tree in `docs/INSTALLER.md#Project Structure`:

```text
feralos_installer/
├── src/
│   ├── main.go, go.mod, variables.yaml
│   ├── variables/ (variables.yaml, internal_rules.yaml)
│   ├── utils/
│   │   ├── config/config.go
│   │   ├── shell/execute.go
│   │   ├── logic/logic.go
│   │   ├── swap/swap.go
│   │   └── createConfig/
│   ├── hostPhase/
│   │   ├── host_phase.go
│   │   ├── internal/ (bootstrap.go, bind_mounts.go, chroot_env.go, cleanup.go, hooks.go)
│   │   ├── features/ (install_config.go, secureboot.go, tpm.go)
│   │   └── disks/ (btrfsStandard/, btrfsRaid0/, btrfsStandardServer/, btrfsUnencrypted/)
│   ├── firstBoot/ (secureboot/main.go, tpm/main.go)
│   └── chrootPhase/
│       ├── chroot_phase.go
│       ├── internal/ (packages.go, boot.go, storage.go, uki.go, ...)
│       ├── features/ (hardening.go, zswap.go, nix.go, ...)
│       └── desktops/ (desktops.go)
├── tests/
│   ├── fixtures/variables/ (30 fixtures, full template mirror)
│   ├── harness/ (cmd/skea_harness, internal/)
│   └── artifacts/ (gitignored)
├── docs/ (PROJECT.md, INSTALLER.md, guides/, architecture/, adr/, harness/, intent/, ideas/)
├── state/session_state.json
└── handoff/latest.md
```

## Key Decisions

- `hostPhase` writes files (no chroot), `chrootPhase` runs commands inside chroot
- Module isolation, data-driven YAML/Go
- Security first: LUKS2, Secure Boot, AppArmor
