# ADR-003: hostPhase/internal Import Restriction

- **Status:** Accepted
- **Date:** 2026-08-26
- **Deciders:** FeralOS team

## Context

Go `internal` packages are importable only by parent tree. Need structural guarantee that host-side helpers (bind mounts, bootstrap, chroot env) cannot be called from `main` or `chrootPhase`. Sources: `AGENTS.md#Entry Point & Phase Isolation`, `docs/architecture/isolation.md#Internal Package Rule`, `src/hostPhase/host_phase.go`.

## Decision

- Place host helpers in `hostPhase/internal/*` (`bind_mounts.go`, `bootstrap.go`, `chroot_env.go`, `hooks.go`).
- Only `hostPhase` and its subpackages may import `hostPhase/internal`. `main` cannot `import "feralos/src/hostPhase/internal"` — compile fails.
- `main` reaches helpers exclusively via `hostPhase.Run()` wrapper.
- Cleanup moved to `utils/cleanup/cleanup.go` (importable by `main` directly — no longer internal).
- FirstBoot features moved to `utils/firstboot/` (importable by `main` directly).

Example:

```go
// src/main.go delegates:
cleanup.Cleanup(conf)            // pre-host cleanup (direct import)
hostPhase.Run(conf)              // disks→mount→bootstrap→chroot prep
chrootPhase.Run(conf)            // chroot operations
firstboot.WriteInstallConfig(conf) // reads cmdline → config.yaml
firstboot.SetupSecureboot(conf)    // compile + copy
firstboot.SetupTpm(conf)           // compile + copy
cleanup.Cleanup(conf)            // final unmount
```

## Consequences

- Compile-time enforcement of phase isolation; no runtime check needed.
- Cleanup runs before hostPhase (clean state) and after all phases (final unmount).
- Adding new host helper requires placement under `hostPhase/internal`, not top-level `utils`.

## References

- `AGENTS.md#Entry Point & Phase Isolation`, `docs/architecture/isolation.md`, `src/hostPhase/host_phase.go`, `src/utils/cleanup/cleanup.go`
