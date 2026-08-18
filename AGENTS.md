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

### Memory (session-memory skill)

Storage: `.agent-memory/` in project root. Scripts in `.agents/skills/session-memory/scripts/`.

```bash
# Start session — load recent context
.agents/skills/session-memory/scripts/memory.sh context --days 14

# Search memories
.agents/skills/session-memory/scripts/memory.sh recall "vector" --limit 5
.agents/skills/session-memory/scripts/memory.sh recall "decision" --topic decision

# Stats
.agents/skills/session-memory/scripts/memory.sh stats

# Save — during work
.agents/skills/session-memory/scripts/memory.sh save "decision" "Chose X because Y" tag1 tag2
MEMORY_IMPORTANCE=high .agents/skills/session-memory/scripts/memory.sh save "insight" "Key finding" installer

# Save — end of session
.agents/skills/session-memory/scripts/memory.sh save "summary" "Today: implemented X, decided Y. Next: Z" -Tags installer go feralos

# Consolidate (weekly review)
.agents/skills/session-memory/scripts/memory.sh consolidate --since 2026-08-01
```

**Importance levels**: `low` (search only), `normal` (default), `high` (always in context), `critical` (top priority)

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

## Lint (Native Linux)

Run lint and vet from the `src/` directory:

```bash
cd src && go vet ./...
cd src && golangci-lint run ./...
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
│   │   ├── shell/                # Command execution (ExecuteCmd)
│   │   │   └── execute.go
│   │   ├── logic/                # Dynamic rule application
│   │   │   └── logic.go
│   │   └── createConfig/         # Configuration wizard + generator
│   │       ├── create_values.go  # Interactive wizard (PromptForConfig)
│   │       └── createValues/
│   │           └── main.go       # Standalone generator
│   ├── hostPhase/            # Host-side operations (writes files, NO chroot commands)
│   │   ├── host_phase.go     # Orchestrator: MountVirtualFilesystems + BootstrapAlpine + PrepareChrootEnv + InjectApkHooks
│   │   ├── internal/         # Implementation files
│   │   │   ├── bootstrap.go  # BootstrapAlpine
│   │   │   ├── bind_mounts.go # MountVirtualFilesystems, UmountVirtualFilesystems
│   │   │   ├── chroot_env.go # PrepareChrootEnv (APK repos)
│   │   │   └── hooks.go      # InjectApkHooks (EFI sync for dual-disk)
│   │   └── disks/            # Storage layouts (Go packages)
│   │       ├── btrfsStandardServer/
│   │       │   ├── partitioning.go
│   │       │   ├── fstab.go
│   │       │   └── luks.go
│   │       └── btrfsStandard/
│   │           ├── partitioning.go
│   │           └── fstab.go
│   ├── firstBoot/            # Standalone first-boot programs (Go)
│   │   ├── secureboot/       # Secure Boot enrollment via sbctl
│   │   │   └── main.go
│   │   └── tpm/              # Clevis TPM2 binding for LUKS auto-unlock
│   │       └── main.go
│   └── chrootPhase/          # Chroot operations (runs INSIDE chroot)
│       ├── chroot_phase.go   # Run() orchestrator (12 steps)
│       ├── internal/         # Implementation files
│       │   ├── packages.go   # SetupPackages, InstallDummyPackages, FinalUpgrade
│       │   ├── firmware.go   # DetectFirmwarePackages
│       │   ├── network.go    # SetupNetwork (hostname, hosts, resolv.conf, interfaces)
│       │   ├── boot.go       # SetupBoot (booster.yaml, kernel cmdline, hooks)
│       │   ├── storage.go    # SetupStorage (btrbk, bridging sysctl)
│       │   ├── logging.go    # SetupLogging (vector config + vector-setup service)
│       │   ├── misc.go       # SetupMisc (aliases, editor, PS1, timezone, pager, subuid/subgid, limits, sudoers, LXC, mkinitfs, smartd, cron, keymap)
│       │   ├── services.go   # SetupServices (OpenRC runlevels only)
│       │   ├── users.go      # SetupUsers (user creation, groups, SSH, shell)
│       │   └── uki.go        # BuildUKI (booster build, efi-mkuki)
│       └── features/         # Activatable features (Enable* flags)
│           ├── hardening.go  # SetupHardening (AppArmor, sysctl, chmod, sed)
│           ├── zswap.go      # SetupZswap + SetupDmcryptSwap (LUKS swap + compressed swap)
│           └── nix.go        # SetupNix
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
| Phase 1 | Test suite: harness + debug-run | Completed |
| Phase 1 | Nix integration | Completed |
| Phase 1 | Go installer: btrfsRaid0 | Planned |
| Phase 1 | Go installer: btrfsStandardUnencrypted | Planned |
| Phase 1 | Refining: fix errors that block on real hardware | Planned |
| Phase 2 | niri + wayvnc | Planned |
| Phase 3 | Dinit init system | Planned |
| Phase 3 | Custom FeralOS repository | Planned |
| Phase 3 | Full distribution release | Planned |

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
| ~~Vector crashes silently~~ | Resolved | Fixed: socket on `/run/vector/syslog.sock`, `vector-setup` boot service, config written after apk install. Harness vector_* checks pass. |
| ~~Dummy packages: wrapper vs symlink~~ | Resolved | Wrapper scripts broke `efi-mkuki`. Reverted to symlink approach. `DummyPackageRule.Bin` field added for binary name override. |
| ~~Ripgrep as grep drop-in~~ | Won't Fix | rg doesn't support POSIX BRE (`\+`). `efi-mkuki` uses `grep -q '^[0-9]\+\.[0-9]...'` for kernel version validation. GNU grep removed — BusyBox grep used instead. |
| ~~Swapfile swapon Invalid argument~~ | Resolved | Root cause: `btrfs property set compression none` sets `m` attribute → conflicts with `chattr +C` (NixOS/nixpkgs #384047). Fix: `chattr +C` on dir before mount, `btrfs filesystem mkswapfile`. 29/29 checks pass, `swapon [ok]` at boot. |
| ~~Swap shutdown: /swap in use~~ | Resolved | OpenRC swap service has `before localmount` → wrong stop order (localmount unmounts /swap while swapfile active, "in use but fuser finds nothing"). Fix: swap moved to boot runlevel + `sed 's/before localmount/need localmount/' /etc/init.d/swap` in SetupServices. Safe on upgrade: apk preserves modified /etc files as `.apk-new` (verified in Alpine docs) — the sed survives. |

---

*Updated: 2026-08-18*

### 2026-08-18 18:45 - Log Error Harness Check + apk-new Verified
**Agent:** codebase
**Summary:** New `log_errors` non-regression check in harness + apk-new behavior verified (swap sed survives upgrades)
- **New harness check `log_errors`** (Verify, service.go): scans `target-serial.log` + `installer.log` for `error|failed|critical|[ !! ]|in use but fuser finds nothing|invalid argument|cannot set` (case-insensitive), fails the run on any match — non-regression guard so boot/shutdown regressions surface immediately
- **Whitelist** (innocuous): counters/negations (`0 errors`, `no failed`, `errors: 0`), package name `libgpg-error`, efibootmgr `error trace:` on QEMU (no real EFI vars — expected, `|| true`, exit 0)
- **First run**: 29/30 (5 false positives found → whitelisted), simulated re-check on same run: 0 matches, harness build clean
- **apk-new verified** (Alpine docs): "apk will avoid overwriting files you may have changed... writes its file to that filename with `.apk-new` appended" — `/etc/init.d/swap` is protected, the sed survives openrc upgrades, no re-apply needed (corrected earlier claim that the sed was lost)

### 2026-08-18 17:30 - Swap Service Ordering Fixed: before → need localmount
**Agent:** codebase
**Summary:** Shutdown "Unmounting /swap ... in use but fuser finds nothing" fixed — swap now in boot runlevel with hard dependency on localmount
- **Root cause found**: OpenRC upstream `swap.initd` has `before localmount` — `before` guarantees start order only, NOT stop order (OpenRC service-script-guide: "on calling stop the two are independent"). At shutdown localmount unmounted /swap while swapfile still active; fuser can't see swapfiles (not processes)
- **Runlevel analysis**: localmount in boot, swap in default — moving swap to boot alone would BREAK start (swap would run before localmount, swapon -a fails silently via `eend 0`). Verified btrfs-scan (`before localmount`) has no broken dependency: chain btrfs-scan → localmount → swap is correct
- **Fix**: swap moved to boot runlevel (internalRules.yaml) + `sed -i 's/before localmount/need localmount/' /etc/init.d/swap` in SetupServices — `need` = hard dependency: start after localmount, stop BEFORE localmount (swapoff before umount /swap)
- **Harness**: 29/29 checks pass, boot order `Mounting local filesystems [ ok ]` → `Activating swap devices [ ok ]`, shutdown order `Deactivating swap devices [ ok ]` → `Unmounting /swap [ ok ]`, zero "in use but fuser finds nothing"
- **Known risk**: apk upgrade preserves modified /etc files as `.apk-new` (verified in Alpine docs) — the sed survives upgrades, no re-apply needed

### 2026-08-18 16:00 - Swapfile Fixed: m Attribute Conflict + NoCoW Cleanup
**Agent:** codebase
**Summary:** btrfs swapfile works + compression property removed from ALL NoCoW subvolumes
- **Root cause found**: `btrfs property set compression none` sets the `m` attribute ("excluded from compression") on every file. `chattr +C` (NODATACOW) fails on files with `m` → `btrfs filesystem mkswapfile` aborts with `ERROR: cannot set NOCOW flag: Invalid argument`. Documented in NixOS/nixpkgs issue #384047 (identical scenario)
- **Swapfile fix (Debian guide)**: `chattr +C` on the @swap directory BEFORE mount (files inherit NODATACOW), skip compression property, then `btrfs filesystem mkswapfile --size 8g`
- **NoCoW cleanup (Option A)**: compression property removed from ALL NoCoW subvolumes — NODATACOW implies no compression (btrfs docs), property was redundant AND harmful (`m` blocks `chattr +C` on existing files). `NoCompressionProp` field removed entirely
- **New harness check `lsattr_nocow`**: verifies all 7 NoCoW subvolumes (dir + first file) have `C` and no `m`; verifies 19 compressed subvolumes stay CoW (no `C`)
- **Harness check fix**: `swap_status` used `swapon --show` (util-linux flag) — busybox swapon only has `-s`; now `swapon -s || cat /proc/swaps`
- **Harness**: 29/29 checks pass, `swapon [ok]` at boot, swapfile active (8388604 KB), `lsattr` shows `C` on dir + swapfile
- **btrfsStandardServer**: no swapfile needed — dedicated LUKS swap partitions (`mkswap` on partitions, not files)

### 2026-08-17 23:30 - Dummy Packages Investigation: Symlink vs Wrapper
**Agent:** codebase
**Summary:** Investigated wrapper scripts for dummy packages, reverted to symlinks, found ripgrep incompatibility
- **Wrapper scripts tested**: Created wrapper scripts (`#!/bin/sh\nexec rg "$@"`) to replace symlinks — broke `efi-mkuki`
- **Root cause found**: `efi-mkuki` is a POSIX shell script that uses `grep` with POSIX BRE (`\+`). `rg` (Rust regex) doesn't support `\+` → `invalid kernel version` error
- **Aliases removed**: Shell aliases (ll, nano, rm -i, apk --progress, untar, mv -v) removed from `misc.go`
- **Dummy packages**: Reverted to symlink approach (APK content, not wrapper scripts)
- **Bin field added**: `DummyPackageRule.Bin` allows overriding replacement binary name (e.g., `rg` for `ripgrep`)
- **Ripgrep NOT a drop-in**: rg doesn't support POSIX BRE (`\+`), `-E`/`-G` flags, `--include`/`--exclude` (uses `--glob` instead)
- **GNU grep removed**: Zero reverse dependencies. BusyBox grep provides POSIX BRE (supports `\+`). Saves package space.
- **Memory-safe audit**: 480 packages installed, ~13% memory-safe. Most C packages are Alpine base (musl, busybox, kernel) with no memory-safe alternatives available.
- **Harness**: 27/27 checks pass
- **Build**: All 5 binaries compile, go vet clean

### 2026-08-14 10:00 - Folder Reorganization + Lint + Fixes
**Agent:** codebase
**Summary:** Final folder reorganization, lint clean, bash removal, incus-admin fix, firmware silent
- **Folder structure**: `hostPhase/internal/` (bootstrap, bind_mounts, chroot_env, hooks), `chrootPhase/internal/` (packages, firmware, network, boot, storage, logging, misc, services, users, uki), `chrootPhase/features/` (hardening, zswap, nix)
- **`hostPhase/disks/`** stays in root — Go internal package rule prevents moving to internal/ (main.go imports disk layouts)
- **`utils/shell/silent.go`**: New `ExecSilent(chroot, cmd)` for command execution without logging
- **Firmware detection**: Uses `ExecSilent` to suppress verbose readlink/ls output
- **Lint fully clean**: go vet + golangci-lint zero errors (errcheck, gofmt, gosec, gocritic, prealloc, errorlint, errorlint, gosimple, govet shadow, unparam, unused)
- **Removed all bash aliases**: Fish-only environment (bash never used)
- **incus-admin group fix**: `ensureIncusAdminGroup` creates group before adding user, exact string match
- **Log formatting**: `\n` before commands, `\n\n` after results for readability
- **Build**: All 5 binaries compile, go vet clean, golangci-lint clean
- **Harness**: 25/25 checks pass with all features enabled

### 2026-08-13 16:00 - Rev. 8 Reorganization: Orchestrators + Modular Files
**Agent:** codebase
**Summary:** Major repository reorganization — both phases get orchestrators, main.go simplified to 2 calls
- **New chrootPhase files**: `boot.go` (booster/cmdline/hooks), `storage.go` (btrbk/bridging), `logging.go` (vector/vector-setup), `misc.go` (aliases/editor/PS1/timezone/pager/subuid/subgid/limits/sudoers/LXC/mkinitfs/smartd/cron/keymap), `network.go` (hostname/hosts/resolv.conf/interfaces)
- **New orchestrators**: `hostPhase/host_phase.go` (MountVirtualFilesystems + BootstrapAlpine + PrepareChrootEnv + InjectApkHooks), `chrootPhase/chroot_phase.go` (Run with 14 steps)
- **Simplified main.go**: Only 2 calls: `hostPhase.Run(conf)` + `chrootPhase.Run(conf)`
- **Reduced services.go**: Only OpenRC runlevels (all other config moved to misc.go/network.go)
- **Merged dmcrypt.go → zswap.go**: Both LUKS swap keyfile and zswap compressed swap in one file; local `getPartitionSpacer`/`getDiskSuffix` (deleted `chrootPhase/utils.go`)
- **Renamed functions**: `InstallPackages` → `SetupPackages`, `BuildAndDeployUKI` → `BuildUKI`, `WriteHardening` → `SetupHardening`
- **Removed enableEncryption**: Was defined but never checked (LUKS always applied) — bug fix, separate from reorg
- **Deleted 7 old files**: `hostPhase/chroot_preparation.go`, `hostPhase/keyfile.go`, `hostPhase/booster.go`, `hostPhase/hooks.go`, `chrootPhase/utils.go`, `chrootPhase/dmcrypt.go`, `chrootPhase/configurator.go`
- **Build**: All 5 binaries compile (installer + secureboot + tpm + generator), go vet clean

### 2026-08-12 12:00 - Code Review + packageLogic Modules + incus Chain
**Agent:** codebase
**Summary:** Hardcoded modules moved to packageLogic, incus dependency chain implemented
- **Removed additional modules from services.go** (16 modules, booster doesn't use /etc/modules)
- **Removed group creation before user add** in users.go (packages create groups)
- **Moved qemu/kvm groups** from hardcoded extraGroups to internalRules.yaml (packageLogic.qemu-system-x86_64)
- **Modules to packageLogic**: 8021q→vlan, bridge/br_netfilter→bridge, zstd_compress→zstd, overlay→incus/podman/docker
- **baseModules** in internalRules.yaml: r8169, bonding, binfmt_misc, zsmalloc (kernel modules without packages)
- **booster.go**: Removed boosterDefaultModules map, uses u.Modules for modules_force_load
- **incus chain**: incus-feature → packages: [incus-feature-vm, incus-feature-client, incus-feature-utils]; incus-feature-vm → packages: [qemu-system-x86_64]
- **Fixed kvm module**: kvm-amd/kvm-intel → kvm (Alpine kernel)
- **Harness**: 25/25 checks pass

### 2026-08-12 10:30 - Unified Command Execution: ExecuteCmd Migration
**Agent:** codebase
**Summary:** Replaced RunCommand/RunChrootCommand/WriteFileInChroot with unified ExecuteCmd
- **Deleted**: `utils/shell/shell.go`, `utils/files/files.go` (no backward compatibility)
- **Created**: `utils/shell/execute.go` — `ExecuteCmd(chroot, cmd)` with retry 3x, user prompt, logging
- **Migrated**: ~152 calls across 22+ files (main.go, hostPhase/*, chrootPhase/*, firstBoot/*)
- **File writes via shell**: echo/cat instead of direct file writes (matches old bash installer behavior)
- **Logging**: timestamp + command, stdout/stderr, exit code, duration
- **Fix**: Added `mkdir -p /etc/kernel` before writing cmdline (harness 25/25)
- **Build**: All 3 binaries compile (installer + secureboot + tpm), go vet clean

### 2026-08-10 22:30 - README Rewrite + Roadmap Update + umount Fix
**Agent:** codebase
**Summary:** Complete README rewrite, roadmap restructured, umount warning fixed
- **README rewritten**: memory-safe alternatives table (booster, grub→UKI), Dinit replacing OpenRC (with choice), semi-immutable approach (apk core + nix userland), Virtual Machines section (btrfsStandardUnencrypted), Enterprise merged into Virtualization Host (k3s + LINSTOR/Pirateus + ZFS)
- **Roadmap restructured**: Phase 1 (Installer) incl btrfsRaid0 + btrfsStandardUnencrypted + refining; Phase 2 (Desktop) niri + wayvnc; Phase 3 (Distribution) Dinit + custom repo + full release
- **umount fix**: `mountpoint -q` before `umount -fR` in both partitioning.go files to avoid warning when /mnt not mounted
- **session-memory skill fixed**: memory.sh wrapper (removed broken -Days/-Limit flags), SKILL.md storage path corrected
- **Known Issues updated**: Vector marked as resolved
- All 22/22 harness checks pass (Secure Boot confirmed)

### 2026-08-09 13:00 - Dummy Packages: APK v3 + providePackages + uutils-coreutils-more
**Agent:** codebase
**Summary:** APK v3 dummy packages working, uutils-coreutils-more conflict resolved
- **APK v3 dummies**: `apk mkpkg` creates valid packages (no handcraft needed), installed BEFORE main `apk add`
- **providePackages**: New YAML section for empty dummies that satisfy dependencies (e.g., `util-linux-misc`)
- **`DummyPackageRule` type**: `Replace string` field; binary path auto-inferred as `/usr/bin/<replaced>`
- **`InstallDummyPackages()`**: Creates symlinks, builds APK, installs; called before main apk add
- **`CreateDummySymlinkPackage()`**: Creates temp dir with symlinks, runs `apk mkpkg`, installs with `--allow-untrusted`
- **Harness improved**: `less_dummy` check verifies symlink chain, readlink, and bat version output
- **Removed old `ReplacePackageBinaries()`**: Binary replacement approach fully removed
- **`uutils-coreutils-more` conflict resolved**: Dummy `util-linux-misc` satisfies `incus-feature-vm` dependency while `uutils-coreutils-more` provides `/usr/bin/more`
- **All 22/22 checks pass** with all features (Secure Boot + TPM + Nix + Zswap + Hardening + Dummy packages)

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
