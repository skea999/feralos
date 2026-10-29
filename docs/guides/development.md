# Development — How to Code / Separate

> Source: `AGENTS.md` is source of truth (double docs — this file is reference copy). Canonical sections: Naming Conventions, Entry Point & Phase Isolation, Logging Rule, Activatable Feature Confinement, Compatibility.

## Go Conventions

| Element | Convention | Example |
|---------|------------|---------|
| Files | snake_case | `bind_mounts.go`, `config.go` |
| Folders | camelCase | `hostPhase/`, `chrootPhase/` |
| Variables | camelCase | `targetDisk` |
| Functions | camelCase/PascalCase | `getMinDiskSize()`, `RunBtrfsSetup()` |
| Types | PascalCase | `Config`, `Subvolume` |

Ref: `AGENTS.md#Naming Conventions`.

## Module Map

```text
src/
├── main.go                          # entry only — collect config, delegate
├── variables/ (variables.yaml, internal_rules.yaml)
├── utils/
│   ├── config/config.go             # LoadConfig, Config struct
│   ├── shell/execute.go             # ExecuteCmd(chroot, cmd) — only logger
│   ├── logic/logic.go               # ApplyRules — generic, no feature logic
│   ├── swap/swap.go                 # ResolveSwapSizeMiB (auto=2×RAM)
│   ├── cleanup/cleanup.go           # Cleanup — pre/post host cleanup (swapoff, unmount, LUKS close)
│   ├── firstboot/
│   │   ├── install_config.go        # WriteInstallConfig → /opt/feralos/config.yaml
│   │   ├── secureboot.go            # SetupSecureboot — compile → copy → chmod 755
│   │   └── tpm.go                   # SetupTpm — compile → copy → chmod 755
│   └── createConfig/createValues/   # feralos_create_config generator
├── hostPhase/
│   ├── host_phase.go                # Run(conf) — fixed order 1-4
│   ├── internal/                    # importable ONLY by hostPhase
│   │   ├── bind_mounts.go           # MountVirtualFilesystems — step 2
│   │   ├── bootstrap.go             # BootstrapAlpine — step 3
│   │   ├── chroot_env.go            # PrepareChrootEnv — step 4
│   │   └── hooks.go
│   └── disks/
│       ├── btrfsStandard/           # 1 disk, swapfile
│       ├── btrfsRaid0/              # 2 disks RAID0, 2 LUKS swap p2 (EnableSwap)
│       └── btrfsStandardServer/     # 2 disks RAID1, LUKS swap
├── firstBoot/
│   ├── secureboot/main.go           # standalone — installs sbctl, keys, signs UKI
│   ├── tpm/main.go                  # standalone — installs clevis, binds LUKS
│   └── webmin/main.go               # standalone podman-based — compose + seed + oneshot
└── chrootPhase/
    ├── chroot_phase.go              # Run(conf) — 14-step orchestrator (incl. flatpak, desktops)
    ├── internal/ (packages.go, boot.go, storage.go, uki.go, services.go, users.go, ...) # packages.go: /lib/modules before apk, dummy+firmware+feature packages
    ├── features/ (hardening.go, nix.go, zswap.go, zswap_server.go, podman.go, flatpak.go) # flatpak: flathub + install; podman: btrfs driver
    └── desktops/ (desktops.go, niri.go) # niri: greetd/regreet/niri + NetworkManager + wayvnc (user dinit) + xdg-su/dms
```

Full tree: `docs/INSTALLER.md#Project Structure`, `docs/architecture/overview.md`.

## Isolation Rules

**Entry point**: `main.go` collects/generates config (`config.LoadConfig`, `createConfig.PromptForConfig`, `logic.ApplyRules`) then delegates — `cleanup.Cleanup(conf)` → `hostPhase.Run(conf)` → `chrootPhase.Run(conf)` → `firstboot.*` → `cleanup.Cleanup(conf)`. `main` must NOT call `hostPhase/disks/*`, `hostPhase/internal/*`, or `chrootPhase/internal/*` directly.

**Internal package rule**: `hostPhase/internal/*` is Go `internal` — importable only by `hostPhase` and subpackages. Compile-time guarantee. Ref: `AGENTS.md#Entry Point & Phase Isolation`, `docs/architecture/isolation.md`.

**`hostPhase.Run()` fixed order** (`src/hostPhase/host_phase.go`):

1. `hostPhase/disks/*` switch by `conf.User.DiskLayout`
2. `internal.MountVirtualFilesystems` — /dev,, /proc, /sys, slave
3. `internal.BootstrapAlpine` — apk-tools-static, base, repos
4. `internal.PrepareChrootEnv` — networking, locale, limits, APK, sudo-rs, modules

Cleanup (before/after) and firstboot (install config, secureboot/tpm compile) are now outside `hostPhase.Run()` — called directly by `main`.

## Feature Confinement

**Rule**: every `Enable*` flag (and `DesktopEnvironment`) confined to single owner file. All packages + config + guards live there. Callers must NOT contain feature logic.

**6-step pattern** for new feature:

1. Create `src/chrootPhase/features/<name>.go` (or `src/utils/firstboot/` for compile+copy)
2. Export `XxxPackages(conf) []string`
3. Export `SetupXxx(conf) error`
4. Guard: `if !conf.User.EnableXxx { return nil }`
5. Call from `packages.go` (packages) and `chroot_phase.go` or `host_phase.go` (setup)
6. Never mutate flags outside owner (exception: `NixPackages()` auto-enables)

**Map**:

| Feature | Owner file | Called from |
|---------|-----------|-------------|
| `EnableSecureboot` | `utils/firstboot/secureboot.go` | `main.go` |
| `EnableTPM` | `utils/firstboot/tpm.go` | `main.go` |
| `EnableWebmin` | `utils/firstboot/webmin.go` | `main.go` (podman-based, verifies podman) |
| `EnablePodman` | `chrootPhase/features/podman.go` | `packages.go` + `chroot_phase.go` (btrfs driver, cgroups) |
| `EnableHardening` | `chrootPhase/features/hardening.go` | `packages.go` + `chroot_phase.go` |
| `EnableNix` | `chrootPhase/features/nix.go` | `packages.go` + `chroot_phase.go` (upstream default build dir /tmp, exec since round 48) |
| `EnableZswap` | `chrootPhase/features/zswap.go` + `zswap_server.go` | `packages.go` + `chroot_phase.go` |
| `EnableDebug` | `chrootPhase/features/hardening.go` | via `HardeningPackages()` |
| `EnableSwap` | `hostPhase/disks/*` | `host_phase.go` (disk layout) |
| `DesktopEnvironment` | `chrootPhase/desktops/desktops.go` + `niri.go` | `chroot_phase.go` (niri: NetworkManager wiring + CUPS admin via lpadmin group) |
| `FlatpakPackages` | `chrootPhase/features/flatpak.go` | `packages.go` + `chroot_phase.go` (flathub + install, `/lib/modules` before apk; no niri default) |

Violations: inline packages in `packages.go`, config in `chroot_phase.go`/`host_phase.go`, logic in `logic.go`. Ref: `docs/architecture/feature_confinement.md`.

## Logging Rule

**Always `shell.ExecuteCmd` in `hostPhase`/`chrootPhase` — never `os.*`.** `MkdirAll`/`Create`/`WriteFile`/`ReadFile` and bare `exec.Command` bypass logging. `ExecuteCmd` logs CMD + stdout/stderr + exit + retry (3×, 3s) + `SKEA_NONINTERACTIVE` handling. `import "os"` forbidden there.

```go
shell.ExecuteCmd("", "mkdir -p /mnt/etc")   // host
shell.ExecuteCmd(chroot, "mkdir -p /etc")   // chroot
```

Exceptions: `firstBoot/*` (standalone), `utils/shell` (impl). Ref: `src/utils/shell/execute.go:47`, `docs/architecture/logging.md`.

## No Compatibility

No backward compat. Breaking changes allowed without fallback (e.g., `internal_rules.yaml` schema). Installer runs once, always fresh. Ref: `AGENTS.md#Compatibility`, `docs/adr/README.md` ADR-006.

## Build / Test / Lint

```bash
# vet + lint (native Linux, pre-commit does this)
cd src && go vet ./...
cd src && golangci-lint run ./...   # incl. wastedassign + nakedret; revive disabled (var-naming false-positives on camelCase package names = project convention)

# static analysis suite (ursine pattern; same checks as scripts/ci.sh,
# configs at repo root, tool versions pinned in the Dockerfile lint-tools)
yamllint -c .yamllint.yml src/variables/internal_rules.yaml \
  src/variables/variables.yaml tests/fixtures/variables/*.yaml   # BLOCKING
gitleaks detect --source . --config .gitleaks.toml --no-banner --redact  # BLOCKING (history; no .git -> `gitleaks dir .`)
typos --config typos.toml --format brief .                       # warn-only
markdownlint-cli2 "docs/**/*.md" "*.md"                          # warn-only
ec --config .ecrc .                                              # warn-only
taplo lint --config taplo.toml .gitleaks.toml typos.toml         # warn-only, taplo optional

# build
cd src && go build -o ../feralos_installer .
cd src/utils/createConfig/createValues && go build -o ../../../../feralos_create_config .
cd src && go build -o ../feralos_secureboot ./firstBoot/secureboot/
cd src && go build -o ../feralos_tpm ./firstBoot/tpm/
cd tests/harness && go build -o skea_harness ./cmd/skea_harness/

# test
cd src && go test ./...
cd tests/harness && go test ./...

# harness (QEMU) — see docs/harness/README.md
cd tests/harness
./skea_harness prepare-runner
./skea_harness run -f ~/Progetti/feralos/src/variables/variables.yaml
./skea_harness run -f tests/fixtures/variables/btrfs_raid0_minimal.yaml  # 30 fixtures, full template mirror
./skea_harness status && ./skea_harness cleanup

# artifacts (gitignored): tests/artifacts/{feralos_create_config,harness}/{cache,prepare,runs,state}
```

## Commit Convention

Conventional Commits: `feat:`, `fix:`, `refactor:`, `docs:`, `test:`, `chore:` + scope (`harness`, `hostPhase`). Subject ≤50 chars, body only if why not obvious. No vendor, no backward-compat shims. Recent: `fix(harness): remove dead btrfs data disks`, `refactor: EFI sync hook moved to chrootPhase`.

## References

- `AGENTS.md` — all rules (source)
- `docs/INSTALLER.md` — flow + structure
- `docs/architecture/isolation.md`, `logging.md`, `feature_confinement.md`, `storage.md`, `firstboot.md`, `dbus-activation.md`, `boot-oneshots.md`
- `docs/adr/README.md` — ADRs 001-006
