# Logging Rule

Source: `AGENTS.md#Logging Rule` (double docs — AGENTS.md is source of truth, this copy is reference).

## Rule

**Commands: `shell.ExecuteCmd`; file writes: `shell.WriteFile`/`shell.WriteFileAppend` — in `hostPhase`/`chrootPhase`, never raw `os.*`.**

File ops (`MkdirAll`, `Create`, `WriteFile`, `ReadFile`) and `exec.Command` bypass logging. `ExecuteCmd` is the only path that guarantees observability for COMMANDS; `shell.WriteFile` is the sanctioned logged path for FILE WRITES (P2-5).

`import "os"` is forbidden in those packages.

## Commands — What ExecuteCmd Logs

`src/utils/shell/execute.go` — `ExecuteCmd(chroot, cmd)` / `ExecuteCmdWithAllowFail`:

- **CMD**: timestamp + tag (`CMD` host, `CHROOT` chroot) + full shell string
- **stdout / stderr**: line-by-line, prefixed `[stdout]` / `[stderr]`
- **exit**: exit code + status `OK`/`FAIL` + duration (`[RESULT] exit=0 time=...`)
- **retry**: up to `MaxRetries=3`, 3s delay, `[RETRY] Attempt N/3 failed`
- **SKEA_NONINTERACTIVE handling**: after 3 failures
  - `SKEA_NONINTERACTIVE=1` + `SKEA_NONINTERACTIVE_DECISION=ignore|skip` → warn, continue
  - `SKEA_NONINTERACTIVE=1` + other/unset → abort, return error
  - interactive → prompt `[s]top / [i]gnore / [r]etry`: `r` resets the full 3-attempt cycle (command re-executes immediately), `i` returns last stdout as success, `s` aborts installer

See `docs/INSTALLER.md#Error Handling` for retry/user-feedback detail.

## File Writes — shell.WriteFile / shell.WriteFileAppend (P2-5)

`src/utils/shell/write.go` — `WriteFile(chroot, path, content []byte, perm)` / `WriteFileAppend`:

- **Direct write** on the chroot mount (`filepath.Join(chroot, path)` — no chroot(2)); empty chroot = host path.
- **Logged**: `[WRITE]` / `[APPEND]` line (target path + byte size; content NEVER logged) + `[RESULT] ok|FAIL` line, same timestamp/caller style as ExecuteCmd.
- **Parent MkdirAll** (0755) first; **perm forced via chmod when the file is newly created** (umask-independent; an existing file keeps its mode — same as `echo > file` truncating in place).
- **SKEA_IGNORE_ERRORS=1** honored: failure logs `[ERROR][IGNORED] write failed (ignoreErrors=true)` and returns nil (same best-effort semantics as ExecuteCmd — harness runs stay non-fatal).
- **Why the rule changed**: the former `echo '<escaped>' > path` pattern (via ExecuteCmd) required single-quote escaping of arbitrary content — one missed `'` corrupts the seed — and very large payloads can hit ARG_MAX. The escaped-echo call sites were converted to `WriteFile`; the pattern is forbidden for new code.

### Keep ExecuteCmd for

- **Conditional appends**: `grep -q … || echo … >> file` — the idempotency guard is part of the operation (e.g. PAM lines, `/etc/shells`).
- **Shell-runtime-computed writes**: e.g. the zswap `awk /proc/meminfo` append.
- **In-place edits**: sed/awk transforms of existing files.

## Host vs Chroot

```go
// host command — empty chroot, runs via /bin/sh -c
shell.ExecuteCmd("", "mkdir -p /mnt/etc")

// chroot command — non-empty, runs via chroot <dir> /bin/sh -c
shell.ExecuteCmd(chroot, "mkdir -p /etc")

// chroot file write — direct on the host mount
shell.WriteFile(chroot, "/etc/hostname", []byte(hostname+"\n"), 0644)

// unconditional append
shell.WriteFileAppend(chroot, "/etc/crontabs/root", []byte(cronLine+"\n"), 0644)
```

## Bypass = Bug

`os.MkdirAll`, `os.Create`, `os.WriteFile`, `os.ReadFile`, bare `exec.Command` bypass logging — never use in `hostPhase`/`chrootPhase` (write files through `shell.WriteFile`; run commands through `shell.ExecuteCmd`). Every command and file write must be visible in installer logs and harness artifacts.

## Exceptions

- `src/firstBoot/*` — standalone binaries, run outside installer (own logging).
- `src/utils/shell` — implementation itself (uses `os`/`exec` internally).

### Documented exec carve-outs (secrets)

Two deliberate direct-exec sites bypass `ExecuteCmd` because the secret must never appear in logs (`ExecuteCmd` logs CMD + output verbatim):

- **passwd via stdin pipe** — `src/chrootPhase/internal/users.go` (root + user password): `exec.Command` with `Stdin` reader.
- **cryptsetup stdin** — `src/hostPhase/disks/common/luks.go` (`LuksFormat`/`LuksOpen`/`LuksAddKey`): LUKS password/keyfile via `Stdin` reader.

Both are comment-annotated in source. Adding a new carve-out requires a comment at the site + an entry here and in `AGENTS.md#Logging Rule`.

## References

- `AGENTS.md#Logging Rule`
- `src/utils/shell/execute.go`
- `src/utils/shell/write.go`
- `docs/adr/README.md` → ADR-004 ExecuteCmd
