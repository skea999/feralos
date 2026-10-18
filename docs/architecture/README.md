# Architecture

Index of FeralOS installer architecture docs.

| Doc | Description |
|-----|-------------|
| [overview.md](overview.md) | Project overview, stack, principles, project structure |
| [isolation.md](isolation.md) | Entry point and phase isolation — `main.go` delegation, `internal` package rule, `hostPhase.Run` order |
| [logging.md](logging.md) | Logging rule — `shell.ExecuteCmd` requirement, `os`/`exec` ban, host vs chroot usage |
| [feature_confinement.md](feature_confinement.md) | Activatable feature confinement — single-owner file rule, pattern, confinement map |
| [storage.md](storage.md) | Disk layouts — btrfsStandard, btrfsStandardServer, btrfsRaid0, LUKS2, Btrfs RAID1, swap handling |
| [firstboot.md](firstboot.md) | FirstBoot architecture — `feralos_secureboot`/`feralos_tpm` autonomous binaries, 3-boot harness flow |
| [dbus-activation.md](dbus-activation.md) | D-Bus activation → dinit services (ADR-010) — rules R1/R2, the launch-helper gate, override locations, checklist for new services, diagnostics |
| [boot-oneshots.md](boot-oneshots.md) | Boot-time oneshot services (both inits) — file layout, ordering, enablement, materializer recipe, gotchas |

Source of truth: `AGENTS.md` and `docs/INSTALLER.md`.
