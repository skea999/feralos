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

**R1 — naming.** Every DBus-activatable daemon has one dinit unit named after its
canonical well-known bus name; user-instance units use the same name plus the
`.user` suffix. Aliases are separate activation files mapped to the same
canonical unit. Existing units are renamed (no backward compatibility).

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

**Enablement** is orthogonal to activation: units that must run before login stay
enabled through the rules (`dinitServices`), activation works regardless.

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
