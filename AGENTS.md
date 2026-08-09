# Project Context

**Stack**: Go 1.22 (installer) + Shell (legacy reference) + YAML config
**Project**: FeralOS — new Linux distribution based on Alpine Linux
**Focus**: Memory safety, security, modern components
**Status**: Installer development (Phase 1)

## Distribution Vision

FeralOS is a new Linux distribution based on Alpine Linux, focused on memory safety and security. It replaces traditional C components with modern alternatives (uutils, micro, doas). For software not in Alpine repos or requiring glibc, **Nix** is used. After the installer is stable, **Dinit** will replace OpenRC, and a **custom repository** will be created for FeralOS services.

## Workflow

**Chain** (complex): context/recall → interview/refine → @planner → approve → @codebase → PARALLEL(@review+@code-reviewer+@vulnerability-scan+@information-scan) → @debugger/tests → @docs/docs → save.

**Direct** (simple): @codebase or specialist direct.

State: `state/session-state.json`.

### Installer Development Workflow

1. **Understand Requirements**: Read CONTEXT.md, ROADMAP.md, and reference shell scripts
2. **Plan Implementation**: Break down into hostPhase/chrootPhase modules
3. **Implement Module**: Write Go code following naming conventions
4. **Test Locally**: Build and test on live environment
5. **Test with Harness**: Use QEMU-based harness for automated testing
6. **Review**: Code review, security scan, documentation update
7. **Commit**: Conventional commit format

### Key Decisions

- **Phase Separation**: hostPhase (writes files) vs chrootPhase (runs commands)
- **Module Isolation**: Each functionality is a separate module
- **Data-Driven**: Installer logic never changes, only YAML/Go layouts change
- **Security First**: LUKS2, Secure Boot, AppArmor, sysctl hardening

## Session

```powershell
# Start
.agents\skills\session-memory\scripts\memory.ps1 context
.agents\skills\session-memory\scripts\memory.ps1 recall "project" --limit 10
.agents\skills\session-memory\scripts\memory.ps1 stats

# End
.agents\skills\session-memory\scripts\memory.ps1 save "summary" "Today: X, next: Y" -Tags @("installer","go","feralos")
```

### Final verification

```powershell
.agents\skills\session-memory\scripts\memory.ps1 context --days 14
.agents\skills\session-memory\scripts\memory.ps1 stats
.agents\skills\session-memory\scripts\memory.ps1 recall "project" --limit 5
```

## Git hooks (first session only)

```bash
git config core.hooksPath .githooks && chmod +x .githooks/pre-commit
```

Pre-commit: secrets scan + lint-staged (post-stack).

## Build & Test

```bash
# Build Go installer
cd src && go build -o ../feralos_installer .

# Build Go generator
cd src/utils/createConfig/createValues && go build -o ../../../../feralos_create_config .

# Build first-boot programs
cd src && go build -o ../feralos_secureboot ./firstBoot/secureboot/
cd src && go build -o ../feralos_tpm ./firstBoot/tpm/

# Build harness
cd tests/harness && go build -o skea-harness ./cmd/skea-harness/

# Run installer (requires root + live environment)
sudo ./feralos_installer -config variables.yaml

# Run generator (interactive or standalone)
./feralos_create_config

# Run harness tests
cd tests/harness
./skea-harness prepare-runner
./skea-harness run -f ~/Progetti/skeaAlpineInstaller/src/variables/variables.yaml
./skea-harness status
./skea-harness cleanup

# Run tests (when available)
cd src && go test ./...
```

## Test Artifacts

Output from test runs goes to `tests/artifacts/<tool-name>/<timestamp>/`:

```
tests/artifacts/
├── feralos_create_config/     # Config generator test output
│   └── .gitkeep
└── harness/                   # QEMU harness artifacts
    ├── cache/
    │   ├── base-images/       # Debian cloud images
    │   ├── keys/              # SSH key pairs
    │   ├── runner-images/     # Prepared runner qcow2
    │   └── web/alpine/        # Alpine package cache
    ├── prepare/               # Runner preparation logs
    ├── runs/                  # Per-install run dirs (logs + disks)
    └── state/                 # Active run metadata (active.json)
```

**Rules:**
- All artifacts gitignored except `.gitkeep` (preserves directory structure)
- Never commit test outputs to repository
- Harness auto-cleans old runs, keeps last 2

## Lint (WSL / Native)

Development happens on Windows but lint/build runs on Linux. Use WSL on Windows, native on Linux.

### WSL Setup (one-time)

```bash
wsl -d Alpine -u root -- sh -c "apk add --no-cache go golangci-lint"
```

### Run Lint

**Windows (WSL):**
Convert the project path from Windows to WSL format (`C:\` → `/mnt/c/`, `\` → `/`), then:
```powershell
wsl -d Alpine -u root -- sh -c "cd <project-path>/src && GOFLAGS=-buildvcs=false golangci-lint run ./..."
```

**Linux (native):**
```bash
cd src && GOFLAGS=-buildvcs=false golangci-lint run ./...
```

### Naming Conventions

| Element | Convention | Example |
|---------|------------|---------|
| Files | snake_case | `bind_mounts.go`, `config.go` |
| Folders | camelCase | `hostPhase/`, `chrootPhase/` |
| Variables | camelCase | `targetDisk`, `minSizeBytes` |
| Functions | camelCase (unexported), PascalCase (exported) | `getMinDiskSize()`, `RunBtrfsSetup()` |
| Types | PascalCase | `Config`, `Subvolume` |
| Constants | UPPER_SNAKE | — |

## Architecture Principle

**Standard core, pluggable data.**

- Installer logic never changes
- Storage layouts are Go packages
- Essential packages hardcoded in Go
- Optional packages + rules in YAML
- Parser always same code, only data changes
- booster replaces mkinitfs for initramfs
- Signed UKI replaces GRUB for boot
- LUKS2 encryption on RAID and Swap partitions
- Btrfs RAID1 on LUKS (not mdadm)

## Agents

### User-created
- `researcher`
- `information-scan`
- `vulnerability-scan`

## Skills

- `.opencode/skills/` — agents-opencode pkg
- `.agents/skills/` — skills.sh CLI

### User-modified (no auto-update)
- `session-memory`
- `using-agent-skills`

### User-created
None.

External skills:
- `customize-opencode` → built-in

## Docs

- `docs/PROJECT.md` — distribution vision, architecture, milestones
- `docs/INSTALLER.md` — installer architecture and components
- `docs/USAGE.md` — usage guide and configuration
- `CONTEXT.md` — domain vocabulary (memory safety, Nix, Dinit)
- `ROADMAP.md` — milestones and pipeline
- `CHANGELOG.md` — version history

## Project Structure

```
feralos_installer/
├── src/                          # Go installer (primary)
│   ├── main.go                   # Installer entry point
│   ├── go.mod                    # Go module definition
│   ├── variables.yaml            # User config (runtime)
│   ├── variables/
│   │   ├── variables.yaml        # Template with defaults
│   │   └── internalRules.yaml    # Dynamic rules
│   ├── utils/                    # Shared utilities (sub-packages)
│   │   ├── config/               # Config types + LoadConfig
│   │   │   └── config.go
│   │   ├── shell/                # Command execution (RunCommand, RunChrootCommand)
│   │   │   └── shell.go
│   │   ├── files/                # File operations (WriteFileInChroot)
│   │   │   └── files.go
│   │   ├── logic/                # Dynamic rule application
│   │   │   └── logic.go
│   │   └── createConfig/         # Configuration wizard + generator
│   │       ├── create_values.go  # Interactive wizard (PromptForConfig)
│   │       └── createValues/
│   │           └── main.go       # Standalone generator
│   ├── hostPhase/            # Host-side operations (writes files, NO chroot commands)
│   │   ├── disks/            # Storage layouts (Go packages)
│   │   │   ├── btrfsStandardServer/
│   │   │   │   ├── partitioning.go
│   │   │   │   ├── fstab.go
│   │   │   │   └── luks.go
│   │   │   └── btrfsStandard/
│   │   │       ├── partitioning.go
│   │   │       └── fstab.go
│   │   ├── booster.go        # booster.yaml + kernel cmdline + hooks
│   │   ├── bootstrap.go
│   │   ├── bind_mounts.go
│   │   ├── chroot_preparation.go
│   │   └── hooks.go
│   ├── firstBoot/            # Standalone first-boot programs (Go)
│   │   ├── secureboot/       # Secure Boot enrollment via sbctl
│   │   │   └── main.go
│   │   └── tpm/              # Clevis TPM2 binding for LUKS auto-unlock
│   │       └── main.go
│   └── chrootPhase/          # Chroot operations (runs INSIDE chroot)
│       ├── configurator.go   # Run() principal
│       ├── packages.go       # apk add, TPM packages
│       ├── services.go       # OpenRC, aliases, symlinks, timezone
│       ├── users.go          # User creation, groups, SSH, shell
│       ├── dmcrypt.go        # LUKS swap keyfile + luksAddKey
│       ├── hardening.go      # AppArmor, sysctl, chmod, sed
│       ├── uki.go            # booster build, efi-mkuki, sbsign
│       ├── zswap.go          # zswap compressed swap (optional)
│       └── utils.go          # getPartitionSpacer, getDiskSuffix
├── internal/                     # Shell scripts (reference)
├── docs/
├── state/
├── handoff/
└── tests/
```

## Roadmap

| Phase | Milestone | Status |
|-------|-----------|--------|
| Phase 1 | Go installer: btrfsStandardServer | Completed |
| Phase 1 | Go installer: btrfsStandard | Completed |
| Phase 1 | Generator (interactive + standalone) | Completed |
| Phase 1 | Test suite + CI | In Progress |
| Phase 2 | Dinit init system | Planned |
| Phase 2 | Custom FeralOS repository | Planned |
| Phase 2 | Nix integration | Planned |
| Phase 2 | Full distribution release | Planned |

## Memory-Safe Alternatives

| Traditional (C) | FeralOS Alternative | Language |
|-----------------|------------------|----------|
| coreutils | uutils-coreutils | Rust |
| vim/nano | micro | Go |
| sudo | sudo-rs | Rust |
| rsyslog/logrotate | vector | Rust |
| bash/zsh | fish | Rust |
| lxd | incus | Go |
| openrc | dinit (future) | C++ |
| grep (GNU) | ripgrep (optional) | Rust |
| find (GNU) | fd (optional) | Rust |

## Known Issues

| Issue | Status | Notes |
|-------|--------|-------|
| Vector crashes silently | Open | Vector 0.55.0 validates config OK but crashes at runtime (empty log file, respawns 5x then dies). Likely socket source `/dev/log` issue on Alpine mdev. Needs investigation. |

---

*Updated: 2026-08-07*

### 2026-08-07 00:15 - Vector 0.55.0 Config Fix + Investigation
**Agent:** codebase
**Summary:** Fixed Vector VRL error, but Vector crashes silently at runtime
- Fixed `error[E103]: unhandled fallible assignment` in syslog_parser remap transform (added `?? {}` fallback)
- Vector now validates config OK and OpenRC starts it, but crashes after startup
- supervise-daemon respawns 5x then gives up, vector.log is EMPTY
- Root cause unknown: likely `/dev/log` socket issue on Alpine mdev or permissions
- All other 18/20 checks pass (packages, btrbk, storage, boot, etc.)
- Added `vector_diag` harness check for debugging
- **TODO**: Investigate vector crash — needs `--keep-target-running` debug session

### 2026-08-06 18:15 - Harness Refactoring: -f variables.yaml + efivars
**Agent:** codebase
**Summary:** Harness now accepts -f variables.yaml, efivars working
- Added `-f` flag to `run` command to load variables.yaml (same as installer)
- Harness binary built to `tests/harness/skea-harness` (not /tmp)
- Added yaml.v3 dependency to harness go.mod
- Secureboot binary handles missing efivars gracefully
- btrfsStandard: 18/18 with ALL features (Secure Boot + TPM + Nix + Zswap + Hardening)

### 2026-08-06 02:15 - Test Artifacts Cleanup + Gitignore
**Agent:** codebase
**Summary:** Cleaned test artifacts structure, fixed gitignore for .gitkeep
- Cleaned all files from tests/artifacts/ (qcow2, logs, keys, web cache)
- 14 .gitkeep files preserve directory structure
- .gitignore: `tests/artifacts/**/*` ignored, `!tests/artifacts/**/.gitkeep` kept
- Removed unused cache/state and cache/prepare dirs (not referenced in Go code)
- Updated AGENTS.md and harness README with correct artifact paths
- Both layouts pass 17/17 with ALL features

### 2026-08-06 19:10 - First Boot Programs: Bash to Go
**Agent:** codebase
**Summary:** Converted bash scripts to standalone Go programs
- `src/firstBoot/secureboot/main.go` — Secure Boot enrollment via sbctl
- `src/firstBoot/tpm/main.go` — Clevis TPM2 binding for LUKS auto-unlock
- Both build successfully (2.5MB + 3.0MB)
- Removed old `1-secure-boot-sbctl.sh` and `2-tpm.sh`
- Updated AGENTS.md project structure and build commands

### 2026-08-05 03:15 - Strict TPM Verification + Layout-Aware Disks
**Agent:** codebase
**Summary:** Harness now verifies TPM auto-unlock without password fallback; creates only needed disks
- Made `tpmBindingSuccessful` true after 2nd-boot Clevis binding succeeds → verification boot has NO password fallback
- `tpm_auto_unlock` check now FAILS if "Enter passphrase" appears in serial log after Clevis reboot
- Harness creates only 1 install disk for btrfsStandard, 2 for btrfsStandardServer (was always 2)
- `writeHarnessSetupInput` passes `/dev/vdc` only for btrfsStandardServer
- QEMU args (runner + target) conditional on layout
- btrfsStandard: 18/18 with Secure Boot + TPM (TPM auto-unlock confirmed, no password)
- btrfsStandardServer: TPM auto-unlock check fails — 2-device RAID1 doesn't auto-unlock (PCR7 mismatch or booster/clevis path issue), needs investigation

### 2026-08-05 02:00 - TPM Auto-Unlock + Secure Boot Enrollment
**Agent:** codebase
**Summary:** Both layouts pass all 18 checks with Secure Boot + TPM enabled
- Added `src/firstBoot/1-secure-boot-sbctl.sh` (sbctl enrollment) and `src/firstBoot/2-tpm.sh` (Clevis TPM2 binding)
- `2-tpm.sh` installs `clevis@edge`, binds LUKS to TPM2 with `pcr_ids:7`, rebuilds initramfs + signed UKI
- Fixed Secure Boot key generation ordering: EFI keys now generated in `InstallPackages` before `secureboot-hook` post-install runs
- Updated harness `copyGuidesToTarget` to read from `src/firstBoot`
- Made `clevis_bindings` check layout-aware (1 binding for btrfsStandard, 2 for btrfsStandardServer)
- Both btrfsStandard and btrfsStandardServer pass 18/18 checks with `--enable-secureboot=true --enable-tpm=true`
- Committed `e530824`, pushed to main

### 2026-07-30 00:45 - Layout-Aware Verification Checks
**Agent:** codebase
**Summary:** Both btrfsStandard and btrfsStandardServer pass all 17 verification checks
- Added `hasHealthyBtrfsSingleLayout()` for btrfsStandard (1 LUKS device, single data, DUP metadata)
- Fixed `hasHealthyBtrfsRaid1Layout()` mount regex to accept `/dev/dm-*` (not just `/dev/mapper/luks-*`)
- Fixed mapper section extraction — was counting `/dev/mapper/luks-*` from btrfs_show section, causing false "2 devices" on single-disk layout
- Accept 1+ `rd.luks.name` entries in `kernel_cmdline` check (btrfsStandard has 1, btrfsStandardServer has 2)
- Added `layout_test.go` unit test with real verify output
- Committed `fcea001`, pushed to main

### 2026-07-29 21:06 - Full Harness Test Pass (15/15 checks)
**Agent:** codebase
**Summary:** Fixed networking, SSH key delivery, and post-install — all verification checks pass
- **Root cause 1**: No `/etc/network/interfaces` → networking service fails → no IP → SSH unreachable
- **Fix 1**: Added `DefaultNetworkInterface` to config + variables.yaml, created `/etc/network/interfaces` in `services.go`
- **Root cause 2**: SSH key empty in `setup-input.txt` → `authorized_keys` never written
- **Fix 2**: Pass `keyPair.PublicKey` instead of `""` in `writeHarnessSetupInput`
- **Root cause 3**: `setup.sh` ignores `SKEA_RUNTIME_OVERRIDES_FILE` → harness overrides lost
- **Fix 3**: `setup.sh` now sources `.skea-harness-overrides.sh` when present
- **Root cause 4**: Post-install script runs missing files (`debug.sh`, guides) unconditionally
- **Fix 4**: Fixed `debug.sh` path to `tests/harness/debug.sh`, added existence checks before running
- All 15 verification checks pass: booster, fstab, hostname, incus, user, kernel cmdline, network, secureboot config, sshd, storage layout RAID1+LUKS2+btrfs, subvolumes, tpm, UKI artifacts

### 2026-08-07 00:15 - Vector 0.55.0 Config Fix + Investigation
**Agent:** codebase
**Summary:** Fixed Vector VRL error, but Vector crashes silently at runtime
- Fixed `error[E103]: unhandled fallible assignment` in syslog_parser remap transform (added `?? {}` fallback)
- Vector now validates config OK and OpenRC starts it, but crashes after startup
- supervise-daemon respawns 5x then gives up, vector.log is EMPTY
- Root cause unknown: likely `/dev/log` socket issue on Alpine mdev or permissions
- All other 18/20 checks pass (packages, btrbk, storage, boot, etc.)
- Added `vector_diag` harness check for debugging
- **TODO**: Investigate vector crash — needs `--keep-target-running` debug session

### 2026-08-07 01:30 - Alpine Default Vector Config Investigation
**Agent:** codebase
**Summary:** Alpine ships Vector with demo config, crashes at runtime
- Alpine's default `/etc/vector/vector.yaml` uses `demo_logs` source (fake syslog to console)
- Our custom config uses `socket` source on `/dev/log` which crashes Vector
- Saved custom config to `docs/vector-config-reference.yaml`
- Disabled custom config in installer for testing
- **Next step**: Debug with `--keep-target-running` session to investigate `/dev/log` socket

### 2026-08-09 12:00 - Vector, Compression, SecureBoot Fix, Docs + Push
**Agent:** codebase
**Summary:** Session work committed and pushed (eaa5dad)
- **Per-subvolume btrfs compression**: mount root without compression, `btrfs property set` zstd per data subvol; NoCoW via `chattr +C`, swapfile `chattr +m`; fstab `compress=no` for NoCoW
- **All packages single `apk add`** (base + TPM + microcode + Nix + hardening); earlyoom in standardPackages; uutils subpackages fixed (groups/more/uptime, stat removed)
- **Secure Boot fix**: EFI keys generated AFTER package install (matches reference chroot.sh); `WriteSecurebootConf` skips if db.key missing — pending harness revalidation
- **Harness**: 22/22 checks (fstab_vs_mount, vector_*, apk_packages dumps); `tests/debug-run/` boots existing run without reinstall
- **Docs**: CHANGELOG, memorySafeAlternatives (less → dummy package plan, Vector fixed), INSTALLER.md updated; session memory saved
- **Less dummy package**: implemented (v999-r0), replaces real C less with no-op stub
- **uutils-coreutils-more conflict**: removed from packages — conflicts with util-linux-misc (incus-feature-vm dependency). TODO: investigate masking util-linux-misc

### 2026-08-08 22:00 - Vector Fully Working + internalRules Cleanup
**Agent:** codebase
**Summary:** Vector now works correctly, internalRules cleaned up
- **Root cause fixed**: Vector socket on `/run/vector/syslog.sock` (not `/dev/log`)
- **vector-setup service**: boot stage, creates /run/vector, /dev/log symlink, chown /var/log
- **Vector config**: Written AFTER package install (apk overwrites with demo_logs)
- **Vector check**: `apk info | grep -q '^vector$'` (init-system agnostic)
- **internalRules.yaml cleaned**: Only base OS in defaultRunlevels, services in packageLogic
- **udev services**: Moved to packageLogic under eudev
- **modules service**: Removed (booster loads /etc/modules)
- **btrbk**: Added subvolume @btrbk, fixed snapshot_dir path
- **All 20/20 checks pass** with Vector working correctly

### 2026-08-06 18:15 - Harness Refactoring: -f variables.yaml + efivars
**Agent:** codebase
**Summary:** Harness now accepts -f variables.yaml, efivars working
- Added `-f` flag to `run` command to load variables.yaml (same as installer)
- Harness binary built to `tests/harness/skea-harness` (not /tmp)
- Added yaml.v3 dependency to harness go.mod
- Secureboot binary handles missing efivars gracefully
- btrfsStandard: 18/18 with ALL features (Secure Boot + TPM + Nix + Zswap + Hardening)

### 2026-08-06 02:15 - Test Artifacts Cleanup + Gitignore
**Agent:** codebase
**Summary:** Cleaned test artifacts structure, fixed gitignore for .gitkeep
- Cleaned all files from tests/artifacts/ (qcow2, logs, keys, web cache)
- 14 .gitkeep files preserve directory structure
- .gitignore: `tests/artifacts/**/*` ignored, `!tests/artifacts/**/.gitkeep` kept
- Removed unused cache/state and cache/prepare dirs (not referenced in Go code)
- Updated AGENTS.md and harness README with correct artifact paths
- Both layouts pass 17/17 with ALL features

### 2026-08-06 19:10 - First Boot Programs: Bash to Go
**Agent:** codebase
**Summary:** Converted bash scripts to standalone Go programs
- `src/firstBoot/secureboot/main.go` — Secure Boot enrollment via sbctl
- `src/firstBoot/tpm/main.go` — Clevis TPM2 binding for LUKS auto-unlock
- Both build successfully (2.5MB + 3.0MB)
- Removed old `1-secure-boot-sbctl.sh` and `2-tpm.sh`
- Updated AGENTS.md project structure and build commands

### 2026-08-05 03:15 - Strict TPM Verification + Layout-Aware Disks
**Agent:** codebase
**Summary:** Harness now verifies TPM auto-unlock without password fallback; creates only needed disks
- Made `tpmBindingSuccessful` true after 2nd-boot Clevis binding succeeds → verification boot has NO password fallback
- `tpm_auto_unlock` check now FAILS if "Enter passphrase" appears in serial log after Clevis reboot
- Harness creates only 1 install disk for btrfsStandard, 2 for btrfsStandardServer (was always 2)
- `writeHarnessSetupInput` passes `/dev/vdc` only for btrfsStandardServer
- QEMU args (runner + target) conditional on layout
- btrfsStandard: 18/18 with Secure Boot + TPM (TPM auto-unlock confirmed, no password)
- btrfsStandardServer: TPM auto-unlock check fails — 2-device RAID1 doesn't auto-unlock (PCR7 mismatch or booster/clevis path issue), needs investigation

### 2026-08-05 02:00 - TPM Auto-Unlock + Secure Boot Enrollment
**Agent:** codebase
**Summary:** Both layouts pass all 18 checks with Secure Boot + TPM enabled
- Added `src/firstBoot/1-secure-boot-sbctl.sh` (sbctl enrollment) and `src/firstBoot/2-tpm.sh` (Clevis TPM2 binding)
- `2-tpm.sh` installs `clevis@edge`, binds LUKS to TPM2 with `pcr_ids:7`, rebuilds initramfs + signed UKI
- Fixed Secure Boot key generation ordering: EFI keys now generated in `InstallPackages` before `secureboot-hook` post-install runs
- Updated harness `copyGuidesToTarget` to read from `src/firstBoot`
- Made `clevis_bindings` check layout-aware (1 binding for btrfsStandard, 2 for btrfsStandardServer)
- Both btrfsStandard and btrfsStandardServer pass 18/18 checks with `--enable-secureboot=true --enable-tpm=true`
- Committed `e530824`, pushed to main
