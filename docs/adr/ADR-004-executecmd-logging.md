# ADR-004: Mandatory shell.ExecuteCmd Logging

- **Status:** Accepted
- **Date:** 2026-08-26
- **Deciders:** FeralOS team

## Context

Installer runs hundreds of shell commands across host and chroot. Need uniform observability for debugging and harness artifacts: CMD, stdout/stderr, exit code, retry, non-interactive handling. Direct `os.*` and bare `exec.Command` bypass logging. Sources: `AGENTS.md#Logging Rule`, `docs/architecture/logging.md`, `src/utils/shell/execute.go:47`, `docs/INSTALLER.md#Error Handling`.

## Decision

- All `hostPhase` and `chrootPhase` code **must** use `shell.ExecuteCmd(chroot, cmd)` / `ExecuteCmdWithAllowFail`.
- `import "os"` forbidden in those packages. File ops via shell (`mkdir -p`, `cat > ... <<'EOF'`, `echo`) so every operation is logged.
- `ExecuteCmd` logs: timestamp + tag (`CMD`/`CHROOT`) + command, `[stdout]`/`[stderr]` lines, `[RESULT] exit= code time=`, `[RETRY] Attempt N/3` (MaxRetries=3, 3s delay), `SKEA_NONINTERACTIVE` handling (ignore → warn continue, else abort; interactive → `[s]top/[i]gnore/[r]etry`).

Usage:

```go
shell.ExecuteCmd("", "mkdir -p /mnt/etc") // host
shell.ExecuteCmd(chroot, "mkdir -p /etc") // chroot via chroot <dir> /bin/sh -c
```

## Consequences

- `os.MkdirAll`, `os.Create`, `os.WriteFile`, `os.ReadFile`, bare `exec.Command` banned — violation = bug.
- Every command visible in installer logs and harness artifacts; retry logic centralized.
- Exceptions: `firstBoot/*` (standalone binaries, own logging) and `utils/shell` (implementation itself).

## References

- `AGENTS.md#Logging Rule`, `docs/architecture/logging.md`, `src/utils/shell/execute.go:47`, `docs/adr/README.md`
