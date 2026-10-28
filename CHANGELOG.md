# Changelog

All notable changes to FeralOS will be documented in this file.

## 2026-10-25 — ADR-010 R1 amendment: system units carry the OpenRC initd name; fwupd/PPD activation-only
- Naming rule follows WHO activates the service: user (session) services = canonical bus name + `.user`; system services WE boot-enable = exactly the OpenRC initd name (the `services` map alone drives both inits; `dinitServices` removed/empty — mechanism kept for exceptional cases); dbus-activated on-demand system services (fwupd, PPD, upower, udisks2, avahi, wpa_supplicant, nm_dispatcher, accountsservice, bolt, cups-pk-helper, flatpak SystemHelper, fprintd, geoclue, kcron, the Plasma KAuth helpers) keep the bus name and are NEVER boot-enabled. Bus names stay on the D-Bus side (override filenames/`Name=`, `dbus-wait-for -n` args, polkit action ids).
- aports `e7f1cc8`: `elogind`/`polkit`/`bluetooth` unit renames + dependents retargeted (turnstile, greetd, sddm, cosmic-greeter); rules cleanup — polkit/fwupd/bluetooth/elogind removed from `dinitSkipServices` (else the renamed units would never be enabled); fwupd on-demand on BOTH inits (was: always-running).
- Harness: expected service lists updated; `power_profiles` reworked (PPD activation-only — the real switch proves on-demand activation); `dbus_activation_env` system half race-proofed.
- Verified: kde 77/77 · cosmic 77/77 · niri 113/113 · niriWayle dinit 109/109 · niriWayle OpenRC 92/92.

## 2026-10-25 — Cosmic unified on elogind: seatd daemon dropped — harness cosmic 77/77
- Nothing on Alpine 3.24 depends on the seatd daemon (cosmic-comp reaches the seat via libseat's logind backend) — dropped from the DE; packages/services `seatd`→`elogind`, groups += `seat`.
- aports: `cosmic-greeter-dinit` r6 greeter units `depends-on = org.freedesktop.login1` (r5 wrong unit name live-caught: "service description not found"); NEW `playerctld-user-dinit`; rules `dinitServices {org.freedesktop.login1: default}` + `glib-networking-user-dinit`.
- Verified: cosmic full-feature dinit `2026-10-25T14-19-46Z` 77/77 — the turnstile user dinit + materializer stack now runs on ALL four DEs.

## 2026-10-25 — KDE session-bus ADR-010 coverage + installer.log WARNING sweep — harness kde 77/77
- KDE session D-Bus targets now run as supervised per-user dinit units (turnstile + `dbus-session-materializer` already in the stack; `pam_turnstile` arrives via Alpine's `base-session` include in sddm's PAM).
- aports `1289145` (3 unit-only packages): `obexd-user-dinit`, `ibus-user-dinit`, `system-config-printer-user-dinit`; KDE list += 9 session packages. `org.kde.kcookiejar5` documented exemption (kded5 binary absent on Alpine 3.24 — no unit possible).
- Harness: `log_errors` now fails any non-allowlisted WARNING in installer.log (allowlist: the 2 apt notices + ssh known-hosts); installer boot.d symlink resolves units in `/lib/dinit.d` OR `/etc/dinit.d` (fixed the turnstiled enable warning).
- Verified: kde full-feature dinit `2026-10-25T03-41-55Z` 77/77.

## 2026-10-25 — Code-polish plan registered
- Read-only analysis → approved refinement plan (P0 pre-release docs/CI, P1 robustness, P2 maintainability, P3 cleanups/tests); intentional-duplication policy recorded (niri.go/niri_wayle.go treated like disk layouts — do NOT merge).

## 2026-10-24 — KDE/COSMIC session autologin + runtime checks — harness 74/74 each
- Autologin generalized to all four DEs (kde via sddm `[Autologin]` drop-in; cosmic via greetd `[initial_session]`); NEW `kde_session`/`cosmic_session` checks (compositor + session shell + wayland socket); `pgrep -f` anchored (busybox `pgrep -x` false-negative class — processes proven RUNNING while `pgrep -x` polled 90s).
- Product fixes (aports `bd4db35`): cosmic-greeter 1.0 is a greetd frontend — now runs `greetd` with the FeralOS config (VT 7, base-only PAM override, `pam-rundir` dep); cosmic `groups` += `seat` (libseat seatd socket `root:seat 0770` — debug_run-proven).
- Verified: kde `2026-10-24T23-35-39Z` 74/74 (kwin_wayland + plasmashell live); cosmic `23-50-51Z` 74/74 (cosmic-comp + cosmic-session, wayland-1).

## 2026-10-21 — Full-feature DE matrix (niri/kde/cosmic) + ADR-010 system-bus coverage everywhere
- dinit niri 113/113 · kde 73/73 · cosmic 73/73 · niriWayle 109/109 + OpenRC 92/92. Missing services found+implemented: niri += `gnome-keyring-user-dinit` (secrets/prompter/seahorse units), upower, portal-gnome; kde += `plasma-kauth-dinit` (15 KAuth units), `fprintd-dinit`, `bolt-dinit`, PPD/accountsservice/upower; cosmic += geoclue/upower.
- NEW unconditional `dbus_system_activation` check (system-bus coverage on EVERY DE); `dinit_shutdown_commands` zz-power gated to the niri family.
- Idle-exit fix (aports `ee2b824`): `smooth-recovery = false` for geoclue/fprintd/EDS factories — a legitimate idle exit stops the unit, the next activation restarts it (long-running daemons keep smooth-recovery).
- Remaining gap documented → KDE/cosmic SESSION-bus coverage (closed in the 10-25 rounds).

## 2026-10-20 — ADR-010 third-party activation units (app set) + migration gate — 108/108 dinit + 92/92 OpenRC
- 15 D-Bus activation targets with no supervised unit covered: 7 new aports packages (avahi, udisks2, geoclue, cups-pk-helper, kcron system; goa ×2 binaries, EDS ×4, GNOME apps session); avahi's upstream `Exec=/bin/false` systemd-only trap replaced by a real foreground `avahi-daemon` unit; exact Alpine Exec paths/lifecycles verified.
- Harness: `dbus_activation_units_live` (real activation of UDisks2 + GOA through the shim → dinit, no `DBUS_STARTER_*`) + `dbus_activation_migration` (forward-looking retirement gate: enumerates every activation file live; fails only when dbus-broker is installed AND every target is unit-covered).

## 2026-10-19 — niriWayle follow-up: kglobalaccel stays KDE-only (wiring reverted) — 106/106 dinit + 90/90 OpenRC
- `kglobalaccel`/`kglobalacceld` is a Plasma global-shortcuts service for KDE apps, NOT a niri/Wayle dependency — the earlier wiring reverted; the aports unit + override remain optional coverage (the KF6 `kglobalaccel` LIBRARY stays as a kio/kded/kwallet transitive dep — only the daemon package matters).
- Decisions closed: kglobalaccel KDE/Plasma-only (never in niri/niriWayle); dms-shell split DROPPED; booster initramfs module policy final (parked behavior).

## 2026-10-18 — D-Bus activation → dinit (ADR-010) CLOSED: system-bus helper-eligibility fix — 106/106 dinit + 90/90 OpenRC
- Root cause (live debug_run + dbus 1.16.2 source): the setuid `dbus-daemon-launch-helper` resolves names ONLY against the standard dirs (`/usr/local/share` > `/usr/share` > `/lib`); `/etc/dbus-1/system-services` is daemon-enumerated but NEVER helper-activatable (`Spawn.ServiceNotFound`).
- Fix (aports `2e0caa8` + `5f729ea`): 13 system overrides → `/usr/share/feralos/dbus-system-services` + NEW `dbus-services-setup` (boot materializer into `/usr/local/share/dbus-1/system-services`, both inits, ordered before dbus); `dbus_activation_uniform`/`dbus_activation_env` reworked init-aware.
- Docs: ADR-010 + `docs/architecture/dbus-activation.md` (checklist + diagnostics) + `docs/architecture/boot-oneshots.md` + aports `DBUS-ACTIVATION.md`.

## 2026-10-18 — power-profiles-daemon (niriWayle): verified + polkit switch fix — 104/104 dinit + 88/88 OpenRC
- Audit answer: PPD 0.30 IS in both DE lists and Wayle ships a `power_profile` dropdown — but under dinit `switch-profile` was silently denied (polkit `NoSessionForPID`: no session cgroups → no allow_active). Alpine ships PPD D-Bus-activated by design (no initd; both bus names activatable via the shared template).
- Fix: `writePowerPolkitRule` extended with both PowerProfiles switch/hold actions (both inits); harness NEW `power_profiles` — REAL switch round trip as the session user (`balanced → power-saver → balanced` green verbatim on both inits).

## 2026-10-18 — stasis system config shipped in aports + veilad env wrapper + suspend polkit — 103/103 dinit + 87/87 OpenRC
- Real gap: stasis had NO FeralOS config — its first-run bootstrap would generate a generic systemd template (`swaylock` + `systemctl suspend`, absent) and PREFER it. Fixed the NixOS-module way: `/etc/stasis/stasis.rune` (veila lock 5min → niri power-off-monitors → `loginctl suspend`; laptop ac/battery plans; inhibitors), `stasis-session` execs `stasis -c` (bootstrap skipped).
- `veilad-session` env wrapper (WAYLAND_DISPLAY/NIRI_SOCKET absent in user-dinit env); installer polkit `login1.suspend` added for BOTH inits; harness `stasis_idle` + `power_polkit_rule` new, `veila_daemon` extended.

## 2026-10-18 — Wayle session actions wired + stasis declared shipped — harness 102/102
- Wayle v0.7.0 dashboard dropdown buttons exec `sh -c <command>` (source-verified) — installer seeds `~/.config/wayle/config.toml`: lock=`veila lock`, logout=`niri msg action quit`, reboot=`sudo -n /usr/bin/reboot` (dinit zz-power NOPASSWD) / `loginctl reboot` (OpenRC), poweroff=`loginctl poweroff`; power module → dashboard dropdown. Upstream defaults are systemd-only — the seeds ARE the product.
- stasis no longer "v2" (shipped de facto since wayle-session r3); harness `wayle_session_actions` (7 asserts) + `stasis_idle`; oo7 unlock-count parse fix (ssh known_hosts warning broke Atoi).

## 2026-10-18 — FERALOS_NAME: wolf→bear, based-on-Alpine framing; template default niriWayle
- `docs/FERALOS_NAME.md`: the Bear as spirit (brown bear = the Alpine rewilding symbol, *Life Ursus*); wording settled — FeralOS is **based on Alpine** (it consumes Alpine's repositories; "Alpine fork" stays on the vocabulary Avoid-list). `variables.yaml` template default DE → niriWayle.

## 2026-10-18 — Review nits + whole-project doc refresh; niriWayle declared flagship
- 7 harness-side nits fixed (PAM labels/constants, count-delta secrets proof, recovery-wrapper assert); docs refreshed project-wide; README/ROADMAP/AGENTS declare niriWayle the flagship / true FeralOS experience (other DEs = user convenience, never "ready-to-use").

## 2026-10-18 — oo7_pam_unlock GREEN: authenticated PAM proof — niriWayle harness 100/100
- Zero product changes — every failure was harness-side. Final transport: direct PAM chain (python3+ctypes libpam over plain exec SSH: `pam_start("greetd")` → authenticate → acct_mgmt → open_session) — autologin session + live daemon untouched, no race.
- Root cause that hid everything: ctypes RTLD_LOCAL — pam_oo7 has NO `DT_NEEDED libpam.so.0` → musl dlopen failed → PAM silently skipped the `optional` module; fix = one line `RTLD_GLOBAL`. Optional upstream note: Alpine could link libpam into pam_oo7 like every other Alpine PAM module.
- Ladder: 99/100 → 99/100 → 100/100 (run `2026-10-18T03-03-11Z`, 0 fail/skip).

## 2026-10-17 — oo7 full-desktop unlock parity (niriWayle): PAM login-unlock + password re-sync
- Source-verified oo7 0.6.0: pam_oo7 = 3 stacks (auth stash / session auto_start fork / password re-sync on chauthtok); the daemon's PAM listener socket (`oo7-pam.sock`) unlocks ALL locked collections at login. An Alpine packaging accident keeps supervision intact: upstream's hardcoded `/usr/bin/oo7-daemon` fallback is a no-op on Alpine → the retry loop hits OUR supervised daemon.
- First-use bootstrap: the auto-created empty "Login" keyring accepts any password — the PAM unlock binds it to the login password; no prompt, ever. Installer: `pam_oo7` auth + session (LAST) lines in `/etc/pam.d/greetd` + `/etc/pam.d/passwd` override.
- Known limitation (universal, same as GNOME): root-forced `chpasswd` resets carry no old authtok → keyrings prompt once on next login.

## 2026-10-17 — aports: stasis 1.6.3 idle manager + wayle-session r3 published (CI green)
- stasis 1.6.3 (Rust, context-aware Wayland idle manager) + supervised `stasis.user` + `stasis-session`; AGPL §13 posture kept (canonical AGPL text vendored, hash-pinned — upstream tarball ships only LICENSE).
- CI catch recorded: an APKBUILD sum bump must always bring its source files in the SAME commit.

## 2026-10-17 — user-services completion + Key Rack flatpak + Secret portal pin + supervision rule — niriWayle 99/99, niri 99/99, niri OpenRC 83/83
- Seahorse/oo7-server file conflict (both ship `org.freedesktop.secrets.service` — permanent apk breakage): seahorse removed from niriWayle; Key Rack secrets GUI via Flathub (`app.drey.KeyRack`, new `flatpakPackages` key + `keyrack_flatpak` check); kept on niri (gnome-keyring owns secrets there, proven green).
- niriWayle portals pin `org.freedesktop.impl.portal.Secret=oo7-portal`; nix XDG_DATA_DIRS gap fixed in BOTH DEs (`/etc/environment` + niri config); SUPERVISION RULE written into AGENTS: every user-session daemon = supervised per-user dinit service — nothing unsupervised (D-Bus activation, autostart, self-spawn), BOTH inits.
- aports `xdg-desktop-portal-user-dinit` r1: portal-order name gates (permission-store → document-portal → frontend) kill the activation twin race (exit 20); aports CI skip-if-unchanged (38-54 min → ~3 min).

## 2026-10-16 — niriWayle: second desktop environment (niri + Wayle shell + Veila + Raffi + oo7) — harness 97/97
- New `desktopEnvironment: niriWayle`: niri + Wayle shell (bar/notifications/OSD/wallpaper built-in), Veila lock screen, Raffi launcher, oo7 secrets (gnome-keyring replacement); shares turnstile/per-user dinit, pipewire stack, portals, polkit agent, wayvnc, greetd/regreet, elogind, matugen.
- aports: wayle 0.7.0, veila 0.4.4, raffi 0.24.0, wl-clipboard-rs 0.9.4, upower-dinit, wayle-session (session script + user units + .desktop).
- Session gotchas live-caught: `wayle shell` (foreground) not `panel start` (daemonizes+exits → dinit restart loop, started marker never held); `wayle-shell-session` wrapper resolves WAYLAND_DISPLAY/NIRI_SOCKET (user-dinit env has neither).

## 2026-10-14 — openrc-always: OpenRC enablement never conditional — dinit 98/98, OpenRC 82/82
- User review correction: a dinit install must remain bootable under OpenRC (fallback) — every rc-update enablement now runs regardless of the active init (the links are inert under dinit); the dinit-side enablement stays conditional. `klogd-openrc` installed on BOTH inits; NEW `openrc_fallback_ready` harness check.

## 2026-10-14 — turnstile: OpenRC user-session stack (dual-init) + rules cleanup — dinit 97/97, OpenRC 82/82
- OpenRC gets the identical per-user dinit stack via turnstile (service-manager-agnostic; only its backend is dinit). aports: restored `turnstiled.initd`/`.confd` + boot-order fix `need elogind; before greetd` — greetd's autologin opened before turnstiled's socket, `pam_turnstile` optional/silent → no user dinit (8 failed checks, live-diagnosed via debug_run).
- Harness: `turnstile_pkg` init-conditional, busybox `pgrep -x` → `pgrep -f`, 5 session-stack checks un-gated to both inits; `wireless-regdb` restored (accidental removal caught by `nm_wifi_support`).

## 2026-10-13 — matugen packaged in feralos-aports + wired into the niri stack
- matugen 4.2.0 — the wallpaper→palette backend DMS runs for dynamic theming — absent from Alpine entirely: FIRST Rust/cargo package in the repo (Alpine `bottom` pattern: `fetch --target --locked` + `build/test --frozen`), docker-rehearsed (sha512-pinned source, signed apk, signature-verified install), CI green + published; niri list += matugen; harness `matugen_installed`.

## 2026-10-13 — theme: ALL manual theme seeds removed (dms-theme-sync plugin adoption, Phase A)
- User decision: third-party DMS plugin `arqueon/dms-theme-sync` (GPL-3.0) becomes the theming authority (propagates DMS appearance to GTK/Qt/KDE/Flatpak) — kdeglobals seed, gtk-3/4 settings.ini seeds, flatpak override, dconf theme default all removed to avoid conflicts; KEPT: dconf profile infrastructure (gsettings needs it), session.json dark seed, Qt portal theme env.
- First-run catch: Alpine's dconf ships NO `/etc/dconf` at all — installer `mkdir -p /etc/dconf/db/local.d` before `dconf update` (caught by harness `log_errors`).

## 2026-10-12 — flatpak theming binds + hotkey overlay + chown sweep (corrections applied same day)
- Flatpak user-global override = binds ONLY (`kdeglobals`/`gtk-3.0`/`gtk-4.0` ro) — NO forced `GTK_THEME` (apps must FOLLOW the theme chosen in DMS); hotkey-overlay `skip-at-startup` REVERTED (the "Important Hotkeys" cheatsheet at login is wanted); final `chown -R user:user ~/.config ~/.local` sweep fixed root-owned seeded dirs (per-seed chowns covered files, mkdir-created dirs stayed root).

## 2026-10-12 — KDE apps dark via kdeglobals (okular/ksystemlog were light)
- KF6 source-verified: KDE apps read colors via `kdeglobals [Colors:*]`, NOT the portal/QPalette path; manager-instantiation apps (okular) bypass kdeglobals and resolve embedded scheme files by id → `[UiSettings] ColorScheme=BreezeDark` pin added. Seed = self-contained BreezeDark `[Colors:*]` + breeze-dark icons (avoids Alpine `breeze` + qt5/kf5 ~100MB).
- Greeter user hidden from ReGreet 0.2.0's UID-filtered list (now a system user via `adduser -S -G greeter`; busybox `adduser -S` without `-G` → primary group nogroup, chown failure caught live); fish single-quote escaping gotcha in the harness fixed (double-quoted echo).

## 2026-10-12 — DMS reboot via custom power command (no elogind rebuild)
- Live diagnosis: `loginctl reboot` → elogind execs the ABSOLUTE `/sbin/reboot` (busybox SIGTERM) AFTER its hooks → dinit's handler OVERWRITES the REBOOT type with HALT ("System halted", machine frozen). Fix: DMS `customPowerActionReboot = sudo -n /usr/bin/reboot` + `/etc/sudoers.d/zz-power` NOPASSWD (`zz` sorts after `wheel` — sudo-rs applies the LAST matching rule; live-verified: reboot ~20s, fresh uptime, TPM auto-unlock).

## 2026-10-12 — theme: system-wide dark preference (portal color-scheme fixed)
- Portals reported "no preference" — dconf was ABSENT and `color-scheme` never set; `QT_QPA_PLATFORMTHEME=gtk3` was DEAD (Alpine ships no QGtk3 plugin, only xdgdesktopportal). Fix: `dconf` + system db `prefer-dark` + `/etc/dconf/profile/user` (Alpine ships none — GLib ignored the system db entirely) + gtk-4.0 settings.ini seed + Qt portal theme env. Live re-verified: `gsettings get` = `'prefer-dark'`.

## 2026-10-12 — niri: prefer-no-csd in the user config (frameless, Windows-like)
- Global `prefer-no-csd` (no per-window toggle exists; DMS's embedded config ships it active, ours now matches). Documented reality: GTK3/4 windows without a custom titlebar go frameless; libadwaita/GNOME apps and Firefox KEEP their header (client-requested CSD honored by design; `GTK_CSD=0` is dead code on Wayland); Qt drops decorations via xdg-decoration + `QT_WAYLAND_DISABLE_WINDOWDECORATION=1` (numeric 1 — "true" parses to 0 in Qt).

## 2026-10-12 — dms: window-buttons plugin wired system-wide (bar back on top)
- New repo `dms-window-buttons` (ISC) → aports package into DMS's system plugin dir; settings seed uses DMS's own v1→v2 migration (bottom bar + right widgets + runningApps — no default duplication that would drift with upstream). Plugin v1.0.1: buttons reworked to `niri msg action` with self-resolved socket — v1.0.0 used `NiriService.send` behind a NIRI_SOCKET guard DMS doesn't have (only native close worked).

## 2026-10-11 — feralos-update script wired to the DMS System Update button
- DMS has no native apk backend (pacman/dnf/apt/zypper/xbps/pkg/portage/flatpak only) → `updaterUseCustomCommand` + `/usr/local/bin/feralos-update`: apk update/upgrade → flatpak (if installed) → nix (channels, profile upgrade, GC); best-effort error counter, exit code = errors. Script written via ExecSilent (the echoed body tripped `log_errors`). Caveats: DMS upgrade timeout 30 min; no harness check by user decision (edge-package upgrade limitation in-VM).

## 2026-10-11 — DMS power off/reboot dead under dinit (elogind → busybox) + polkit allow_active gap
- Root cause: elogind BYPASSES PATH and execs the absolute `/sbin/poweroff` (busybox SIGUSR2, ignored by dinit) AFTER running its hooks — the `/usr/bin` dinit wrappers are never consulted. Fix: `/etc/elogind/system-shutdown/50-feralos-dinit.sh` hook execs the dinit wrappers per verb (before the harmless busybox exec).
- Bundled finding: under dinit elogind cannot map PIDs to sessions (`NoSessionForPID`) → polkit never sees allow_active → every login1 power action = auth challenge. Fix: `/etc/polkit-1/rules.d/49-feralos-power.rules` (wheel → power-off/reboot/halt YES — standard desktop posture).

## 2026-10-11 — nix: flake registry pinned to stable 26.05 system-wide (unstable leak fixed)
- The stable CHANNEL existed but the FLAKE registry pin lived inside `if len(NixPackages) > 0` — enableNix without nixPackages (the niri default) left the builtin nixpkgs→unstable active for every `nixpkgs#...` reference; root never pinned. Fix: system-wide `/etc/nix/registry.json` (every user incl. root) + user pin unconditional; harness `nix_registry_stable`.

## 2026-10-11 — pkexec refused the FeralOS login-shell (DMS user creation dead)
- pkexec validates the session user's SHELL against `/etc/shells`; the login-shell path was deliberately unlisted (the old upstream re-exec trap) → EVERY pkexec in a niri session failed (all DMS privileged ops, RC=127). The trap is gone from our niri-session — fix: idempotently list `/usr/libexec/feralos/login-shell` in `/etc/shells`; harness `login_shell_trick` extended (falsifiable).

## 2026-10-11 — niri: DMS theme seeded green + dark seed + bottom bar + running apps + default apps + icons
- DMS 1.6.2 source-verified seeds: green theme (`currentThemeCategory: generic` + `currentThemeName: green`), `session.json` `isLightMode: false` (portal-independent dark), bar-at-bottom + runningApps via DMS's own v1→v2 migration (zero default duplication), `xdg-terminals.list` + `mimeapps.list` defaults (desktop IDs verified against Alpine contents — eog is `org.gnome.eog.desktop`), gnome-maps/calendar scheme handlers.
- Icons: `adwaita-icon-theme` + `hicolor-icon-theme` as dms-shell deps (aports r3) + GTK settings seed (DMS resolves icons via the GTK theme chain; user-reported missing icons on real hardware). GOTCHA: per-file chowns AFTER the general `chown -R` (later seeds would land root-owned).

## 2026-10-11 — session apps bound to the niri session (add-wants model) — greeter-DMS bug class eliminated
- Research (niri/DMS/turnstile docs): session apps must be compositor-session scope, not login scope. aports `niri-user-dinit` r3: `niri-session` starts polkit-agent/dms/wayvnc existence-filtered right after `dinitctl --user start niri`; installer boot.d = infrastructure only (5 links); harness asserts the 3 session apps ABSENT from boot.d. Lifetime = upstream semantics (start at real login, teardown at last logout).

## 2026-10-11 — aports: DMS no longer starts on the greeter session (dms-shell guard)
- `dms.user` was linked system-wide → the greeter user's own user-dinit started the full desktop stack (the `wait-wayland` gate opens on the greeter's niri too). Fix: `dms-session` clean `exit 0` for greeter (username + `XDG_SESSION_CLASS` guards); `dms.user` restart dropped (restart=true would loop on the clean exit; smooth-recovery kept for real crashes).

## 2026-10-11 — config: edge-repos auto-enable + wizard pre-filled on invalid config
- `extraPackagesEdge` without `enableEdgeRepos` hard-errored BEFORE the wizard — the user could not fix the value interactively. Fix 1: `applyImpliedFlags` at load (edge list ⇒ flag, single source; hard error removed). Fix 2: `LoadConfig` returns conf on validation errors → wizard pre-filled; still fatal before any disk work when still invalid (unattended). Tests added.

## 2026-10-10 — branches: developDinit → develop → main merged; work continues on develop
- developDinit → develop fast-forward; develop → main merge commit `8568df0` with 9 conflicts resolved (develop side everywhere — main's 2 dinit reverts cancelled intentionally). All 3 branches pushed, SHA parity verified.

## 2026-10-10 — validation: tests + harness dinit/niri 94/94 + debug_run live; QEMU flag fix
- Post-rename regression green: harness 94/94 (runner qcow2 rebase, disk 3G→8G, cloud-localds reinstall); debug_run live boot 8/8 (dinit PID1, boot chain, user dinit stack, greetd socket, TPM auto-unlock).
- Bug: debug_run passed `virtio-vga-gl,venus=on,blob=true,hostmem=1G` — apt QEMU 8.2.2 rejects `venus` → QEMU exit 1. Fixed to plain `virtio-vga-gl` (`dccb79a`), validated with a native boot.

## 2026-10-10 — src: Go module alpineInstaller → feralos/src (completes the rename)
- `src/go.mod` → `module feralos/src`; ~50 files' imports updated (121 insertions / 121 deletions); docs synced; branch name `alpineInstallerGolang` kept (historical).

## 2026-10-10 — repo: Go modules + paths renamed skeaAlpineInstaller → feralos
- harness + debug_run modules → `feralos/...`; runner-VM repo root → `/home/<user>/feralos` (repo copied at runtime — safe for existing runner images); historical occurrences kept deliberately.

## 2026-10-10 — repo: hosting migrated Gitea → GitHub (skea999/feralos)
- Parity verified BEFORE switching (`git ls-remote` both sides: identical SHAs, zero tags; fast-forward push only). Old `origin` → `gitea` remote during confidence period; `gh auth setup-git`; no CI to port.

## 2026-10-10 — host: Nix QEMU stack removed, apt QEMU 8.2.2 validated 94/94
- All 5 wrapper locations + the nix `qemu` profile removed (swtpm/OVMF/virt-manager/mesa KEPT); AppArmor nix overrides cleaned (backups kept). Insight: the GL path niri uses = guest mesa virgl ↔ host libvirglrenderer — the QEMU version is NOT the discriminator; full run 94/94 with zero GL regressions.

## 2026-10-10 — DMS wifi/BT harness checks: 2 real bugs found+fixed — 94/94
- NEW `bluetooth_service` (started marker + bluetoothd + org.bluez registered on the system bus — what the DMS BT widget talks to) + `nm_wifi_support` (NM wifi plugin + wireless-regdb + live `nmcli radio`). Bug 1 (aports `fb64d77` r3): bluez-dinit ran the Debian bluetoothd path → never started. Bug 2 (harness): wrong wpa_supplicant path — the wifi stack itself was healthy.

## 2026-10-10 — bluetooth service unified + niri-only enablement
- feralos owns bluez-dinit → the name split was pointless AND the base-list scope was wrong (bluetooth would be enabled on every dinit install). aports `e9ac00c`: service renamed `bluez`→`bluetooth` (same name as the OpenRC initd); rules back to clean; enablement = single `bluetooth: default` in the NIRI services map only.

## 2026-10-10 — DMS bluetooth + NM wifi completeness (data-only round)
- Live-diagnosed via `dms doctor`: bluetooth was in NO package list (the DMS BT widget speaks org.bluez) → niri += bluez/bluez-openrc/bluez-dinit; `networkmanager-wifi` missing meant NM had NO wifi support at all → += networkmanager-wifi + wireless-regdb. Names verified against Alpine 3.24 via `apk add -s`.

## 2026-10-10 — wizard password double-entry confirmation
- Password + LUKS passphrase now asked twice (masked; mismatch = both re-asked from the first entry); empty LUKS (reuse user password) skips confirmation; min length unchanged (validators.go single source, shared with validateConfig).

## 2026-10-09 — niri/DMS dinit runtime depth: 5 checks added/hardened, validated 91/91
- NEW `niri_layers` (layer-shell surfaces MAPPED = the DMS bar actually renders), `greetd_socket` (socket is PID-suffixed `/run/greetd-<pid>.sock` — glob gotcha), `session_dbus_socket`, `session_pipewire_socket`; `niri_screenshot` hardened to PNG magic bytes (220926-byte capture). regreet runtime not checkable on verify boots (harness autologin bypasses the greeter).

## 2026-10-09 — less shim: aports less-dummy package replaces broken symlink dummy
- The `less`→bat symlink made bat re-exec ITSELF (its hardcoded pager ignores `$PAGER`; only `cat` short-circuits) → clap `unexpected argument '-R'` — fish history, git log, man all broke. Fix: `less-dummy` 999.0.0 (`provides`+`replaces`): POSIX-sh wrapper drops all less flags, dumps once via `bat --color=always --paging=never` (chain always terminates). Harness falsifiable: bat's EXACT flag set + content passthrough + NEW `less_fish_history`.
- CI infra: workflow container → `public.ecr.aws/docker/library/alpine` (Docker Hub anonymous pull rate limit had blocked 3 runs).

## 2026-10-09 — bug-3 DEAD: niri renders in QEMU (86/86 zero skips); edge repos = confined feature
- Fix chain: host QEMU wrappers + `virtio-vga-gl` + `egl-headless,gl=on` + HWE kernel + guest mesa 26.2.4@edge (virgl gallium restored upstream) → niri has a REAL output (`Virtual-1` 1280x800), wayvnc listens :5900, screenshot writes a real PNG. First fully-real niri run in QEMU.
- Installer: `enableEdgeRepos` + `extraPackagesEdge` confined feature (append @edge repos → install the list → comment the repos out; apk v3 tagged-repo masking kills mid-install edge mixing — verified live). Console keymap from user variables; harness QEMU DMI skip-gates removed — all checks real on every host.

## 2026-10-08 — all 28 fixtures mirror variables.yaml template structure
- Every fixture carries the COMPLETE 33-key template field set (missing fields = behavior-preserving Go zero values; niri fixtures carry the 5-pkg edge mesa stack in `extraPackagesEdge`); distinctive values preserved everywhere; verified by before/after JSON diff + 28/28 yaml parse.

## 2026-10-08 — extraPackagesEdge (edge-tagged packages installed LAST)
- New variable: packages from the @edge tagged repos, installed AFTER all other package installs + FinalUpgrade (base completes from stable first). Requires `enableEdgeRepos` (hard validation error otherwise); per-package `@tag` accepted; no wizard prompt (template-only).

## 2026-10-07 — console keymap (loadkmap) restored under dinit
- The OpenRC→dinit migration lost the busybox-initscripts loadkmap service. New aports `busybox-loadkmap-dinit`: scripted service + `/usr/lib/dinit/loadkmap` wrapper (sources `/etc/conf.d/loadkmap`, default `us`) — installer rewrites KEYMAP= from user variables (kbd-bkeymaps layout: keymap == layout dir). chimera early-console documented as no-op for bkeymaps (paths don't exist).

## 2026-10-05 — check mechanics fixes validated on LIVE target (standalone verify 85/85)
- 5 output-dependent niri checks re-guarded with DMI product_name+sys_vendor concat (q35 values verified verbatim on the live target); `dms_ipc` real syntax `dms ipc call outputs status` — GOTCHA: dms answers UNKNOWN targets rc 0 too, only real target/function pairs prove. Working read-only dms calls catalogued (audio/lock/outputs/notifications).

## 2026-10-05 — greeter dinit log dir; 4 new niri/dms runtime checks; OpenRC minimal 49/49
- Greeter user services log under `$HOME/.local/share/dinit` — dir was missing for `/var/lib/greetd` ("opening log file" failures). Dead eth0 write documented as non-niri-flow-only. NEW checks: `niri_msg_version`, `niri_windows`, `niri_outputs_count` (QEMU skip), `dms_ipc`; realhw script parity (`niri_outputs_count` HARD there).

## 2026-10-03 — caef07b fixes VALIDATED; bug-3 root cause = zero wl_outputs in QEMU (77/80 ×2)
- All 3 aports fixes work: race poll (the earlier failure was a harness regex false negative — niri IS supervised), niri.log captures stderr, launchers green. Root cause captured verbatim: bochs card0 (no driver) + virtio-gpu-gl card1 (`driver missing` — guest mesa lacks venus/virgl) → niri listens with ZERO devices → layer surfaces closed, wayvnc crash-loop, screenshot writes nothing. Fix directions → the 10-09 round.

## 2026-10-03 — niri-session fix VALIDATED; 3 product bugs diagnosed (77/80 ×2)
- Session fix works: verify boots hold `New session c1` @4.9s → removed only at teardown; 3 runtime checks green for the first time. Diagnosed: (1) single-shot pgrep race → nondeterministic fallback exec (compositor outside user dinit); (2) user-dinit env lacks WAYLAND_DISPLAY → wayvnc/polkit-agent crash-loop every boot; (3) screenshot exits 0 writing nothing (bug-3). Harness: screenshot fresh-PNG mechanics + Pictures/Screenshots ownership fix.

## 2026-10-02 — earlyoom tiered victim selection
- Tier 1 user apps killed FIRST (`--prefer` +300: 172 apps; flatpak comm names; 15-byte truncation-correct); Tier 3 system core NEVER chosen (`--ignore` hard skip — key insight: `--avoid` is soft and can be outbid; covers PID1/session mgr/bus/network/logging/ssh/login path); Tier 2 desktop session natural oom_score order. POSIX-sh wrapper; config per the ADR-008 pattern.

## 2026-10-02 — dinit services aligned to Artix/Chimera (ADR-009) — 74/74 dinit + 49/49 OpenRC
- 48 -dinit pkgs after a full audit: `restart=true` removed from ALL daemons (smooth-recovery only); getty keeps restart + `termsignal=HUP` + config-driven baud; `before=login.target` ×33; network ordering (firewalls before pre-network, NM → network.target, avahi/rsync depend networking); apparmor before local.target; dnsmasq privilege drop (Artix exact); dbus `smooth-recovery=no` + stop guard; NEW `dbus-wait-for-dinit` readiness helper; lxc restart-loop fixed; sshd keygen prereq. Dead pipewire/wireplumber system dinit pkgs removed. Audit facts recorded (Artix dbus-activates polkit/udisks2/fwupd by design; irqbalance/rasdaemon/incus/nix/vector/klogd/cosmic-greeter not packaged there).

## 2026-10-01 — conf.d-equivalent config files for all -dinit services (ADR-008) — 74/74 dinit + 46/46 OpenRC
- 22 -dinit packages converted to the wrapper+config-file pattern (Artix): service → `/usr/lib/dinit/<svc>` wrapper → sources `/etc/dinit.d/config/<svc>.conf` → exec daemon — user-editable, apk-protected; the OpenRC `/etc/conf.d` convention restored (defaults match Alpine conf.d; dnsmasq keeps its real config path; cpufrequtils gained stop opts).
- Harness: permanent `dinit_config` check (config dir + ≥10 `.conf` + wiring spot-checks).

## 2026-10-01 — klogd-openrc package (989 series): no more runtime file writes

- OpenRC klogd now ships as **klogd-openrc 989.0.1.0** in feralos-aports (commit `2d5e05f`, CI green): dependency-free `/etc/init.d/klogd` (dmesg snapshot + `klogd -n` → `/dev/log`), `replaces="busybox-openrc"` (apk refuses the file overwrite without it — found by rehearsal)
- Installer: removed the runtime initd write from `logging.go`; the OpenRC path installs the package (`packages.go`); enablement unchanged (boot runlevel via `internal_rules`)
- Harness: `klogd_service` now also asserts the package file marker (`klogd (vector)`) on OpenRC
- Verified: OpenRC minimal **48/48** (run 2026-10-01T18-14-18Z — `Installing klogd-openrc (989.0.1.0-r0)`, klogd `[ started ]`) + dinit niri **72/72** regression (18-17-10Z)
- Note: **earlyoom** verified as already configured — defaults `-m 10 -s 10` (SIGTERM at 10%, SIGKILL at 5%), `-r 60` (memory report interval 60s vs default 1s), `earlyoom-dinit` service enabled/running; Alpine's `conf.d/earlyoom` is OpenRC-only and intentionally ignored

## 2026-10-01 — Disk maintenance schedule (scrub/TRIM/btrbk) + OpenRC klogd override

- Weekly disk maintenance via cronie (binary paths resolved at install; output in dedicated logs, logrotate 7d): **btrfs scrub** (`btrfs scrub start -B /`, Sun 03:30), **weekly TRIM** (Sun 03:00, dedicated script), **daily btrbk** snapshots (02:00; retention already in `btrbk.conf`: 7 daily / 4 weekly / 3 monthly)
- TRIM on encrypted volumes without touching luksFormat/booster: the script enables discard passthrough on active dm-crypt mappings (`cryptsetup refresh --allow-discards`), trims, then clears it again (minimal exposure window). Effective only when the underlying device supports TRIM (no-op in QEMU)
- **OpenRC klogd fixed** ("non c'è un modo?" → yes): the vendor script needs `logger` (busybox syslogd, replaced by vector); the installer now overrides `/etc/init.d/klogd` (dmesg snapshot + `klogd -n` → `/dev/log`, `after localmount vector`), enabled in the boot runlevel — kernel ring → `kern.log` works on BOTH inits
- Harness: new `disk_schedule` check (trim script + 3 cron entries); `klogd_*` and `log_coverage` validations now unconditional (both inits)
- Verified: dinit niri **72/72** + OpenRC minimal **48/48** (runs 2026-10-01T17-44-16Z / 17-48-47Z); OpenRC serial shows `Starting klogd (vector) ... [ ok ]`

## 2026-09-30 — Logging Round 2: split sinks, JSON view, debug API, conditional sources

- New sinks: `sudo.log` (authpriv + sudo), `firewalld.log`, and `/var/log/feralos.json.log` (JSON view of syslog + vector internal + dinit service logs [+ incus], with the dinit service name extracted from the file path); vector `internal_logs` now flows into `messages`/JSON so vector errors join the central timeline
- Local Vector API (`vector top` / `vector tap`) on `127.0.0.1:8686` — **only with `enableDebug`** (test-only flag); production configs never expose it
- Extra sources are conditional — never configure software that isn't installed: `/var/log/incus/*.log` only when `incus`/`incus-feature` is installed. Webmin skipped: its logs live in the container tmpfs (no stable host path)
- Logrotate retention split: auth/sudo 30d (security), messages/daemon/kern 14d, tool logs 7d
- Harness: 3 new checks — `json_log`, `vector_api` (debug-gated validation), `vector_validate` (`vector validate --skip-healthchecks`) — plus `log_coverage` (core sinks must have content; `kern.log`/`dmesg` dinit-gated)
- Verified: config pre-validated with `vector validate` in docker (caught 2 VRL 0.55 strictness errors before any harness run), harness niri+dinit **70/70 PASS** (run 2026-09-30T23-09-27Z, json 523 lines, kernel ring OK)

## 2026-09-30 — Logging Round 1: kernel ring relay (klogd) + vector hardening

- **klogd-dinit** (feralos-aports `f2842dc`, CI green): new dinit core package forwarding the kernel ring (`/sbin/klogd -n`, busybox) to `/dev/log` → vector routes facility `kern` to `/var/log/kern.log`; the wrapper also writes a one-shot `/var/log/dmesg` boot snapshot. Context: after rsyslog was replaced, nothing forwarded the kernel ring (vector has no kmsg source) → kern.log was orphaned (AppArmor denials, OOM, ACPI, hardware errors only on console). dinit-only: Alpine's OpenRC klogd `depend() need logger` (busybox syslogd — replaced by vector)
- Installer: `klogd-dinit` added to dinit core packages + enabled via `defaultRunlevelsDinit`
- **Vector fixes**: removed `*.log.1` from the dinit file source (copytruncate makes the rotated copy a new file → vector re-read it from the start = duplicate entries); disk buffers on all four sinks (`max_size = 268435488`, Vector's 256 MiB minimum) so a crash can't lose unflushed lines
- Harness: 4 new falsifiable checks — `klogd_service`, `klogd_running`, `dmesg_saved`, `kern_log`
- Verified: rehearsal green (klogd-dinit), harness niri+dinit **67/67 PASS** (run 2026-09-30T22-52-14Z, kern.log 56 lines, 0 bus errors)

## 2026-09-30 — Artix-parity audit: dinit services round 2 (18 pkgs) + dead-code cleanup

- Full audit of FeralOS dinit services vs Artix (49 services compared; sources: `packages/<name>-dinit`, base `artix/dinit-rc`, `turnstile-dinit`)
- Fixes shipped in feralos-aports (`90197ed`, single push, CI green): seatd readiness (`-g seat -n 3` + `ready-notification = pipefd:3` + `before login.target`); nftables `stop-command` flush; iptables `-w 3`; sddm `+= depends-on elogind`; cosmic-greeter `+= depends-on seatd`; avahi/cupsd/dnsmasq `+= depends-on dbus`; turnstiled log → `/var/log/dinit/turnstiled.log`; `smooth-recovery = true` where Artix has it (14 services)
- Cleanup: removed dead packages `pipewire-dinit` + `wireplumber-dinit` (system services, never installed — Artix ships user-only; rules reference only the `-user-dinit` variants) + stale `_override/` strays
- Verified: docker rehearsal (18 pkgs — checksum-stable builds + content checks + dinit-check), CI build+deploy+client-test, harness niri+dinit **63/63** (run 2026-09-30T22-04-51Z, 0 bus errors, new pkgrels installed)

## 2026-09-30 — dinit shutdown ordering: turnstiled stops before bus/elogind

- Root cause (source-verified + reproduced with a local dinit 0.21 build): dinit releases a dependency when its dependents *begin* stopping — at shutdown `dbus-daemon`/`elogind` could die while `turnstiled` was still closing PAM sessions (`pam_elogind(turnstiled:session): Failed to connect to system bus: Connection refused`, run 2026-09-29T22-35-57Z 62/63). `depends-on` cannot express stop-after-dependent-exit
- Fix in feralos-aports (commit `75c566a`, CI green): `dbus-dinit` r6 + `elogind-dinit` r5 add `stop-command` guards — dinit runs the script INSTEAD of signalling the process: wait (bounded, busybox-safe `pidof`) for `turnstiled` then stop the daemon; `stop-timeout = 30` failsafe; no-op without turnstile
- Harness `log_errors` allowlist entry for the pam line removed — a regression now fails the run (no masking)
- Verified: niri+dinit fixture **63/63 PASS** (run 2026-09-30T00-08-04Z, 0 bus errors, `turnstiled` stops before `dbus`/`elogind` in both shutdowns)

## 2026-09-28 — niri: gnome-keyring (secrets service) + PAM unlock in greetd

- `gnome-keyring` package added to the niri set (closes installer-gaps 4.5: no `org.freedesktop.secrets` provider — gvfs/Thunderbird could not store passwords)
- `pam_gnome_keyring` in `/etc/pam.d/greetd` (auth + session auto_start): login keyring unlocked with the login password, daemon autostarts at session open
- Secrets component is D-Bus activated on demand (`org.freedesktop.secrets` ships with the package); session bus provided by the niri-session wrapper
- Harness `gnome_keyring` check (pkg + dbus service file + PAM line) — niri+dinit run 60/60

## 2026-09-28 — LUKS swap unlock: dual-init (crypttab + conf.d/dmcrypt)

- `btrfsRaid0` + `btrfsStandardServer` write BOTH unlock configs after subvol mounts: `/etc/crypttab` (dinit — chimera `early-cryptdisks` contract) AND `/etc/conf.d/dmcrypt` (OpenRC — legacy `dmcrypt` initd format), same keyfile `/etc/dmcrypt/swap.key` — LUKS swap unlocks under either init (earlier arc wrote crypttab only, "replacing" conf.d/dmcrypt; both kept for dual-init)

## 2026-09-27 — dbus readiness for dinit dependents

- `dbus-dinit` r5 (feralos-aports) = `--print-address=3` + ready-notification `pipefd:3`: dinit now gates dependents (elogind, polkit, NM) on the system bus being live, not merely on the dbus process being spawned — the elogind "connect before socket exists" race is gone at the source
- elogind bus-connect failures eliminated: 0 occurrences across both boots in the harness serial (was: `Failed to connect to system bus` / `Failed to allocate manager object` noise)
- Harness `log_errors` allowRe no longer masks the elogind bus-error lines — regression would now surface as a failure

## 2026-09-27 — AppArmor profiles loaded under dinit

- `apparmor-dinit` r4 (feralos-aports) = scripted profile loader: sources Alpine's `rc.apparmor.functions` and runs `parse_profiles` load (openrc logging stubbed) — the `apparmor start()` equivalent under dinit
- `hardening` feature enables the service via boot.d symlink when installed (was: warning "not wired", profiles never loaded — LSM active but unconfined)
- Harness `chmod_77_check`: excludes `/mnt/tmp` (legit sticky 1777 from layout-time `SetTempDirModes` — false positive)
- Validated: harness niri+dinit 59/59 with 33 `profile_load` kernel events in serial

## 2026-09-27 — Nix: permanent trusted-users @wheel + chroot channel unpack fix

- `nix.go` ships permanent `trusted-users = root @wheel` in nix.conf (NixOS upstream default) — wheel users run `nix profile add`/flakes without sudo; per-user injection + post-install `sed` cleanup removed (trusted users are ~root-equivalent by definition, so wheel adds zero escalation)
- `nix-channel --update` inside the installer chroot now runs with `NIX_CONFIG='sandbox = false'` — the channel unpack derivation uses the build sandbox, whose pivot_root fails EINVAL in a chroot (`/` not a mountpoint); same chroot-only workaround as the profile installs, installed system keeps the default sandbox
- Harness: new `nix_trusted_wheel` check (grep `@wheel` in nix.conf, enableNix-gated); `trusted_users_clean` still asserts no per-user injection
- Full-stack harness (btrfsStandard, OpenRC, enableNix): 49/50 — only `log_errors` (QEMU udevd `/bin/false` module noise, pre-existing)

## 2026-09-27 — niri: NetworkManager switch, wayvnc remote desktop, /tmp exec, session fixes

- NetworkManager replaces ifupdown as the niri network service (DMS network/VPN UI needs the NM backend): `/etc/network/interfaces` reduced to lo-only, `/etc/NetworkManager/conf.d/10-manage.conf` `managed=true`, plugdev+netdev groups, `networkmanager` on default, WireGuard via NM (`wireguard-tools`), wifi via `wpa_supplicant`
- Remote desktop = wayvnc (`wayvnc` + `wayvnc-user-dinit` from feralos-aports, user dinit service, loopback `127.0.0.1:5900` default, TLS/auth config for LAN) — RustDesk flatpak + `spawn-at-startup` + firewall ports `21115-21119/tcp`/`21116/udp` removed (never in the validated reference stack, round 43 cleanup)
- Scanner stack (`sane`/`sane-backends`/`sane-utils`/`sane-airscan`/`simple-scan`) + `qt6-qtmultimedia` + `i2c-tools` + `wtype` (DMS optdepends promoted)
- `/tmp` subvolume mounted exec (`nosuid` kept, all four btrfs layouts) — OpenTUI/Bun `.so` extraction needs it; `/var/tmp` created `1777` — 0755 breaks `O_TMPFILE` (flatpak --user, podman)
- CUPS `SystemGroup wheel` in `cups-files.conf` (Alpine lpadmin default 403s wheel users)
- Dinit conversion skips session/NM-managed services (pipewire stack, `wpa_supplicant`, `wireguard-tools`)
- aports: `+networkmanager-dinit` +`wayvnc-user-dinit`
- Harness niri+dinit fixture: 59/59 PASS (rustdesk checks removed, `/var/tmp` 1777 exempt in chmod-777 log scan)

## 2026-09-27 — Dinit user services for the desktop session

- 6 new feralos-aports packages: `dbus-user-dinit`, `pipewire-user-dinit`, `pipewire-pulse-user-dinit`, `wireplumber-user-dinit`, `wayland-wait-user-dinit`, `polkit-agent-user-dinit` (Artix `dinit-userservd-services` model: `.user` suffix, sub-vars, smooth-recovery, dbus readiness `pipefd:3` + `dinitctl setenv`) + `dms.user` shipped by `dms-shell` (wayland-gated, compositor-agnostic)
- niri-session wrapper starts the USER dinit instance (`dinit -d /etc/dinit.d/user`) existence-filtered; idempotent across re-logins via `$XDG_RUNTIME_DIR/dinit.ctl`; config.kdl loses all session spawns (supervision + ordering now native)
- niri packages list uses the user-dinit packages instead of raw pipewire/pulse/wireplumber; audio group + `~/.local/share/dinit` log dir retained
- Harness niri+dinit fixture: 61/61 PASS (`dinit_user_services` + `niri_session_wrapper` checks)

## 2026-09-27 — DMS native-apk migration + niri real-hardware fixes

- DMS/quickshell are native musl apks in feralos-aports: `dms-shell` 1.6.2, `quickshell` 0.3.1, `cli11` 2.7.2 — nix/nixGL/execfilter no longer part of the desktop stack
- `niri.go` session wiring per `references/niri-alpine/installer-gaps.md`: POSIX niri-session wrapper (dbus session bus), session spawns (pipewire/pipewire-pulse/wireplumber wait-loop/dms run/polkit-gnome agent), user portals conf (ScreenCast=wlr), `/var/lib/regreet` 700 greeter:greeter, dms fragments via `dms setup binds/layout/colors` + background-effect strip (niri 25.11 lacks ext-background-effect), audio group
- No OpenRC dms service (hung boot) — spawns live in the niri session
- Feralos repo bootstrap unconditional (desktop packages live there too)
- Dinit: audio session services skipped in conversion; `services.go` rc-update gated to OpenRC; features (hardening/podman/nix) rc-update gated
- niri `internal_rules.yaml`: +dms-shell/quickshell/xwayland/accountsservice/power-profiles-daemon/cups-pk-helper/pipewire-pulse/wireplumber/polkit/cupsd services, audio group, `nixPackages` removed
- Harness niri+dinit fixture: 59/59 PASS (dms_installed apk check, vector dinit branch, serial redial, log allowlist)

## [Unreleased]

### Added
- Go installer with btrfsStandardServer layout (2-disk RAID1)
- Go installer with btrfsStandard layout (single disk, swapfile)
- Dynamic rules engine (internal_rules.yaml + logic.go)
- APK hooks for kernel fallback and EFI sync
- Btrfs subvolume creation with mount options
- Fstab generation per layout
- Interactive command execution with retry/ignore/abort
- Config loading from YAML
- Generator (interactive + standalone)
- QEMU-based test harness with runner + target VMs
- Non-interactive mode for automated testing
- LUKS keyfile auto-unlock for headless boot (rd.luks.key)
- Improved logging with module context and timestamps
- Vector logging pipeline (Rust) replacing rsyslog/logrotate
  - Socket on `/run/vector/syslog.sock` (symlinked from `/dev/log`)
  - `vector-setup` service (boot stage) creates dirs, symlink, chowns `/var/log`
  - Config written AFTER package install (apk overwrites with demo_logs)
  - Init-system agnostic check: `apk info | grep -q '^vector$'`
- Per-subvolume btrfs compression via `btrfs property set` (zstd for data, none for NoCoW)
- Swapfile NoCoW + NOCOMPRESS (`chattr +C` + `chattr +m`)
- `@btrbk` and `@var-lib-vector` subvolumes in both disk layouts
- btrbk backup/snapshot tool with `snapshot_dir /btrbk/snapshots`
- earlyoom in standardPackages with service in packageLogic
- uutils subpackages: groups, more, uptime (stat removed — doesn't exist)
- Harness checks: `fstab_vs_mount`, `vector_service`, `vector_logs`, `vector_running`, `vector_diag`, `apk_packages` (dumps to run dir), `sudo_installed/wheel/works`
- `tests/debug_run/` — boot existing run without reinstalling (auto-detects latest run)
- EFI keys generated AFTER package install (matches reference flow)
- `WriteSecurebootConf` skips when `/etc/uefi-keys/db.key` missing
- APK v3 dummy packages via `apk mkpkg` (replaces binary manipulation)
- `providePackages` in internal_rules.yaml for empty dependency-satisfying dummies
- `util-linux-misc` dummy to resolve `uutils-coreutils-more` conflict with `incus-feature-vm`
- `less_dummy` harness check (verifies symlink chain to bat)
- `log_errors` harness check — non-regression guard scanning target-serial.log + installer.log for error/failed markers, fails the run on any match (whitelist for known innocuous lines)
- `services_status` harness check — verifies configured OpenRC services are actually STARTED via `rc-status --all` (not just `[ ok ]` at boot); excludes QEMU-only services (smartd, fsck, udev-settle); parses supervise-daemon format `[ started 00:00:36 (0) ]`
- `lsattr_nocow` harness check — verifies C flag on all 7 NoCoW subvolumes, no `m` flag, compressed subvolumes stay CoW
- **niri desktop environment** — greetd + regreet + niri + wayvnc + DMS:
  - `nixPackages` field in internal_rules (same behavior as apk — data selects install, auto-enables Nix)
  - `chrootPhase/desktops/niri.go`: greetd config, greeter.sh, regreet.toml, niri configs (greeter + user), wayvnc config, xdg-su wrapper, OpenRC dms service
  - DMS (DankMaterialShell) + quickshell from stable nixpkgs channel (nixos-26.05)
  - Harness checks: niri_config_valid, greetd_config, niri_greeter, wayvnc_installed, dms_installed
- **btrfsRaid0 layout** — dual-disk pure RAID0 (data+metadata), EFI on both disks, 2 LUKS swap partitions (partition 2, swapSize/2 each) gated by `enableSwap`, root on partition 3 (remainder — no size formulas)
- **`enableSwap` + `swapSize`** — generic swap toggle replacing `enableSwapfile`: btrfsStandard keeps swapfile, btrfsRaid0 uses LUKS swap partitions, btrfsStandardServer unchanged (always LUKS swap). `swapSize` default `auto` = 2× RAM (via `utils/swap.ResolveSwapSizeMiB`), or explicit `8G`/`4096M`
- **Harness run phases** — `[harness] phase N/9: …` (1 boot runner → 2 run installer → 3 reboot runner → 4 boot target → 5 run sbctl → 6 run tpm → 7 reboot target → 8 boot target verification → 9 verify target) printed to stdout + `runs/<ts>/harness.log`; all harness progress messages mirrored to stdout; MCP mode diverts progress to stderr (JSON-RPC stream stays clean)
- **`dinit-shutdown` core package + shutdown commands** — installer dinit core now includes `dinit-shutdown` (`dinit-poweroff/reboot/halt` + the `/usr/sbin/dinit-shutdown --system -p` finalizer the daemon execs to complete any shutdown); conventional commands wired Artix/Chimera-style: `/usr/bin/{poweroff,halt,shutdown}` symlinks → `/usr/sbin/dinit-*` + `/usr/bin/{reboot,soft-reboot}` exec-wrappers (invocation-name detection requires the `dinit-` prefix); new harness check `dinit_shutdown_commands`

### Fixed
- nix_packages_missing harness check: `grep -oE | sort -u | wc -l` unique-name count (nix profile list prints 3 lines/package, old wc -l overcounted); post-processing strips `\r` + SSH warning prefix, parses last line, FAIL messages embed raw output
- Nix client commands (`su - user` registry add / profile add / smoke tests) now prefix `NIX_SSL_CERT_FILE=/etc/ssl/certs/ca-certificates.crt` — daemon ssl-cert-file alone didn't cover user-side nix client TLS
- Pointer copy bug in configurator.go, hardening.go, zswap.go (u := conf.User → u := &conf.User)
- zswap cmdline flow: now appends to KernelCmdline slice, not overwrite file
- dmcrypt skips for btrfsStandard (uses swapfile, no swap partition)
- Harness hardcoded feature toggles: added EnableSecureboot/EnableHardening/EnableZswap/EnableNix to RunConfig
- efibootmgr guarded with EFI availability check (no error on BIOS runner)
- Harness disk layout changed to btrfsStandardServer (matches RAID1 verification)
- Unreachable code in harness service.go (shift key sleep)
- booster per-device keyfile args: rd.luks.key=/keyfile:luks-<uuid> for multi-device LUKS
- Vector VRL E103: `parse_syslog(.message) ?? {}` fallback
- Vector crash: socket moved from `/dev/log` to `/run/vector/syslog.sock` (permissions)
- Vector config overwritten by apk — now written after package install
- Secure Boot hook failure: EFI keys now generated after package install
- btrbk `snapshot_dir` path (was `/mnt/btrbk/snapshots`)
- "FeralOS Server" → "FeralOS" in user-facing strings
- Swap shutdown: localmount unmounted /swap while swapfile active ("in use but fuser finds nothing") — swap moved to boot runlevel + `sed 's/before localmount/need localmount/'` in SetupServices (`before` guarantees start order only, not stop order)
- Swapfile: `btrfs property set compression none` set the `m` attribute conflicting with `chattr +C` (NixOS/nixpkgs #384047) — compression property removed from ALL NoCoW subvolumes, `chattr +C` on dir before mount + `btrfs filesystem mkswapfile`
- `swap_status` harness check: busybox swapon has no `--show` flag — now uses `swapon -s || cat /proc/swaps`
- gofmt in src (config.go comment alignment, packages.go EOF newline)
- Harness lint cleanup: errcheck, nilerr, dead code removal, goconst, shadow, gosec, gocritic, hugeParam pointer refactors (commit `6b42ff3`)
- niri greetd crash: missing `wayland-libs-server` (niri dlopens libwayland-server.so.0 at runtime → NoWaylandLib panic) + greeter not in `greetd` group (couldn't traverse /var/lib/greetd 750)
- nix build in chroot: sandbox pivot_root EINVAL — `--option sandbox false` chroot-only (trusted-users = root @wheel), TMPDIR on @nix-var-nix-build subvolume (/tmp is noexec+0755)
- nix stable channel: default flake registry maps nixpkgs→unstable — `nix registry add nixpkgs github:NixOS/nixpkgs/nixos-26.05`
- sudoers: Alpine default `/etc/sudoers` has `@includedir` commented out → `/etc/sudoers.d/wheel` ineffective (wheel sudo denied) — enabled with sed + new `sudoers_includedir` harness check
- nix.conf: removed `allow-unfree`/`allow-insecure` (unknown settings in Alpine's nix build — warning noise; NIXPKGS_ALLOW_* env vars carry it permanently)
- btrfsStandardServer LUKS swap never activated: dmcrypt keyfile+conf.d written in hostPhase BEFORE the btrfs root was mounted at /mnt → files lost in host /mnt. Fixed: keyfile written directly into chroot after subvol mounts (step 8a) + fstab swap `LABEL=swap-*` → `/dev/mapper/swap-*` (busybox `swapon -a` can't resolve LABEL in fstab) + `swap_status` check now truthful
- btrfsRaid0 swap never activated (latent bug): swap service was never added to the runlevel — now conditional on `enableSwap`
- **dinit target shutdown (harness)**: busybox `poweroff` is a no-op under dinit (sends SIGUSR2, ignored; `dinitctl shutdown` only HALTs) → `powerOffTarget()` is init-aware (`dinit-poweroff` when `enableDinit`, `poweroff` otherwise) and bounded by `targetShutdownTimeout` (5m — fails the run instead of hanging; no force quit). Applied at boot1→boot2 + final shutdown; `debug_run` same
- **harness dinit checks were unfalsifiable** (false green): `&& echo ok || echo fail` always exits 0 and `Verify` marks PASS on exit code 0 → 8 of 9 `dinit_*` checks could never fail, with 3 broken commands: busybox `ps` has no `-p` (→ read `/proc/1/comm`), bare `dinitctl status` requires `<service-name>` (→ `dinitctl list`), `dinit_feralos_repo` grepped `/etc/apk/repositories` (keyring installs `/etc/apk/repositories.d/feralos.list`). All dinit checks now `COND && echo '<ok>' || { echo '<fail>'; exit 1; }`; re-verified `btrfs_standard_dinit` **48/48 PASS** with truthful details (run 2026-09-24T23-42-02Z)

### Changed
- Migrated from monolithic shell script to modular Go architecture
- Separated host phase and chroot phase into distinct packages
- Console output via console=ttyS0,115200 in kernel cmdline
- Keyfile generation added as Step 6 in main.go (between bootstrap and chroot prep)
- All packages installed in single `apk add` (base + TPM + microcode + Nix + hardening)
- `internal_rules.yaml` cleaned: only base OS in defaultRunlevels, services in packageLogic
- udev services moved to packageLogic under eudev (Alpine uses mdev by default)
- `modules` service removed (booster loads `/etc/modules` automatically)
- earlyoom moved to standardPackages
- e2fsprogs removed from standardPackages
- Package masking removed (apk `!pkg` conflicts with explicit dependencies)
- Swap service in boot runlevel (moved from default) + `need localmount` dependency
- apk `.apk-new` behavior verified: modified /etc files (swap init.d sed) survive openrc upgrades
- log_errors whitelist: `start-stop-daemon: no matching processes found` (smartd, no SMART hardware in QEMU) + `fsck.vfat: No such file or directory` (fsck, QEMU-only)
- `enableSwapfile` → `enableSwap` (generic swap toggle) + new `swapSize` variable (default `auto` = 2× RAM)

### Open Issues
- Secure Boot fix (efi-mkkeys after install) — pending harness revalidation

### Planned (Phase 2)
- Dinit init system (replace OpenRC)
- Custom FeralOS repository
- Nix package manager integration
- Full distribution release

## Milestone Log

> Detailed implementation history, moved here from `AGENTS.md` (2026-09-25); newest first.

### 2026-10-10 12:15 — DMS wifi/BT harness checks: 2 real bugs found+fixed, 94/94
**Agent:** codebase
- **NEW `bluetooth_service`** (init-conditional): started marker + `pgrep bluetoothd` + org.bluez registered on the system bus (dbus-send ListNames) — what the DMS BT widget talks to. **NEW `nm_wifi_support`** (init-agnostic): wifi plugin subpackage + regdb + wpa_supplicant + regulatory.db + `nmcli radio wifi` live answer.
- **BUG 1 — aports `bluez-dinit`**: service `command` used the Debian path `/usr/libexec/bluetooth/bluetoothd`; Alpine installs `/usr/lib/bluetooth/bluetoothd` → bluetoothd never started, service STOPPED-failed, `log_errors` cascaded. Fixed `fb64d77` r3 (CI green, Pages live).
- **BUG 2 — harness**: check tested `/usr/sbin/wpa_supplicant`; Alpine installs `/sbin/wpa_supplicant`. Check fixed; the wifi stack itself was healthy.
- **Validated**: run `2026-10-10T12-12-01Z` **94/94 PASS, zero failures** — `bluetooth (pid: 1562)` + `org.bluez on bus`, `radio: enabled`, all regression sentinels green. The checks caught two real bugs on their first run — falsifiable verification works.

### 2026-10-10 01:40 — bluetooth service unified + niri-only enablement
**Agent:** codebase
- **User review corrections** on the previous round: `defaultRunlevelsDinit.boot += bluez` was BASE-list scope (bluetooth would be enabled on every dinit install — wrong, niri-only wanted); and since feralos owns `bluez-dinit`, the name split was pointless.
- **aports `e9ac00c`** (CI green, Pages r2): bluez-dinit service renamed `/lib/dinit.d/bluez` → `/lib/dinit.d/bluetooth` — same name as the OpenRC initd. Published apk tar listing verified (only `lib/dinit.d/bluetooth`).
- **internal_rules back to clean**: no bluez in `defaultRunlevelsDinit.boot`, no bluetooth in `dinitSkipServices` (the service file now EXISTS under dinit — skipping would break enablement). Single `"bluetooth": "default"` in the NIRI desktop services map enables both inits — desktop services merge into per-desktop Runlevels, never into the base lists. Server/minimal/kde/cosmic installs stay bluetooth-free.
- **wireless-regdb kept** (user asked what it is): kernel regulatory database for cfg80211 — legal channels/frequency/power per country; required for real wifi hardware.

### 2026-10-10 01:10 — DMS bluetooth + NM wifi completeness (data-only round)
**Agent:** codebase
- **Bluetooth absent entirely**: DMS BT widget speaks org.bluez dbus — bluez was in NO package list. niri packages += `bluez`, `bluez-openrc`, `bluez-dinit` (feralos bluez-dinit ships `/lib/dinit.d/bluez` with dbus-wait-for readiness — existed in repo+Pages since ADR-009 but was never wired into the installer). Verified live on Alpine 3.24: bluez 5.86-r2.
- **Service-name split gotcha**: OpenRC initd = `bluetooth`, bluez-dinit service = `bluez`. Wiring: niri services `{"bluetooth": "default"}` + `defaultRunlevelsDinit.boot += "bluez"` + `dinitSkipServices += bluetooth` (dinit loop must not try the nonexistent `/lib/dinit.d/bluetooth`).
- **WiFi incomplete**: Alpine splits the NM wifi plugin — `networkmanager-wifi` was never installed, NM had NO wifi support, DMS network widget showed no wifi section. niri packages += `networkmanager-wifi` (1.52.2-r0) + `wireless-regdb` (2025.10.07-r0). wpa_supplicant already present (NM spawns it); NM config untouched; real-hw wifi firmware = installer hardware detection.
- **DMS doctor inventory** (definitive, live): hard warnings matugen + cava (user handled separately); NOT gaps: dgop (embedded in the dms Go binary — Capabilities list proves it), mission-center, rclone-ui; soft: khal, danksearch/dankcalendar (GitHub-only), fprintd, adw-gtk3. Data-only round: zero Go changes, YAML parse + installer build/vet green.

### 2026-10-09 23:50 — niri/DMS dinit runtime depth: 5 checks added/hardened (91/91)
**Agent:** codebase
- **NEW `niri_layers`** (dinit): `niri msg layers` must list ≥1 `^Output` — layer-shell surfaces mapped = the DMS bar RENDERS (deeper than dms_runtime process + dms_ipc IPC). Live shape: `Output "Virtual-1": Background layer: Namespace: "quickshell"`.
- **NEW `greetd_socket`** (both inits): greetd IPC socket — GOTCHA verified live: socket is PID-suffixed `/run/greetd-<pid>.sock` (clients learn the path via GREETD_SOCKET env), NOT `/run/greetd.sock` → glob `/run/greetd-*.sock`. First run 91/92 (literal path), fixed, standalone re-verify 91/91.
- **NEW `session_dbus_socket` + `session_pipewire_socket`** (dinit): `/run/user/$uid/bus` + `/run/user/$uid/pipewire-0` — started services ≠ consumer endpoints exist.
- **`niri_screenshot` hardened**: PNG magic bytes asserted — fresh file must be a REAL PNG (validated 220926-byte capture).
- **regreet runtime untestable** on verify boots: test-only autologin starts the user session directly, greeter UI never runs — greetd daemon + static config stay the greeter coverage.
- **Validated**: run `2026-10-09T22-30-58Z` (dinit+edge+niri) 91/92 → 91/91; existing dinit checks all green (`niri_user_dinit_runtime`, `dms_runtime`, `dms_ipc`, wayvnc :5900; windows shows DMS window `com.danklinux.dms` "Benvenuto"). OpenRC niri fixture: 2 known dinit-only FAILs remain by user decision (`dms_ipc`, `niri_wayvnc_port`).

### 2026-10-09 21:45 — less shim: aports less-dummy package (fish history clap bug)
**Agent:** codebase
- **Bug confirmed on live target** (debug_run of run `2026-10-09T18-21-04Z`): fish `history` on a TTY pipes to `$PAGER`=bat → bat execs its hardcoded pager `less -R -F -K --no-init` (ignores `$PAGER`; only `cat` short-circuits) → the runtime mkpkg symlink `/usr/bin/less → /usr/bin/bat` made bat re-exec itself → clap `unexpected argument '-R'`. All TTY pager consumers broke (git log, man); ash unaffected only because busybox ash has no pager-piping history builtin.
- **Fix (feralos-aports `f8dcee5`)**: new package `less-dummy` 999.0.0-r2 — `/usr/bin/less` = POSIX-sh wrapper: drops all less flags (keeps file operands, post-`--` args kept via nodash loop), execs `bat --color=always --paging=never` (child never re-pages → chain terminates). `provides="less=999.0.0-r0"` + `replaces="less"` (file conflict blocks real C less). CI infra: workflow `container:` moved to `public.ecr.aws/docker/library/alpine:latest` (Docker Hub anonymous pull rate limit had blocked 3 runs).
- **Installer**: `internal_rules.yaml` — `less-dummy` in standardPackages; `dummyPackages: {}` (bat→less symlink rule removed, comments explain why). No Go changes.
- **Harness**: `less_dummy` check now falsifiable (package installed + not a symlink + `printf 'shim-test\n' | less -R` exits 0); symlink-enforcing Go post-processing block deleted. src + harness gofmt/vet/golangci-lint/build green.
- **Validated live** (run `2026-10-09T22-03-03Z`, fixture `btrfs_standard_niri`, 68/71): `less_dummy` deepened to 4 assertions (pkg + regular file + bat's EXACT flag set `-R -F -K --no-init` + stdin/file/`--` content passthrough via markers) and NEW `less_fish_history` (no clap `unexpected argument` in fish history output) — both GREEN on the installed target (clap parses args before any TTY check, so the flag-set test discriminates even on non-TTY verify SSH). 3 unrelated FAILs = pre-existing gating gap (fixture `enableDinit: false` but `console_keymap`/`dms_ipc`/`niri_wayvnc_port` assume dinit; keymap actually loaded: `loadkmap [ started ]`) — `console_keymap` NOW FIXED init-conditional (dinit: dinitctl `[[+]]`/`[{+}]` marker; OpenRC: `rc-status --all` `loadkmap [  started  ]` marker, regex validated against the real run output); `dms_ipc`/`niri_wayvnc_port` still dinit-only (user-dinit services don't exist under OpenRC).
- **Tradeoff**: "less" no longer interactive (no scroll/search — dumps). Upgrade path: package moar/ov in aports, re-point wrapper — zero installer change. NEXT: optional `BAT_PAGER=cat` env belt+braces.

### 2026-09-26 — hardware-readiness pass: setup.sh, wizard rewrite, enableDebug serial gate (39/39)
**Agent:** codebase
- **setup.sh**: apt install (incl. `git`) moved before repository clone — fresh cloud-init Debian live ships no git; `cd` into cloned repo; `-config` resolved to absolute path at parse time (survives later `cd`).
- **Wizard rewrite** (`createConfig`): every user option asked (unattended, layout, disks, all toggles, swapSize, extraPackages), per-field validation with immediate re-ask (exported validators in `utils/config/validators.go` — single source shared with `validateConfig`; `LoadInternalRules` feeds enum options from `internal_rules.yaml`; `swap.ValidSwapSize` without /proc/meminfo), disks cross-checked vs layout count, min-6 masked passwords (LUKS empty = reuse user password), summary + yes/no confirm with previous answers as defaults.
- **Shell retry bug**: manual retry (`"r"`) set `attempt=0` but never left the prompt loop → command never re-executed; fixed with labelled `askLoop` + `break` in `ExecuteCmdWithAllowFail` and `ExecuteCmdArgsWithAllowFail`.
- **Serial console gated**: `console=ttyS0,115200` was hardcoded in all 4 disk layouts; now appended in `SetupBoot` only when `enableDebug=true` (test-only) — harness keeps boot-watch + LUKS serial injection, real hardware gets a clean hardened cmdline. All fixtures + runner template set `enableDebug: true`.
- **Rules + harness**: `btrfsUnencrypted` layout entry added (btrbk, no dm_mod/dm_crypt); intel drivers modernized (mesa-vulkan-intel, intel-media-driver, linux-firmware-i915, no xf86-video-intel); `log_errors` allowlist for `liberror-perl` (Debian git dep printed by apt). Verified: static green (build/vet/golangci-lint), `btrfs_standard_minimal` harness 38/39 → equivalent 39/39 (only false positive fixed). Next: real hardware NVMe test.

### 2026-09-25 — falsifiable dinit verify checks (48/48)
**Agent:** codebase
- **False green root cause**: `Verify` marks a check PASS when the SSH command exits 0 (`Pass: err == nil`) — but 8 of 9 `dinit_*` checks ended in `&& echo ok || echo fail`, and the fallback `echo` always exits 0 → unfalsifiable checks reported as green. Symptoms in the check files: `dinit_pid1` "NOT dinit" (busybox `ps` has no `-p`), `dinitctl_status` "dinitctl failed" / `dinit_services` "no dinit services" (bare `dinitctl status` needs `<service-name>` — source-verified `dinitctl.cc:449/513`), `dinit_feralos_repo` "repo MISSING" (keyring APKBUILD installs `/etc/apk/repositories.d/feralos.list`, not `/etc/apk/repositories`).
- **Fix**: all dinit checks now `COND && echo '<ok>' || { echo '<fail>'; exit 1; }`; `dinit_pid1` reads `/proc/1/comm`, `dinitctl_status` uses `dinitctl list`, `dinit_feralos_repo` greps the keyring list, `dinit_shutdown_commands` gets a failure message. The system was correct all along — only the checks lied.
- **Verified**: `btrfs_standard_dinit` **48/48 PASS** (run 2026-09-24T23-42-02Z) with real details (`dinit PID1`, full `dinitctl list`, service lines, `repo OK`); `gofmt`/`go vet`/`golangci-lint` clean. Docs: `docs/harness/development.md` (falsifiable-check rule).

### 2026-09-25 — harness phases + init-aware dinit shutdown + shutdown commands (48/48 dinit, 39/39 OpenRC)
**Agent:** codebase
- **Run-hang root cause**: target shutdown used BusyBox `poweroff` → sends SIGUSR2, which dinit ignores (handles SIGINT=reboot / SIGTERM=halt / SIGQUIT=now only) → exit 0 with no effect → `targetInstance.Wait()` (no timeout) hung at boot1→boot2 (runs died externally). `dinitctl shutdown` sends HALT (QEMU stays) — source-verified dinitctl.cc v0.21.0; the POWEROFF request is `dinit-poweroff` (`dinit-shutdown` package).
- **Installer**: core dinit packages += `dinit-shutdown` (`chrootPhase/internal/packages.go`) — ships `dinit-poweroff/reboot/halt` + the `/usr/sbin/dinit-shutdown --system -p` finalizer the daemon execs to complete ANY shutdown. `services.go` `setupDinitShutdownCommands` wires conventional commands (Artix/Chimera style): symlinks `/usr/bin/{poweroff,halt,shutdown}` → `/usr/sbin/dinit-*` (default = power down) and exec-wrappers `/usr/bin/{reboot,soft-reboot}` (invocation-name detection requires the `dinit-` prefix — a "reboot"-named link would power off instead); `/usr/bin` shadows the busybox applets.
- **Harness**: `powerOffTarget()` init-aware (`enableDinit` → `/usr/sbin/dinit-poweroff`, else `poweroff`), bounded by `targetShutdownTimeout` (5m — fail, not hang; no force quit); new verify check `dinit_shutdown_commands`; `debug_run` shutdown same (loads `enableDinit`).
- **Phases**: `phase(n, name)` prints `[harness] phase N/9: …` (1 boot runner → 2 run installer → 3 reboot runner → 4 boot target → 5 run sbctl → 6 run tpm → 7 reboot target → 8 boot target verification → 9 verify target) to stdout **and** `runs/<ts>/harness.log`; all `logHarness` messages mirrored to stdout; MCP server diverts progress to stderr (`SetProgressWriter`) to keep the JSON-RPC stream clean.
- **Verified**: `btrfs_standard_dinit` **48/48 PASS (2m41s)**, `btrfs_standard_minimal` OpenRC **39/39 PASS (2m32s)**; live `debug_run` test — `reboot` restarts the target (QEMU stays, fresh uptime), `poweroff` powers off (`Issuing shutdown via kernel... → reboot: Power down`). Docs: harness development/workflow/artifacts + `docs/dinit/TROUBLESHOOTING.md`.

### 2026-09-20 — dinit COMPLETE: Alpine/Artix model + logs + harness hard limits (47/47)
**Agent:** codebase
- **Enablement model (Artix-verified)**: `-dinit` packages ship the service file ONLY (boot.d links removed from all, getty included; install ≠ enabled). Installer: boot.d symlinks driven by rules from `defaultRunlevelsDinit` + `packageLogic.services` — service names = Alpine initd names (chronyd, sshd, incusd, smartd, nix-daemon, mdev, incus-agent) → identical rules for OpenRC and dinit, zero aliases. `dinitctl --offline` crashes on Alpine 0.21 (cpbuffer assert) → identical direct symlinks. `system` service is critical: `boot` depends on `system`, and `system` processes boot.d — without it no boot.d services start.
- **New aports packages**: networking-dinit (ifup -a — nothing brought eth0 up under dinit), incus-feature-dinit (incusd host daemon), dinit-boot-log (mirrors the PID1 trail /run/dinit-boot.log → /var/log/dinit/boot.log), sshd-dinit DELETED (real sshd in openssh-server-common-dinit); getty = single template body `agetty -L $1 115200` (port before baud).
- **Logs**: every `-dinit` type=process → `log-type=file` /var/log/dinit/<name>.log; vector `dinit_services` source + messages sink; **logrotate RE-ADDED** (vector does NOT rotate — it only tails rotated files; copytruncate+delaycompress; supersedes the old "vector replaces it" decision) — installer config only, Alpine package, execution via cronie/periodic. `dinit_log_file=/run/dinit-boot.log` on the cmdline; dinit-chimera r5 wrapper patch: reads it from /proc/cmdline (booster does not forward unknown params to init's env).
- **Blocking fixes found by tests**: vlan↔ifupdown-ng conflict blocked the whole core apk add (hard-fail core verification now); agetty port-before-baud (3 prompts = respawn); dbus/vector /run dirs tmpfiles; our sd-tools r993 (GCC use-after-scope, upstream #5); `services_status` + standalone verify dinit-aware.
- **Harness**: **hard 7-minute limit for the whole run** (`runHardTimeout` constant, watchdog kills VMs + exit 124 — non-configurable by design), boot 300s, instant runner kill + guest sync + swtpm race fix, `--keep-target-running` respected by the watchdog (target spared for the separate verify). **Final result: 47/47 PASS** (run 2026-09-20T02-31-53Z).

### 2026-09-19 — dinit Alpine/Artix model: rules-driven enablement + initd-aligned names
**Agent:** codebase
- **Root cause** (boot OK but wrong service set): our `-dinit` packages auto-enabled via `boot.d` links in `package()` (Chimera model install=enable) → every installed dep got a running service (dnsmasq, lxc, iptables, rasdaemon...) while real ones were missing (networking: nothing ran `ifup -a`; sshd: placeholder service; incusd: wrong service enabled).
- **Model adopted (Artix-verified: alpm-hook prints `dinitctl enable`, cronie-dinit ships file only; volume techs = optional pkgs)**: packages ship service files ONLY (no boot.d links), enablement data-driven: `dinitctl --offline -d /lib/dinit.d enable <svc>` (upstream offline command; links land in boot's waits-for.d `/etc/dinit.d/boot.d` = admin dir). Services renamed to Alpine initd names (chronyd, smartd, sshd, incusd, nix-daemon, mdev, incus-agent) → `packageLogic.services` works identically for both inits, zero aliases.
- **aports**: all 41 `-dinit` ship-only (pkgrel r2, agent r3); getty-dinit = single `getty` template body (depends agetty; dropped systemd-style `getty@.service` — dinit carries the arg in the instance NAME); NEW networking-dinit (`ifup -a` — critical gap) + incus-feature-dinit (`incusd`); DELETED sshd-dinit (sshd now real in openssh-server-common-dinit, mirroring Alpine's openssh-server-common-openrc→sshd.initd); vector-dinit depends-on vector-setup + config path .toml→.yaml; build.yml getty/chronyd/networking key-list updated.
- **Installer**: `services.go` dinit branch = `dinitctl --offline enable` from `defaultRunlevelsDinit` (base: system, getty@tty1/getty@ttyS0) + `Runlevels` (default+boot levels; sysinit/shutdown skipped) with `dinitChimeraServices` skip-list (17 OpenRC-only names chimera covers — hwclock/udev/cgroups/btrfs-scan/dmcrypt...), verify via `boot.d` link existence (SKEA_NONINTERACTIVE-proof), fallback symlink for template instances; `config.go` `DinitChimeraServices`; `packages.go` core += networking-dinit; `logging.go` writes `/lib/dinit.d/vector-setup` oneshot when EnableDinit.
- **Chimera volume services (zfs/lvm/mdadm/dmraid) left untouched**: upstream design = no-op when tool missing (we never install those tools); `dinit_skip_volumes` exists (scripts read /proc/cmdline) but NOT used (would also skip btrfs scan; booster already covers).
- Expected enabled set (btrfsStandard): getty@tty1/ttyS0, networking, incusd, vector, vector-setup, cronie, chronyd, dbus, sshd, smartd, acpid, irqbalance, fwupd, earlyoom = same as OpenRC.

### 2026-09-18 — dinit boot fix: services → /lib/dinit.d (Option B, Alpine standard)
**Agent:** codebase
- **Root cause** (run 2026-09-17T22-53-05Z, `dinit: boot: could not find service description.`): stock Alpine dinit 0.21 scans `/etc/dinit.d`, `/run/dinit.d`, `/usr/local/lib/dinit.d`, `/lib/dinit.d` — **NOT** `/usr/lib/dinit.d` (source-verified `options-processing.cc build_paths`; Chimera patches their dinit, Alpine doesn't). dinit-chimera installed all services to `/usr/lib/dinit.d` → PID 1 found nothing.
- **aports**: dinit-chimera APKBUILD `prepare()` seds meson `srvdir = '/lib/dinit.d'` (meson `/` operator: absolute RHS discards prefix → EARLY_PATH/HELPER_PATH/SCRIPT_PATH coherent; tmpfdir/dlibdir unaffected), pkgrel 1→2; all 40 `*-dinit` APKBUILDs install paths → `/lib/dinit.d`, pkgrel 0→1. aports STATUS.md updated (Step 6b: needs push + CI + harness re-run).
- **Installer**: `services.go` boot.d symlinks → `/lib/dinit.d`; `internal_rules.yaml` `defaultRunlevelsDinit: {}` (getty owned by getty-dinit pkg boot.d links; plain-name link of a dinit template is invalid — `$1` only expands for instances) + new `dinitReplacedByChimera` skip-list (agetty, busybox, cryptsetup, eudev, incus-feature, kbd, makedumpfile, udev-init-scripts, util-linux — covered by chimera suite); `config.go` `InternalRules.DinitReplacedByChimera`; `packages.go installDinitPackages` skips replaced bases (no warning/retry) + detects missing via output parse (`no such package`) since `SKEA_NONINTERACTIVE` swallows the error (old counter reported `30 installed, 0 missing` while 9 were missing, ~66s wasted in retries).
- **Docs**: `docs/dinit/BOOT.md` (scan-list note + path fixes), `docs/dinit/TROUBLESHOOTING.md` (new symptom row), `docs/dinit/PLAN.md` path, aports STATUS.md.

### 2026-09-10 — feralos-aports: full APK pipeline (rule 0) + docs split
**Agent:** codebase
- Repo `references/feralos-aports` (github.com/skea999/feralos-aports, nested clone, gitignored): GitHub Actions CI on `alpine:latest` container → abuild → signed apk index (abuild-sign) → GitHub Pages; 3 jobs (build/deploy/client-test); secrets `ABUILD_PRIVKEY`/`ABUILD_KEYNAME` as **Repository secrets** (NOT environment!)
- **RULE 0**: use Alpine packages whenever they exist → sd-tools DROPPED (Alpine community v3.24+edge ships 0.99.0-r3 with cmd:sd-tmpfiles/sysusers); Steps 1a/1b DONE-SUPERSEDED but remain as PIPELINE PROOF: signature verified client-side (fresh container, only feralos.pub trusted, install without --allow-untrusted, provenance via apk policy)
- Source-verified facts: dinit-chimera init wrapper → meson installs it to **/usr/sbin/init** → package() relocates to /usr/libexec/dinit/init (OpenRC coexistence); Alpine eudev: **udevd=/sbin/udevd, udevadm=/bin/udevadm** (not usr-merged); NO scdoc (pre-rendered .8 manpage); sd-tools v0.99.0 single tag; mkinitfs honors init= (source-verified initramfs-init.in) → initramfs-agnostic packages, mkinitfs NOT tested by us
- Policy: optional PACKAGES all mandatory; optional FEATURES best-effort, droppable only if build fails AND boot doesn't need them → registry `docs/DROPPED-FEATURES.md`; GCC toolchain (cports rejected: Chimera libc++ build root, Alpine-host bootstrap unsupported due to patched musl SONAME) — record in aports `docs/TOOLCHAIN.md`
- Docs split by competence: see feralos-aports repo — system docs (BOOT, TROUBLESHOOTING) + installer-side README/PLAN in `docs/dinit/`
- **Audit + anonymization**: no credentials in either repo (fixtures `Alpine.1` = documented test cred); webmin firstboot hardcoded `/home/user`+`chown skea999:skea999` → `cfg.UserName`; harness `trusted_users_clean` literal → `cfg.InstalledUser`; `~/projects` paths → `~/Progetti`; history scrub tooling added (`scripts/history-scrub.sh` + `scripts/history-replacements.txt` + `docs/guides/audit.md`) — Italian/machine-user scrub across ALL branches via git-filter-repo, run before public pushes

### 2026-09-10 — Dinit planning + docs split (installer vs feralos-aports) + KDE/COSMIC DEs
**Agent:** codebase
- **COSMIC + KDE desktop environments added**: `internal_rules.yaml` `desktopEnvironments.cosmic` (seatd — NOT elogind, verified: Alpine cosmic-greeter OpenRC init requires seatd; polkit not polkit-elogind) + `kde` fixture; `desktops.go` unicode sed in rc.conf for ALL DEs (musl default off) + "not implemented" warning removed (packages/services come from internal_rules); harness fixtures `btrfs_standard_cosmic_full_test.yaml` + `btrfs_standard_kde_full_test.yaml` both **41/41 PASS**; `log_errors` allowlist += `abseil-cpp-strerror` (pkg name contains "error")
- **Dinit planning complete** (branch `developDinit`): dinit from Alpine community 0.21.0 (`--shutdown-prefix=dinit-`); boot = Chimera suite `dinit-chimera` v0.99.24 built from upstream tarball (52 services, 12 compiled C++ helpers, only `kmod` links libkmod); our glue = 2 hook scripts (`dinit-devd` eudev, `dinit-cryptdisks`) + `getty@` (upstream ships none); switch via cmdline `init=` (booster init-agnostic, OpenRC coexists); Phase 4 turnstile (chimera project, ex `dinit-userservd`, Dinit backend is the upstream reference) → `dinit --user`, DMS becomes a user service; Chimera polkit `turnstile.patch` REQUIRED with turnstile (verified: Alpine 127-r2 does NOT have it, versions aligned 127=127-r2)
- **Docs split by competence**: packaging docs (PACKAGES/SERVICES/REPOSITORY/TOOLCHAIN/CONVERSION) → repo `references/feralos-aports` (github.com/skea999/feralos-aports, nested clone, own .git); system docs (BOOT/TROUBLESHOOTING) + rewritten README/PLAN stay in `docs/dinit/`; aports repo hassss `PLAN.md` step-by-step (8 steps, checkboxes) + `STATUS.md` (update-per-step) + `.github/workflows/build.yml` draft + `.gitignore`
- **Toolchain decision CLOSED**: aports + abuild + GCC (no clang/LLVM/libc++); cports rejected (build root = Chimera base libc++, source-bootstrap unsupported on Alpine host due to patched musl SONAME) — record in aports `docs/TOOLCHAIN.md`
- **Note**: `references/*` is gitignored in the outer repo — the nested clone does not touch the installer repo

### 2026-09-09 — Niri rustdesk flatpak + firewall offline-cmd, harness 51/51 PASS

- **Niri:** conditional `spawn-at-startup "flatpak" "run" "com.rustdesk.RustDesk" "--service"` in `~/.config/niri/config.kdl` only when `flatpakPackages` contains `rustdesk` (`hasRustdesk` over `conf.User.FlatpakPackages`); `firewall-offline-cmd --zone=public --add-port=21115-21119/tcp` + `21116/udp` (was `firewall-cmd --permanent` which fails when `firewalld` not running in chroot); `regreet.toml` `reboot = ["reboot"]` / `poweroff = ["poweroff"]` (was `systemctl reboot` — Alpine OpenRC); `xdg-user-dirs` added to `standardPackages` for flatpak portal; `/lib/modules` mkdir moved to `chrootPhase/internal/packages.go:SetupPackages` **before** `apk update`/`apk add` (was in `features/flatpak.go` after install — flatpak checks module path early); `wayvnc` removed (`spawn-at-startup "wayvnc"` + wayvnc config dropped, remote via flatpak RustDesk)
- **Flatpak:** `chrootPhase/features/flatpak.go` — `FlatpakPackages()` returns `["flatpak"]` when `len(FlatpakPackages)>0`, `SetupFlatpak` does `flatpak remote-add --if-not-exists flathub` + per-entry `flatpak install -y --noninteractive <remote> <appID>` (split `flathub:com.rustdesk.RustDesk`); `internal_rules.yaml:desktopEnvironments.niri` now `flatpakPackages: ["flathub:com.rustdesk.RustDesk"]` + 6 nix packages (quickshell, dms-shell, rclone, rclone-ui, mission-center, cups-browsed — `nix-software-center` manual due to crates.io 403 on QEMU IPs); `config.go:FlatpakPackages []string yaml:"flatpakPackages"`, `logic.go:applyRule` propagates `FlatpakPackages`
- **Podman/Webmin:** webmin **podman-based** verified — `podman` APK (`podman-openrc`, `iptables`, `podman-compose`, `podman-docker`, `fuse-overlayfs`) with `btrfs` driver via `sed` on `/etc/containers/storage.conf`, `cgroups`+`podman` services; webmin standalone binary `src/firstBoot/webmin/main.go` verifies `podman --version` only (podman already in chrootPhase), builds `/opt/feralos_webmin` → `podman compose up -d` with `read_only`+`tmpfs`, seed via `webmin-seed`, `webmin-ensure` oneshot; `nix` tmp `775`/`1777` `root:nixbld`, `TMPDIR` scoped to daemon via `/etc/conf.d/nix-daemon`, locale `it→it_IT.UTF-8`
- **Harness:** `firewall_rustdesk` check (`firewall-cmd --permanent --list-ports | grep 21115` || `grep -rq 21115 /etc/firewalld/` — permanent ports, not runtime), `flatpak_installed` (`flatpak list | grep rustdesk`), allowlist widened `commitpartial|bwrap.*pivot_root|To enable access to XDG directories` (flatpak XDG portal noise, was `commitpartial: No such file` exact), `/lib/modules` timing fix verified, `xdg-user-dirs` no longer missing; **51/51 PASS** (was 45/45, now +6: `firewall_rustdesk`, `flatpak_installed`, `niri_*` 4)
- **Docs:** new `docs/guides/niri.md` (greetd/regreet/niri/rustdesk autostart conditional + firewall offline-cmd + flatpak + `xdg-user-dirs`/`/lib/modules`/`regreet` fixes + podman/webmin integration + troubleshooting); updated `AGENTS.md` history + Docs Index + Roadmap (niri completed), `docs/guides/README.md` (niri index), `docs/guides/variables.md` (flatpakPackages + firewall ports), `docs/guides/development.md` (feature map + module map), `docs/harness/artifacts.md` (51/51 table, new checks, allowlist, /lib/modules)

### 2026-09-08 — Nix SSL env + nix_packages_missing parse fix

- **Nix:** user-side `su - user` nix commands (registry add, profile add loop, flake metadata + profile list smoke tests) now prefix `NIX_SSL_CERT_FILE=/etc/ssl/certs/ca-certificates.crt` — daemon-side `ssl-cert-file` in nix.conf didn't cover the nix client TLS
- **Harness:** `nix_packages_missing` check rewritten: `grep -oE "quickshell|dms-shell|rclone-ui|rclone|mission-center|nix-software-center|cups-browsed" | sort -u | wc -l` (unique names — `nix profile list` prints 3 lines/package so old `wc -l` overcounted); post-processing strips `\r` + takes last line before `Atoi` (SSH warning prefixed output broke parse); expected count = `len(cfg.NixPackages)` when set (7 for niri), FAIL messages embed raw output

### 2026-09-06 — Nix perms + Podman btrfs + Webmin divider + Harness checks

- **Nix:** `trusted-users = root <user>` temp inject during chroot install (allows `--option sandbox false`), then cleaned to `root` via `sed` + `rc-service nix-daemon restart`; `/nix/var/nix/build` is `775` + `chown root:nixbld`; `/nix/var/nix/build/tmp` is `1777` + `chown root:nixbld` (world-writable sticky); `TMPDIR` scoped to daemon via `/etc/conf.d/nix-daemon` (was global export leaked into podman → broke `containers-storage`); install loop non-fatal (collect `failedPkgs` warning), dry-run `nix flake update` + `nix profile upgrade`; single-block `trusted-users` install
- **Podman:** `btrfs` driver via `sed -i 's/^#*driver.*/driver = "btrfs"/' /etc/containers/storage.conf` (keep Alpine's full config), `cgroups` service enabled alongside `podman`, `iptables` added, `fuse-overlayfs` kept as fallback
- **Webmin:** standalone binary `src/firstBoot/webmin/main.go`, compose `read_only: true` + `tmpfs: /tmp /run /var/log /var/webmin`, image `demosthenesk/webmin:noble` (pschatzmann dead), seed `/etc/webmin` via `webmin-seed` container before first start, `webmin-ensure` oneshot service, `changepass.pl` + `chpasswd`, quoted heredocs, `---` divider in `docs/guides/webmin.md`
- **Locale:** `it` → `it_IT.UTF-8` (`%s_%s.UTF-8` expansion) — bare `it` crashes `conmon` `setlocale` (musl)
- **Harness:** functional checks `webmin_binary`/`webmin_container`/`webmin_port`/`webmin_http` (8×5s wget retry, no `|| echo` fallback), `trusted_users_clean`, `works_in_real_life` (no non-interactive warning in installer.log), `nix_build_tmp_perms` (stat 777), `nix_packages_missing` (7 expected for niri), `chmod_77_check` now excludes `/nix/var/nix/build/tmp` and checks `chmod 777/1777`; 45/45 PASS with `demosthenesk` image; `runWebminOnTarget` + `runScriptOnTarget` DRY
- **Niri:** `cosmic-files/edit/screenshot/player/store/settings`, `okular`, `eog`, `cups`, `firewalld-gui`, `gnome-firmware`, `thunderbird`, `gnome-disk-utility`, `kcron`, `cosmic-store`, `ksystemlog`, `rclone`/`rclone-ui`, `mission-center`, `nix-software-center`; `cups-browsed` moved to nix (`nixpkgs#cups-browsed`); removed `wl-clipboard`; 7 nix packages for niri
- **Fixtures:** `full_stack.yaml` updated (enablePodman+enableWebmin true), new per-layout `btrfs_*_full_stack.yaml` (raid0, server, unencrypted); `btrfs_standard_niri_full_test.yaml` untracked test fixture
- **Hooks:** `govulncheck` now warning-only (not blocking commit)
- **Docs:** `docs/guides/webmin.md` (divider, seed, tmpfs, demosthenesk, Implementation Notes), `docs/guides/podman.md` (btrfs sed, cgroups, iptables, fuse-overlayfs), `docs/harness/artifacts.md` (new checks table, nix perms, podman/webmin details)

### 2026-09-02 — Webmin + Locale + TMPDIR + Podman fixes

- **Locale:** `it` → `it_IT.UTF-8` (bare `it` crashes conmon setlocale)
- **Nix TMPDIR:** scoped to daemon via `/etc/conf.d/nix-daemon` (was global export, leaked into podman and broke containers-storage)
- **Image:** `demosthenesk/webmin:noble` (pschatzmann dead — webmin 1.820 2016)
- **Seed:** `/etc/webmin` from image before first start (`webmin-seed` container, bind mount shadowed `miniserv.conf`)
- **tmpfs:** `/tmp /run /var/log /var/webmin` with `read_only: true`
- **Heredocs:** quoted heredocs — no escaping (was breaking `webmin-ensure` script)
- **Binary:** exits 1 on compose failure, verifies `podman ps` (harness reports honestly)
- **Podman:** `btrfs` driver via `sed` (was `vfs`), `cgroups` service
- **Harness:** functional webmin checks (`webmin_container`/`port`/`http`), `runWebminOnTarget` on first boot like tpm pattern; 45/45 PASS

### 2026-08-29 — Config markers + verify checks + cleanup enabled
- Installer now appends `# Generated by FeralOS Installer` at END of all config files: /etc/kernel/cmdline, /etc/booster.yaml, /etc/network/interfaces, /etc/fstab (4 layouts)
- Fstab header marker removed (only footer marker for single-point verification)
- Harness verify: 4 new marker checks (cmdline_marker, booster_marker, fstab_marker, network_marker) — grep -c must be > 0
- All 4 layouts PASS 35/35 (btrfsStandard 35/35, btrfsRaid0 35/35, btrfsStandardServer 35/35, btrfsUnencrypted 35/35)
- cleanup.Cleanup(conf) enabled (final unmount works after sleep 20s)
- log_errors: BTRFS Transaction aborted + forced readonly allowed for btrfs layouts
- verify.md: SSH warning stripped, table cleaned
- harnessLog shadow fix, serialLogHasPrompt offset-based
- LUKS single injection per boot
- eth0 restored, virtio modules added to baseModules

### 2026-08-29 — Network fix + verify.md + log_errors + unmount investigation
- network: `defaultNetworkInterface: eth0` restored in fixtures + harness writeHarnessVariables; `virtio_net,virtio_pci,virtio` added to `baseModules` in `internal_rules.yaml`. `btrfsStandard` 33/33 PASS.
- verify.log → verify.md (markdown table, SSH warnings stripped, detail truncated)
- log_errors allowlist: added `errors=remount-ro` (fstab), `BTRFS error.*is missing/failed/open_ctree` (transient booster)
- harnessLog shadow fix (`:=` → `=`), serialLogHasPrompt offset-based
- LUKS injection: single per boot (`<1`), not 4x
- enableEncryption removed (leftover)
- Final cleanup disabled (BTRFS Transaction aborted -5) — sleep 20s before shutdown
- **TODO**: investigate why final `cleanup.Cleanup(conf)` causes `BTRFS Transaction aborted -5, forced readonly` during runner shutdown

### 2026-08-30 — Dedup: dupl linter + harness DRY + firstboot DRY
**Agent:** codebase
- **dupl linter**: added `dupl` to `.golangci.yml` (threshold 100), exclusions for `src/hostPhase/disks/` + `src/utils/firstboot/` (intentional duplication). Pre-commit hook runs golangci-lint which includes dupl.
- **Harness dedup**: `hasHealthyBtrfsRaidLayout(detail, level string)` parametrized (was 2 identical functions for RAID1/RAID0). `formatCheckTable()` extracted (was 2 identical verify table loops). Zero dupl findings in harness.
- **firstboot DRY**: `buildBinary()` extracted to `firstboot.go`, `secureboot.go` + `tpm.go` now thin wrappers (13 lines each). Standalone binaries (`src/firstBoot/`) untouched.
- **swap_test.go fix**: `TestParseSwapSizeMiB` 1T/1P cases now expect error (32 GiB `maxSwapMiB` cap). 26/26 pass.
- **Docs**: `review.md` updated (§3.5 dedup entry, Appendix dedup row), `AGENTS.md` lint section updated with dupl info.

### 2026-08-26 — Entry-point delegation + isolation doc + harness prepare-runner fix
**Agent:** codebase
- **Architecture fix**: `main.go` no longer calls cleanup or disk layouts directly. It collects/generates config then delegates to `hostPhase.Run()` + `chrootPhase.Run()`. `hostPhase.Run()` owns: cleanup (`internal/cleanup.go`, step 0) → disk layout switch (step 1) → mount → bootstrap → chroot prep → install-config → firstBoot compile. `cleanup.go` stays in `hostPhase/internal` (called by `host_phase.go` via `internal.` — `main` cannot import internal).
- **Docs**: added "Entry Point & Phase Isolation" to `AGENTS.md`; rewrote `docs/INSTALLER.md` Installation Flow (Phase 1 now shows `hostPhase.Run()` delegation + cleanup step 0). Created `docs/architecture/` (6 files), `docs/adr/` (6 ADRs), `docs/guides/` (getting_started real hardware, first_boot, variables, development), `docs/harness/` (6 files), `docs/intent/`+`ideas/` from `CONTEXT.md`/`ROADMAP.md`. Added `AGENTS.md` Docs Index with pointers (double docs, no strip).
- **Harness `prepare-runner` fix** (`tests/harness/internal/service/service.go`): overlay inherited 327MB base size → cloud-init "No space". Added `RunnerImageSize` (default 8G) + `qemu.ResizeDisk` after `CreateOverlay`, and `growpart`+`resizefs` to cloud-init user-data. Runner packages reduced to `tar,curl,parted` (installer installs its own deps via apt). Verified: cloud-init finishes clean.
- **Logging Rule**: added `AGENTS.md` Logging Rule (always `shell.ExecuteCmd`, never `os.*`), fixed 8 `hostPhase` files (`bind_mounts.go`, `disks/*/fstab.go` + `partitioning.go` with `fdisk` heredoc, `features/install_config.go` heredoc) + audited `chrootPhase` (already clean). `docs/architecture/logging.md` copy.
- **PENDING**: test `run` fails at `go build` inside runner with `go.sum` hash mismatch for `yaml.v3`. Harness builds installer via `setup.sh` (copies local repo, NOT git clone — confirmed in `tests/harness/README.md`). Fix must be in harness build env (GOPROXY/GOFLAGS/module cache), NOT `go mod vendor` (user rejected vendor). Need documented harness build mechanism — not yet located.

> Older history → `CHANGELOG.md` + `git log` + `.agent-memory/` (session-memory).

## [0.1.0] — Shell Script

### Added
- Working shell script installer (internal/start.sh)
- Dual-disk RAID1 partitioning
- Btrfs subvolume layout (20+ subvolumes)
- Alpine bootstrap via apk-tools-static
- Basic fstab and GRUB configuration

---

*Format based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/)*
