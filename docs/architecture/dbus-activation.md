# D-Bus activation → dinit services (FeralOS rule)

**Policy (user directive 2026-10-18):** no D-Bus-activatable daemon may ever run
as an unsupervised process. Every such daemon is a first-class dinit service;
D-Bus activation is only a trigger that starts that service. The rule is uniform
— system and session bus, daemons and on-demand helpers — with no
per-application exceptions.

Decision record: `docs/adr/ADR-010-dbus-activation-dinit.md`.

## The two universal rules

**R1 — Naming (activation targets) — amended 2026-10-25**

The unit name follows **who activates the service in FeralOS** (full rationale in
`docs/adr/ADR-010-dbus-activation-dinit.md`):

- **user (session) services** — dbus/turnstile-activated on demand, never
  boot-enabled by us: unit = canonical bus name + `.user`
  (`/etc/dinit.d/user/<Name>.user`).
- **system services we activate at boot**, and an OpenRC initd exists: unit =
  **exactly the OpenRC initd name** (`/lib/dinit.d/<initd-name>`) — the
  `services` map alone drives both inits (rc-update + boot.d symlink), no
  corrective mapping.
- **system services dbus-activated on demand** (we never boot-enable them —
  dbus decides when they run): unit = canonical bus name
  (`/lib/dinit.d/<Name>`).

Invariants:
- The bus name always stays on the D-Bus side: override files are named after
  and carry `Name=<bus name>`; the override `Exec=` aliases the bus name onto
  the unit whatever it is called.
- Bus names inside commands (e.g. `dbus-wait-for -n <bus name>`) and polkit
  action ids are NOT unit names — they never change in a rename.
- `dinitServices` stays **empty** under this rule (mechanism kept for
  exceptional cases only).
- Aliases (e.g. `net.hadess.PowerProfiles` →
  `org.freedesktop.UPower.PowerProfiles`) are separate activation files that
  map to the same canonical unit.

**R2 — Activation (one shim, no exceptions)**

The *effective* D-Bus service file carries:

```text
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
does the same for the session overrides at login. `dbus-services-setup` is also
the reference implementation of the general boot-oneshot pattern — see
`docs/architecture/boot-oneshots.md`.

## Activation environment

D-Bus keeps an activation environment (`UpdateActivationEnvironment`, e.g.
`dbus-update-activation-environment --all` in session scripts). systemd forwards
it to the user manager so units inherit it. The shim forwards it to dinit with a
single `dinitctl [--system|--user] setenv NAME1 NAME2 ...` call (dinitctl reads
each value from its own environment) before starting the unit. Failures are
logged to stderr, never masked; activation continues with the existing dinit
environment.

## Checklist — adding a new D-Bus-activatable service

1. Identify the canonical well-known bus name and any aliases (each alias gets
   its own `<Alias>.service` file pointing at the same unit).
2. aports: ship the dinit unit, named per R1 — the OpenRC initd name for
   system services we boot-enable, the canonical bus name for dbus-activated
   on-demand services (system) and for all user services (`+.user`; ADR-008
   wrapper+config for daemons with configuration; `depends-on = dbus` like the
   other bus daemons).
3. aports: ship the override `.service` file in
   `/usr/share/feralos/dbus-system-services/` (system bus) or
   `/usr/share/feralos/dbus-services/` (session bus) — **never** in
   `/etc/dbus-1/system-services` (not helper-activatable). Content:

   ```ini
   [D-BUS Service]
   Name=<Name>
   Exec=/usr/libexec/feralos/dbus-activate <unit> -- <upstream exec...>
   User=root
   ```

   (`User=` is required by the system-bus launch helper.) APKBUILD:
   `depends="feralos-dbus-activate"`, pkgrel bump, sha512sums. No materializer
   change needed — `dbus-services-setup` / `dbus-session` pick up new files
   automatically.
4. **NEVER** add the supervision package to `internal_rules.yaml` — R1b names
   it after the daemon's apk package (`<owner-of-Exec>-user-dinit` /
   `<owner-of-Exec>-dinit`) and `feralos-dbus-audit` installs it at install
   time (chroot step 8a). In aports, name the package after the daemon's apk
   package (NOT the bus name — a bus name is an API identity: two daemons can
   claim one name, e.g. `org.freedesktop.secrets` = gnome-keyring/oo7) and
   group a daemon's activation files into ONE owner-named package (e.g. the
   4 powerdevil KAuth helpers live in `powerdevil-dinit`; GNOME apps are four
   owner-named packages). Declare **VERSIONED** provides for the alias names
   ONLY where the Exec gives no derivable owner (bare command like
   `obexd -n …`, or an `/bin/sh` wrapper like signon-ui):
   `provides="org.bluez.obex-user-dinit=${pkgver}-r${pkgrel}"` — apk treats
   UNVERSIONED provides as virtual and `apk add <alias>` refuses to select
   the provider (harness run 2026-11-03T22-09-15Z, 6 gaps proved it); note
   abuild checksum verification is POSITIONAL (sums must follow `source=`
   order). `services` += the OpenRC name (only when an OpenRC script exists
   and the service must be boot-enabled).
5. Harness: `dbus_activation_uniform` covers the override automatically
   (packaged sources + boot materialization + upstream coverage on dinit); add
   the unit to the per-DE service lists when it must be boot-enabled.
6. Docs: this file's inventory + the matching desktop guide + the aports
   pattern doc (`feralos-aports` `DBUS-ACTIVATION.md`).

## Verifying / troubleshooting activation routing (on a target)

- `dbus-send --system --print-reply --dest=org.freedesktop.DBus / org.freedesktop.DBus.ListActivatableNames`
  — a name in this list only means "enumerated", NOT "activatable" (the
  launch-helper gate above).
- trigger activation:
  `dbus-send --system --print-reply --dest=<Name> /org/freedesktop/DBus org.freedesktop.DBus.Peer.Ping`
  (or `org.freedesktop.DBus.StartServiceByName string:<Name> uint32:0`).
- did it route through dinit? `dinitctl --system list | grep <unit>` must show
  a started marker AND the daemon environ must NOT carry `DBUS_STARTER_*`
  (`tr '\0' '\n' < /proc/<pid>/environ | grep DBUS_STARTER_`) — the shim
  excludes them; a raw helper spawn leaves them. Exactly what
  `dbus_activation_env` asserts.
- raw spawn / `Spawn.ServiceNotFound` → the helper could not resolve the name
  in the standard dirs: check the materialized override
  `/usr/local/share/dbus-1/system-services/<Name>.service` (source in
  `/usr/share/feralos/dbus-system-services/`) and that `dbus-services-setup`
  ran (`dinitctl list` / `rc-status`); pattern details in `boot-oneshots.md`.
- wrong `Exec=` used → the helper picks the first standard-dir twin
  (`/usr/local/share` > `/usr/share`); stray files in
  `/etc/dbus-1/system-services` are inert (daemon-enumerated only) — the
  uniform check fails if any are left there.
- session side: `$XDG_RUNTIME_DIR/dbus-1/services/<Name>.service` (materialized
  at login by `dbus-session`); no helper — `Exec=` runs directly.

## Supervision audit — `feralos-dbus-audit` (auto-installer + enforcement)

Standalone Go binary (`src/firstBoot/dbus-audit/` in the feralos repo,
packaged as `feralos-dbus-audit` in feralos-aports, `/usr/libexec/feralos/feralos-dbus-audit`)
that turns the ADR-010 rule into the AUTO-INSTALL mechanism (R1b,
2026-11-03): it discovers every activation target and installs its
supervision package, so `internal_rules.yaml` carries no `*-dinit`
supervision packages at all (the go-test guard in `src/utils/config`
enforces the allowlist).

- scans BOTH upstream activation inventories — session
  `/usr/share/dbus-1/services/*.service` AND system
  `/usr/share/dbus-1/system-services/*.service` (the same enumeration as the
  harness `dbus_activation_migration` check) — parses `Name=` (bus name) +
  `Exec=` (binary path); placeholder Execs (`/bin/false`/`/bin/true` upstream
  systemd traps) are skipped, documented exemptions (e.g.
  `org.kde.kcookiejar5` — no unit possible on Alpine 3.24) are builtin.
- for every target checks the unit-existence paths (session:
  `/etc/dinit.d/user/<bus>.user` and `<bin>.user`; system: `<bus>`/`<bin>`
  under `/lib/dinit.d`, `/usr/lib/dinit.d`, `/etc/dinit.d`) **plus** the
  packaged FeralOS override (`/usr/share/feralos/dbus-services` /
  `dbus-system-services`): the override's shim Exec aliases the bus name
  onto the unit it starts (e.g. `org.freedesktop.login1` → `elogind`) → OK
  when any path exists.
- missing unit → resolve + `apk add` the supervision package, candidates in
  order: `<who-owns(Exec)>-user-dinit`/`-dinit` (the daemon's apk package —
  disambiguates same-API daemons like `org.freedesktop.secrets` and covers
  multi-target daemons like plasma-workspace/kio with zero splits), then
  `<bus>-user-dinit`/`-dinit` (resolves `provides=` aliases, e.g. the
  plasma-kauth bus names). Installed + unit re-verified → FIXED; no package
  → MISS line ("create it in aports — package = daemon apk name + suffix").
- core session infra boot.d check (report-only): dbus, pipewire,
  pipewire-pulse, wireplumber, wait-wayland, oo7/portal units must be
  linked in `/usr/lib/dinit.d/user/boot.d` (or `/etc/.../boot.d`) WHEN
  installed; `polkit-agent`/`wayvnc` are session-scoped (started by the
  session script, never boot-linked — session-apps model) and are only
  reported. Exit code is NOT affected by this section.
- also reports the user's materialized session overrides
  (`$XDG_RUNTIME_DIR/dbus-1/services`) for visibility.
- runs in three modes: install-time (chroot_phase step 8a — AFTER all
  installations incl. nix/flatpak, BEFORE the `@edge` install at step 8b,
  because that step comments the `@edge` repos out and would hard-fail the
  audit's `apk add` fallback against a `mesa@edge` world — the fallback
  can fix gaps on the spot against stable-only repos), boot-time (dinit oneshot + OpenRC initd, both `local.target`-
  ordered because the report lands in the `@var-log` subvol;
  `--log-file /var/log/feralos-dbus-audit.log`), CLI manual.
- flags: `--log-file <path>` (boot mode), `--user <name>` (report
  annotation + runtime override check; units are system-wide).
- **exit code — strict by default (boot oneshot, CLI, CI consumers): 0 all
  covered, 1 = at least one target has no supervision package.** That 1 is
  the post-install signal the user must notice: the dinit oneshot marks
  itself FAILED at boot and the report lands in
  `/var/log/feralos-dbus-audit.log`. The INSTALLER passes `--no-fail`
  (always exit 0): an extra package without a ready `*-user-dinit` must
  never stop an interactive install — the dbus raw-exec fallback keeps the
  system working until the supervision package lands in aports, and the
  next boot's audit installs it automatically.

## Inventory / status (tracking)

| Target | Bus | Unit | Status |
|---|---|---|---|
| power-profiles-daemon | system | `org.freedesktop.UPower.PowerProfiles` (alias `net.hadess.PowerProfiles`) | DONE (`c123254`; R1 amendment 2026-10-25: **activation-only** — dbus decides, never boot-enabled) |
| accountsservice | system | `org.freedesktop.Accounts` | DONE (`c123254`) |
| polkit | system | unit `polkit` (OpenRC initd name — R1 amendment `e7f1cc8`); override `org.freedesktop.PolicyKit1.service` aliases onto it | DONE (`c123254`, superseded by `e7f1cc8`) |
| fwupd | system | `org.freedesktop.fwupd` — **activation-only** (R1 amendment 2026-10-25: dbus decides, never boot-enabled on either init) | DONE (`c123254`; amended 2026-10-25) |
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
| kglobalaccel | session | `org.kde.kglobalaccel.user` | DONE (`be98f7f`; NO upstream activation file — our override is the only route; daemon `kglobalacceld` (`/usr/lib/libexec/kglobalacceld`) is a KDE **global-shortcuts** component for KDE apps — **KDE/Plasma-only, NOT in the niri/niriWayle package lists** (no KWin/GlobalShortcuts integration for Wayland key grabbing; the KDE apps shipped on niri do not use it); unit/override stay optional — `dbus_activation_uniform` covers them if the package is ever installed; installing the daemon alone gives no activation route, add `kglobalaccel-user-dinit` too; the KF6 `kglobalaccel` LIBRARY is a transitive dep of kio/kded/kwallet and gets installed regardless — only the daemon package matters) |
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
| bluez | system | daemon unit `bluetooth` (OpenRC initd name — R1 amendment `e7f1cc8`); override `org.bluez.service` aliases onto it; `org.bluez.mesh` unchanged | DONE (`85646f4`, superseded by `e7f1cc8`; Alpine files had `Exec=/bin/false` — systemd-only; our override is the activation route) |
| elogind | system | unit `elogind` (OpenRC initd name — R1 amendment `e7f1cc8`); override `org.freedesktop.login1.service` aliases the bus name onto it; turnstile/greetd/sddm/cosmic-greeter unit deps updated | DONE (`85646f4` rename, superseded; `e7f1cc8` amendment) |
| wpa_supplicant | system | `fi.w1.wpa_supplicant1` | DONE (`85646f4`) |
| avahi | system | `org.freedesktop.Avahi` | DONE (`44c598a`; upstream file is `Exec=/bin/false` — systemd-only; unit runs `avahi-daemon` foreground, no `-D`) |
| udisks2 | system | `org.freedesktop.UDisks2` | DONE (`44c598a`; Alpine path `/usr/libexec/udisks2/udisksd`) |
| geoclue | system | `org.freedesktop.GeoClue2` | DONE (`44c598a`; `run-as = geoclue`; 60 s idle exit preserved — activation-only) |
| cups-pk-helper | system | `org.opensuse.CupsPkHelper.Mechanism` | DONE (`44c598a`; depends on `cupsd`) |
| kcron | system | `local.kcron.crontab` | DONE (`44c598a`; short-lived KAuth helper — no smooth-recovery) |
| gnome-online-accounts | session | `org.gnome.OnlineAccounts`, `org.gnome.Identity` | DONE (`44c598a`; two distinct binaries — no alias dedupe) |
| evolution-data-server | session | `org.gnome.evolution.dataserver.{Sources5,AddressBook10,Calendar8,UserPrompter0}` | DONE (`44c598a`) |
| gnome apps | session | `org.gnome.{Calendar,DiskUtility,Firmware,Maps}` | DONE (`44c598a`; GUI units in `--gapplication-service` mode) |
| gnome-keyring (niri) | session | `org.freedesktop.secrets.user` (+ aliases `org.gnome.keyring`, `org.freedesktop.impl.portal.Secret`) | DONE (`0c68215`; niri DE; gnome-keyring daemon, 3 overrides → 1 unit) |
| gcr prompter (niri) | session | `org.gnome.keyring.PrivatePrompter.user` (+ alias `org.gnome.keyring.SystemPrompter`) | DONE (`0c68215`; transient GUI prompt) |
| seahorse (niri) | session | `org.gnome.seahorse.Application.user` | DONE (`0c68215`; GUI app unit) |
| fprintd | system | `net.reactivated.Fprint` | DONE (`d78d3e3`; KDE app-set pull-in; idle-exit, no respawn) |
| bolt | system | `org.freedesktop.bolt` | DONE (`d78d3e3`) |
| Plasma KAuth helpers | system | `org.kde.{discover.alpineapkbackend, fontinst, kameleonhelper, kcontrol.kcmclock, kcontrol.kcmsddm, kded.smart, kinfocenter.dmidecode, ksysguard.processlisthelper, ktexteditor6.katetextbuffer, localegenhelper, powerdevil.backlighthelper, powerdevil.chargethresholdhelper, powerdevil.discretegpuhelper, powerdevil.wakeupsourcehelper, ufw}` | DONE (`0c68215`; 15 units, per-request KAuth helpers) |
| ibus | session | `org.freedesktop.IBus.user`, `org.freedesktop.portal.IBus.user` | DONE (`1289145`; two binaries — `ibus-daemon --replace --panel disable --xim` + `/usr/lib/ibus/ibus-portal`) |
| obexd | session | `org.bluez.obex.user` | DONE (`1289145`; `obexd-enhanced` binary `obexd -n --capability=!/usr/bin/obex-capabilities`) |
| system-config-printer | session | `org.fedoraproject.Config.Printing.user` | DONE (`1289145`; `/usr/bin/scp-dbus-service` wrapper) |
| kcookiejar5 | session | — | EXEMPT (upstream broken on Alpine 3.24: activation file `Exec=/usr/bin/kded5`, binary absent — no unit possible; harness skips it with an explicit message) |
| playerctld | session | `org.mpris.MediaPlayer2.playerctld.user` | DONE (`2c87f7a`; cosmic; the Alpine package is `playerctl` — binary + activation file together; `/usr/bin/playerctld`) |

All system rows: override source `/usr/share/feralos/dbus-system-services/`,
materialized at boot to `/usr/local/share/dbus-1/system-services/` by
`dbus-services-setup` (2026-10-18 validation round — `/etc` proved
helper-ineligible). All session rows: source `/usr/share/feralos/dbus-services/`,
materialized at login to `$XDG_RUNTIME_DIR/dbus-1/services/`.

**R1 amendment (2026-10-25, user decision):** system units WE boot-enable carry
the **OpenRC initd name** (`elogind`, `polkit`, `bluetooth`) — the `services`
map alone drives both inits and `dinitServices` stays empty; dbus-activated
system services (fwupd, PPD, upower, udisks2, avahi, wpa_supplicant,
nm_dispatcher, accountsservice, bolt, cups-pk-helper, flatpak SystemHelper,
fprintd, geoclue, kcron, the KAuth helpers) keep the **bus name** and are
**never boot-enabled** — dbus decides when they run (fwupd/PPD were
boot-enabled before the amendment; corrected). Bus names remain on the D-Bus
side: override filenames, `Name=`, `dbus-wait-for -n` arguments, polkit action
ids. Validated on the full matrix (see phases item 11).

**Third-party targets (the rows from `44c598a`)** live in feralos-aports as
owner-named supervision packages (`avahi-dinit`, `udisks2-dinit`, `geoclue-dinit`,
`cups-pk-helper-dinit`, `kcron-dinit`, `gnome-online-accounts-user-dinit`,
`evolution-data-server-user-dinit`, and the R1b purity split: GNOME apps as four
owner-named packages, the 15 KAuth helpers as nine owner-named system packages
— powerdevil/plasma-workspace/kdeplasma-addons/plasma-desktop/sddm-kcm/
plasma-disks/kinfocenter/libksysguard/ktexteditor/discover/plasma-firewall —
`gnome-apps-user-dinit` and `plasma-kauth-dinit` are RETIRED). They are the first
half of the dbus-broker model (every activation target has a manager unit) and
are **retired** when the second half lands. Retirement gate: harness
`dbus_activation_migration` — enumerates **every** activation file on the target
live (no static list) and fails only when **dbus-broker is installed AND all
targets are unit-covered** (exact-name package unit, or an override aliasing the
name onto an existing package unit). While not both it passes with "keep the
FeralOS activation mechanism", so the migration cannot be forgotten.

**System-bus coverage is global** (harness `dbus_system_activation`, dinit-only,
on EVERY DE): kde brings the 15 KAuth helpers + upower/PPD/accountsservice/
fprintd/bolt; cosmic upower/geoclue; niri upower + the gnome-keyring stack.
**Session-bus coverage is now niri-family + KDE + cosmic** (2026-10-25): KDE runs the
same turnstile user dinit (`pam_turnstile` arrives via Alpine's `base-session`
include in sddm's PAM) and `dbus-session-materializer-user-dinit` materializes
the overrides into `$XDG_RUNTIME_DIR/dbus-1/services`, which Plasma's own
`dbus-run-session` bus reads — the KDE list carries the session unit packages
(plasma-workspace, kded/kwallet/kio, portals, signon/signond, goa/e-d-s,
glib-networking, ibus/obexd/system-config-printer). Cosmic was unified on
elogind the same day (user decision): the seatd daemon dropped (nothing on
Alpine 3.24 depends on it — cosmic-comp reaches the seat via libseat's logind
backend), turnstiled's `elogind` unit dep resolves, and the turnstile
user dinit + materializer run there too; cosmic session targets are fully
covered (`playerctld-user-dinit` added for `org.mpris.MediaPlayer2.playerctld`).
`dbus_activation_env` session half skips honestly on DE-owned buses (kde/
cosmic). **Idle-exit units** (geoclue, fprintd, EDS factories) use
`smooth-recovery = false`: the unit stops on a legitimate idle exit and the
next activation restarts it.

## Implementation phases (tracking)

1. [x] aports: `feralos-dbus-activate` shim package (`9bfb440`)
2. [x] aports: system units + overrides (`c123254`)
3. [x] aports: session units + overrides (`be98f7f`)
4. [x] installer: `dinitServices` schema + `services.go` support; rules/package lists (`dbus-services-setup` in `defaultRunlevels.boot`, `feralos-dbus-activate` in `standardPackages`)
5. [x] harness: service lists, `dbus_activation_uniform` (init-aware), `dbus_activation_env` (real activation + forwarding proof)
6. [x] docs: AGENTS policy section, ADR index, aports guide checklist, niriwayle/artifacts rows
7. [x] validation round 1: builds, aports CI, harness runs on dinit + OpenRC, iterative fixes (runs `18-33-05Z` … `21-40-20Z`)
8. [x] validation round 2 (system-bus helper eligibility): live + source proof that the launch helper resolves names ONLY against the standard dirs (trivial parser — `/etc` + `/run` never searched) → overrides moved to `/usr/share/feralos/dbus-system-services` + `dbus-services-setup` boot materializer into `/usr/local/share/dbus-1/system-services` (aports `2e0caa8`; comment follow-up `5f729ea`); uniform check reworked (sources + materialization + `/etc` clean; coverage loop dinit-only), env check reworked (no `DBUS_STARTER_*` + direct-shim `setenv` proof). Validated: dinit `2026-10-18T22-56-20Z` **106/106**; OpenRC `2026-10-18T23-03-31Z` **90/90**.
9. [x] KDE session-bus coverage (2026-10-25): 3 new unit-only packages (`obexd-user-dinit`, `ibus-user-dinit`, `system-config-printer-user-dinit`, aports `1289145`) + KDE list gains 9 session packages; `kcookiejar5` documented exemption; `log_errors` WARNING sweep with the 3-line allowlist; `services.go` boot.d symlink `/lib`-or-`/etc` resolution. Validated: kde full-feature dinit `2026-10-25T03-41-55Z` **77/77**.
10. [x] cosmic session-bus coverage (2026-10-25): cosmic unified on elogind (user decision) — seatd daemon dropped (Alpine 3.24: nothing depends on it; cosmic-comp uses libseat's logind backend), `cosmic-greeter-dinit` r6 units `depends-on = org.freedesktop.login1`, NEW `playerctld-user-dinit`, rules `dinitServices` login1 mapping. Validated: cosmic full-feature dinit `2026-10-25T14-19-46Z` **77/77** (migration 0/14 uncovered).
11. [x] R1 amendment (2026-10-25, user correction): system units we boot-enable carry the OpenRC initd name (cosmic-greeter r7 deps → `elogind` in `e7f1cc8`); dbus-activated system services stay bus-named and never boot-enabled (fwupd/PPD corrected to on-demand). aports `e7f1cc8` (elogind/polkit/bluetooth renames + dependents), `dinitServices` removed from the rules, skip-list cleanup, harness expected lists + `power_profiles` on-demand proof + `dbus_activation_env` race-proofing. Validated: kde `17-19-00Z` **77/77**, cosmic `17-02-52Z` **77/77**, niri `17-50-09Z` **113/113**, niriWayle dinit `17-57-48Z` **109/109**, niriWayle OpenRC `18-16-31Z` **92/92**.

Update this checklist and the handoff after each phase.
