# Harness — debug_run

> Boot existing harness run without reinstalling. Shim: none. Source: `tests/debug_run/main.go`. Project rules: `AGENTS.md`. See [workflow.md](workflow.md) (3-phase), [artifacts.md](artifacts.md).

## Overview

`tests/debug_run` reboots an already-installed target VM from `tests/artifacts/harness/runs/<timestamp>/` — no `prepare-runner`, no `setup.sh` reinstall. Auto-detects latest run when `--run-dir` omitted (`latestRunDir` in `tests/debug_run/main.go`). Inspect failed install, retry target boot, check post-install state, manual SSH, LUKS/TPM debug.

## When to use

- `skea-harness run` failed at target boot / verification — retry boot without full reinstall.
- Inspect installed system (files, services, LUKS, TPM, sbctl) after harness `run`.
- Manual SSH session against installed Alpine target.
- Debug LUKS passphrase handling or TPM auto-unlock without re-running installer.

## Quick start

```bash
cd tests/debug_run && go build -o debug_run . && ./debug_run
# auto-detects latest run in tests/artifacts/harness/runs/

./debug_run --run-dir ~/Progetti/skeaAlpineInstaller/tests/artifacts/harness/runs/2026-08-26T12-00-00Z --ssh-port 2224 --password Alpine.1

# after "=== SSH ready ==="
ssh -p 2224 root@127.0.0.1
# or printed form: ssh -i tests/artifacts/harness/cache/keys/id_ed25519 -p 2224 root@127.0.0.1

# Ctrl+C in debug_run terminal → guest poweroff via SSH, fallback QMP powerdown
```

Defaults: `--ssh-port 2224`, `--memory 4096`, `--cpus 2`, `--enable-kvm=true`, `--qemu-binary qemu-system-x86_64`, `--password Alpine.1`, `--boot-timeout-seconds 300`, `--run-dir ""` (latest).

## How it works

Reuses harness logic as standalone copy (no `tests/harness/internal` import).

1. **Run dir resolve** — `detectArtifactsRoot` walks up from `cwd` to repo root (`setup.sh` or `tests/artifacts/harness` ancestor) → `artifactsRoot`; `latestRunDir(artifactsRoot/runs)` picks lexicographically last directory when `--run-dir` empty; else uses `--run-dir` verbatim. Loads `variables.yaml` (`loadVariables`) for `DiskLayout`, `EnableTPM`, `UserName` (SSH user), SSH key via `ensureKeyPair(cache/keys)`.

2. **OVMF** — `findOVMF` probes `OVMF_CODE.fd`/`VARS` candidates (`/usr/share/OVMF/`, `/usr/share/edk2/ovmf/`). Copies `OVMF_VARS.fd` to `OVMF_VARS.fd` in run dir if missing (`copyFile`).

3. **swtpm** — `startSwtpm` with `tpm.sock` (data), `tpm.sock.ctrl` (ctrl `type=unixio`), `tpm-state/`, `tpm.pid`, `tpm.log` under run dir; `socket --tpm2 --daemon --flags not-need-init,startup-clear`; waits for `tpm.sock.ctrl` + `tpm.pid`; stopped via `stopSwtpm(tpm.pid)` on exit.

4. **QEMU** — `buildTargetArgs` (`tests/debug_run/main.go:buildTargetArgs`) builds args: `q35,smm=on`, `OVMF_CODE` (ro pflash) + `OVMF_VARS` (pflash), `disk1 install-disk-1.qcow2` + `disk2` if `btrfsStandardServer` (+ `btrfs-disk-*.qcow2` if present), `virtio-net hostfwd 2224→22`, `swtpm chardev tpm-tis` (always), KVM if `--enable-kvm`, serial via `-chardev socket,path=d-tser.sock,server=on,wait=off,logfile=d-serial.log,logappend=on` + `-serial chardev:targetserial`, QMP `d-qmp.sock`, firmware `d-firmware.log`, debugcon `isa-debugcon.iobase=0x402`. Started via `qemuStart` (`tests/debug_run/main.go:qemuStart`) with persistent `exec.Command` (not `CommandContext`), log `d-qemu.log`, QMP wait, early-exit guard.

5. **waitForSSH** — `waitForSSH` (`tests/debug_run/main.go:waitForSSH`) polls `sshRun("true")` every 2 s until `BootTimeout`. LUKS handling: counts `"Enter passphrase for "` in `d-serial.log` (`serialLogCount`), injects `--password` via `writeSerial(d-tser.sock, password+"\r")` (dial unix socket, write text, `\r`, sleeps) — at most 5 attempts, 5 s intervals (`unlockAttempts`, `lastPromptUnlock`).

6. **Ready + shutdown** — prints `ssh -i <key> -p <port> <user>@127.0.0.1`, blocks on `signal.NotifyContext(Interrupt, SIGTERM)` (Ctrl+C), then `sshRun("poweroff")` else `qmpSystemPowerdown(d-qmp.sock)` via `qmpExecute` (`qmp_capabilities` + `system_powerdown`), `instance.wait()`.

## Artifacts

Reuses given run dir — no new disk created. Writes under that `runs/<timestamp>/`:

- `d-serial.log`, `d-qemu.log`, `d-firmware.log` — target serial/QEMU/firmware logs for this debug boot.
- `d-qmp.sock`, `d-tser.sock` — QMP and serial sockets.
- `tpm.log`, `tpm.sock`, `tpm.sock.ctrl`, `tpm-state/`, `tpm.pid` — swtpm state (reused/overwritten).
- `OVMF_VARS.fd` — copied once if missing.

All under `tests/artifacts/harness/runs/` → gitignored via `tests/artifacts/**/*` (`.gitignore:47`). No new `runs/<timestamp>/` directory is created by `debug_run`.

## Building

```bash
cd tests/debug_run && go build -o debug_run .
./debug_run --run-dir tests/artifacts/harness/runs/2026-08-27T10-00-00Z
# alternative without binary
go run . -- --run-dir tests/artifacts/harness/runs/2026-08-27T10-00-00Z
```

Module `skeaAlpineInstaller/tests/debug_run` (`tests/debug_run/go.mod`, Go 1.22, `gopkg.in/yaml.v3`). Binary `tests/debug_run/debug_run` is gitignored (`.gitignore:44` `tests/debug_run/debug_run`) — do not commit.

Verify: `git check-ignore -v tests/debug_run/debug_run` → `.gitignore:44:tests/debug_run/debug_run`.

## Development

- **File** — `tests/debug_run/main.go` (808 lines), single `package main`, standalone (no `tests/harness/internal` import — minimal deps, copy of harness logic).

- **Key funcs** (`tests/debug_run/main.go`):
  - `qemuStart` — start QEMU, tail logs, QMP socket wait, early-exit check.
  - `qmpExecute` / `qmpReadResponse` / `qmpSystemPowerdown` / `qmpQuit` — QMP JSON over unix socket.
  - `findOVMF` / `copyFile` — OVMF locate + copy.
  - `startSwtpm` / `stopSwtpm` — swtpm daemon lifecycle.
  - `sshArgs` / `sshRun` — SSH invocation.
  - `writeSerial` / `serialLogCount` — LUKS serial injection + prompt counting.
  - `waitForSSH` — SSH poll + LUKS handling (5 attempts, 5 s intervals).
  - `ensureKeyPair` — `ssh-keygen -t ed25519` in `cache/keys`.
  - `buildTargetArgs` — QEMU arg builder (q35, OVMF, disks, net, TPM, KVM, serial, QMP, firmware).
  - `latestRunDir` / `detectArtifactsRoot` — run discovery.
  - `run` / `main` — flag parse, orchestration, Ctrl+C shutdown.

- **To add checks** — modify `waitForSSH` (e.g., extra prompt strings) or add post-SSH steps after the `waitForSSH` return and before `<-ctx.Done()` (e.g., `sshRun` probes, then print results before blocking).

## References

- `tests/debug_run/main.go`, `tests/debug_run/go.mod`
- `AGENTS.md` — project rules, build/test, naming, harness overview
- [workflow.md](workflow.md) — 3-phase harness flow + 3-boot
- [artifacts.md](artifacts.md) — `tests/artifacts/harness/` layout, retention, gitignore
- [building.md](building.md) — binary placement / gitignore pattern
- [development.md](development.md) — harness code patterns (swtpm always, QMP helpers)
- `tests/artifacts/harness/runs/<ts>/variables.yaml` — per-run config consumed by `loadVariables`
