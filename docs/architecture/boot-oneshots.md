# Boot-time oneshot services (both inits)

**Pattern:** a small, idempotent task that must run once at boot, before a
specific consumer service — on BOTH init systems. Reference implementations:
`dbus-services-setup` (materializes the ADR-010 D-Bus overrides before dbus
starts; shipped by `feralos-dbus-activate` in feralos-aports) and `loadkmap`
(console keymap; `busybox-loadkmap-dinit`). Related decisions: ADR-009 (service
conventions), ADR-010 (why the D-Bus materializer exists).

## When to use

- materializing packaged files into a runtime location (e.g. `/usr/local/share`
  — forbidden to abuild — or `/run`) before a daemon reads them at startup
- one-time environment preparation that a daemon depends on at start
- NOT for periodic work (use cron/periodic) and NOT for supervising a
  long-running daemon (that is a regular `type = process` service).

## File layout (all shipped by ONE aports package)

| File | Installed as | Notes |
|---|---|---|
| `<name>` (dinit unit) | `/lib/dinit.d/<name>` | `type = scripted`, `command = ...`, `restart = false`; ordering via `before = <consumer>`; dependency via `depends-on = <target>` (mirror the consumer's own deps — e.g. both `dbus-services-setup` and `dbus` use `depends-on = local.target`) |
| `<name>.initd` (OpenRC) | `/etc/init.d/<name>` | `#!/sbin/openrc-run`; `depend() { before <consumer> }`; `start() { ebegin "..." ; /usr/libexec/feralos/<script> ; eend $? ; }` |
| the task script | `/usr/libexec/feralos/<name>-setup` | POSIX sh, busybox-safe; idempotent; self-healing (re-run every boot); `exit 0` when there is nothing to do; loud non-zero exit on failure (visible in the dinit service log / rc output) — never mask |

Use the SAME service name on both inits (like `dbus-services-setup`): one
`defaultRunlevels.boot` entry enables both. Names that differ would need a
`dinitServices` mapping (ADR-010) for the dinit side.

## Enablement (feralos installer)

One entry in `internal_rules.yaml` → `defaultRunlevels.boot`:

- OpenRC: `rc-update add <name> boot` — ALWAYS (the "OpenRC is never
  conditional" rule: a dinit install must stay OpenRC-bootable)
- dinit: `ln -sf /lib/dinit.d/<name> /etc/dinit.d/boot.d/<name>` (when
  `enableDinit`); the unit's `before =` orders it before its consumer.

Base-level oneshots belong in `defaultRunlevels.boot` (NOT the per-DE
`services:` maps). Do NOT add them to `dinitSkipServices` when the name is
identical on both inits. Ship the owning package everywhere the oneshot is
enabled (e.g. `feralos-dbus-activate` is in `standardPackages`) — a missing
unit file only warns at install time (`[WARNING] dinit: could not enable ...`)
and the oneshot would silently not run.

## Ordering rules (dinit)

- `before = <consumer>` — start before the consumer; it is NOT a dependency:
  the consumer still starts if the oneshot fails. A failed materialization
  must degrade, never block boot.
- `depends-on = <target>` — mirror the consumer's own dependency so the
  oneshot is never ordered earlier than the filesystem state the consumer
  expects.
- `restart = false` — scripted one-shots never restart (ADR-009).

## Idempotency / self-healing (materializer recipe)

`dbus-services-setup` is the canonical recipe
(`feralos-aports/src/feralos-dbus-activate/dbus-system-services-setup`):

1. `[ -d <source-dir> ] || exit 0` — no-op when nothing is installed
2. `mkdir -p <dest>`; copy each file (`cp -f`), track `rc=1` on failure
3. prune stale copies (dest files whose source disappeared)
4. `exit $rc`

Never delete the destination directory wholesale (other packages or admins may
have put files there); never assume the destination survives (re-run every
boot).

## Harness coverage

- static: the destination state exists after the verify boot (e.g.
  `dbus_activation_uniform` asserts the `/usr/local/share/dbus-1/system-services`
  materialization)
- started marker: `dinitctl list | grep <name>` (`[{+}]` / `[[+]]`) or
  `rc-status --all` (`[  started  ]` under OpenRC — see `services_status`)
- behavior: assert the consumer actually sees the materialized state.

## Gotchas (learned live)

- abuild FORBIDS packaging under `/usr/local` ("Packages must not put anything
  under /usr/local") → packaged sources always live under `/usr/share`; the
  oneshot materializes them elsewhere.
- dinit scripted services run `command` directly — no shell parsing: use an
  absolute path to a script (quote-free) when shell logic is needed.
- OpenRC `depend() { before X }` orders within the runlevel — put the service
  in `boot` when the consumer is in `boot`.
- failures surface in the dinit service log (`log-type = file`) / rc output;
  the `log_errors` harness check scans `target-serial.log` + `installer.log`.
- when the oneshot must run before a service that is itself ordered early
  (like `dbus`), test the ordering on a real boot — the harness verify boot
  plus the consumer's own behavior check is the proof.
