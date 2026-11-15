# log-notify — standalone repo handoff spec

**Purpose**: bootstrap a NEW, independent repo (`log-notify`) from zero. This file is the complete brief: an agent (or human) reading only this must be able to scaffold, implement, test, and ship v0.1 without touching the FeralOS installer. log-notify is a GENERIC tool that happens to ship a FeralOS-flavored default config; it has nothing to do with the installer and no FeralOS code dependency.

**Origin decisions (2026-11-15, FeralOS security-stack round)**:
- Read logs only (no native tool hooks like clamd `VirusEvent`) — uniform relay-on-log.
- Editable config declaring WHICH logs to read + per-source regex patterns.
- Name: `log-notify`. Separate repo from FeralOS.
- Default config shipped IN-REPO is the FeralOS security-stack set (others modify it for their needs).
- No notification replay/re-emission after login.
- Language: **Rust** (memory-safe daemon; binary weight is tiny — inotify-driven, no polling, few MB RSS).

---

## 1. Project

- **Name**: `log-notify`
- **Type**: single-binary Rust daemon (user-session service)
- **One-liner**: tail configured log files, match regex patterns, emit freedesktop desktop notifications.
- **Platform**: Linux (inotify); Wayland/X11 agnostic (notifications go through D-Bus `org.freedesktop.Notifications`).
- **License**: recommend dual MIT / Apache-2.0 (Rust ecosystem convention) — DECIDE before first publish.
- **Repo layout** (suggested):
  ```
  Cargo.toml
  src/main.rs            # CLI + startup + config load + fail-loud validation
  src/config.rs          # TOML parse + validation (regex compile at startup)
  src/tail.rs            # inotify tailer: offsets, partial lines, shrink reset
  src/notify.rs          # notification backend
  src/cooldown.rs        # per-source rate limiting
  tests/                 # pattern engine + rotation/truncation unit tests
  config/log-notify.toml # THE default config (FeralOS sources — see §4)
  README.md              # generic tool doc: usage + config reference
  ```

## 2. Runtime behavior

- Reads N files from config, from END (offset = current size at startup; **no history replay at startup**).
- On new data: read complete lines only (buffer partial trailing line until `\n`), apply the source's regex per line, on match emit a notification.
- **Truncation/shrink detection** (copytruncate rotation): if current file size < last offset → reset offset to 0 and re-read (also guards inode-change: if `stat().st_ino` differs → reopen path from end).
- **Line cap**: truncate lines longer than 8 KiB before matching (tracee emits long JSON lines).
- **Cooldown per source** (v0.1): after a notification, suppress further matches from the SAME source for N seconds (default 60; per-source override). Tracee without cooldown would spam.
- **No notification daemon available** (D-Bus call fails / no session): drop the event silently, keep tailing, log to stderr. NEVER retry-spam.
- **No re-emission/persistence**: no cursor files, no unread counters — logs themselves are the record.
- **Graceful shutdown**: SIGTERM/SIGINT → close watchers, exit 0.
- **Single instance**: advisory lockfile (`$XDG_RUNTIME_DIR/log-notify.lock`) — second instance exits with error.
- **Logging**: one line per event to stderr (the service manager captures it); `--verbose` for tail-debug lines.
- **Signals**: SIGHUP → config reload (re-validate; on invalid config keep running with the old one, print error) — acceptable to defer to v0.2, but design the config load so reload is possible.

## 3. CLI

```
log-notify [--config PATH] [--dry-run] [--once] [--verbose] [--version]
```
- `--config` default `/etc/log-notify/log-notify.toml`, then `~/.config/log-notify/log-notify.toml` if the first is absent.
- `--dry-run`: everything runs, matched events print to stdout instead of notifying (pattern testing).
- `--once`: exit after the first match (scripting/testing).
- Config validation failure at startup = exit non-zero with a precise message (file, line, reason). Fail LOUD — silent misconfiguration is the worst failure mode for a security notifier.

## 4. Config format (TOML) + the shipped default

```toml
# /etc/log-notify/log-notify.toml
[general]
line_cap_bytes = 8192

[[source]]
path = "/var/log/fail2ban.log"
pattern = ' Ban [0-9]+\.[0-9]+\.[0-9]+\.[0-9]+ '
summary = "fail2ban ban"
body    = "{line}"
urgency = "normal"
cooldown_secs = 60

[[source]]
path = "/var/log/clamav/clamd.log"
pattern = ' FOUND$'
summary = "clamav detection"
body    = "{line}"
urgency = "critical"
cooldown_secs = 0

[[source]]
path = "/var/log/dinit/tracee.log"
# TODO verify against real tracee JSON output — match signature-detection events only
pattern = '"signature"'
summary = "tracee detection"
body    = "{line}"
urgency = "critical"
cooldown_secs = 30

[[source]]
path = "/var/log/aide/aide.log"
pattern = '^(added|removed|changed): '
summary = "aide file change"
body    = "{line}"
urgency = "normal"
cooldown_secs = 0

[[source]]
path = "/var/log/audit/audit.log"
pattern = ' type=\S+ msg=.*key="(identity|sudoers|time-change|modules|system_init|sshd_config)"'
summary = "auditd watch hit"
body    = "{line}"
urgency = "critical"
cooldown_secs = 30
```

**Source fields** (required unless noted): `path`, `pattern` (Rust `regex` crate syntax, compiled at startup — invalid regex = config error), `summary` (notification title; `{line}` placeholder = matched line, named capture groups available as `{name}`), optional `body`, optional `urgency` (`low|normal|critical`, default `normal`), optional `cooldown_secs` (default 0), optional `enabled` (bool, default true).

**IMPORTANT — patterns are per-deployment**: the shipped default targets FeralOS log paths; verify each pattern against REAL log output before release (the tracee JSON one especially — placeholder). Do not "fix" patterns by guessing.

## 5. Notification backend

- Recommended: **`zbus`** (pure-Rust D-Bus, no libdbus C dependency) → `org.freedesktop.Notifications.Notify(app_name, replaces_id, app_icon, summary, body, actions, hints, expire_timeout)` with `urgency` hint (byte: 0/1/2) and `expire_timeout = -1` (respect daemon policy... or severity mapping: critical = never expire, normal = default).
- Alternative (lighter deps): spawn `notify-send -u <urgency> <summary> <body>` — less control, one process per event. Pick zbus unless dependency tree becomes a problem.
- `app_name = "log-notify"`, `replaces_id` reuse per source (so cooldown-suppressed bursts do not stack windows — optional polish).

## 6. Implementation notes / gotchas (from the FeralOS round that motivated this)

- Best-effort service managers swallow process exit codes: never rely on exit codes for outcome signaling — make the daemon's stderr log lines truthful.
- `pgrep -f` in system scripts self-matches its own `sh -c` cmdline — irrelevant here but a recurring footgun in the surrounding tooling.
- Keep the daemon DROP-IN replaceable: if the notification daemon is absent (headless), log-notify must keep running (tailing continues, notifications silently dropped) — it is a notifier, not a security control.

## 7. Tests (minimum bar)

1. Pattern engine: named captures, placeholder substitution, invalid regex rejected at load.
2. Tail: append → line emitted; partial line (no trailing `\n`) buffered; write-then-truncate (copytruncate) → re-read; file replaced (inode change) → reopen; 64 KiB single line → capped, no panic.
3. Cooldown: second match inside window suppressed, after window emitted.
4. Config: missing file / bad TOML / bad regex / empty sources → precise startup errors.
5. Integration (manual or `--dry-run` + fixture logs): append lines from §4 sources → correct stdout events.

## 8. Repo scaffolding (same conventions as the FeralOS repo — replicate there)

- GitHub Actions CI: `cargo fmt --check`, `cargo clippy -- -D warnings`, `cargo test`, `cargo build --release` (ubuntu-latest, rust stable + toolchain pinned via rust-toolchain.toml).
- Conventional commits (`feat:`, `fix:`, `docs:`...), pre-commit hooks mirroring the FeralOS repo (fmt/clippy/scan), `AGENTS.md`-style lean project context at repo root.
- README: what it is, config reference table, the example config, notification-urgency table, "works with any freedesktop notification daemon" statement, non-goals (§2).
- Releases: tag-driven `cargo build --release` artifact upload (musl static build target `x86_64-unknown-linux-musl` is a plus for packagers).

## 9. FeralOS-side integration (DOWNSTREAM — NOT part of the new repo)

Listed for context only; done later in the FeralOS repos:
- aports package `log-notify` (cargo vendored tarball, tag-pinned per source-pinning policy).
- User dinit unit `log-notify.user` + wrapper pattern (ADR-008/009 conventions), enabled per-DE via rules `userServices` (niriWayle/niri first).
- The repo's shipped default config doubles as the FeralOS config (paths already FeralOS-shaped) — installed to `/etc/log-notify/log-notify.toml`.

## 10. Non-goals

- No notification replay/persistence, no GUI, no log storage/indexing, no filtering beyond per-line regex, no multi-user routing (notifications go to the session that is up), no remote/syslog input (files only).
