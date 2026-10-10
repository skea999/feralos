# ADR-001: Go 1.22 for Installer + Shell Legacy Ref + YAML Config

- **Status:** Accepted
- **Date:** 2026-08-26
- **Deciders:** FeralOS team

## Context

Installer must partition disks, bootstrap Alpine, configure system reliably across RAID layouts. Shell reference exists in `internal/` but lacks type safety, testability, module isolation. Need cross-compiled firstBoot binaries, declarative user config, data-driven package rules. Sources: `CONTEXT.md#File Locations`, `docs/PROJECT.md#Stack`, `AGENTS.md#Stack`, `docs/architecture/overview.md`.

## Decision

- Use **Go 1.22** for installer (`go.mod: feralos/src`), Shell kept only as legacy reference in `internal/`.
- User config + dynamic rules in **YAML**: `variables.yaml` (user) + `variables/internal_rules.yaml` (rules), parsed by `utils/config` + `utils/logic`.
- Generator (`utils/createConfig`) interactive or unattended; same Go toolchain builds firstBoot binaries and harness.

## Consequences

- Type safety, `go vet`/`golangci-lint`, unit tests (`utils/swap`), modules for storage layouts as Go packages.
- Cross-compile firstBoot (`firstBoot/secureboot`, `firstBoot/tpm`) and harness in same language — single toolchain, no runtime deps.
- Shell reference not executed; divergence risk if legacy scripts drift — mitigated by deprecating Shell path.

## Alternatives Considered

- **Shell only:** Simple but no types, hard to test, error handling inconsistent — rejected.
- **Python:** Easier scripting but heavier runtime on live ISO, weaker static guarantees for disk/LUKS logic — rejected.

## References

- `AGENTS.md#Stack`, `CONTEXT.md`, `docs/PROJECT.md#Stack`, `docs/INSTALLER.md#Project Structure`, `src/go.mod`
