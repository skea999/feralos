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

Source of truth: `AGENTS.md` and `docs/INSTALLER.md`.
