# Logging Rule

Source: `AGENTS.md#Logging Rule` (double docs — AGENTS.md is source of truth, this copy is reference).

## Rule

**Always use `shell.ExecuteCmd` in `hostPhase`/`chrootPhase` — never `os.*`.**

File ops (`MkdirAll`, `Create`, `WriteFile`, `ReadFile`) and `exec.Command` bypass logging. `ExecuteCmd` is the only path that guarantees observability.

`import "os"` is forbidden in those packages.

## What ExecuteCmd Logs

`src/utils/shell/execute.go:47` — `ExecuteCmd(chroot, cmd)` / `ExecuteCmdWithAllowFail`:

- **CMD**: timestamp + tag (`CMD` host, `CHROOT` chroot) + full shell string
- **stdout / stderr**: line-by-line, prefixed `[stdout]` / `[stderr]`
- **exit**: exit code + status `OK`/`FAIL` + duration (`[RESULT] exit=0 time=...`)
- **retry**: up to `MaxRetries=3`, 3s delay, `[RETRY] Attempt N/3 failed`
- **SKEA_NONINTERACTIVE handling**: after 3 failures
  - `SKEA_NONINTERACTIVE=1` + `SKEA_NONINTERACTIVE_DECISION=ignore|skip` → warn, continue
  - `SKEA_NONINTERACTIVE=1` + other/unset → abort, return error
  - interactive → prompt `[s]top / [i]gnore / [r]etry`: `r` resets the full 3-attempt cycle (command re-executes immediately), `i` returns last stdout as success, `s` aborts installer

See `docs/INSTALLER.md#Error Handling` for retry/user-feedback detail.

## Host vs Chroot

`chroot` param controls execution:

```go
// host — empty chroot, runs via /bin/sh -c
shell.ExecuteCmd("", "mkdir -p /mnt/etc")

// chroot — non-empty, runs via chroot <dir> /bin/sh -c
shell.ExecuteCmd(chroot, "mkdir -p /etc")
```

Host: `shell.ExecuteCmd("", "mkdir -p /mnt/etc")`.
Chroot: `shell.ExecuteCmd(chroot, "mkdir -p /etc")`.

All hostPhase file writes use this pattern (`cat > ... <<'EOF'`, `echo`, `mkdir -p`) instead of `os.WriteFile`.

## Bypass = Bug

`os.MkdirAll`, `os.Create`, `os.WriteFile`, `os.ReadFile`, bare `exec.Command` bypass logging — never use in `hostPhase`/`chrootPhase`. Every command must be visible in installer logs and harness artifacts.

## Exceptions

- `src/firstBoot/*` — standalone binaries, run outside installer (own logging).
- `src/utils/shell` — implementation itself (uses `os`/`exec` internally).

### Documented exec carve-outs (secrets)

Two deliberate direct-exec sites bypass `ExecuteCmd` because the secret must never appear in logs (`ExecuteCmd` logs CMD + output verbatim):

- **passwd via stdin pipe** — `src/chrootPhase/internal/users.go` (root + user password): `exec.Command` with `Stdin` reader.
- **cryptsetup stdin** — `src/hostPhase/disks/common/luks.go` (`LuksFormat`/`LuksOpen`/`LuksAddKey`): LUKS password/keyfile via `Stdin` reader.

Both are comment-annotated in source (`polishCode P3-8`). Adding a new carve-out requires a comment at the site + an entry here and in `AGENTS.md#Logging Rule`.

## References

- `AGENTS.md#Logging Rule`
- `src/utils/shell/execute.go:47`
- `docs/adr/README.md` → ADR-004 ExecuteCmd
