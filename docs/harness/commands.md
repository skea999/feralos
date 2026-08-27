# Harness — Commands

> CLI and MCP reference. Shim: `tests/harness/README.md`. Orchestration: `internal/service/service.go`. CLI parsing: `cmd/skea_harness/main.go`.

## CLI

Binary: `tests/harness/skea-harness` (build: `cd tests/harness && go build -o skea-harness ./cmd/skea_harness/`) or `go run ./cmd/skea_harness -- <cmd>`. All commands emit pretty JSON to stdout; errors to stderr.

### `prepare-runner`

Download/cache Debian cloud image and build reusable runner qcow2.

```bash
go run ./cmd/skea_harness -- prepare-runner [--force] [--artifacts-root <path>] [--cache-dir <path>]
  [--runner-image <path>] [--base-image-url <url>] [--base-image-path <qcow2>]
  [--qemu-binary <bin>] [--qemu-img-binary <bin>] [--cloud-localds-binary <bin>]
  [--apt-proxy <url>] [--ssh-user <user>] [--ssh-port <port>] [--memory <MiB>] [--cpus <n>]
  [--boot-timeout-seconds <s>] [--enable-kvm=true|false]
```

Defaults: `artifactsRoot=<repo>/tests/artifacts/harness`, `cacheDir=<artifacts>/cache`, `runnerImage=<cache>/runner-images/skea-runner.qcow2`, `baseImageURL=cloud.debian.org trixie genericcloud`, `qemu-system-x86_64`/`qemu-img`/`cloud-localds`, `sshPort=2222`/`sshUser=skea`, `memory=4096`/`cpus=2`, `bootTimeout=5m`, `enableKVM=true`. Impl: `service.PrepareRunner`.

### `run`

Boot runner, copy repo, run `setup.sh --test`, boot target, verify. Requires `-f`.

```bash
go run ./cmd/skea_harness -- run -f <path-to-variables.yaml>
  [--artifacts-root <path>] [--runner-image <path>] [--qemu-binary <bin>] [--qemu-img-binary <bin>]
  [--runner-ssh-user <u>] [--runner-ssh-port <p>] [--target-ssh-port <p>]
  [--memory <MiB>] [--target-memory <MiB>] [--cpus <n>] [--disk-size <size>]
  [--boot-timeout-seconds <s>] [--verify-timeout-seconds <s>]
  [--enable-kvm=true|false] [--keep-target-running]
```

Defaults: `runnerPort=2222`/`targetPort=2223`, `memory=4096`/`targetMemory=4096`, `diskSize=16G`, `bootTimeout=5m`/`verifyTimeout=2m`. `-f` loads `variables.yaml` via `service.LoadVariablesFile` (diskLayout, enable* flags, desktopEnvironment, mirror). Cleans stale VMs unless `KeepTargetRunning`. Impl: `service.RunInstall`.

### `verify`

Run checks against a reachable target.

```bash
go run ./cmd/skea_harness -- verify [--run-dir <runDir>] [--artifacts-root <path>]
  [--ssh-port <p>] [--ssh-user <user>] [--private-key <path>] [--installed-user <u>] [--timeout-seconds <s>]
```

Defaults: `sshPort=2223`/`sshUser=root`/`privateKey=<artifacts>/cache/keys/id_ed25519`/`installedUser=skea`/`timeout=2m`. Impl: `service.Verify`.

### `status` / `stop` / `cleanup` / `logs`

```bash
go run ./cmd/skea_harness -- status
# reads tests/artifacts/harness/state/active.json, queries QMP if present
# impl: service.Status

go run ./cmd/skea_harness -- stop [--force]
# QMP system_powerdown (or quit with --force) for active runner/target
# impl: service.Stop

go run ./cmd/skea_harness -- cleanup [--force]
# kill stale QEMU/swtpm by pgrep, clear active.json (refuses if active still healthy unless --force)
# impl: service.Cleanup (also called automatically before prepare-runner/run)

go run ./cmd/skea_harness -- logs [--run-dir <runDir>] [--log <name>] [--lines <n>]
# tails a log; names: installer, post-install, runner-serial, runner-qemu, target-serial, target-qemu, verify
# defaults: log=installer, lines=200
# impl: service.Logs
# example: go run ./cmd/skea_harness -- logs --log installer --run-dir tests/artifacts/harness/runs/2026-08-27T10-00-00Z
```

### `mcp`

```bash
go run ./cmd/skea_harness -- mcp [--repo-root <path>] [--artifacts-root <path>]
# stdio MCP server (internal/mcp). Wrapper: ./tests/harness/run_mcp.sh auto-rebuilds artifacts/harness/bin/skea-harness
# generate config: ./tests/harness/generate_mcp_config.sh
```

## MCP tools

Exposed by `mcp` server (`internal/mcp/`, `mcp_server.example.json`, `.vscode/mcp.json` delegates to `run_mcp.sh`):

| Tool | Maps to |
|------|---------|
| `prepare_runner` | `service.PrepareRunner` |
| `run_install_async` | `service.RunInstall` (async) |
| `get_status` | `service.Status` |
| `stop_active_run` | `service.Stop` |
| `cleanup_stale` | `service.Cleanup` |
| `tail_log` | `service.Logs` |
| `write_serial` | `qemu.WriteSerial` to `target-serial.sock` (or `runDir` if given) — for initramfs LUKS prompt |
| `verify_target` | `service.Verify` |

Before `prepare-runner`/`run`, harness auto-runs stale-state cleanup; `cleanup --force` kills even healthy-tracked VMs.
