# ADR-006: No Backward Compatibility

- **Status:** Accepted
- **Date:** 2026-08-26
- **Deciders:** FeralOS team

## Context

Installer runs once per machine, always fresh — no upgrades of installed system via installer, no persisted old config to migrate. Maintaining deprecated paths for `internal_rules.yaml`, `variables.yaml`, or layouts adds complexity with no user benefit. Sources: `AGENTS.md#Architecture Principle` + `#Compatibility`, `docs/architecture/overview.md`.

## Decision

- **No backward compatibility.** Breaking changes allowed without fallback or deprecation period.
- `internal_rules.yaml` can change schema without deprecated field handling; parser (`utils/logic`) stays same code, only data changes.
- No migration shims for old `variables.yaml`; users regenerate via generator.

## Consequences

- Simpler code: no version checks, no dual-path logic, no `deprecated` branches.
- Operators must regenerate config from current template/fixture; old YAML may fail to validate.
- Faster iteration on storage layouts and feature flags.

## References

- `AGENTS.md#Compatibility` + `#Architecture Principle`, `docs/architecture/overview.md`, `CONTEXT.md`, `src/variables/internal_rules.yaml`, `src/utils/logic/logic.go`
