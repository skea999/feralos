# ADR-002: hostPhase vs chrootPhase Separation

- **Status:** Accepted
- **Date:** 2026-08-26
- **Deciders:** FeralOS team

## Context

Installer runs partly on live host (partitioning, LUKS, mounts, bootstrap) and partly inside target chroot (APK, services, UKI). Without boundary, disk/cleanup logic leaks into `main` and chroot commands run on host. Need clear ownership. Sources: `AGENTS.md#Entry Point & Phase Isolation`, `docs/architecture/isolation.md`, `docs/INSTALLER.md#Entry Point & Phase Isolation`.

## Decision

- `main.go` is entry point **only**: loads `variables.yaml` + `internal_rules.yaml`, runs wizard (`createConfig.PromptForConfig`), applies `logic.ApplyRules`, checks host deps, then delegates:
  ```go
  hostPhase.Run(conf)   // all host-side
  chrootPhase.Run(conf) // all chroot-side
  ```
- `main` must NOT call cleanup, disk layouts, or chroot commands directly.
- `hostPhase.Run` owns fixed order: 0 cleanup → 1 disk layout switch → 2 bind mounts → 3 bootstrap → 4 chroot prep → 5 install_config → 6 firstBoot compile. `chrootPhase.Run` owns APK/services/UKI.

## Consequences

- Single responsibility: `main` collects config; phases encapsulate environment-specific logic.
- Prevents host/chroot mix-ups; testable in isolation (hostPhase unit + harness QEMU).
- Requires wrapper for shared helpers (see ADR-003).

## References

- `AGENTS.md#Entry Point & Phase Isolation`, `docs/architecture/isolation.md`, `docs/INSTALLER.md#Installation Flow`, `src/main.go:58`, `src/hostPhase/host_phase.go:17`
