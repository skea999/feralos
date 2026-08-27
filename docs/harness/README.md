# Harness Documentation — Index

> Canonical harness docs live here (`docs/harness/`). `tests/harness/README.md` is a pointer/shim — see that file for repo-relative quick-start, this directory for authoritative detail. Global project rules: `AGENTS.md`.

## Files

| File | One-line |
|------|----------|
| [overview.md](overview.md) | What the harness does — scope, KVM, simulation model, fixtures |
| [workflow.md](workflow.md) | Detailed 3-phase flow: prepare-runner → run → verify, with diagram |
| [artifacts.md](artifacts.md) | Where artifacts land under `tests/artifacts/harness/`, how to inspect/clean |
| [building.md](building.md) | Where to put the compiled `skea-harness` binary so it stays gitignored |
| [development.md](development.md) | How to code the harness — module structure, key patterns, adding checks |
| [commands.md](commands.md) | CLI reference for all `skea-harness` commands and MCP tools |
| [debug_run.md](debug_run.md) | `tests/debug_run/` — boot existing run without reinstall, auto-detect latest, LUKS inject |

## Quick links

- Code: `tests/harness/cmd/skea_harness/main.go`, `tests/harness/internal/service/service.go`
- Fixtures: `tests/fixtures/variables/` (14 templates, `tests/fixtures/variables/README.md`)
- Artifacts root: `tests/artifacts/harness/` (see [artifacts.md](artifacts.md))
- Harness shim: `tests/harness/README.md`
