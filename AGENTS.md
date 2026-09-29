# Project Context

**Stack**: Go 1.22 (installer) + Shell (legacy reference) + YAML config
**Project**: FeralOS — Alpine-based distribution, memory safety + security, Nix for glibc, Dinit replaces OpenRC later, custom repo
**Status**: Installer Phase 1

> **Primary context for AI agents.** This file keeps full project info. Detailed copies live in `docs/` (double documentation). Read this file first, then follow `→` pointers for deep dives.

## Workflow

**Chain (complex)**: `context/recall` → `interview/refine` → `@planner` → *approve* → `@codebase` → PARALLEL(`@review`+`@code-reviewer`+`@vulnerability-scan`+`@information-scan`) → `@debugger/tests` → `@docs/docs` → save
**Direct (simple)**: `@codebase` or specialist
State: `state/session_state.json` | Handoff: `handoff/latest.md`

**Installer Workflow**: Understand Requirements (`CONTEXT.md`, `ROADMAP.md`, shell ref) → Plan (`hostPhase`/`chrootPhase` modules) → Implement (Go, naming conventions) → Test locally → Test harness (QEMU) → Review → Commit (conventional)

Key Decisions: `hostPhase` writes files (NO chroot), `chrootPhase` runs commands, module isolation, data-driven YAML/Go, security first (LUKS2, Secure Boot, AppArmor)

→ Details: `docs/guides/development.md` (how to code / separation), `docs/INSTALLER.md` (flow)

## Documentation Rule (MANDATORY)

**Every code change MUST update documentation.** After you change code, update `AGENTS.md` (primary context, keeps full info) **and** the matching `docs/` copy (architecture/adr/guides/harness/intent/ideas) to keep double documentation in sync. No code without doc update. `AGENTS.md` → `docs/` is single source duplication; keep both identical in substance. Update `Docs Index` pointers if you add/move files. Harness docs → `docs/harness/` (canonical) — `tests/harness/README.md` stays pointer only.

**ADRs:** If you make an architectural decision (new pattern, isolation rule, storage layout, boot method, logging, compatibility, etc.) and no ADR exists, **write one** in `docs/adr/ADR-XXX-title.md` (next number) + update `docs/adr/README.md` index. Template: Status / Date / Context / Decision / Consequences. No decision without ADR.

→ Details: `docs/architecture/`, `docs/adr/`, `docs/guides/`, `docs/harness/`, `docs/intent/`, `docs/ideas/`

## Session Memory

Storage `.agent-memory/`, scripts `.agents/skills/session-memory/scripts/`

**The memory must be pushed.** `.agent-memory/` is tracked in git — after saving, commit and push it with the session's changes.

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
cd tests/harness && go build -o skea_harness ./cmd/skea_harness/

# Run installer (requires root + live)
sudo ./feralos_installer -config variables.yaml

# Run generator
./feralos_create_config

# Run harness tests
cd tests/harness
./skea_harness prepare-runner
./skea_harness run -f ~/Progetti/skeaAlpineInstaller/src/variables/variables.yaml
./skea_harness status
./skea_harness cleanup

# Run harness with fixture templates (covers all scenarios)
./skea_harness run -f tests/fixtures/variables/btrfs_raid0_minimal.yaml
./skea_harness run -f tests/fixtures/variables/btrfs_raid0_noswap.yaml
./skea_harness run -f tests/fixtures/variables/btrfs_standard_swap_explicit_8g.yaml
ls tests/fixtures/variables/  # 14 templates: minimal, noswap, explicit, full, isolate (scenarios)

# Run tests (when available)
cd src && go test ./...
```

→ Details: `docs/guides/development.md` (build/test/lint + conventions), `docs/harness/commands.md` (CLI), `docs/harness/workflow.md` (3-phase flow)

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

→ Details: `docs/harness/artifacts.md` (where generated, gitignored, how to inspect)

## Lint (Native Linux)

```bash
cd src && go vet ./...
cd src && golangci-lint run ./...       # includes dupl (code clone detection)
cd tests/harness && go vet ./...
cd tests/harness && golangci-lint run ./...
```

**Dupl exclusions** (intentional duplication): `src/hostPhase/disks/` (layout packages), `src/utils/firstboot/` (standalone binaries). Config: `.golangci.yml` → `dupl.threshold: 100` + `exclude-rules`.

→ Details: `docs/guides/development.md` (lint/format)

## Naming Conventions

| Element | Convention | Example |
|---------|------------|---------|
| Files | snake_case | `bind_mounts.go`, `config.go` |
| Folders | camelCase | `hostPhase/`, `chrootPhase/` |
| Variables | camelCase | `targetDisk` |
| Functions | camelCase/PascalCase | `getMinDiskSize()`, `RunBtrfsSetup()` |
| Types | PascalCase | `Config`, `Subvolume` |

→ Details: `docs/guides/development.md` (module map + conventions)

## Architecture Principle

Standard core, pluggable data — installer logic never changes, storage layouts are Go packages, essential packages hardcoded, optional+rules in YAML, parser same code only data changes, booster replaces mkinitfs, UKI replaces GRUB, LUKS2 on RAID/Swap, Btrfs RAID1 on LUKS. **No backward compatibility — installer runs once, always fresh.**

→ Details: `docs/architecture/overview.md` (principle + tree + decisions), `docs/adr/ADR-001-go-installer.md`, `ADR-005-luks-btrfs-booster-uki.md`, `ADR-006-no-backward-compat.md`

## Entry Point & Phase Isolation

`main.go` is entry point ONLY. It collects/generates config then delegates:

- `cleanup.Cleanup(conf)` — pre-host cleanup (swapoff, unmount, LUKS close)
- `hostPhase.Run(conf)` — all host-side installation
- `chrootPhase.Run(conf)` — all chroot-side installation
- `firstboot.WriteInstallConfig(conf)` — reads cmdline, writes `/opt/feralos/config.yaml`
- `firstboot.SetupSecureboot(conf)` — compile → copy → chmod 755
- `firstboot.SetupTpm(conf)` — compile → copy → chmod 755
- `cleanup.Cleanup(conf)` — final unmount before reboot

`main` must NOT call disk layouts or chroot commands directly.

**Internal package rule**: `hostPhase/internal/*` is importable ONLY by `hostPhase` (and its subpackages). `main` cannot import it. This is the structural guarantee of host/chroot isolation.

**`hostPhase.Run()` fixed order**:

1. Disk layout switch (`hostPhase/disks/*`) by `conf.User.DiskLayout`
2. Mount virtual FS (`internal/bind_mounts.go`)
3. Alpine bootstrap (`internal/bootstrap.go`)
4. Chroot prep (`internal/chroot_env.go`)

Cleanup (before/after) and firstboot (install config, secureboot/tpm compile) are outside `hostPhase.Run()` — called directly by `main`.

→ Details: `docs/architecture/isolation.md` (entry-point + internal rule + order), `docs/INSTALLER.md#installation-flow`, `docs/adr/ADR-002-host-chroot-isolation.md`, `ADR-003-internal-package-rule.md`

## Logging Rule

**Always use `shell.ExecuteCmd` in `hostPhase`/`chrootPhase` — never `os.*`.** File ops (`MkdirAll`, `Create`, `WriteFile`, `ReadFile`) and `exec.Command` bypass logging. `ExecuteCmd` logs CMD, stdout/stderr, exit, retry, and `SKEA_NONINTERACTIVE` handling. `import "os"` forbidden in those packages (exceptions: `firstBoot/` standalone, `utils/shell` impl). Host: `shell.ExecuteCmd("", "mkdir -p /mnt/etc")`. Chroot: `shell.ExecuteCmd(chroot, "mkdir -p /etc")`.

→ Details: `docs/architecture/logging.md`, `docs/adr/ADR-004-executecmd-logging.md`, `src/utils/shell/execute.go`

## Activatable Feature Confinement

**Rule: Every activatable feature MUST be confined to a single owner file.**

Each `Enable*` flag (and `DesktopEnvironment`) is owned by exactly ONE file under `src/chrootPhase/features/` or `src/utils/firstboot/` or `src/chrootPhase/desktops/`. All logic — packages, configuration, guards — lives in that file.

**Callers must NOT contain feature logic:**

- `packages.go` → calls `features.XxxPackages(conf)`, never contains inline feature packages
- `logic.go` → zero feature-specific logic, only generic rule application
- `chroot_phase.go` → calls `features.SetupXxx(conf)`, never contains feature configuration
- `host_phase.go` → calls `features.SetupXxx(conf)`, never contains feature compilation

**Pattern for new features:**

1. Create `src/chrootPhase/features/<name>.go` (or `src/utils/firstboot/` for compile+copy)
2. Export `XxxPackages(conf) []string` — returns APK packages needed
3. Export `SetupXxx(conf) error` — configures the feature inside chroot
4. Guard: `if !conf.User.EnableXxx { return nil }`
5. Call from `packages.go` (packages) and `chroot_phase.go` or `host_phase.go` (setup)
6. NEVER mutate flags from outside the feature file (exception: `NixPackages()` auto-enables internally)

**Current confinement map:**

| Feature | Owner file | Called from |
|---------|-----------|-------------|
| `EnableSecureboot` | `utils/firstboot/secureboot.go` | `main.go` |
| `EnableTPM` | `utils/firstboot/tpm.go` | `main.go` |
| `EnableWebmin` | `utils/firstboot/webmin.go` | `main.go` |
| `EnablePodman` | `chrootPhase/features/podman.go` | `packages.go` + `chroot_phase.go` |
| `EnableHardening` | `chrootPhase/features/hardening.go` | `packages.go` + `chroot_phase.go` |
| `EnableNix` | `chrootPhase/features/nix.go` | `packages.go` + `chroot_phase.go` |
| `EnableZswap` | `chrootPhase/features/zswap.go` + `zswap_server.go` | `packages.go` + `chroot_phase.go` |
| `EnableDebug` | `chrootPhase/features/hardening.go` | via `HardeningPackages()` |
| `EnableSwap` | `hostPhase/disks/*` | `host_phase.go` (disk layout layer) |
| `DesktopEnvironment` | `chrootPhase/desktops/desktops.go` | `chroot_phase.go` |

**Desktop environments supported:**
- `niri` — custom config (greetd, regreet, niri config, DMS, xdg-su) in `chrootPhase/desktops/niri.go`; session startup = turnstile (PAM → user dinit) + unmodified Alpine `niri-session` via login-shell trick (`-s /usr/libexec/feralos/login-shell`, not in `/etc/shells`, symlink re-pointed to fish — busybox applet dispatch breaks the package's `/bin/ash` target; user `chsh` skipped) + `niri`/`niri-shutdown` user services (`niri-user-dinit`); installer enables `/etc/dinit.d/boot.d/turnstiled` + user `boot.d` stack in `/usr/lib/dinit.d/user/boot.d/`
- `kde` — packages only via `internal_rules.yaml`, no custom config
- `cosmic` — packages only via `internal_rules.yaml` (uses seatd, not elogind), no custom config

→ Details: `docs/architecture/feature_confinement.md`, `docs/guides/development.md` (how to add feature)

## Compatibility

No backward compat. Breaking changes allowed without fallback (e.g., breaking changes to `internal_rules.yaml` without deprecated path). Installer is single-run.

→ Details: `docs/adr/ADR-006-no-backward-compat.md`

## FirstBoot Architecture

Activatable features (Secure Boot, TPM, Webmin) use standalone binaries in `src/firstBoot/`:

- `src/firstBoot/secureboot/main.go` — autonomous: installs packages, generates EFI keys, enrolls sbctl, builds+signs UKI, writes hooks
- `src/firstBoot/tpm/main.go` — autonomous: checks TPM, installs clevis@edge + tpm2-tools, binds LUKS to TPM2, rebuilds initramfs+UKI
- `src/firstBoot/webmin/main.go` — autonomous: verifies podman, creates `/opt/webmin`, generates compose + module configs, seeds `/etc/webmin` from image (`webmin-seed` container), starts via `podman compose up -d`, creates `webmin-ensure` oneshot (dual-init: PID1 via `cat /proc/1/comm` → dinit `/etc/dinit.d/webmin-ensure` + `/usr/libexec/dinit-webmin-ensure` + boot.d link, OpenRC `/etc/init.d/webmin-ensure`), sets root password via `changepass.pl` + `chpasswd`

**Installer flow**:

1. `firstboot.SetupSecureboot(conf)` / `firstboot.SetupTpm(conf)` / `firstboot.SetupWebmin(conf)` compile binaries → copy to `/opt/feralos_*`
2. `firstboot.WriteInstallConfig(conf)` writes `/opt/feralos/config.yaml` (disk layout, LUKS UUIDs, user info)
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

→ Details: `docs/architecture/firstboot.md` (hostPhase/firstBoot design), `docs/guides/first_boot.md` (real hardware steps), `docs/guides/webmin.md` (webmin compose/seed), `docs/guides/podman.md` (btrfs driver), `docs/FIRST_BOOT.md` is deprecated → `guides/first_boot.md`

## Project Structure

Full tree → `docs/INSTALLER.md#project-structure` and `docs/architecture/overview.md#project-structure`. Pointer only here to keep file concise, but info kept (double documentation).

## Docs Index (double documentation — AGENTS.md keeps full info, copies live in `docs/`)

- **Architecture**: `docs/architecture/overview.md` (principle + tree), `isolation.md` (entry-point + internal rule + hostPhase order), `logging.md` (ExecuteCmd rule), `feature_confinement.md` (single-owner), `storage.md` (layouts: btrfsStandardServer, btrfsStandard, btrfsRaid0, btrfsUnencrypted), `firstboot.md` (secureboot/tpm 3-boot) — all double of AGENTS.md above
- **ADRs**: `docs/adr/README.md` → ADR-001 Go installer, 002 isolation, 003 internal rule, 004 ExecuteCmd, 005 LUKS+Btrfs+booster+UKI, 006 no compat (decisions from AGENTS.md)
- **Guides** (user guides only): `docs/guides/README.md` (index) → `getting_started.md` (real hardware), `first_boot.md` (secureboot/tpm post-install), `variables.md` (variables.yaml compilation), `niri.md` (niri + NetworkManager + wayvnc), `webmin.md` (podman-based web admin), `podman.md` (btrfs driver), `development.md` (how to code / separation)
- **Harness**: `docs/harness/README.md` (index) → `overview.md` (what it does), `workflow.md` (3-phase + 3-boot), `artifacts.md` (where generated, gitignored), `building.md` (where compiled binary goes), `development.md` (how to code), `commands.md` (CLI+MCP), `debug_run.md` (`tests/debug_run/` boot existing run, LUKS inject). `tests/harness/README.md` is pointer only.
- **Intent / Ideas**: `docs/intent/vision.md`, `docs/intent/goals.md`, `docs/intent/vocabulary.md`, `docs/ideas/backlog.md` ← `CONTEXT.md` / `ROADMAP.md` / `TASKS.md`

→ All `docs/` files are double of AGENTS.md sections above. AI agents: read AGENTS.md first for full context, then follow `→` pointers for deep dives.

## Roadmap

| Phase | Milestone | Status |
|-------|-----------|--------|
| Phase 1 | btrfsStandardServer, btrfsStandard, btrfsRaid0 | Completed |
| Phase 1 | Generator, Test harness, Nix | Completed |
| Phase 1 | btrfsUnencrypted | Completed |
| Phase 1 | niri + NetworkManager + wayvnc + firewall + webmin/podman | Completed |
| Phase 1 | Refining | Planned |
| Phase 2 | Dinit, Custom repo, Full release | Planned |

See `ROADMAP.md` for pipeline priorities. Memory-safe alternatives table → `CONTEXT.md`. Backlog → `docs/ideas/backlog.md`.

## Known Issues

All prior swap/vector/dummy issues resolved. See `CHANGELOG.md` Fixed section. Current: `log_errors` shows QEMU-only noise (`smartd`/`fsck` whitelisted; netns `mount/umount /run/netns ... Invalid argument` QEMU early-chain noise allowlisted; udevd `/bin/false` module install errors — parport/psmouse/intel_gtt/input_leds/intel_pmc_bxt — from OpenRC udev path, harmless in QEMU, nix pivot fix landed).
