# Project Context

**Stack**: Go 1.22 (installer) + Shell (legacy reference) + YAML config
**Project**: FeralOS — Alpine-based distribution, memory safety + security, Nix for glibc, Dinit replaces OpenRC later, custom repo
**Status**: Installer Phase 1

## Workflow

**Chain** (complex): context/recall → interview/refine → @planner → approve → @codebase → PARALLEL(@review+@code-reviewer+@vulnerability-scan+@information-scan) → @debugger/tests → @docs/docs → save
**Direct** (simple): @codebase or specialist
State: `state/session_state.json` | Handoff: `handoff/latest.md`

Installer Workflow: Understand Requirements (CONTEXT, ROADMAP, shell ref) → Plan (hostPhase/chrootPhase modules) → Implement (Go, naming conventions) → Test locally → Test harness (QEMU) → Review → Commit (conventional)

Key Decisions: hostPhase writes files (NO chroot), chrootPhase runs commands, module isolation, data-driven YAML/Go, security first (LUKS2, Secure Boot, AppArmor)

## Session Memory

Storage `.agent-memory/`, scripts `.agents/skills/session-memory/scripts/`
```bash
.agents/skills/session-memory/scripts/memory.sh context --days 14
.agents/skills/session-memory/scripts/memory.sh recall "vector" --limit 5
.agents/skills/session-memory/scripts/memory.sh stats
.agents/skills/session-memory/scripts/memory.sh save "decision" "Chose X because Y" tag1 tag2
MEMORY_IMPORTANCE=high .agents/skills/session-memory/scripts/memory.sh save "insight" "Key finding" installer
.agents/skills/session-memory/scripts/memory.sh save "summary" "Today: implemented X. Next: Z" -Tags installer go feralos
```

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
cd tests/harness && go build -o skea-harness ./cmd/skea_harness/

# Run installer (requires root + live)
sudo ./feralos_installer -config variables.yaml

# Run generator
./feralos_create_config

# Run harness tests
cd tests/harness
./skea-harness prepare-runner
./skea-harness run -f ~/Progetti/skeaAlpineInstaller/src/variables/variables.yaml
./skea-harness status
./skea-harness cleanup

# Run harness with fixture templates (covers all scenarios)
./skea-harness run -f tests/fixtures/variables/btrfs_raid0_minimal.yaml
./skea-harness run -f tests/fixtures/variables/btrfs_raid0_noswap.yaml
./skea-harness run -f tests/fixtures/variables/btrfs_standard_swap_explicit_8g.yaml
ls tests/fixtures/variables/  # 14 templates: minimal, noswap, explicit, full, isolate (scenarios)

# Run tests (when available)
cd src && go test ./...
```

## Test Artifacts

Output to `tests/artifacts/<tool-name>/<timestamp>/`:
```
tests/artifacts/
├── feralos_create_config/
│   └── .gitkeep
└── harness/
    ├── cache/ (base-images, keys, runner-images, web/alpine)
    ├── prepare/ (logs)
    ├── runs/ (per-install logs + disks)
    └── state/ (active.json)
```
Rules: gitignored `tests/artifacts/**/*` except `.gitkeep`, never commit outputs, harness keeps last 2 runs.

## Lint (Native Linux)

```bash
cd src && go vet ./...
cd src && golangci-lint run ./...
```

## Naming Conventions

| Element | Convention | Example |
|---------|------------|---------|
| Files | snake_case | `bind_mounts.go`, `config.go` |
| Folders | camelCase | `hostPhase/`, `chrootPhase/` |
| Variables | camelCase | `targetDisk` |
| Functions | camelCase/PascalCase | `getMinDiskSize()`, `RunBtrfsSetup()` |
| Types | PascalCase | `Config`, `Subvolume` |

## Architecture Principle

Standard core, pluggable data — installer logic never changes, storage layouts are Go packages, essential packages hardcoded, optional+rules in YAML, parser same code only data changes, booster replaces mkinitfs, UKI replaces GRUB, LUKS2 on RAID/Swap, Btrfs RAID1 on LUKS. **No backward compatibility — installer runs once, always fresh.**

## Activatable Feature Confinement

**Rule: Every activatable feature MUST be confined to a single owner file.**

Each `Enable*` flag (and `DesktopEnvironment`) is owned by exactly ONE file under `src/chrootPhase/features/` or `src/hostPhase/features/` or `src/chrootPhase/desktops/`. All logic — packages, configuration, guards — lives in that file.

**Callers must NOT contain feature logic:**
- `packages.go` → calls `features.XxxPackages(conf)`, never contains inline feature packages
- `logic.go` → zero feature-specific logic, only generic rule application
- `chroot_phase.go` → calls `features.SetupXxx(conf)`, never contains feature configuration
- `host_phase.go` → calls `features.SetupXxx(conf)`, never contains feature compilation

**Pattern for new features:**

1. Create `src/chrootPhase/features/<name>.go` (or `hostPhase/features/` for compile+copy)
2. Export `XxxPackages(conf) []string` — returns APK packages needed
3. Export `SetupXxx(conf) error` — configures the feature inside chroot
4. Guard: `if !conf.User.EnableXxx { return nil }`
5. Call from `packages.go` (packages) and `chroot_phase.go` or `host_phase.go` (setup)
6. NEVER mutate flags from outside the feature file (exception: `NixPackages()` auto-enables internally)

**Current confinement map:**

| Feature | Owner file | Called from |
|---------|-----------|-------------|
| `EnableSecureboot` | `hostPhase/features/secureboot.go` | `host_phase.go` |
| `EnableTPM` | `hostPhase/features/tpm.go` | `host_phase.go` |
| `EnableHardening` | `chrootPhase/features/hardening.go` | `packages.go` + `chroot_phase.go` |
| `EnableNix` | `chrootPhase/features/nix.go` | `packages.go` + `chroot_phase.go` |
| `EnableZswap` | `chrootPhase/features/zswap.go` + `zswap_server.go` | `packages.go` + `chroot_phase.go` |
| `EnableDebug` | `chrootPhase/features/hardening.go` | via `HardeningPackages()` |
| `EnableSwap` | `hostPhase/disks/*` | `host_phase.go` (disk layout layer) |
| `DesktopEnvironment` | `chrootPhase/desktops/desktops.go` | `chroot_phase.go` |

## Compatibility

No backward compat. Breaking changes allowed without fallback (e.g., breaking changes to `internal_rules.yaml` without deprecated path). Installer is single-run.

## FirstBoot Architecture

Activatable features (Secure Boot, TPM) use standalone binaries in `src/firstBoot/`:
- `src/firstBoot/secureboot/main.go` — autonomous: installs packages, generates EFI keys, enrolls sbctl, builds+signs UKI, writes hooks
- `src/firstBoot/tpm/main.go` — autonomous: checks TPM, installs clevis@edge + tpm2-tools, binds LUKS to TPM2, rebuilds initramfs+UKI

**Installer flow** (hostPhase):
1. `features/secureboot.go` / `features/tpm.go` compile binaries → copy to `/opt/feralos_*`
2. `features/install_config.go` writes `/opt/feralos/config.yaml` (disk layout, LUKS UUIDs, user info)
3. Binaries are autonomous — installer does NOT run them

**3-boot harness flow**:
1. **Runner (Debian)**: installer runs, compiles binaries, writes config. swtpm always present.
2. **Target (Alpine) — first boot**: harness runs `feralos_secureboot` + `feralos_tpm` via SSH. Binaries install packages, configure features, bind LUKS to TPM2. swtpm always present.
3. **Target (Alpine) — verification**: harness checks sbctl status, clevis bindings, LUKS auto-unlock. swtpm always present.

**Key rules**:
- swtpm is always passed to runner + target VMs (even if EnableTPM=false) — simplifies harness
- firstBoot binaries read `/opt/feralos/config.yaml` for disk layout + LUKS UUIDs (fallback: kernel cmdline)
- On real hardware: user runs `/opt/feralos_secureboot` and `/opt/feralos_tpm` manually after first boot

**Known limitation**: Secure Boot enrollment cannot be confirmed in QEMU (no real efivarfs). TPM auto-unlock CAN be confirmed and works reliably.

## Project Structure

Full tree → `docs/INSTALLER.md#project-structure`. Pointer only here to keep file <200 lines.

## Roadmap

| Phase | Milestone | Status |
|-------|-----------|--------|
| Phase 1 | btrfsStandardServer, btrfsStandard, btrfsRaid0 | Completed |
| Phase 1 | Generator, Test harness, Nix | Completed |
| Phase 1 | btrfsStandardUnencrypted, Refining | Planned |
| Phase 2 | niri + wayvnc | Planned |
| Phase 3 | Dinit, Custom repo, Full release | Planned |

See `ROADMAP.md` for pipeline priorities. Memory-safe alternatives table → `CONTEXT.md`.

## Known Issues

All prior swap/vector/dummy issues resolved. See `CHANGELOG.md` Fixed section. Current: none blocking (QEMU-only `smartd`/`fsck` whitelisted).

---

*Updated: 2026-08-24*

### 2026-08-24 — btrfsStandardServer RAID1 fix (dead data disks removed)
**Agent:** codebase — **All 3 layouts PASS with secureboot+tpm: standard 31/31, raid0 33/33, server 32/32**
- Removed unused 8GB btrfs data disks (`btrfsDisk1Path`/`btrfsDisk2Path`) from harness QEMU args — created but never mounted/referenced by installer, causing udev race → `dm-1` missing → busybox fallback
- `BootTimeout` increased to 10min for 3-boot flow with always-present swtpm
- 3-boot harness flow stable: runner installs, target first-boot runs secureboot+tpm, verification passes

### 2026-08-23 17:00 - AGENTS compress + githooks vet/lint + fixtures validation
**Agent:** codebase — **AGENTS <200 lines + hook vet/lint + 3 harness runs PASS**
- Compressed `AGENTS.md` 596→145 lines: structure moved to `docs/INSTALLER.md#project-structure` (pointer only), history truncated to last 2 entries → `CHANGELOG.md`, Git hooks section removed per user
- Fixed `.githooks/pre-commit`: staged `*.go` → `go vet ./...` (src/harness) + `golangci-lint` (`$HOME/go/bin` fallback), fixed `grep --` leading dash bug, set `core.hooksPath=.githooks`
- Validated: `go vet`+`lint` clean, `swap` 25/25 PASS, YAML 14/14 valid, harness `btrfsRaid0-noswap` 32/32 (header-only), `btrfsStandard`/`btrfsRaid0` `8G` explicit 32/32 (8192M file / 4177916 KiB per partition)

### 2026-08-20 11:00 - Fixtures library: 14 curated variables.yaml templates
**Agent:** codebase — **14 templates + swap unit tests + docs (128/128 PASS on refactors)**
- `tests/fixtures/variables/`: btrfsStandard/Server/Raid0 × swap on/off × swapSize auto/8G × secureboot/tpm/hardening isolate × niri × full_stack. README matrix + Tier1 CI (minimal+noswap)
- `src/utils/swap/swap_test.go`: auto/G/M/K/T/P, whitespace, invalid; fixed `K` truncation bug `n*1/1024→0`
- Compat: old harness `variables-*.yaml` kept `DEPRECATED` → new fixtures

### 2026-08-20 10:00 - btrfsRaid0 Layout + enableSwap/swapSize Refactor
**Agent:** codebase — **btrfsRaid0 done + enableSwap generic (128/128)**
- btrfsRaid0 (`48ad7bc`): pure RAID0, EFI both disks, 2 LUKS swap p2 (`swapSize/2`) gated `enableSwap`, root p3 remainder, dmcrypt step 6a, fstab `/dev/mapper/swap-*`
- `swapSize` `auto`=2×RAM via `utils/swap.ResolveSwapSizeMiB` or `8G`/`4096M`; `btrfsStandardServer` unchanged
- Harness `EnableSwap`+`SwapSize` plumbed, `swap_status` truthful

> Older history → `CHANGELOG.md` + `git log` + `.agent-memory/` (session-memory). Keep AGENTS.md <250 lines.
