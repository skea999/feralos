# Skea Harness

> **Pointer**: Full harness documentation lives in [`docs/harness/`](../../docs/harness/) — overview, workflow, artifacts, building, development, commands. This file is minimal by design.

Quick commands (see `docs/harness/commands.md` and `docs/harness/workflow.md`):

```bash
cd tests/harness
go run ./cmd/skea_harness prepare-runner
go run ./cmd/skea_harness run -f ../../tests/fixtures/variables/btrfs_standard_minimal.yaml
go run ./cmd/skea_harness status
go run ./cmd/skea_harness cleanup
```

Artifacts: `tests/artifacts/harness/` (see `docs/harness/artifacts.md`). Compiled binary `tests/harness/skea-harness` is gitignored (see `docs/harness/building.md`).

*Updated: 2026-08-26 — docs moved to `docs/harness/`*
