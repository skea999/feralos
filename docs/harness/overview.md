# Harness — Overview

> Pointer: `tests/harness/README.md` is the repo-relative shim. This is the canonical detail. Global rules: `AGENTS.md`.

## What it does

Go QEMU harness that automates the full FeralOS install lifecycle without touching user-facing installer scripts:

- **Runner preparation** — download/cache a Debian cloud image, create a reusable runner qcow2.
- **Installation runs** — boot runner VM, copy local repo, run `setup.sh --test`, boot the installed Alpine target, run first-boot binaries.
- **Target verification** — SSH into the installed target and run ~30+ checks (`internal/service/service.go:Verify`).
- **MCP exposure** — stdio MCP server (`internal/mcp/`) so agents can drive the harness.

## How it simulates a real install

```
local repo (host) ──tar.gz──▶ runner VM (Debian) ──setup.sh──▶ disks (qcow2) ──boot──▶ target VM (Alpine) ──verify──▶ result
```

- **Not a git clone** — `createRepositoryArchive` (`internal/service/service.go`) tars the *local* repo tree, excluding `tests/artifacts/`, `.git/`, `*.qcow2`; `guest.CopyToGuest` + `tar -xzf` unpacks it inside the runner. Tests exactly what is on disk.
- Runner boots via QEMU `virtio` with `hostfwd tcp::2222-:22`; target via OVMF + `virtio-blk` + `hostfwd tcp::2223-:22`.
- **KVM default** — `-enable-kvm` when `/dev/kvm` exists (`EnableKVM` flag in `cmd/skea_harness/main.go`). Pass `--enable-kvm=false` only on hosts without KVM.

## Fixtures

Curated `variables.yaml` templates at `tests/fixtures/variables/` — 14 files covering all scenario axes (see `tests/fixtures/variables/README.md`):

- Layouts: `btrfsStandard` (1-disk single), `btrfsRaid0` (2-disk RAID0), `btrfsStandardServer` (2-disk RAID1)
- Swap: `auto` (2×RAM via `src/utils/swap`), explicit `8G`/`4096M`, `noswap`
- Features: `secureboot`/`tpm`/`hardening` isolate + `niri` desktop + `full_stack`

Tier1 CI gate: `minimal` + `noswap` per layout. Tier2: explicit/isolate. Tier3: full. Each `run` requires `-f <path-to-variables.yaml>`.

## Key code paths

- CLI: `tests/harness/cmd/skea_harness/main.go` (flag parsing, `detectRepoRoot`, `cleanupVMs`)
- Orchestration: `tests/harness/internal/service/service.go` (`PrepareRunner`, `RunInstall`, `bootAndVerifyTarget`, `Verify`)
- QEMU: `tests/harness/internal/qemu/qemu.go` (`Start`, `CreateOverlay`, `ResizeDisk`, `StartSoftwareTPM`, `FindOVMF`)
- Guest SSH: `tests/harness/internal/guest/ssh.go` (`WaitForSSH`, `CopyToGuest`, `Run`, `Output`)
- Cache/state: `tests/harness/internal/cache/`, `tests/harness/internal/runstate/state.go`
