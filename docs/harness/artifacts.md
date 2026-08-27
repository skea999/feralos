# Harness — Artifacts

> Canonical path docs. Pointer shim: `tests/harness/README.md`. Global: `AGENTS.md`.

## Root

All harness outputs under repo-relative `tests/artifacts/harness/` (resolved via `detectRepoRoot` in `cmd/skea_harness/main.go`, `artifactsRoot = <repo>/tests/artifacts/harness`). Created on demand; structure preserved by `.gitkeep`.

```
tests/artifacts/harness/
├── cache/
│   ├── base-images/        # Debian genericcloud qcow2 (downloaded once)
│   ├── runner-images/      # skea-runner.qcow2 (overlay+resized 8G)
│   ├── keys/               # id_ed25519 / id_ed25519.pub (guest.EnsureKeyPair)
│   └── web/alpine/         # cache-first Alpine mirror (cache.HTTPMirrorServer)
├── prepare/<timestamp>/    # one per prepare-runner (keep last 1)
│   ├── serial.log, qemu.log
│   ├── seed.iso, user-data, meta-data
│   └── qmp.sock
├── runs/<timestamp>/       # one per run (keep last 2, cleanOldRuns)
│   ├── install-disk-1.qcow2 [, install-disk-2.qcow2]  # 16G default (qemu.CreateDisk)
│   ├── runner-serial.log, runner-qemu.log, runner-qmp.sock
│   ├── installer.log, post-install.log, harness.log (run summary at end), sbctl.log, tpm-post-install.log
│   ├── target-serial.log, target-qemu.log, target-firmware.log, tser.sock, target-qmp.sock
│   ├── verify.log (markdown summary table), OVMF_VARS.fd
│   ├── tpm.sock, tpm.sock.ctrl, tpm-state/, tpm.pid, tpm.log
│   ├── repo.tgz, variables.yaml, git/bash/fdisk shims
│   ├── target-serial-clevis-reboot.offset
│   └── tests/                        # verification results (per-run)
│       ├── summary.md                # markdown table: check | status | detail
│       └── <check_name>.txt          # individual check: PASS/FAIL + detail
└── state/active.json       # runstate.Store — ActiveRun{RunID, Mode, Phase, RunDir, QMP paths, serial logs, SSH ports}
```

## Retention

- `prepare/` keeps last **1**, `runs/` keeps last **2** (`service.cleanOldRuns`). Older dirs pruned at start of next `prepare-runner` / `run`.
- `cache/` never pruned automatically — delete manually to force re-download/rebuild.

## Gitignore

All outputs gitignored; only `.gitkeep` tracked:

```gitignore
# .gitignore (root)
tests/artifacts/**/*
!tests/artifacts/**/
!tests/artifacts/**/.gitkeep
tests/harness/skea-harness
*.qcow2
*.iso
feralos_installer
feralos_create_config
```

The `tests/artifacts/**` pattern covers `cache/`, `prepare/`, `runs/`, `state/` — never commit `*.qcow2`, `*.iso`, `*.log`, or `state/active.json`. See [building.md](building.md) for binary placement.

## Inspecting logs

Per-run logs listed above; `service.Logs` tails them:

```bash
# via binary
go run ./cmd/skea_harness -- logs --log installer --run-dir tests/artifacts/harness/runs/2026-08-27T10-00-00Z --lines 200
# or compiled
./skea-harness logs --log installer
./skea-harness logs --log target-serial --lines 500
./skea-harness logs --log verify
```

Log names: `installer`, `post-install`, `runner-serial`, `runner-qemu`, `target-serial`, `target-qemu`, `verify`, `harness` (`parseLogsArgs` in `cmd/skea_harness/main.go`). Without `--run-dir`, resolves latest run via `runstate`.

Raw inspect: `cat tests/artifacts/harness/runs/<ts>/installer.log`, `tail -f target-serial.log`, `jq . verify.log`.

## Cache reuse

- `cache/base-images/<file>` keyed by URL basename; `cache.EnsureDownloaded` skips if present.
- `cache/runner-images/skea-runner.qcow2` reused unless `--force`.
- `cache/keys/id_ed25519` generated once, reused for runner+target SSH.
- `cache/web/alpine/` persists upstream Alpine APKs; `StartHTTPMirrorServer` serves it guest-side to avoid re-downloading.
