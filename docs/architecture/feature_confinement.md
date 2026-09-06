# Activatable Feature Confinement

Source: `AGENTS.md#Activatable Feature Confinement` (double docs — AGENTS.md is source of truth, this copy is reference).

## Rule

**Every activatable feature MUST be confined to a single owner file.**

Each `Enable*` flag (and `DesktopEnvironment`) is owned by exactly ONE file under `src/chrootPhase/features/` or `src/utils/firstboot/` or `src/chrootPhase/desktops/`. All logic — packages, configuration, guards — lives in that file.

## Callers Must NOT Contain Feature Logic

- `packages.go` → calls `features.XxxPackages(conf)`, never inline feature packages
- `logic.go` → zero feature-specific logic, only generic rule application
- `chroot_phase.go` → calls `features.SetupXxx(conf)`, never feature configuration
- `host_phase.go` → calls `features.SetupXxx(conf)`, never feature compilation

Violation = feature logic leaked outside owner.

## Pattern for New Features

1. Create `src/chrootPhase/features/<name>.go` (or `src/utils/firstboot/` for compile+copy)
2. Export `XxxPackages(conf) []string` — returns APK packages needed
3. Export `SetupXxx(conf) error` — configures feature inside chroot
4. Guard: `if !conf.User.EnableXxx { return nil }`
5. Call from `packages.go` (packages) and `chroot_phase.go` or `host_phase.go` (setup)
6. NEVER mutate flags from outside the feature file (exception: `NixPackages()` auto-enables internally)

## Confinement Map

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

## References

- `AGENTS.md#Activatable Feature Confinement`
- `docs/adr/README.md` → ADR-002 isolation
