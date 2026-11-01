# ADR-010: D-Bus activation routes through dinit services

**Status:** Accepted
**Date:** 2026-10-18

## Context

D-Bus service activation traditionally spawns the `Exec=` process directly
(`dbus-daemon` "traditional activation"). On systemd distros activation starts a
systemd unit (system or per-user), so every activated daemon is supervised. On
Alpine/Artix/Chimera traditional activation leaves the daemon unsupervised.

FeralOS user directive (2026-10-18): **no D-Bus-activatable daemon may ever run
unsupervised**. Every such daemon must be a first-class dinit service (system bus
→ system dinit service, session bus → user dinit service via turnstile), and
D-Bus activation must only be a trigger that starts that service. The rule is
uniform: no per-application exceptions, no service-specific hacks.

Chimera Linux designed an equivalent architecture (`dbus-daemon --dinit` +
`dinit-dbus` `org.chimera.dinit.Activator`), but the activation-routing piece is
not implemented upstream yet. Artix does not route activation at all. FeralOS
implements the full mechanism itself.

## Decision

Two universal rules, enforced by the harness:

**R1 — naming (amended 2026-10-25; the original blanket bus-name rule is
superseded).** The unit name follows **who activates the service in FeralOS**:

| category | unit name | started by | enablement |
|---|---|---|---|
| user (session) services — dbus/turnstile-activated on demand, never boot-enabled by us | canonical bus name + `.user` | shim → user dinit | never in `services` |
| system services **we** activate at boot, and an OpenRC initd exists | **exactly the OpenRC initd name** | boot (`services` map) | `services` map alone — same name for rc-update and the boot.d symlink |
| system services dbus-activated on demand (we never boot-enable them) | canonical bus name | shim → system dinit | never boot-enabled — dbus decides when they run |

Rules of thumb for future agents:

- The **bus name never leaves the D-Bus side**: override files are named after
  and carry `Name=<bus name>`; only the dinit unit name changes, and the
  override's `Exec=/usr/libexec/feralos/dbus-activate <unit> -- …` aliases the
  bus name onto the (possibly renamed) unit. Activation therefore works
  regardless of the unit's name.
- Readiness arguments that reference bus names (e.g.
  `dbus-wait-for -n org.freedesktop.login1`) are bus names — they do NOT change
  when the unit is renamed. Same for polkit action ids
  (`org.freedesktop.login1.power-off`).
- `dinitServices` is a mechanism, not a goal: under this rule it stays **empty**
  in the rules. A system unit with an initd is enabled through `services`
  alone; a dbus-activated system unit is never boot-enabled.
- Boot-enabling a dbus-by-design daemon (power-profiles-daemon, fwupd) keeps it
  always running without dbus deciding when — corrected on 2026-10-25: those
  are activation-only, bus-named, supervised through the shim.
- Rationale for the explicit/openrc-name pair: the `services` map alone drives
  both inits and the initd↔dinit conversion stays name-symmetric — no
  corrective mapping, no divergence to remember. The original ADR generalized
  the bus-name convention to system services; that created the `dinitServices`
  patchwork and a boot failure (cosmic-greeter r5, wrong unit name), and was
  rolled back to the classification above.

**R2 — activation.** The effective D-Bus service file carries
`Exec=/usr/libexec/feralos/dbus-activate <unit> -- <upstream exec...>`.
Overrides ship under `/usr/share/feralos/dbus-system-services` (system bus) and
`/usr/share/feralos/dbus-services` (session bus); both are materialized into
the effective, highest-precedence servicedirs (`/usr/local` is forbidden by
abuild, so nothing ships there directly):

- system bus: the `dbus-services-setup` unit (both inits, ordered before dbus)
  copies them to `/usr/local/share/dbus-1/system-services` at boot. This is
  required: the system bus activates through the setuid
  `dbus-daemon-launch-helper`, which resolves names only against the standard
  dirs (`/usr/local/share` first; its config parser is the "trivial" one).
  `/etc/dbus-1/system-services` is daemon-enumerated (`ListActivatableNames`)
  but **not helper-activatable** (dbus 1.16.2, live-verified: raw spawn /
  `Spawn.ServiceNotFound`).
- session bus: the `dbus-session` wrapper copies them at login into
  `$XDG_RUNTIME_DIR/dbus-1/services` — the highest-precedence session servicedir
  (strict naming, read at bus start).

Upstream package files are never modified. The shim forwards the
D-Bus activation environment to dinit with a single `dinitctl setenv` call, then
runs `dinitctl start` (waits for the unit), then exits; `dbus-daemon` completes
activation when the daemon claims its name (standard forking-activation
semantics).

**OpenRC fallback.** The session bus uses the turnstile user dinit on both inits.
For the system bus under OpenRC there is no system dinit: the shim detects this
(`dinitctl --system list` unreachable) and executes the upstream fallback command
stored in the override. Supervision is a dinit property; the OpenRC fallback
remains a bootable fallback, never the primary mode.

**Third-party activation targets.** The app set brings activatable daemons of
its own (avahi, udisks2, geoclue, cups-pk-helper, kcron, GNOME online accounts,
evolution-data-server, and the GNOME app-launch files). FeralOS covers them with
transitional `-dinit`/`-user-dinit` units plus overrides — exactly the shape
dbus-broker's launcher expects (one manager unit per activation target). These
are not FeralOS services: the retirement gate is the harness check
`dbus_activation_migration`, which enumerates **every** activation file on the
target live and fails only when **dbus-broker is installed AND every target is
unit-covered** (an exact-name package unit, or an override aliasing the name onto
an existing package unit). At that point the FeralOS overrides/shim are dropped
and the units can move to the owning packages. Lifecycle notes: avahi runs
foreground (its upstream `.service` is `Exec=/bin/false`, systemd-only), geoclue
keeps its 60 s idle exit (activation-only semantics), kcron's KAuth helper is
short-lived (no smooth-recovery), and the GNOME app files are GUI units in
`--gapplication-service` mode.

**System-bus coverage is global; session-bus coverage follows the session
stack.** Every desktop (and server installs) must cover every installed system
activation file — enforced by the dinit-only `dbus_system_activation` check. KDE
adds 15 system KAuth helpers (`plasma-kauth-dinit`) plus upower,
power-profiles-daemon, accountsservice, fprintd and bolt; cosmic adds upower and
geoclue; niri adds upower and the gnome-keyring/gcr/seahorse stack
(`gnome-keyring-user-dinit`). Session-bus coverage (2026-10-25) is complete on
all four DEs: kde and cosmic run the same turnstile user dinit + session
materializer, and their session unit packages are in the rules.

**Idle-exit daemons use `smooth-recovery = false`.** dinit's smooth-recovery
restarts a process that exits for any reason other than a stop order — including
a legitimate idle exit (geoclue 60 s, fprintd 30 s, the EDS addressbook/calendar
factories after the last client). For those activation-only units the correct
policy is `smooth-recovery = false`: the unit stops when the daemon idles out and
the next activation starts it again. Long-running daemons keep smooth-recovery
for crash resilience.

**Enablement** is orthogonal to activation: units that must run before login are
enabled through the rules (`services` map; under the amended R1 the
`dinitServices` corrective mapping stays empty). Dbus-by-design daemons
(power-profiles-daemon, fwupd, upower, udisks2, geoclue, …) are never
boot-enabled — activation starts them when a client first asks, supervised by
the shim.

## Consequences

- Every activated process is supervised (restart policy, dependencies, logging).
- One uniform mechanism, one helper, one naming convention; new services follow
  the checklist in `docs/architecture/dbus-activation.md`.
- We maintain the override files (data only) and the shim; no dbus fork.
- `dbus_activation_uniform` harness check enforces the rule on the target
  (fails if any effective `.service` does not route through the shim or its unit
  is missing).
- The activation environment is forwarded to dinit; failures are logged, not
  masked.
- If Chimera completes native routing upstream, the shim can be retired in favor
  of the native mechanism without changing the naming/enablement model.
