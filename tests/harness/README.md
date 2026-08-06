# Skea Harness

This directory contains the Go-based QEMU harness for automated runner preparation, user-equivalent installation runs, target verification, and MCP exposure.

KVM acceleration is enabled by default for runner and install VMs. Disable it only when the host does not provide `/dev/kvm`.

## Commands

The harness exposes these CLI commands:

- `prepare-runner`: download or reuse a Debian cloud image and prepare a reusable runner qcow2 image with the packages needed by `start.sh`.
- `run`: boot the runner VM, copy the repository into it, run `setup.sh`, boot the installed target in a second QEMU phase, run `/root/post-install.sh` when TPM is enabled, and verify the resulting Alpine system.
- `verify`: run the verification checks against a reachable installed target VM.
- `status`: read the persisted active-run state and query the active QMP socket when present.
- `stop`: stop the active runner or target VM via QMP.
- `cleanup`: clear stale active-run state and kill stale harness-owned QEMU instances before the next run.
- `logs`: return the tail of a named harness log.
- `mcp`: start the harness MCP server on stdio.

## Quick start

From inside `installer/tools/harness/`:

```bash
go run ./cmd/skea-harness prepare-runner
go run ./cmd/skea-harness run --keep-target-running
go run ./cmd/skea-harness status
go run ./cmd/skea-harness cleanup
go run ./cmd/skea-harness logs --log installer
go run ./cmd/skea-harness stop
```

## Artifacts

By default, the harness stores everything under:

```text
<repo>/tests/artifacts/harness/
```

Important subdirectories:

- `cache/`: reusable cloud image downloads, SSH keys, and prepared runner images.
- `cache/base-images/`: Debian cloud images (downloaded once, reused).
- `cache/runner-images/`: prepared runner qcow2 with installer dependencies.
- `cache/keys/`: SSH key pairs for target access.
- `cache/web/alpine/`: persistent cache-first mirror store for Alpine packages.
- `prepare/`: runner-image preparation logs.
- `runs/`: per-install run directories containing target disks and all logs.
- `state/active.json`: persisted active run metadata used by `status`, `stop`, and the MCP server.

All files are gitignored. Only `.gitkeep` files are tracked to preserve directory structure.

## Important logs

Per run, the harness writes these files when relevant:

- `runner-qemu.log`
- `runner-serial.log`
- `installer.log`
- `post-install.log`
- `target-qemu.log`
- `target-serial.log`
- `verify.log`

Use:

```bash
go run ./cmd/skea-harness logs --run-dir /abs/path/to/run --log installer
```

## Installer integration

On `btrfsRaid`, the harness keeps the installer contract intentionally small.

- `start.sh` and `chroot.sh` can source a hidden automation override file through `SKEA_RUNTIME_OVERRIDES_FILE`.
- `executeCmd` supports `SKEA_NONINTERACTIVE=1` with `SKEA_NONINTERACTIVE_DECISION=stop|ignore`.

The harness leaves the user-facing scripts alone and injects automation-only settings through `.skea-harness-overrides.sh` for:

- `console=ttyS0,115200` for serial-friendly boot debugging,
- a DHCP config on `eth0` for automated target boot verification,
- Secure Boot signing key and certificate paths during harness runs.

The user-facing values that normally come from `setup.sh` prompts (disk choices, username, password, TPM selection, SSH key, mirror) are fed through a generated setup input file so the harness still exercises the same top-level script a real user would run.

### TPM / Clevis flow

When TPM is enabled (`enableTPM=yes`), the runner VM is started **with the same swtpm device** that the target VM will later use. This lets `chroot.sh` bind Clevis tokens to both LUKS volumes **at install time** via the available `/dev/tpmrm0`.

After the runner powers off, swtpm exits (QEMU sends `CMD_SHUTDOWN`). The harness **restarts swtpm** — preserving `tpm-state/` — before booting the target, so the sealed keys are valid for the target's OVMF boot.

On the first target boot the harness waits for SSH. If Clevis tokens are already enrolled (the common case), booster auto-unlocks both volumes without prompting. If tokens are absent (e.g., swtpm was unavailable during install), the harness injects **two passphrases concurrently** on the first serial prompt so both KDF operations start simultaneously, preventing a btrfs RAID1 open_ctree race.

`/root/post-install.sh` is always run over SSH after the first boot. It exits immediately when all roots already have Clevis TPM2 bindings, so it is idempotent. It enrolls and rebuilds the UKI only when bindings are missing.

The target then reboots for a second boot, which validates that TPM auto-unlock works unattended.

## MCP server

The harness exposes a stdio MCP server through:

```bash
go run ./cmd/skea-harness mcp
```

For path-agnostic local launches from a built workspace, use:

```bash
./installer/tools/harness/runMcp.sh
```

The wrapper rebuilds `artifacts/harness/bin/skea-harness` automatically when the binary is missing or older than the current harness Go sources.

After building the harness binary, you can generate a ready-to-use absolute-path MCP config with:

```bash
./installer/tools/harness/generateMcpConfig.sh
```

Available MCP tools:

- `prepare_runner`
- `run_install_async`
- `get_status`
- `stop_active_run`
- `cleanup_stale`
- `tail_log`
- `write_serial`
- `verify_target`

`write_serial` writes raw data to `target-serial.sock` for the active run by default, or for a specific `runDir` when provided. This is intended for cases like manually answering the initramfs LUKS prompt.

An example client configuration is provided in `mcpServer.example.json`.

The checked-in workspace config in `.vscode/mcp.json` delegates to `installer/tools/harness/runMcp.sh`. The wrapper resolves the real repository path from its own location so symlinked workspaces under `/media` still work.

Before `prepare-runner` and `run`, the harness now performs stale-state cleanup automatically. If an actual active run is still healthy, `cleanup` refuses to kill it unless you pass `--force`.
