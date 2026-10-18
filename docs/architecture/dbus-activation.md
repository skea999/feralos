# D-Bus activation → dinit services (FeralOS rule)

**Policy (user directive 2026-10-18):** no D-Bus-activatable daemon may ever run
as an unsupervised process. Every such daemon is a first-class dinit service;
D-Bus activation is only a trigger that starts that service. The rule is uniform
— system and session bus, daemons and on-demand helpers — with no
per-application exceptions.

Decision record: `docs/adr/ADR-010-dbus-activation-dinit.md`.

## The two universal rules

**R1 — Naming (activation targets)**
- unit name = the canonical D-Bus well-known name, verbatim
- user-instance units: same name + `.user` suffix
- aliases (e.g. `net.hadess.PowerProfiles` and
  `org.freedesktop.UPower.PowerProfiles`) are separate activation files that map
  to the same canonical unit
- system units: `/lib/dinit.d/<Name>`; user units: `/etc/dinit.d/user/<Name>.user`
- enablement: `internal_rules.yaml` `services` keeps the OpenRC fallback names;
  `dinitServices` carries the dinit unit names (they differ where the bus name
  and the OpenRC script name differ)

**R2 — Activation (one shim, no exceptions)**

The *effective* D-Bus service file carries:

```
Exec=/usr/libexec/feralos/dbus-activate <unit> -- <upstream exec...>
```

Override locations — packaged sources + the effective (materialized) files
(`/usr/local` is forbidden by abuild, so nothing ships there directly):

| Bus | Shipped as (package files) | Effective location |
|---|---|---|
| system | `/usr/share/feralos/dbus-system-services/<Name>.service` | materialized at boot by `dbus-services-setup` (both inits, ordered before dbus) into `/usr/local/share/dbus-1/system-services/<Name>.service` — the launch helper's first eligible dir |
| session | `/usr/share/feralos/dbus-services/<Name>.service` | materialized at login by the `dbus-session` wrapper into `$XDG_RUNTIME_DIR/dbus-1/services/<Name>.service` — the highest-precedence session servicedir (strict naming, read at bus startup) |

Why the system side cannot use `/etc/dbus-1/system-services` (the dbus-daemon
precedence list puts it first, but that is not the full story): the system bus
activates through the setuid `dbus-daemon-launch-helper`, which re-resolves the
name against **its own** service-dir list — the "trivial" config parser expands
`<standard_system_servicedirs/>` with the standard dirs only
(`/usr/local/share`, `/usr/share`, `/lib`); `/etc` and `/run` are never
searched there. The daemon still enumerates `/etc` entries (they appear in
`ListActivatableNames`) but the helper refuses them: live-verified on dbus
1.16.2 — activation spawns the `/usr/share` twin raw, or fails with
`Spawn.ServiceNotFound: Bus name not found in system service directory` when no
standard twin exists. Hence the boot materialization into
`/usr/local/share/dbus-1/system-services` (first helper-eligible dir;
live-verified: activation then starts the dinit unit through the shim).

Why not `/usr/local/share/dbus-1/services` for the session side: **abuild
forbids packages installing under `/usr/local`** (CI: "Packages must not put
anything under /usr/local"). The runtime-dir materialization keeps every
packaged file under `/usr/share` and makes session precedence deterministic
(first servicedir, no env dependence). Session activation runs `Exec=` directly
(no setuid helper), so every session servicedir is eligible — the `/etc`
restriction is system-bus only.

Shim behavior (`feralos-dbus-activate`):
- **system bus**: if a system dinit is reachable (`dinitctl --system list`):
  forward the activation environment (one `dinitctl setenv` call), then
  `dinitctl start <unit>` (waits). Otherwise (OpenRC boot): execute the upstream
  fallback command stored after `--`.
- **session bus**: forward the environment to the user dinit, then
  `dinitctl --user start <unit>`; no fallback — a missing user dinit is an error
  (turnstile provides the user dinit on both inits).
- `dinitctl start` marks the unit explicitly activated (systemd-like: it stays up
  until stopped). `is-started` is checked first so repeated activation is a no-op.
- dbus-daemon completes activation when the daemon claims its well-known name
  (`service_start_timeout`) — the standard forking-activation semantics; a shim
  that exits after starting the unit is correct.

**Boot enablement is orthogonal**: units that must run before login stay enabled
via the rules; activation targets work regardless. `dbus-services-setup`
(feralos-dbus-activate, enabled on both inits via `defaultRunlevels.boot`)
materializes the system overrides before dbus starts; the `dbus-session` wrapper
does the same for the session overrides at login.

## Activation environment

D-Bus keeps an activation environment (`UpdateActivationEnvironment`, e.g.
`dbus-update-activation-environment --all` in session scripts). systemd forwards
it to the user manager so units inherit it. The shim forwards it to dinit with a
single `dinitctl [--system|--user] setenv NAME1 NAME2 ...` call (dinitctl reads
each value from its own environment) before starting the unit. Failures are
logged to stderr, never masked; activation continues with the existing dinit
environment.

## Checklist — adding a new D-Bus-activatable service

1. Identify the canonical well-known bus name and any aliases.
2. aports: ship the dinit unit (bus-name naming; ADR-008 wrapper+config for
   daemons with configuration).
3. aports: ship the override `.service` file (shim Exec + upstream fallback)
   in `/usr/share/feralos/dbus-system-services/` (system) or
   `/usr/share/feralos/dbus-services/` (session) — **never** in
   `/etc/dbus-1/system-services` (not helper-activatable). No further step:
   the boot/login materializers pick up new files automatically.
4. `internal_rules.yaml`: package list += the `-dinit` package; `dinitServices`
   += unit name; `services` += OpenRC name only if an OpenRC script exists.
5. Harness: the generic `dbus_activation_uniform` check covers the override
   automatically; add the unit to the per-DE service lists when required.
6. Docs: update this file's inventory and the matching desktop guide.

## Inventory / status (tracking)

| Target | Bus | Unit | Status |
|---|---|---|---|
| power-profiles-daemon | system | `org.freedesktop.UPower.PowerProfiles` (alias `net.hadess.PowerProfiles`) | DONE (`c123254`; boot-enabled via `dinitServices`) |
| accountsservice | system | `org.freedesktop.Accounts` | DONE (`c123254`) |
| polkit | system | `org.freedesktop.PolicyKit1` (renamed from `polkit`) | DONE (`c123254`) |
| fwupd | system | `org.freedesktop.fwupd` (renamed from `fwupd`) | DONE (`c123254`) |
| upower | system | `org.freedesktop.UPower` (renamed from `upower`) | DONE (`c123254`; activation-only, not boot-enabled) |
| NetworkManager helpers | system | `org.freedesktop.nm_dispatcher`, `org.freedesktop.nm_priv_helper` | DONE (`c123254`; daemon `networkmanager` has no activation file — unchanged) |
| flatpak system helper | system | `org.freedesktop.Flatpak.SystemHelper` | DONE (`c123254`) |
| firewalld | system | — | OUT OF SCOPE (verified: Alpine ships no activation file) |
| xdg-desktop-portal | session | `org.freedesktop.portal.Desktop.user` | DONE (`be98f7f`) |
| xdg-document-portal | session | `org.freedesktop.portal.Documents.user` | DONE (`be98f7f`) |
| xdg-permission-store | session | `org.freedesktop.impl.portal.PermissionStore.user` | DONE (`be98f7f`) |
| portal backends | session | `org.freedesktop.impl.portal.desktop.{gtk,wlr,gnome}.user` | DONE (`be98f7f`) |
| oo7-portal | session | `org.freedesktop.impl.portal.desktop.oo7.user` (real bus name; `...portal.Secret` is the portal interface, not a bus name) | DONE (`be98f7f`; unit rename to the real bus name pending in 1d) |
| at-spi | session | `org.a11y.Bus.user` | DONE (`be98f7f`) |
| dconf | session | `ca.desrt.dconf.user` | DONE (`be98f7f`) |
| kwallet | session | `org.kde.kwalletd6.user` (+ `kwalletd5` alias override) | DONE (`be98f7f`) |
| kded | session | `org.kde.kded6.user` | DONE (`be98f7f`) |
| kglobalaccel | session | `org.kde.kglobalaccel.user` | DONE (`be98f7f`; NO upstream activation file — our override is the only route; daemon binary lives in `kglobalacceld`, package optional / not in the DE lists) |
| signond | session | `com.google.code.AccountsSSO.SingleSignOn.user` (+ backup alias override) | DONE (`be98f7f`) |
| localsearch | session | `org.freedesktop.LocalSearch3.user`, `...LocalSearch3.Control.user`, `...LocalSearch3.Writeback.user` (Tracker3 names alias to the same units) | DONE (`be98f7f`) |
| flatpak session helpers | session | `org.freedesktop.Flatpak.user`, `org.freedesktop.portal.Flatpak.user`, `org.flatpak.Authenticator.Oci.user` | DONE (`be98f7f`) |
| oo7 secrets | session | `org.freedesktop.secrets.user` (renamed from `secrets.user`; Alpine DOES ship the activation file — old comment was wrong) | DONE (`85646f4`) |
| kio helpers | session | `org.kde.kiod6.user`, `org.kde.kioexecd6.user`, `org.kde.kpasswdserver6.user`, `org.kde.kssld6.user` (all kiod6 plugins) | DONE (`85646f4`) |
| kwallet extras | session | `org.kde.secretservicecompat.user`, `org.freedesktop.impl.portal.desktop.kwallet.user` (both `/usr/bin/ksecretd`) | DONE (`85646f4`) |
| nautilus | session | `org.gnome.Nautilus.user` (alias `org.freedesktop.FileManager1`) | DONE (`85646f4`) |
| Tracker portal | session | `org.freedesktop.portal.Tracker.user` (`tinysparql` package) | DONE (`85646f4`) |
| glib PACRunner | session | `org.gtk.GLib.PACRunner.user` | DONE (`85646f4`) |
| signon-ui | session | `com.canonical.indicators.webcredentials.user`, `com.nokia.singlesignonui.user` | DONE (`85646f4`) |
| bluez | system | `org.bluez` (renamed from `bluetooth`), `org.bluez.mesh` | DONE (`85646f4`; Alpine files had `Exec=/bin/false` — systemd-only; our override is the activation route) |
| elogind | system | `org.freedesktop.login1` (renamed from `elogind`; greetd/sddm/turnstile unit deps updated) | DONE (`85646f4`) |
| wpa_supplicant | system | `fi.w1.wpa_supplicant1` | DONE (`85646f4`) |

All system rows: override source `/usr/share/feralos/dbus-system-services/`,
materialized at boot to `/usr/local/share/dbus-1/system-services/` by
`dbus-services-setup` (2026-10-18 validation round — `/etc` proved
helper-ineligible). All session rows: source `/usr/share/feralos/dbus-services/`,
materialized at login to `$XDG_RUNTIME_DIR/dbus-1/services/`.

## Implementation phases (tracking)

1. [x] aports: `feralos-dbus-activate` shim package (`9bfb440`)
2. [x] aports: system units + overrides (`c123254`)
3. [x] aports: session units + overrides (`be98f7f`)
4. [x] installer: `dinitServices` schema + `services.go` support; rules/package lists (`dbus-services-setup` in `defaultRunlevels.boot`, `feralos-dbus-activate` in `standardPackages`)
5. [x] harness: service lists, `dbus_activation_uniform` (init-aware), `dbus_activation_env` (real activation + forwarding proof)
6. [x] docs: AGENTS policy section, ADR index, aports guide checklist, niriwayle/artifacts rows
7. [x] validation round 1: builds, aports CI, harness runs on dinit + OpenRC, iterative fixes (runs `18-33-05Z` … `21-40-20Z`)
8. [x] validation round 2 (system-bus helper eligibility): live + source proof that the launch helper resolves names ONLY against the standard dirs (trivial parser — `/etc` + `/run` never searched) → overrides moved to `/usr/share/feralos/dbus-system-services` + `dbus-services-setup` boot materializer into `/usr/local/share/dbus-1/system-services` (aports `2e0caa8`; comment follow-up `5f729ea`); uniform check reworked (sources + materialization + `/etc` clean; coverage loop dinit-only), env check reworked (no `DBUS_STARTER_*` + direct-shim `setenv` proof). Validated: dinit `2026-10-18T22-56-20Z` **106/106**; OpenRC `2026-10-18T23-03-31Z` **90/90**.

Update this checklist and the handoff after each phase.
