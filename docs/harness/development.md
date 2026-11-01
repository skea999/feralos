# Harness — Development

> How to code the harness. Shim: `tests/harness/README.md`. Global: `AGENTS.md`.

## Module structure

Separate Go module `feralos/tests/harness` (`tests/harness/go.mod`, Go 1.22).

```text
tests/harness/
├── cmd/skea_harness/main.go      # CLI: flag parsing, detectRepoRoot, cleanupVMs, printUsage
├── internal/
│   ├── service/service.go        # Orchestration: PrepareRunner, RunInstall, bootAndVerifyTarget, Verify + helpers
│   ├── qemu/qemu.go              # QEMU: Start/Wait/Kill, CreateDisk/Overlay/ResizeDisk, StartSoftwareTPM, FindOVMF, QMP
│   ├── guest/ssh.go              # SSH: WaitForSSH, CopyToGuest, Run/Output, EnsureKeyPair
│   ├── cache/                    # EnsureDownloaded, StartHTTPMirrorServer
│   ├── runstate/state.go         # ActiveRun persistence to tests/artifacts/harness/state/active.json
│   ├── mcp/                      # stdio MCP server (tools: prepare_runner, run_install_async, etc.)
│   └── config/                   # harness config helpers
├── go.mod, go.sum
├── run_mcp.sh, generate_mcp_config.sh, mcp_server.example.json
└── skea_harness                  # compiled binary (gitignored, see building.md)
```

No `internal` package rule like `src/hostPhase/internal` — harness is a standalone module; all `internal/*` is harness-private by Go semantics.

## Key patterns

### PrepareRunner

`service.PrepareRunner` (`internal/service/service.go:PrepareRunner`):

- `cache.EnsureDownloaded` → `qemu.CreateOverlay` → `qemu.ResizeDisk(cfg.RunnerImageSize)` (8G default — base 327 MB would OOM cloud-init).
- `writeCloudInitFiles` + `buildSeedISO` (cloud-localds) with `growpart`/`resizefs`, packages `tar,curl,parted`, SSH key injection.
- `qemu.Start` (`qemu.LaunchConfig{Binary, Args, LogPath, QMPPath, StartTimeout}`) → `guest.WaitForSSH` → wait `boot-finished` → `guest.Run("sudo poweroff")` or `qemu.SystemPowerdown`.
- `runstate.Save` tracks `Phase` (`booting-runner` → `waiting-for-ssh` → `waiting-for-cloud-init` → `powering-off`).

### RunInstall

`service.RunInstall` (`internal/service/service.go:RunInstall`):

- `LoadVariablesFile` → `cache.StartHTTPMirrorServer` (sets `AlpineMirror` guest URL) → `qemu.CreateDisk` (16G) 1 or 2 disks by `DiskLayout`.
- `qemu.StartSoftwareTPM` **always** (`tpm.sock.ctrl` — QEMU 10 `CMD_SET_DATAFD` protocol, not `--server`).
- `qemu.Start` runner (virtio + hostfwd 2222 + swtpm) → `guest.WaitForSSH` → `createRepositoryArchive` (excludes `tests/artifacts/`, `.git/`, `*.qcow2`) → `guest.CopyToGuest` → `tar -xzf` → inject `variables.yaml` + `git`/`bash`/`fdisk` shims into `.skea_harness-bin` on `PATH` → `guest.Run` of `setup.sh --test -config` with `SKEA_HARNESS_LOCAL_REPO` + `SKEA_NONINTERACTIVE=1` → `installer.log`.
- Poweroff runner, **always** restart swtpm (killed by QEMU `CMD_SHUTDOWN` on poweroff — unconditional, not gated on `EnableTPM`), then `bootAndVerifyTarget`: OVMF `CODE`+`VARS` (`FindOVMF`+`CopyFile`), `virtio-gpu-pci` if `desktopNiri`, socket chardev for `target-serial.log`, hostfwd 2223, retry 3×, then 3-boot: firstBoot `sbctl`/`tpm` via SSH → `poweroff` → restart swtpm → verification boot → `Verify`.

### Logging & execution

- Run phases: `phase(n, name)` prints `[harness] phase N/9: …` for every `run` step (1 boot runner → 2 run installer → 3 reboot runner → 4 boot target → 5 run sbctl → 6 run tpm → 7 reboot target → 8 boot target verification → 9 verify target) to **stdout** and to `runs/<ts>/harness.log` — all `logHarness` messages go to both. MCP mode calls `service.SetProgressWriter(os.Stderr)` to keep the stdio JSON-RPC stream clean.
- Target shutdown is init-aware (`powerOffTarget`): OpenRC → `poweroff`; dinit → `dinit-poweroff` (busybox `poweroff` sends SIGUSR2 which dinit ignores; `dinitctl shutdown` only HALTs). Bounded by `targetShutdownTimeout` (5m) — a guest that never powers off fails the run instead of hanging.
- QEMU logs: `qemu.Start` writes `LogPath` (`runner-qemu.log`/`target-qemu.log`); serial logs via `-serial file:` or `-chardev socket,logfile=` (`runner-serial.log`/`target-serial.log` + `tser.sock`).
- Guest execution: `guest.Run(ctx, opts, cmd, stdout, stderr)` and `guest.Output` (SSH exec, `Shell: bash|sh`); installer captured to `installer.log` file handle.
- No `os.*` file-op bypass — harness is not the installer, but keep I/O observable via logs. Use `qemu.CopyFile`, `os.WriteFile` for artifacts with explicit paths.

### Adding a new check

1. Add entry to `checks` map in `service.Verify` (`internal/service/service.go:Verify`): `checks["my_check"] = "shell command; echo detail"`.
2. **Checks must be falsifiable**: the harness marks a check PASS when the SSH command exits 0 (`Pass: err == nil`). A bare `|| echo '<fail>'` fallback always exits 0 → the check can never go red (this made 8 `dinit_*` checks silently green). Use `COND && echo '<ok>' || { echo '<fail>'; exit 1; }` (or Go post-processing, next step).
3. Add conditional block after the loop if it needs pass/fail logic (see `storage_layout`, `fstab`, `tpm_auto_unlock`, `less_dummy` patterns — flip `result.Success=false` and annotate `Detail`).
4. For desktop-gated checks, guard with `if cfg.DesktopEnvironment == desktopNiri` (see `niri_config_valid` etc.).
5. Wire `VerifyConfig` fields through `bootAndVerifyTarget` → `Verify` call if the check needs new config (e.g., `EnableSwap`, `DiskLayout` already plumbed).
6. Test: `go vet ./...` in `tests/harness`, then `go run ./cmd/skea_harness -- verify --run-dir <run>` or a full `run -f <fixture>`.

## Swtpm always present

Do not gate swtpm on `EnableTPM`. Both runner and target always get `tpmQEMUArgs(tpm.sock.ctrl)`; firstBoot binaries probe `/dev/tpmrm0` at runtime. swtpm restart after runner shutdown is **unconditional** — not conditional on `EnableTPM`. Simplifies `service.RunInstall` and `bootAndVerifyTarget`.

## Code style

```bash
cd tests/harness && go vet ./...
# golangci-lint if available (see AGENTS.md lint section)
```

- Keep `internal/service/service.go` helpers small; extract `writeHarness*`/`createRepositoryArchive` rather than inlining.
- QMP helpers: `qemu.QueryStatus`, `qemu.SystemPowerdown`/`Quit`, `qemu.WriteSerial` — use `runstate` QMP paths, not hardcoded.
- `swtpm` lifecycle: `StartSoftwareTPM` → `StopSoftwareTPM(pidFile)` — always clean up on `KeepTargetRunning=false`.
