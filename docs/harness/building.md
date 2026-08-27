# Harness — Building

> Where to put the compiled harness so it never gets committed. Shim: `tests/harness/README.md`. Rules: `AGENTS.md`.

## Build

From repo root:

```bash
cd tests/harness && go build -o skea_harness ./cmd/skea_harness/
# or run without building
go run ./cmd/skea_harness -- prepare-runner
go run ./cmd/skea_harness -- run -f tests/fixtures/variables/btrfs_standard_minimal.yaml
```

The module is `skeaAlpineInstaller/tests/harness` (`tests/harness/go.mod`, Go 1.22, `gopkg.in/yaml.v3`).

## Where the binary lives (gitignored)

Preferred location — `tests/harness/skea_harness` (next to `go.mod`):

```bash
cd tests/harness && go build -o skea_harness ./cmd/skea_harness/
./skea_harness status
./skea_harness logs --log installer
```

This path is explicitly gitignored:

```gitignore
# .gitignore (root, excerpt)
tests/harness/skea_harness
tests/harness/work/
tests/harness/iso/
tests/harness/coverage.out
tests/artifacts/**/*
!tests/artifacts/**/
!tests/artifacts/**/.gitkeep
*.qcow2
*.iso
feralos_installer
feralos_create_config
feralos_secureboot
feralos_tpm
```

Do **not** commit the binary. `go vet ./...` in `tests/harness` still passes with it present.

## Alternative — `artifacts/harness/bin/`

`tests/harness/run_mcp.sh` (MCP wrapper) auto-builds to `artifacts/harness/bin/skea_harness` when missing or stale (checks `*.go` newer than binary via `find -newer`):

```bash
./tests/harness/run_mcp.sh        # builds artifacts/harness/bin/skea_harness if needed, then execs mcp
./generate_mcp_config.sh          # writes absolute-path MCP config for the editor
```

`artifacts/harness/bin/` is under `tests/artifacts/**/*` so it is gitignored by the same pattern — but it is **ephemeral** (prunable with other artifacts). Prefer `tests/harness/skea_harness` for a stable local binary.

## Keys, images, caches — also gitignored

All under `tests/artifacts/harness/`:

- `cache/base-images/*.qcow2`, `cache/runner-images/*.qcow2` → `*.qcow2` + `tests/artifacts/**/*`
- `cache/keys/id_ed25519*` → `tests/artifacts/**/*`
- `runs/*/ *.log`, `prepare/*/ *.log` → `tests/artifacts/**/*` + `*.log`
- `state/active.json` → `tests/artifacts/**/*`

Never commit them. Preserve empty dirs with `.gitkeep` (already present at `tests/artifacts/.gitkeep`, `tests/artifacts/harness/.gitkeep` etc.):

```bash
touch tests/artifacts/harness/cache/.gitkeep
git add -f tests/artifacts/harness/cache/.gitkeep  # tracked via !**/.gitkeep exception
```

## Verify

```bash
git check-ignore -v tests/harness/skea_harness
# .gitignore:43:tests/harness/skea_harness  tests/harness/skea_harness

git check-ignore -v tests/artifacts/harness/cache/base-images/debian-13-genericcloud-amd64.qcow2
# .gitignore:47:tests/artifacts/**/*  ...

git check-ignore -v tests/artifacts/harness/runs/2026-08-27T10-00-00Z/installer.log
# .gitignore:47:tests/artifacts/**/*  ...
```

If `git status` shows any of these, `.gitignore` is misconfigured — fix before committing.
