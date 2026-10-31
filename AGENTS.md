# Project Context

**Stack**: Go 1.22 (installer) + Shell (legacy reference) + YAML config
**Project**: FeralOS — Alpine-based distribution, memory safety + security, Nix for glibc, Dinit selectable (OpenRC fallback), custom repo
**Status**: Phase 1 refining · Phase 2 shipping (niriWayle flagship desktop, Dinit selectable)

> **Primary context for AI agents.** This file keeps full project info. Detailed copies live in `docs/` (double documentation). Read this file first, then follow `→` pointers for deep dives.

## Recent Rounds (latest first)

### 2026-10-31 - raffi 0.26.0 + ursine 0.24.0 bumps; niriWayle window-list module + Mod+W switcher
**Agent:** codebase
**Summary:** two aports version bumps + the niriWayle window switcher wiring (user request).
- **aports `c50ccd0` (raffi 0.26.0):** pkgver bump only — verified same build pattern (cargo `--locked`/`--frozen`, default features = wayland + config-validation unchanged, Cargo.lock/examples/LICENSE in tarball); `-i` (initial query) confirmed in 0.26.0 `Args` (gumdrop `#[options(short = "i")]`).
- **aports `fcfa046` (ursine 0.24.0):** adds `src/ursinectl` — root CLI, in-process ursine-core host (chroot-capable), hard root gate, `--json`/`--yes`; shipped to `/usr/bin/ursinectl` beside ursined+ursine; same workspace deps (no new system makedepends). Packaging assets (dinit unit/wrapper/conf + OpenRC initd) verified IDENTICAL upstream vs vendored. **check() scoped to the 0.22.1 member set** (`-p ursine-api -p ursine-core -p ursined -p ursine`): the new ursinectl e2e/inprocess tests are calibrated for upstream's ROOT CI container ("runs as root … the same condition the CLI enforces") and fail under abuild's non-root sandbox; pure-logic unit tests stay enforced. Feature resolution safe: `pam` lives in ursine-core, forwarded by ursined.
- **niriWayle window list:** wayleConf gains `[[modules.custom]] id = "windows"` (mode=watch, restart-policy=on-exit) — loops on `niri msg event-stream`, refreshes a `niri msg --json windows` jq summary (app-id list · count · focused alt); `left-click = "raffi -i 'win '"`; inserted `custom-windows` into bar right (after notifications, before clock). Mod+W bind added to `niriWayleBinds` — **multi-arg spawn form** `spawn "raffi" "-i" "win ";` (niri does NOT go through a shell; the shell-style single string would exec a literal filename). GOTCHA: backticks inside the TOML comment terminated the Go raw string (illegal rune literal) — comments in the embedded TOML must stay backtick-free.
- **harness:** `wayle_session_actions` += new right-layout list (with `custom-windows`) + `id = "windows"` + `left-click = "raffi -i 'win '"` greps; NEW `ursine_cli` (every desktop: ursine pkg + ursined/ursine/ursinectl executable + `ursinectl --version` answers — version/help work non-root, the root gate only guards Run).
- **Validated:** src + harness gofmt/vet/golangci-lint/build/test green; generated wayleConf TOML parse-verified (tomllib: custom ids launcher→windows, custom-windows positioned notifications<custom-windows<clock); aports CI run `41210924593`.

### 2026-10-31 - dashboard dropdown "doesn't open" investigated: NOT a config bug — panel proven opening live; explicit left-click pinned + harness assert
**Agent:** codebase
**Summary:** user report — the Wayle dashboard dropdown allegedly not opening on click. Source-level + LIVE investigation disproved the config theory; belt-and-braces pin added; zero behavior change.
- **Source (v0.7.0, wayle-rs/wayle):** the click chain is `BarButtonOutput::LeftClick → DashboardMsg::LeftClick → dashboard.left_click.get() → dropdowns::dispatch_click` (`crates/wayle-shell/src/shell/bar/dropdowns/{mod,registry}.rs`); `ClickAction` deserializes `"dropdown:dashboard"` → `Dropdown("dashboard")` (`crates/wayle-config/src/click_action.rs`). The config system is NOT struct-deserialization: `ConfigProperty` layers (default < config.toml < runtime) applied per-leaf-path (`property/config.rs` + `loading/merging.rs` deep-merge) — a partial `[modules.dashboard]` table NEVER loses the `#[default(ClickAction::Dropdown("dashboard"))]` field default. The dashboard dropdown factory (`dropdowns/dashboard/factory.rs`) never returns None.
- **LIVE proof (run `2026-10-28T00-10-10Z` disk booted via qemu + usb-tablet, VNC via vncdotool):** `wayle config get modules.dashboard.left-click` = `dropdown:dashboard` (Default layer intact); REAL VNC click on the far-right dashboard button OPENED the full panel (quick actions/volume/media/network/system + user row + power) anchored above the bottom bar; second mechanism notes: firewalld blocks :5900 inbound (added runtime-only for the test), wayvnc binds 127.0.0.1 by default, VM has no pointer device without usb-tablet, stasis locks the idle session (veila unlock worked via VNC keyboard).
- **Change (pin, not fix):** `niri_wayle.go` step 7b seeds `left-click = "dropdown:dashboard"` explicitly (upstream default equals it — protects against upstream default drift, self-documents the session entry point); `wayle_session_actions` harness check greps the new line.
- **Validated:** src + harness gofmt/vet/golangci-lint/build/test green; niriWayle dinit harness `2026-10-31T02-22-44Z` **117/117 ALL PASS** (`wayle_session_actions` green with the new grep).
- **BONUS harness fix (moor 404, same-day):** run `2026-10-31T02-00-35Z` 116/117 — `log_errors` FAIL on `moor-2.10.2-r5: HTTP 404`. Root cause: the harness web cache (`internal/cache/http_cache_server.go`) cached Alpine APKINDEXes FOREVER; Alpine replaced moor -r5 → deleted the old .apk → stale index → mid-install 404s (package list itself unaffected, `ignoreErrors` carried the run). Fix: APKINDEX responses older than 6 h are revalidated from upstream (apks stay immutable/forever); on upstream failure the stale index is still served (offline-safe); storeAndRespond extracted from fetchAndCache. Stale cached APKINDEXes purged; re-run green.
- **If the failure reproduces on real hardware:** check `~/.local/share/dinit/wayle.log` for `WARN ... click dropped` (factory/registry would log it) and `~/.config/wayle/runtime.toml` (a GUI runtime override beats config.toml; `wayle config reset modules.dashboard.left-click` clears) — neither present on the verified VM.

### 2026-10-31 - raffi-toggle: launcher toggle (close-if-open/spawn-if-not) wired into niriWayle — aports + installer + harness
**Agent:** codebase
**Summary:** user request — Mod+D and the Wayle bar button now TOGGLE raffi (Windows-style) instead of spawning a second instance. New aports package `raffi-toggle` 1.0.0 (ISC; POSIX sh): `niri msg --json windows` → jq by app_id `com.chmouel.raffi` → open = `niri msg action close-window --id`, closed = `setsid raffi &` (detached — wayle click actions wait up to 30 s for the command to exit, raffi blocks until dismissed). Dependency is ONE-WAY `raffi-toggle → raffi` (NOT raffi→raffi-toggle: that cycle is unbuildable — raffi builds first alphabetically and its build env needs raffi-toggle, whose dep raffi is mid-rebuild; CI runs `41153877924`/`41154124571`, fixed `a3e5bde`) — the installer lists both packages explicitly in the niriWayle rules.
- **CI catch (run `41153877924` FAIL, fixed `13df2f1`):** abuild postcheck "Packages must not put anything under /usr/local" — user spec's `/usr/local/bin` corrected to `/usr/bin/raffi-toggle`; local source file needs the standard `source=` + `sha512sums=` entries (abuild checksum verify).
- **feralos:** `niri_wayle.go` — Wayle config.toml custom module `left-click = "raffi"` → `"/usr/bin/raffi-toggle"`, niri binds Mod+D `spawn "raffi"` → `spawn "/usr/bin/raffi-toggle"`; `internal_rules.yaml` niriWayle packages += `raffi-toggle` (niri list untouched — no wayle/raffi there).
- **harness:** `raffi_installed` extended (both packages + both binaries); `wayle_session_actions` += launcher-toggle grep.
- **Validated:** src + harness gofmt/vet/golangci-lint/build/test green; aports CI green, `raffi-toggle-1.0.0-r0` on Pages. Docs: `docs/guides/niriwayle.md` (Launcher + package list).

### 2026-10-27 - ursined boot failure fixed: unit ordered after local.target (log dir in @var-log subvol) + installer belt-and-braces
**Agent:** codebase
**Summary:** boot error `dinit: ursined: execution failed - opening log file: No such file or directory` (run `2026-10-27T22-57-50Z`). Root cause was NOT the post-install (it was present, correct AND executed — `Executing ursine-0.19.0-r0.post-install` in installer.log): `/var/log` is the `@var-log` fstab subvol, and `ursined.dinit` was the ONLY unit with zero dependencies → dinit started it before `early-fs-fstab` mounted the subvol → dir invisible pre-mount → service stuck FAILED (serial log failure printed BEFORE `early-fs-fstab.target` OK).
- **aports `c3ebc12`** (CI `40603536206`): `ursined.dinit` gains `depends-on = local.target` + `before = login.target` (mirrors the proven-green `vector` unit — same logfile-in-/var/log pattern; ADR-009 convention); post-install kept + chmod 0755; **pkgrel 0→1** (CI skip-if-unchanged would otherwise keep serving r0); ursined.dinit sha512sum refreshed.
- **feralos:** `services.go SetupServices` creates `mkdir -p /var/log/ursine && chmod 0750` before service enablement (belt-and-braces beside the post-install).
- **Lesson:** a dinit unit with `logfile=` under a dedicated subvol MUST carry `depends-on = local.target` — subvol mounts complete in the early chain (`early-fs-fstab` → `early-fs-local` → `pre-local` → `local.target`), any no-dep unit races them.

### 2026-10-25 - ADR-010 R1 amendment: system units carry the OpenRC initd name; fwupd/PPD on-demand — full matrix green (niri 113, kde 77, cosmic 77, niriWayle 109 + OpenRC 92)
**Agent:** codebase
**Summary:** user correction of the ADR-010 naming rule — the dbus-name convention applies ONLY to services dbus activates on demand; system services WE boot-enable must carry the OpenRC initd name (the `services` map alone then drives both inits); dbus-by-design daemons are never boot-enabled (dbus decides when they run).
- **Rule (amended R1, in ADR-010 + dbus-activation.md):** user services = bus name + `.user`; explicit system services (initd exists) = initd name; dbus-activated system services = bus name. Bus names stay on the D-Bus side always (override filenames/`Name=`, `dbus-wait-for -n` args, polkit action ids never change). `dinitServices` stays empty.
- **aports `e7f1cc8`** (CI `40212557219` green, Pages live): `elogind-dinit` unit `org.freedesktop.login1`→`elogind` (r11), `polkit-dinit` `org.freedesktop.PolicyKit1`→`polkit` (r8), `bluez-dinit` `org.bluez`→`bluetooth` (r7; mesh untouched); dependents retargeted: turnstile printf (r8), greetd-dinit (r10), sddm-dinit (r5), cosmic-greeter-dinit (r7). NOT renamed (dbus-activated on demand): fwupd, PPD, wpa_supplicant, nm_dispatcher, upower, udisks2, avahi, accountsservice, bolt, cups-pk-helper, flatpak SystemHelper, fprintd, geoclue, kcron, kauth×15.
- **rules:** `dinitServices` removed everywhere; CRITICAL catch — polkit/fwupd/bluetooth/elogind sat in `dinitSkipServices` (bus-name era) and were removed, else the renamed units would never be enabled; fwupd packageLogic `services` removed → on-demand on BOTH inits (was: always-running everywhere).
- **harness:** `services_status` expected lists updated (fwupd out on both inits; dinit niri-family now expects polkit/elogind/bluetooth); `bluetooth_service` dinit grep `bluetooth`; `power_profiles` reworked — PPD activation-only, the real switch proves on-demand activation and the started-marker is asserted AFTER it; `dbus_activation_env` system half made race-proof (graceful stop before pkill + retry loop with fresh per-try markers — smooth-recovery or a desktop client re-activating upowerd could resurrect a marker-less process and short-circuit the shim; dinit 0.21.0 setenv verified container-applied on every path).
- **Validated:** kde `17-19-00Z` **77/77** · cosmic `17-02-52Z` **77/77** · niri `17-50-09Z` **113/113** · niriWayle dinit `17-57-48Z` **109/109** · niriWayle OpenRC `18-16-31Z` **92/92** (finale). One harness assembly bug caught live (missing `;` between `su` and the dinit tail — su swallowed it): debug_run live diagnosis proved the product chain green end-to-end BEFORE the fix. fwupd on OpenRC = raw-exec fallback on activation (documented); smartd crashed = QEMU noise (tolerated).

### 2026-10-25 - Cosmic on elogind: seatd daemon dropped, full session stack — harness cosmic 77/77
**Agent:** codebase
**Summary:** User decision: unify cosmic on elogind + turnstile (same stack as niri/niriWayle/kde). Alpine 3.24 container-verified: NOTHING depends on the seatd daemon (`cosmic-comp` → `libseat` only, and libseat uses the logind backend), so no dummy package needed — the daemon is simply gone from the DE.
- **aports:** `cosmic-greeter-dinit` r6 (`4bfd658` + sum fix `1aa3ce8`; the r5 first attempt `c330c6d` used the wrong unit name `elogind` — since ADR-010 the unit is `org.freedesktop.login1`; live boot caught it: "service description not found"): both greeter units now `depends-on = org.freedesktop.login1`. NEW `playerctld-user-dinit` (`2c87f7a`; `org.mpris.MediaPlayer2.playerctld` → `/usr/bin/playerctld`; the Alpine package is `playerctl` — binary + activation file ship together). CI `40199433062` + `40200929104` green.
- **rules (cosmic):** packages `seatd`→`elogind` (`elogind-dinit` NOT listed — -openrc/-dinit auto-pull/convert), += `glib-networking-user-dinit` + `playerctld-user-dinit`; services `seatd`→`elogind`; NEW `dinitServices {"org.freedesktop.login1": "default"}` (dinit enablement name differs from the OpenRC initd name — same pattern as niri/niriWayle); groups `-seat` (the group came from the seatd package — `adduser <user> seat` failed on run `14-00-14Z`).
- **Validated:** cosmic full-feature dinit `2026-10-25T14-19-46Z` **77/77** (boot: `org.freedesktop.login1` + `turnstiled` `[ OK ]`, user session at 6.7 s; `cosmic_session` live — cosmic-comp + cosmic-session via libseat/logind; `dbus_activation_uniform` all routed; `migration` 0/14 uncovered; `log_errors` clean, only the 2 allowlisted apt warnings). Cosmic session coverage (old ADR-010 phase 10) UNBLOCKED and complete — the turnstile user dinit + materializer stack now runs on ALL four DEs.

### 2026-10-25 - KDE session-bus ADR-010 coverage + installer.log WARNING sweep — harness kde 77/77
**Agent:** codebase
**Summary:** KDE session D-Bus targets now run as supervised per-user dinit units (the turnstile user dinit + `dbus-session-materializer` were already in the KDE stack; `pam_turnstile` arrives via Alpine's `base-session` include in sddm's PAM — verified, no installer PAM edit needed).
- **aports (3 new unit-only packages, CI `40158178885` green, Pages live):** `obexd-user-dinit` (`org.bluez.obex`), `ibus-user-dinit` (`org.freedesktop.IBus` + `org.freedesktop.portal.IBus`), `system-config-printer-user-dinit` (`org.fedoraproject.Config.Printing`). Unit = canonical bus name + `.user`; override in `/usr/share/feralos/dbus-services`; upstream Execs verified verbatim on Alpine 3.24 (`ibus-daemon --replace --panel disable --xim`, `/usr/lib/ibus/ibus-portal`, `obexd -n --capability=!/usr/bin/obex-capabilities`, `/usr/bin/scp-dbus-service`). Git history: `00770f9` → revert `cc0b51d` (packages not requested at first) → restore+verify `1289145`.
- **KDE list** (`internal_rules.yaml`) += 9 packages: the 3 new + 6 existing already used by niri (`signon-ui-user-dinit`, `signond-user-dinit`, `xdg-desktop-portal-gtk-user-dinit`, `glib-networking-user-dinit`, `gnome-online-accounts-user-dinit`, `evolution-data-server-user-dinit`).
- **harness:** `log_errors` now fails on any non-allowlisted `WARNING` line in installer.log (allowlist = the 2 apt "does not have a stable CLI" notices + the ssh known-hosts notice); `org.kde.kcookiejar5` documented exemption (kded5 binary absent on Alpine 3.24 — no unit possible); `dbus_activation_env` session half skips honestly on DE-owned session buses (kde/cosmic).
- **installer:** `services.go` boot.d symlink resolves the unit in `/lib/dinit.d` or `/etc/dinit.d` (fixes `[WARNING] dinit: could not enable turnstiled` — the turnstile unit ships in `/etc/dinit.d`).
- **Validated:** kde full-feature dinit `2026-10-25T03-41-55Z` **77/77** (`dbus_activation_uniform`/`migration`/`env`, `dbus_system_activation`, `kde_session`, `log_errors`, `works_in_real_life` green; installer.log carries only the 2 allowlisted apt warnings); harness + installer gofmt/vet/golangci-lint/build/test green. Deferred: cosmic (turnstiled `depends-on = org.freedesktop.login1` needs elogind, absent on the seatd DE — user decision to leave aside).

### 2026-10-24 - KDE/COSMIC session autologin + runtime checks — harness kde 74/74, cosmic 74/74
**Agent:** codebase
**Summary:** user request — real session verification for kde/cosmic like niri. Harness autologin generalized to all four DEs: kde via sddm `[Autologin]` drop-in (`/etc/sddm.conf.d/zz-feralos-test-autologin.conf`, `Session=plasma`, wayland; sddm-autologin PAM requires pam_shells → shell appended to `/etc/shells`), cosmic via greetd `[initial_session]` (`start-cosmic`) in the FeralOS COSMIC greeter config. NEW runtime checks `kde_session` (kwin_wayland + plasmashell + wayland socket) and `cosmic_session` (cosmic-comp + cosmic-session + wayland socket), `pgrep -f` anchored (busybox `pgrep -x` false-negative class — run 2026-10-24T23-06-35Z proved the processes RUNNING while `pgrep -x` polled 90 s).
- **Product fixes (aports `bd4db35`):** `cosmic-greeter-dinit` ran the greeter frontend directly — cosmic-greeter 1.0 is a **greetd frontend** (needs GREETD_SOCK); now runs `greetd --config /etc/greetd/feralos-cosmic-greeter.toml` (VT 7 — Alpine's VT 1 collides with getty@tty1; base-only FeralOS PAM override — Alpine's file requires pam_gnome_keyring; wrapper ensures the greeter user + runtime dirs; `cosmic-greeter-daemon` unit added; `pam-rundir` dep for XDG_RUNTIME_DIR — cosmic uses seatd, not elogind, and nothing else creates /run/user).
- **cosmic-comp seat fix (debug_run-proven):** libseat could not acquire the seat — seatd socket is `root:seat 0770` and the user was not in `seat` → instant exit ("Failed to acquire session"); cosmic `groups` += `seat` and the comp starts (same binary/env, proof by intervention).
- **Validated:** kde `2026-10-24T23-35-39Z` **74/74** (kwin_wayland + plasmashell live); cosmic `2026-10-24T23-50-51Z` **74/74** (cosmic-comp + cosmic-session live, wayland-1); aports CI `40140550886` green; zero FAILED, log_errors clean.
- **Note:** KDE/cosmic session-bus ADR-010 coverage stays a separate item (own session bus, no user-dinit stack).

### 2026-10-21 - Full-feature DE matrix (niri/kde/cosmic) + ADR-010 system-bus coverage everywhere — niri 113/113, kde 73/73, cosmic 73/73, niriWayle 109/109 + 92/92
**Agent:** codebase
**Summary:** user request — run niri/kde/cosmic with dinit and ALL features (secureboot/tpm/hardening/zswap/nix/podman/webmin/debug/edge) to discover missing services, then implement. Findings + fixes:
- **niri** (110/112 → 113/113): the niri list lacked `upower-dinit` (raw upowerd spawn) and `xdg-desktop-portal-gnome-user-dinit`; the gnome-keyring/seahorse/Secret-portal set had no units. New aports `gnome-keyring-user-dinit` (secrets unit + 3 aliases, gcr prompter unit + 2 aliases, seahorse GUI unit); niri list += `gnome-keyring-user-dinit`, `upower-dinit`, `xdg-desktop-portal-gnome-user-dinit`.
- **kde** (71/72 → 73/73): `dinit_shutdown_commands` wrongly required the niri-only `/etc/sudoers.d/zz-power` (now gated to the niri family). New unconditional dinit check `dbus_system_activation` (system-bus coverage on EVERY DE) exposed 5 uncovered system targets → new `plasma-kauth-dinit` (15 KAuth helper units), `fprintd-dinit`, `bolt-dinit`; kde list += `power-profiles-daemon-dinit`, `accountsservice-dinit`, `upower-dinit`.
- **cosmic** (71/72 → 73/73): `geoclue-dinit` + `upower-dinit` added.
- **idle-exit fix** (`ee2b824`): dinit `smooth-recovery = true` restarts a process that exits for ANY reason — including a legitimate idle exit. Set `smooth-recovery = false` for geoclue (60 s), fprintd (30 s) and the EDS addressbook/calendar factories (exit after last client): the unit stops and the next activation restarts it. Long-running daemons keep smooth-recovery.
- **Validated:** harness gofmt/vet/golangci-lint/build green; niri `2026-10-20T23-30-11Z` **113/113**, kde `23-37-50Z` **73/73**, cosmic `23-43-40Z` **73/73**, niriWayle `23-49-44Z` **109/109** (migration 0/64), OpenRC niriWayle `23-57-22Z` **92/92**; aports CI `39508435007` + `39509305005` + `39513258317` + `39514358385` green; zero FAILED, log_errors clean.
- **Remaining gap (documented):** KDE/cosmic **session** activation targets are not covered yet — those DEs start their own session bus (no turnstile user dinit / `dbus-session` materialization). Covering them needs the turnstile + user-dinit session stack wired into those DEs first (session-bus ownership decision) — tracked as the next ADR-010 item, not silently exempted.

### 2026-10-20 - ADR-010 third-party activation units (app set) + migration gate — harness 108/108 (dinit) + 92/92 (OpenRC)
**Agent:** codebase
**Summary:** the niri/niriWayle app set brings 15 D-Bus activation targets with no supervised unit: avahi, udisks2, geoclue, cups-pk-helper, kcron (system); goa daemon + goa-identity-service, evolution-data-server ×4, GNOME Calendar/DiskUtility/Firmware/Maps (session). Per user decision ("all go under dinit", anticipating dbus-broker's unit-per-activation model), feralos-aports `44c598a` adds transitional units + shim overrides (7 new packages; avahi-dinit/udisks2-dinit extended). Exact Exec paths/args and lifecycle verified on Alpine 3.24: avahi's upstream file is `Exec=/bin/false` (systemd-only trap; unit runs `avahi-daemon` foreground, no `-D`), UDisks2 path `/usr/libexec/udisks2/udisksd`, geoclue keeps its 60 s idle exit, kcron's helper is a short-lived KAuth binary, GNOME apps run `--gapplication-service`; GOA is two distinct binaries (no alias dedupe).
- **Harness NEW** `dbus_activation_units_live` (real activation of org.freedesktop.UDisks2 + org.gnome.OnlineAccounts through the shim → dinit, asserting no `DBUS_STARTER_*`) and `dbus_activation_migration` (forward-looking gate: enumerates **every** activation file live — no static list — and fails only when **dbus-broker is installed AND every target is unit-covered**; aliases count via override→existing unit; otherwise passes with "keep the FeralOS activation mechanism").
- **Validated:** harness gofmt/vet/golangci-lint/build green; dinit niriWayle `2026-10-20T21-22-46Z` **108/108** (migration gate: 0/64 targets uncovered); OpenRC niriWayle `2026-10-20T21-30-15Z` **92/92** (8/64 — only dinit-only system units, expected); aports CI `39494111005` green; zero FAILED, log_errors clean.
- **Docs:** ADR-010 amendment (third-party targets + retirement gate), dbus-activation inventory rows, niri/niriWayle guides, README/ROADMAP/artifacts counts, state/handoff.

### 2026-10-19 - niriWayle follow-up: kglobalaccel stays KDE-only (wiring reverted) + decisions closed — harness 106/106 (dinit) + 90/90 (OpenRC)
**Agent:** codebase
**Summary:** user correction on the open-items pass: `kglobalaccel`/`kglobalacceld` is a KDE **global-shortcuts** service for KDE applications (a Plasma-session component: hotkeys that work regardless of focus), NOT a niri/Wayle dependency — under niri there is no KWin/GlobalShortcuts integration for Wayland key grabbing, and the KDE apps shipped on niri do not use it. The earlier wiring (`92116e4`: `kglobalacceld` + `kglobalaccel-user-dinit` in both DE lists, `kglobalaccel_daemon` check, `org.kde.kglobalaccel.user` in the service lists) was **reverted**. The aports `kglobalaccel-user-dinit` unit + shim override remain available as optional coverage (not installed by the DE lists; `dbus_activation_uniform` covers them if ever installed; the KF6 `kglobalaccel` LIBRARY is a transitive dep of kio/kded/kwallet and does get installed — only the daemon package matters).
- **Decisions (user):** kglobalaccel KDE/Plasma-only (never wire into niri/niriWayle); dms-shell split (`dms` + `dms-dinit`) NOT planned — dropped (annotation in the 2026-10-11 round); Booster initramfs module policy RESOLVED — parked behavior final (no universal mode; removed from next_actions).
- **Validated after revert:** harness gofmt/vet/golangci-lint/build green; dinit niriWayle `2026-10-19T22-01-31Z` **106/106**; OpenRC niriWayle `2026-10-19T22-13-26Z` **90/90**; zero FAILED; `dbus_activation_uniform`/`dbus_activation_env` green on both. Docs: guides, dbus-activation inventory, README/ROADMAP/artifacts counts, state/handoff synced.

### 2026-10-18 23:15 - D-Bus activation → dinit services (ADR-010) CLOSED — system-bus helper-eligibility fix + full validation, harness 106/106 (dinit) + 90/90 (OpenRC)
**Agent:** codebase
**Summary:** user directive "nothing dbus-activatable may run unsupervised" implemented end-to-end and validated on both inits: every activation starts a dinit unit via the uniform shim (`Exec=/usr/libexec/feralos/dbus-activate <unit> -- <upstream exec>`; R1 naming = canonical bus name (+`.user`); activation env forwarded with one `dinitctl setenv`; OpenRC system-bus fallback = exec upstream).
- **Validation round 2 root cause (live debug_run + dbus 1.16.2 source):** system-bus activation runs through the setuid `dbus-daemon-launch-helper`, whose "trivial" parser resolves names ONLY against the standard dirs (`/usr/local/share` > `/usr/share` > `/lib`); `/etc/dbus-1/system-services` is daemon-enumerated (`ListActivatableNames`) but NEVER helper-activatable (raw spawn; `Spawn.ServiceNotFound` with no standard twin). Live-proven: the shim override in `/usr/local/share/dbus-1/system-services` routes activation through dinit instantly (unit started, no `DBUS_STARTER_*`).
- **Fix:** aports `2e0caa8` — 13 system overrides moved to `/usr/share/feralos/dbus-system-services` + NEW `dbus-services-setup` (feralos-dbus-activate r2; dinit unit + OpenRC initd, both ordered before dbus) materializes them at boot into `/usr/local/share/dbus-1/system-services`; `5f729ea` stale unit-file comments. Feralos: `defaultRunlevels.boot` += `dbus-services-setup`, `standardPackages` += `feralos-dbus-activate`; `dbus_activation_uniform` reworked (init-aware: sources + materialization + `/etc` clean; upstream coverage loop dinit-only) + `dbus_activation_env` system half reworked (real activation: no `DBUS_STARTER_*`; direct-shim `setenv` proof).
- **Validated:** harness gofmt/vet/golangci-lint/build green; dinit niriWayle `2026-10-18T22-56-20Z` **106/106**; OpenRC niriWayle `2026-10-18T23-03-31Z` **90/90**; aports CI `39172523572` + `39175407432` green. Docs: ADR-010 + ADR index + `docs/architecture/dbus-activation.md` (incl. new-service checklist + troubleshooting) + AGENTS policy section + NEW `docs/architecture/boot-oneshots.md` (both-init oneshot pattern) + `feralos-aports` `DBUS-ACTIVATION.md` (pattern mirror + diagnostics).

### 2026-10-18 19:10 - power-profiles-daemon for niriWayle: verified + polkit fix — harness 104/104 (dinit) + 88/88 (OpenRC)
**Agent:** codebase
**Summary:** user question "i profile powerdaemon per wayle sono stati fatti?" → audit found: PPD (`power-profiles-daemon` 0.30) IS in both DE package lists and Wayle v0.7.0 ships a battery-dropdown `power_profile` section — but switching was never validated and, under dinit, `switch-profile` (`allow_active=yes` upstream) is unsatisfiable (NoSessionForPID) → switches would be silently denied. Verified, fixed, proven.
- **Facts (source-verified):** Alpine ships PPD **D-Bus ACTIVATED** (no init script by design — `0001-dbus-no-systemd.patch` sets `Exec=/usr/libexec/power-profiles-daemon` on the shared service template → BOTH bus names activatable: `net.hadess.PowerProfiles` alias + `org.freedesktop.UPower.PowerProfiles`); 0.30 renamed the polkit ids to `org.freedesktop.UPower.PowerProfiles.switch-profile/.hold-profile` (policy file `power-profiles-daemon.policy`, defaults `allow_active=yes`); the placeholder driver (QEMU: no platform-profile hw) still offers power-saver + balanced.
- **Installer:** `writePowerPolkitRule` extended with both PowerProfiles actions (same NoSessionForPID class as the login1 power buttons; written for both inits).
- **Harness:** NEW `power_profiles` (niri family, both inits) — static activation wiring (BOTH `.service` files' Exec + the policy file) + **REAL switch round trip** as the session user (`powerprofilesctl get` → `set power-saver` → assert effect → restore); `power_polkit_rule` extended with the switch-profile id.
- **Validated:** src + harness gofmt/vet/golangci-lint/build/test green; dinit `16-42-07Z` **104/104**; OpenRC `16-48-00Z` **88/88** — `power profiles OK (real switch: balanced -> power-saver -> balanced)` verbatim on both inits.

### 2026-10-18 18:10 - stasis system config shipped in aports + veilad env wrapper + suspend polkit — harness 103/103 (dinit) + 87/87 (OpenRC)
**Agent:** codebase
**Summary:** user audit of "lock screen + its trigger + zz-power + automatic suspend" exposed a REAL gap: stasis (the idle orchestrator) had NO FeralOS config — its first-run bootstrap would generate a generic template (`swaylock` + `systemctl suspend`, absent on FeralOS) and PREFER it over any system file (source-verified `daemon_mode.rs`: the bootstrap is skipped only with `-c`) → lock-on-idle and auto-suspend were dead. Fixed following the upstream NixOS-module system-deployment pattern — everything in feralos-aports.
- **aports** (stasis r1 `a654787`, wayle-session r4 `1fbb5bc`, CI `39142166165` green): NEW `/etc/stasis/stasis.rune` FeralOS plan — lock `veila lock` 5 min → niri `power-off-monitors` +1 min → `loginctl suspend` +30 min; laptop `ac:` / `battery:` plans; media/app inhibitors; `enable_loginctl_integration`. `stasis-session` now execs `stasis -c /etc/stasis/stasis.rune` (upstream NixOS module passes exactly `-c /etc/stasis/stasis.rune` — mirrored). NEW `veilad-session` env wrapper (wayle-session pkg): veilad starts without WAYLAND_DISPLAY/NIRI_SOCKET in the user-dinit env (upstream `veilad.service` PassEnvironment's both) — same wayvnc-session pattern; `veila.user` gained the `wait-wayland.user` dep.
- **installer**: polkit rule extracted to `writePowerPolkitRule` — written for BOTH inits + `login1.suspend` added (stasis' automatic `loginctl suspend` would otherwise hit the dinit `NoSessionForPID` auth prompt; the wheel rule is the same posture as the power buttons); `internal_rules.yaml` lists `stasis` explicitly (was only a wayle-session dep — now visible belt+braces).
- **harness**: `stasis_idle` extended (config content: `veila lock` + `loginctl suspend`; `-c` in the wrapper; NO `~/.config/stasis/stasis.rune` — proves the bootstrap was skipped; `stasis info` IPC as the session user), `veila_daemon` extended (wrapper wiring), `dinit_shutdown_commands` trimmed (polkit moved out), NEW `power_polkit_rule` (niri family, both inits), NEW OpenRC fixture `btrfs_standard_niriwayle.yaml`.
- **Validated**: src + harness gofmt/vet/golangci-lint/build/test green; dinit niriWayle `15-58-02Z` **103/103**; OpenRC niriWayle `16-04-08Z` **87/87** (first niriWayle OpenRC run — the turnstile user-dinit stack + stasis/veila config work on BOTH inits). Docs synced (`niriwayle.md` Locking & idle + config paths, `artifacts.md` rows, README/ROADMAP counts).

### 2026-10-18 17:20 - niriWayle follow-ups CLOSED: Wayle session actions wired + stasis included — harness 102/102
**Agent:** codebase
**Summary:** Both open workstreams done in one round: the Wayle dashboard session buttons now run FeralOS-correct commands, and stasis is documented as included (was mislabeled "v2"). Harness **102/102 ALL PASS** (run `2026-10-18T15-10-54Z`, zero fail/skip).
- **(a) Wayle session actions** (v0.7.0 source-verified: dashboard dropdown user-row buttons exec `sh -c <dashboard.dropdown_*_command>`; power module = icon + ClickAction only, default `None`; default bar layout has NO session entry point): `niri_wayle.go` NEW step 7b seeds `~/.config/wayle/config.toml` — lock=`veila lock`, logout=`niri msg action quit --skip-confirmation` (compositor exit → wayle-session dinit-monitor hold ends; no polkit `NoSessionForPID` under dinit), reboot=`sudo -n /usr/bin/reboot` (dinit, zz-power NOPASSWD — logind reboot broken under dinit) / `loginctl reboot` (OpenRC), poweroff=`loginctl poweroff`; `[modules.power] left-click = "dropdown:dashboard"` + bar layout right += power. Upstream defaults are systemd-only (`systemctl`) → the seeded commands ARE the product.
- **(b) stasis** no longer "v2": shipped de facto since wayle-session r3 (depends `stasis`, starts supervised `stasis.user` existence-filtered) — stale "stasis deferred to v2" / "No stasis in v1" comments corrected (`internal_rules.yaml`, docs, AGENTS DE bullet).
- **Harness**: niriWayle lists += `stasis.user` (dinitUserSvcFiles/sessionAppsAbsent/userDinitRuntimeSvcs); `dinit_shutdown_commands` niriWayle branch greps the wayle config; NEW `wayle_session_actions` (7 static asserts) + `stasis_idle` (apk + service file + started-marker poll). Validation fixes: `%q` for the reboot grep (gocritic sprintfQuotedString); `oo7UnlockRequestCount` parses the last line (ssh known_hosts warning broke `Atoi` in run `15-03-04Z`, 101/102).
- **Validated**: src + harness gofmt/vet/golangci-lint/build/test green; `15-03-04Z` 101/102 → **`15-10-54Z` 102/102**. Docs synced (README/ROADMAP counts, `niriwayle.md` "Session actions" section, `artifacts.md` rows, state/handoff/memory).

### 2026-10-18 15:50 - FERALOS_NAME: wolf→bear, based-on-Alpine framing; variables.yaml default niriWayle pushed
**Agent:** codebase
**Summary:** `docs/FERALOS_NAME.md` reworked: "The Wolf as Spirit" → "The Bear as Spirit" (brown bear as the Alpine rewilding symbol — *Life Ursus*; traits: feral/powerful/resilient/self-sufficient; rewilding ↔ wild evolution) and the wording settled — FeralOS is **based on Alpine** (it consumes Alpine's repositories; "Alpine fork" was already on the vocabulary Avoid-list), the feral metaphor = descendant of something established living wild (not an archive-level fork). `src/variables/variables.yaml` template default `desktopEnvironment` niri→niriWayle committed + pushed.
- Review: unique, particular name (double meaning: wild + descendant; Alpine-native animal, less overused than the wolf).
- Files: `docs/FERALOS_NAME.md`, `src/variables/variables.yaml`, AGENTS.md, state/handoff.

### 2026-10-18 15:40 - niriWayle review nits + whole-project doc refresh; README flagship statement
**Agent:** codebase
**Summary:** Post-review polish on green `8808578` (review: APPROVE WITH NITS, 7 nits, all harness-side) + docs refreshed project-wide; **niriWayle declared the flagship / true FeralOS experience** in README/ROADMAP/AGENTS (other DEs = user convenience, never as ready-to-use). Zero product changes; full harness validation green.
- **Harness nits** (`oo7_pam_unlock.go`, `service.go`): stale "greetd-IPC" comments → direct PAM chain over exec SSH; PAM labels fixed (`PAM_BUF_ERR=5`, `PAM_CONV_ERR=19`); exit-code comment = real codes 10-15 + stdout `PAM_LOGIN_OK`; `secrets.log` proof = **count-delta** (baseline before login, poll requires increment — new `oo7UnlockRequestCountCmd/Count` helpers, `strconv` import, sh-safe `c=$(grep -cF ... 2>/dev/null); echo ${c:-0}`); recovery-wrapper assert = `dinitctl --user start`.
- **Docs**: README (flagship paragraph + Dinit "Available" section + up-to-100 checks + niriWayle in DE status + RustDesk→wayvnc), `getting_started.md`/`variables.md` (enum + 28 fixtures), `docs/guides/README.md` + `niri.md` (niriwayle.md pointers), ROADMAP (v2.4 row, Phase 2/3 statuses, DMS native apk), `artifacts.md` (selective list header, Key Rack/`oo7_pam_unlock` rows, latest 100/100), `docs/dinit/README.md`+`PLAN.md` (shipped, definition-of-done checked), AGENTS.md (header, DE bullet, Roadmap table, guides index).
- **Validated**: harness gofmt/vet/golangci-lint/build + `go test` green; embedded python `py_compile` green; count snippet 0/2 in sh.

### 2026-10-18 03:15 - oo7_pam_unlock GREEN: authenticated PAM proof — niriWayle harness 100/100
**Agent:** codebase
**Summary:** The niriWayle `oo7_pam_unlock` check now passes live end-to-end (run `2026-10-18T03-03-11Z`, **100/100 ALL PASS**, 0 fail/skip): statics → authenticated PAM login on service `greetd` → daemon logs "Received unlock request". ZERO product changes needed this round — the shipped installer/aports PAM wiring was correct all along; every failure was harness-side.
- **Transport graveyard (3 dead ends, all diagnosed on live VMs):** (1) host-side `ssh -L` unix forward — stock Alpine sshd `AllowTcpForwarding no` denies ALL local forwards incl. streamlocal (OpenSSH `channel_disable_admin → channel_connect_to_path` source-verified) AND `guest.Output`'s CombinedOutput polluted the socket path with the known_hosts warning → `Bad local forwarding specification`; (2) greetd-IPC second `start_session` — greetd 0.10 = ONE session: a second session PAM-closes the autologin c1 → turnstile tears the whole user dinit down (niri/dbus/secrets STOPPD storm, greeter takeover) → collateral FAILs on `niri_session_user`/`session_dbus_socket` (random Go-map order) + the pam_oo7 delivery thread loses the 2s-retry-vs-5.1s-respawn race; (3) FINAL = **direct PAM chain, python3+ctypes libpam over plain exec SSH** (`pam_start("greetd") → authenticate → acct_mgmt → open_session → sleep 2 → close`) — autologin session + live daemon untouched: no race, no session kill, no greeter takeover.
- **Second root cause (hid everything): ctypes RTLD_LOCAL.** pam_oo7.so (oo7 0.6.0-r4) has NO `DT_NEEDED libpam.so.0` (pam_get_item/pam_set_data/pam_get_data/pam_get_user left undefined — glibc convention: app links libpam → global scope) → musl dlopen under the ctypes default RTLD_LOCAL FAILS → PAM skips the `optional` module SILENTLY → no stash → no delivery, chain still rc=0. Fix = one line `mode=ctypes.RTLD_GLOBAL`. Live probe matrix proved it: `required` variant → rc=28 "Module is unknown" (RTLD_LOCAL) vs rc=0 + secrets.log delta=1 (RTLD_GLOBAL); stock greetd + RTLD_GLOBAL = delta 1 with UNMODIFIED product files. H1 (sufficient short-circuit) DISPROVEN: Alpine base-auth = all `required` + vendordir `-auth` lines, tail modules evaluate (pam_exec sentinels touched). greetd NEEDs libpam → real greeter logins always worked.
- **Source-verified facts (oo7 0.6.0, linux-credentials/oo7):** daemon line `Received unlock request for user:` = `tracing::info!` (RUST_LOG=info suffices; grep string correct); wire = zvariant D-Bus-serialized `PamMessage` + u32 LE prefix (NOT JSON); pam-module logs go to journald via tracing_journald — absent on Alpine → daemon `secrets.log` is the ONLY observable delivery surface; `start_daemon` socket-missing fallback = fork+execv `/usr/bin/oo7-daemon` = OUR recovery wrapper → supervision rule holds on every path.
- **Check final shape** (`tests/harness/internal/service/oo7_pam_unlock.go`): statics (+`python3` presence assert, registered in service.go) → `pamLoginRemote` (script travels `printf %s '<b64>' | base64 -d | sh` so fish never parses the heredoc; user/password base64 via env; PAM conv responses from libc.malloc — PAM frees them; 2s wait for the delivery thread; `PAM_LOGIN_OK` marker) → 30s grep-poll of `Received unlock request` in secrets.log (EnableDebug fixtures). Two Go-raw-string backtick incidents caught by lint — comments inside the python blob use `#`, NEVER a backtick inside a Go raw string.
- **Validated:** harness gofmt/vet/golangci-lint/build + `go test` green; embedded python `py_compile` green; regression ladder 99/100 (`01-49-53Z`, session-kill collateral) → 99/100 (`02-28-16Z`, RTLD_LOCAL) → **100/100 (`03-03-11Z`)**. Also this round (pre-green): aports CI fix `b3970c1` (container via `mirror.gcr.io` — Docker Hub + ECR blocks), `oo7-user-dinit` r1 (ADR-008 `secrets` wrapper + `secrets.conf` + recovery wrapper), installer `RUST_LOG=info` under enableDebug.
- **Optional upstream note:** Alpine/oo7 could link libpam into pam_oo7 (DT_NEEDED) like every other Alpine PAM module — would fix any RTLD_LOCAL consumer permanently.

### 2026-10-17 19:15 - oo7 full-desktop unlock parity (niriWayle): PAM login-unlock + password re-sync
**Agent:** codebase
**Summary:** oo7 reaches full gnome-keyring parity — auto-unlock at login, no-prompt first use, keyring password re-sync on `passwd` — daemon stays a supervised per-user dinit service (`secrets.user` untouched). Zero aports changes.
- **Research (source-verified at the Alpine-shipped oo7 0.6.0, tag `0.6.0`):** `pam_oo7` = 3 stacks — `auth` stashes the login password in the PAM handle; `session auto_start` forks as the user and sends it to the daemon's PAM listener `/run/user/$UID/oo7-pam.sock` (0600, peers root-or-same-UID) with a 20×100ms retry window; `password` re-syncs keyrings on chauthtok. The daemon (`pam_listener`, always active) listens on that socket and unlocks ALL locked collections with the received secret.
- **Race solved by an Alpine packaging accident:** 0.6.0 `auto_start` execs the HARDCODED `/usr/bin/oo7-daemon` when the socket is missing — Alpine installs `/usr/libexec/oo7-daemon` → the exec is a harmless no-op → the retry loop hits OUR supervised daemon (started after `dbus.user`, well within the 2s window). Nothing unsupervised ever runs (supervision rule intact). NOTE: upstream main REDESIGNED this (socket-missing → exec `/usr/libexec/oo7-daemon-login` helper holding the secret via memfd; the main-branch daemon auto-connects to it) — Alpine 3.24 stable ships neither the helper nor the main daemon behavior; 0.6.0 + retry window is what we target.
- **First-use bootstrap (daemon-side, no installer work):** the daemon auto-creates an EMPTY "Login" keyring aliased `default`; an empty keyring accepts ANY password (`if self.items.is_empty() { return Ok(true) }`), so the PAM unlock binds it to the login password and the first `create_item` persists it encrypted with exactly that. No prompt, ever.
- **Installer** (`niri_wayle.go` step 11, was "NO unlock / documented follow-up"): idempotent grep-append of `auth optional pam_oo7.so` + `session optional pam_oo7.so auto_start` to `/etc/pam.d/greetd` (session line LAST — after the base-session includes, so pam_turnstile/pam_elogind ran and /run/user exists) + NEW `/etc/pam.d/passwd` override (`password include base-password` + `password optional pam_oo7.so` — Alpine ships no passwd file, `passwd` falls back to vendor `other`; the override shadows ONLY the passwd service). gosec G101 false positive silenced with the repo-style `//nolint:gosec` (PAM config text, not a credential).
- **Known limitation (universal, same as GNOME):** root-forced resets via `chpasswd` carry no old authtok → keyrings keep the old password → next login prompts once. DMS user-mgmt password changes = chpasswd = same.
- **Harness:** new niriWayle-gated `oo7_pam_unlock` — static (`.so` on disk + both greetd lines + passwd override) + live (listener socket, 30s poll) + RUNTIME PROOF: the daemon's "Received unlock request" line in `~/.local/share/dinit/secrets.log` (harness autologin = REAL greetd PAM login with the configured password; secrets.log only gains lines from real user-dinit sessions — no login during install). `oo7_secrets` comment updated (follow-up wording obsolete).
- **Validated:** src gofmt/vet/lint/build/test green (golangci-lint clean after nolint); harness gofmt/vet/lint/build/test green (5/5). NEXT: niriWayle harness run to validate `oo7_pam_unlock` live (real unlock request in secrets.log on the verify boot).
- **Alpine PAM fact (discovered on the way):** Alpine 3.24 moved the base stacks to the PAM vendordir `/usr/lib/pam.d/` (`base-auth/base-password/base-session/...` — `/etc/pam.d/<svc>` overrides); gnome-keyring/kwallet hooks are ALREADY upstream-baked there (`-auth optional pam_gnome_keyring.so`, `-password optional ... use_authtok`, `-session optional ... auto_start`) — our oo7 lines mirror exactly that pattern; niri.go's extra pam_gnome_keyring auth append is redundant-but-harmless (base-auth already carries it).

### 2026-10-17 17:53 - aports: stasis 1.6.3 idle manager + wayle-session r3 published (CI green)
**Agent:** codebase
**Summary:** Placeholder round completed — sha512 sums computed + filled in both APKBUILDs, 3 commits pushed (`ab002fb`, `6acfe87`, `e9401f4`), CI run `39046348443` all green (build+deploy+client-test), `stasis-1.6.3-r0` + `wayle-session-0.1.0-r3` live on Pages.
- **stasis 1.6.3**: context-aware Wayland idle manager (Rust, matugen cargo pattern: `fetch --target --locked` + `build/test --frozen`), dinit user service `stasis.user` + `stasis-session` script; dual-license GPL-3.0-only + AGPL-3.0-only (bundled lib_game_detector).
- **CI catch 1** (`39045036898`): upstream v1.6.3 tarball ships ONLY `LICENSE` — no LICENSES dir → `package()` install failed. Fix: canonical GNU AGPL-3.0 text (661 lines) vendored from gnu.org into the package dir, hash-pinned (`3568a766…`), installed next to LICENSE (AGPL §13 posture kept).
- **CI catch 2** (`39045865048`): the r3 `wayle-session` script (stasis.user wiring, sum `79437b…`) was left UNSTAGED while only the APKBUILD was committed — abuild checksum verify ran against the stale r2 script. Fix: commit the matching script (`e9401f4`); no checksum edit needed. LESSON: an APKBUILD sum bump must always bring its source files in the SAME commit.
- wayle-session session script ALREADY referenced `stasis.user` in its existence-filtered session-apps loop (committed earlier) — r3 APKBUILD adds the `stasis` depends; stasis now actually starts on niriWayle logins.
- **Next**: installer (PAM oo7 login-unlock + config seeds) → harness → docs.

### 2026-10-17 - user-services completion + Key Rack flatpak GUI + Secret portal pin + nix XDG gap + supervision rule
**Agent:** codebase
**Summary:** Completes the niri/niriWayle per-user service round on top of the validated uncommitted tree (niriWayle dinit 96/97 — ONLY fail = seahorse/oo7-server file conflict; niri dinit 98/98; OpenRC niri 82/82).
- **Seahorse conflict root cause + resolution (user decision)**: seahorse and oo7-server BOTH ship `/usr/share/dbus-1/services/org.freedesktop.secrets.service` — installing both leaves the apk world permanently broken. `seahorse` REMOVED from the niriWayle package list (oo7 stack owns secrets there); KEPT on the niri list (gnome-keyring lives there, no conflict, proven green).
- **Key Rack secrets GUI via Flathub (user decision)**: niriWayle secrets GUI = `app.drey.KeyRack` flatpak (current 0.6.x, Rust + native oo7 client + zeroize, GPL-3.0) — NOT an aports package (would drag the seahorse conflict shape back) and NOT nix (nixpkgs only has 0.4.0, too old). New `flatpakPackages: ["app.drey.KeyRack"]` key in the niriWayle rules block (same flow as niri's rclone-manager/Flatseal: `flatpak` apk + Flathub remote + `flatpak install -y --noninteractive`). NEW harness check `keyrack_flatpak` (niriWayle-gated, no DMI guard — flatpak + portals conf real on every host): `flatpak info app.drey.KeyRack` rc 0 + user niri-portals.conf contains the Secret pin.
- **Secret portal pin (oo7 README config)**: niriWayle user `niri-portals.conf` now maps `org.freedesktop.impl.portal.Secret=oo7-portal;` explicitly (was only ScreenCast=wlr — Secret would resolve implicitly to the gtk backend, which cannot serve secrets without gnome-keyring). niri portals.conf UNTOUCHED (gnome-keyring owns Secret implicitly there).
- **nix XDG_DATA_DIRS gap fixed in BOTH DEs**: `$HOME/.nix-profile/share` appended to XDG_DATA_DIRS in `/etc/environment` AND the niri config `environment` block (niri.go + niri_wayle.go, 2 spots each) — nix-installed desktop entries were invisible to Raffi/gio. Additive; enableNix=false installs carry a harmless nonexistent path entry.
- **Supervision rule (user directive) written into AGENTS.md**: ALL user-session daemons = supervised per-user dinit services (`.user`, started by the turnstile-spawned user dinit) — NOTHING left to unsupervised D-Bus activation, autostart or daemon self-spawn; missing `-user-dinit` packages get created in feralos-aports; applies to BOTH inits (OpenRC gets the identical stack via turnstile).
- **Session env mechanism = the Chimera turnstile pattern, ALREADY in place (verified, no new mechanism)**: research (turnstile source + Artix dinit-userservd fork + Chimera handbook + dinit man pages) proved FeralOS already mirrors Chimera exactly: (1) `/etc/environment` → `pam_env` (our aports turnstile appends `session optional pam_env.so` to `/etc/pam.d/turnstiled`) → `pam_getenvlist` in turnstiled's spawn path → user dinit env; (2) the upstream dinit backend `/usr/libexec/turnstile/dinit` sources `/etc/profile` (profile.d snippets) before `exec dinit --user`; (3) session vars via `dinitctl setenv` from the session scripts (niri-session/wayle-session); (4) `graphical.monitor` (`dinit-monitor -E WAYLAND_DISPLAY DISPLAY`) ships in the backend. Artix's dinit-userservd fork imports NOTHING (anti-pattern, "environment variables of the login do not exist within the user instance by default"); no environment.d equivalent exists on either distro. The compositor-session side keeps the niri `environment` block (equivalent of startplasma's env handling). elogind sets only XDG_RUNTIME_DIR/SESSION_ID/SEAT/VTNR — never XDG_DATA_DIRS.
- **NEW harness check `user_session_env`** (niri-family, init-agnostic): proves the chain END-TO-END — reads `/proc/<pid>/environ` of a user-service process (pipewire, started ONLY by the user dinit) and asserts XDG_DATA_DIRS carries BOTH seed tails (flatpak exports + `.nix-profile/share`).
- **aports CI: skip-if-unchanged (version check)** — new "Seed published packages" step pulls the published repo (APKINDEX + apks from Pages) into REPODEST and pre-indexes it; the build loop sources each APKBUILD and SKIPS when `<pkgname>-<pkgver>-r<N>.apk` is already published (rebuilt packages replace their older published apk first). Run time 38-54 min → ~3 min when only CI/workflow changes; full green validated end-to-end (build+deploy+client-test). GOTCHA fixed on the way: without the pre-index, a rebuilt package whose deps are all skipped fails "no such package" (abuild resolves depends against REPODEST).
- **aports fix `xdg-desktop-portal-user-dinit` r1: portal-order name gates** — live-caught D-Bus activation twin race: xdg-desktop-portal probes `org.freedesktop.portal.Documents` at startup; when the supervised document portal hasn't claimed the name yet, the session bus ACTIVATES an unsupervised twin via the shipped `.service` activation file and the supervised instance loses the name race → exit 20 (`on_name_lost` → `final_exit_status = 20`, document-portal.c; flaky — previous run passed). New `/usr/lib/dinit/portal-order` (POSIX sh, polls DBus NameHasOwner ~60s then execs): chain = permission-store (root) → document-portal (gated on PermissionStore) → frontend (gated on PermissionStore + Documents). Keeps every portal process supervised. dbus-wait-for NOT usable for this (it execs immediately and only signals readiness).
- **Tree basis (pre-existing, validated, uncommitted — kept)**: niriWayle packages += `oo7-user-dinit` + `xdg-desktop-portal-user-dinit`; niri packages += `xdg-desktop-portal-user-dinit` (+ seahorse stays); extended `userServices` boot.d lists in both DE files (niriWayle: dbus/pipewire/pipewire-pulse/wireplumber/wait-wayland/secrets/oo7-portal/xdg-desktop-portal/xdg-document-portal/xdg-permission-store; niri: same minus secrets/oo7-portal); harness per-DE lists.
- **Validated**: builds/vet/gofmt/golangci-lint green; niriWayle dinit `2026-10-17T02-38-31Z` **99/99** (`keyrack_flatpak` green: "Key Rack flatpak installed + Secret=oo7-portal pinned"; `user_session_env` green; seahorse conflict gone; portal-order runtime green); niri full dinit `2026-10-17T02-55-48Z` **99/99**; niri OpenRC `2026-10-17T02-44-05Z` **83/83** (`user_session_env` green under OpenRC — both-init env chain proven) — zero fail/skip all three (one transient `niri_outputs_count` IPC hiccup at `02-49-33Z` re-run clean). aports CI `2026-10-17T02:30:03Z` (run 38995152494) full green with skip logic + `xdg-desktop-portal-user-dinit-0.1.0-r1` on Pages.

### 2026-10-16 - niriWayle: second desktop environment (niri + Wayle shell + Veila + Raffi + oo7) — harness 97/97
**Agent:** codebase
**Summary:** New `desktopEnvironment: niriWayle` — the niri compositor with the Wayle shell (bar/notifications/OSD/wallpaper built-in), Veila lock screen, Raffi launcher (native iced) and oo7 secrets (gnome-keyring replacement). Shares with the niri DE: turnstile + per-user dinit session, pipewire stack, portals, polkit agent, wayvnc, greetd+regreet greeter, elogind, matugen.
- **aports** (`49fa80d` + `f040ebc` + `6934607`, CI green): wayle-0.7.0, veila-0.4.4, raffi-0.24.0, wl-clipboard-rs-0.9.4 (Rust wl-copy/wl-paste — niri author YaLTeR), upower-dinit, wayle-session-0.1.0-r2 (session script + wayle.user/veila.user + niriwayle.desktop; depends niri-user-dinit for login-shell + niri user service).
- **Session gotchas (live-caught)**: (1) `wayle panel start` daemonizes+exits → dinit restart loop, started marker never held → `wayle shell` (foreground); (2) user-dinit services start before the compositor env exists → GTK init panic → `wayle-shell-session` wrapper resolves WAYLAND_DISPLAY/NIRI_SOCKET (mirrors wayvnc-session).
- **installer**: `desktops/niri_wayle.go` (niri.go derivation minus DMS: no settings/plugin/session seeds, no dms includes, own binds block from upstream default-config minus the post-25.11 `toggle-touchpad` action — live-caught invalid in niri 25.11), `users.go` isNiri → niri|niriWayle (login-shell trick applies), gnome-keyring → oo7 (daemon + PAM + portal + CLI from Alpine; login-unlock wiring = follow-up), stasis deferred to v2 (veila daemon idle covers lock-on-idle + before-sleep).
- **harness**: checks split niri-family vs niri-only(DMS) vs niriWayle-only (wayle_running, veila_daemon, raffi_installed, oo7_secrets); per-DE user-service lists; enableNiriTestAutologin DE-aware session command (wayle-session).
- **Validated**: niriWayle dinit `2026-10-15T22-40-19Z` **97/97** (wayle shell + veilad + wayle-bar layer surface + full user dinit stack + wayvnc + screenshot); niri regression `2026-10-15T20-01-45Z` **98/98**.
- **Follow-ups**: oo7 login-unlock PAM wiring (greetd stack); stasis v2 (dpms/suspend plans + media/gamepad); Wayle power-module → zz-power equivalent (Wayle power config key unknown, sudoers rule ready).

### 2026-10-14 - openrc-always: OpenRC enablement never conditional (dinit installs stay OpenRC-bootable) — dinit 98/98, OpenRC 82/82
**Agent:** codebase
**Summary:** User review correction: "OpenRC is never conditional — it gets enabled always". A dinit install must remain bootable under OpenRC (fallback), so every OpenRC enablement now runs regardless of the active init; the dinit-side enablement stays conditional (active-init only). The rc-update links are inert under dinit.
- **services.go**: the rc-update runlevel loop + the swap-ordering sed run ALWAYS (was gated `!EnableDinit`); **niri.go** step 12: `rc-update add turnstiled default` unconditional; **podman.go**/**nix.go**/**hardening.go**: rc-update (podman/cgroups, nix-daemon, apparmor) unconditional; **packages.go**: `klogd-openrc` installed on BOTH inits (vector-compatible klogd for the fallback); **logging.go**: vector-setup comment updated (script already written unconditionally — now enabled always).
- **Harness**: new `openrc_fallback_ready` check (dinit installs): asserts the runlevel links (turnstiled/greetd/vector-setup/klogd) + installer-written OpenRC services exist while dinit is PID1.
- **Validated**: dinit full `2026-10-14T21-53-12Z` **98/98** (51 rc-update adds under dinit, klogd-openrc installed, conversion 0 missing/0 failed, log_errors clean); OpenRC niri `2026-10-14T21-59-42Z` **82/82** regression.

### 2026-10-14 - turnstile: OpenRC user-session stack (dual-init) + rules cleanup validated — dinit 97/97, OpenRC 82/82
**Agent:** codebase
**Summary:** The niri `-user-dinit` session services now run under BOTH inits: turnstile is service-manager-agnostic (only its backend is dinit), so OpenRC gets the identical per-user dinit instance — the DMS/wayvnc/pipewire/polkit-agent session stack is no longer dinit-only.
- **aports turnstile** (`42fac31` + `a963556`, CI green): restored Alpine's `turnstiled.initd` + `turnstiled.confd` in the `turnstile-openrc` subpackage (dropped as "no OpenRC on FeralOS" — obsolete) + boot-order fix `depend(): need elogind; before greetd`.
- **Root cause (debug_run on failed run `2026-10-14T00-24-08Z`)**: boot-order race — greetd's autologin session opened before turnstiled's control socket listened; `pam_turnstile.so` is optional/silent → session never registered → no user dinit (8 failed checks). Proof: greetd restart after the socket is up registers the session + spawns the user dinit. Fix = `before greetd` (dinit parity: `before: login.target`).
- **Installer/harness**: `niri.go` step 12 init-conditional (`rc-update add turnstiled default`); `packages.go` conversion skip += `turnstile` (dinit service ships in the MAIN package — the auto-pulled `turnstile-openrc` no longer triggers a failing `apk add turnstile-dinit` under dinit); `turnstile_pkg` init-conditional + busybox `pgrep -x` false-negative fixed (`pgrep -f '/usr/bin/turnstiled'`); 5 session-stack checks un-gated to both inits.
- **Rules cleanup (user) + restore**: redundant `-openrc`/`-dinit` entries dropped (Alpine `install_if` auto-pull + dinit conversion cover them); `wireless-regdb` restored (accidental removal — `nm_wifi_support` caught it, run `2026-10-13T23-44-16Z` 96/97).
- **Validated**: dinit full `2026-10-14T18-44-11Z` **97/97** (`turnstile OK (dinit)`, conversion `0 missing, 0 failed`); OpenRC niri `2026-10-14T18-50-50Z` **82/82** (`turnstile OK (openrc)`; all 8 former OpenRC fails resolved). Builds/vet/gofmt/lint/tests green. NOTE: `features/podman.go`/`nix.go` still list `podman-openrc`/`nix-openrc` explicitly (redundant, harmless).

### 2026-10-13 - matugen: packaged in feralos-aports + wired into the niri stack (DMS dynamic theming backend)
**Agent:** codebase
**Summary:** matugen — the wallpaper→palette backend `dms-shell` runs for dynamic theming (its generated `DankMatugen` KColorScheme export is what the dms-theme-sync plugin propagates to GTK/Qt/KDE) — is absent from Alpine entirely, so it now ships from OUR repo and is wired into the niri package list.
- **Package** (feralos-aports `ad34f4d`, CI run `38322829277` all-green + published): matugen 4.2.0, GPL-2.0-or-later, FIRST Rust/cargo package in the repo (Alpine `bottom` pattern: `cargo fetch --target="$CTARGET" --locked` + `build`/`test --frozen --release`; upstream suite 6/6 in `check()`). Docker rehearsal: sha512-pinned source, signed 4.8 MB apk, signature-verified install, `matugen --version` = 4.2.0. CI `client-test` now also asserts the matugen binary from Pages (`build.yml`).
- **Installer wiring** (`eb20a4d`): `internal_rules.yaml` niri packages += `matugen` + comment rewrite (old "matugen not on Alpine" note obsolete); new harness check `matugen_installed` (package present + binary runs); `docs/guides/niri.md` theming paragraph updated.
- **Harness**: run `2026-10-12T23-58-32Z` (fixture `btrfs_standard_niri_full_test.yaml` — dinit + edge + secureboot/tpm full stack) **97/97 PASS**, zero failures/skips (96→97); `matugen_installed` = "matugen OK (matugen 4.2.0)".
- **Open (next)**: live-session proof that KDE apps go dark end-to-end (DMS Auto/dynamic theme → matugen palette → store-installed dms-theme-sync plugin → kdeglobals) — a debug_run round; plugin stays user-installed from the DMS store (user decision). Optional: upstream reports for the plugin (light-mode GTK `-dark-` substitution bug; BusyBox find `-printf`/`-regextype`).

### 2026-10-13 - theme: ALL manual theme seeds removed (dms-theme-sync plugin round, Phase A)
**Agent:** codebase
**Summary:** User decision: adopt third-party DMS plugin `arqueon/dms-theme-sync` (GPL-3.0) as the theming authority — it propagates the DMS appearance (mode/colors/icons) to GTK, Qt, KDE, Flatpak. To avoid conflicts, ALL manual theme seeds removed from `niri.go` in the same install: kdeglobals (BreezeDark [Colors:*] + [Icons] breeze-dark + [UiSettings] BreezeDark pin), gtk-3.0/4.0 settings.ini, flatpak user-global override (`overrides/global` + its chown), dconf system default `/etc/dconf/db/local.d/00-feralos-theme`.
- **KEPT**: dconf package + `/etc/dconf/profile/user` + `dconf update` (gsettings INFRASTRUCTURE — without the profile GLib ignores user/system dbs; not a theme override), `session.json` dark seed (DMS session state), XDG env `QT_QPA_PLATFORMTHEME=xdgdesktopportal`. chown batch trimmed to xdg-terminals.list + mimeapps.list. NOTE comment in niri.go documents plugin ownership.
- **Harness**: `theme_dark` check REMOVED (97→96 checks expected; plugin-based check re-added after adoption validation); `dms_icons` trimmed to package + on-disk asserts (settings.ini assert dropped — plugin-owned now).
- **First-run catch (fixed)**: `dconf update` FAILED ("Error opening directory /etc/dconf/db") — Alpine's dconf ships NO `/etc/dconf` at all; the removed `00-feralos-theme` seed write was implicitly creating the dir. Installer now `mkdir -p /etc/dconf/db/local.d` before `dconf update`. Caught by harness run `2026-10-12T22-09-52Z` (95/96, log_errors).
- **Committed**: `899f28b` (develop). Phase B (pending): live plugin test on the fresh harness VM.

### 2026-10-12 20:00 - theme: flatpak overrides + hotkey overlay skip; KDE dark confirmed visually
**Agent:** codebase
**Summary:** User-reported ksystemlog still light (fresh install): live visual validation (screenshots, pixel-checked) on run `2026-10-12T18-52-07Z` shows BOTH okular AND ksystemlog fully DARK (BreezeDark View bg RGB 36,36,36) — host-side theme chain is complete. Remaining gap = FLATPAK apps: the sandbox doesn't see host config (user's app likely a flatpak).
- **Installer**: flatpak user-global override `~/.local/share/flatpak/overrides/global` — `filesystems=xdg-config/kdeglobals:ro;xdg-config/gtk-3.0:ro;xdg-config/gtk-4.0:ro;` + `GTK_THEME=Adwaita:dark` (KDE flatpaks read the bound kdeglobals incl. BreezeDark pin; GTK3 flatpaks get the dark theme env; GTK4/libadwaita + portal-aware flatpaks already follow the portal).
- **UX catch (found in validation)**: the "Important Hotkeys" niri overlay popped on EVERY login — user config lacked `hotkey-overlay { skip-at-startup }` (greeter config had it). Added.
- **Harness**: theme_dark += flatpak override asserts; niri_config_valid += skip-at-startup assert.
- **Corrections (user review, same day)**: (a) hotkey-overlay `skip-at-startup` REVERTED in the user niri config — the "Important Hotkeys" cheatsheet at login is WANTED (user request); do not touch (greeter config untouched); (b) flatpak override = **binds only** — `[Environment] GTK_THEME=Adwaita:dark` removed (user: apps must FOLLOW the theme chosen in DMS, not be force-themed); (c) new step 14 final `chown -R user:user ~/.config ~/.local` sweep — user-reported root-owned seeded dirs (per-seed chowns cover files; the mkdir-created dirs stayed root because the general chown runs before the seeds).

### 2026-10-12 18:30 - theme: KDE apps dark via kdeglobals (okular/ksystemlog were light)
**Agent:** codebase
**Summary:** User-reported: KDE apps (okular, ksystemlog, kcron...) stayed light. Root cause (KF6 source-verified): KDE apps read colors via KColorScheme → `~/.config/kdeglobals` `[Colors:*]` — NOT the portal/QPalette path (that part was already dark via xdgdesktopportal + portal prefer-dark; Qt 6.11 fusion palette is dark). Manager-instantiation apps (okular) look for `.colors` scheme FILES in XDG data dirs; non-manager consumers (ksystemlog) read kdeglobals `[Colors:*]` directly with hardcoded Breeze Light defaults when absent.
- **Fix (installer, no new packages)**: seed `~/.config/kdeglobals` with the self-contained BreezeDark `[Colors:*]` + `[ColorEffects:*]` values (upstream BreezeDark.colors, LGPL-2.0-or-later; translations dropped) + `[Icons] Theme=breeze-dark`. Alpine's `breeze` package (only repo carrier of the .colors files) would pull qt5+kf5 stacks (~100MB) — rejected.
- **Harness**: `theme_dark` extended (kdeglobals + Colors:View + breeze-dark asserts).
- **Follow-up (user report: okular/ksystemlog still light)**: (1) `kcolorscheme` EMBEDS BreezeDark/BreezeLight as Qt resources — manager apps (okular) BYPASS `kdeglobals [Colors:*]` and, unpinned, follow `styleHints colorScheme` which is not dark in their process → embedded LIGHT. Fix: `[UiSettings] ColorScheme=BreezeDark` in the seed (manager resolves by id from the embedded resource; no .colors file needed). Consequence: pinned apps stop following portal toggles (documented). (2) `greeter` user hidden from ReGreet: 0.2.0 lists `/etc/passwd` users with UID in [1000, 60000] — greeter was UID 1001; now created with `adduser -S` (system UID, filtered out; no AccountsService path in 0.2.0). Harness: greetd_config += greeter-UID assert; theme_dark += BreezeDark-pin assert.
- **First-run catches (2, both fixed)**: (a) busybox `adduser -S` WITHOUT `-G` sets the primary group to `nogroup` and creates NO same-name group (source `loginutils/adduser.c: usegroup = "nogroup"`) → the later `chown -R greeter:greeter` failed (log_errors) — fixed with explicit `getent group … || addgroup -S greeter` + `adduser -S -G greeter`. (b) harness `greetd_config` first version used the POSIX `'\''` apostrophe escape INSIDE the command — the ssh command runs through the user's FISH login shell, whose single-quote escaping differs: `fish: Unexpected ')'` (exit 127). Fixed with a double-quoted echo (plain apostrophe, no backslash-nesting — same shape as theme_dark's `'prefer-dark'`). Verified in alpine+fish docker: new R parses, old R reproduces the exact error.
- **Validated**: first harness run `2026-10-12T18-42-54Z` (pre-final) 95/97 — greetd_config fish-quoting FAIL + log_errors chown FAIL; both fixed (see catches above); re-run `2026-10-12T18-52-07Z` **97/97 PASS** — greetd_config green (`greeter is a system user (hidden from ReGreet's UID-filtered list)`) + theme_dark green (`... + BreezeDark pin`), 0 failed/skipped.

### 2026-10-12 04:00 - fix: DMS reboot via custom power command (no elogind rebuild)
**Agent:** codebase
**Summary:** Live diagnosis (debug_run of run 2026-10-11T20-38-40Z): DMS `loginctl reboot` → elogind execs the ABSOLUTE `/sbin/reboot` (busybox) AFTER the hooks → busybox sends SIGTERM (its reboot signal; poweroff=SIGUSR2, ignored by dinit) → dinit's sigterm handler calls `stop_all_services(HALT)` OVERWRITING the REBOOT type → "System halted", machine frozen. Terminal `reboot` is unaffected (PATH → our /usr/bin wrapper). Upstream NOT fixed (dinit master identical; upgrading useless). Artix rebuilds elogind with `-Dreboot-path=/usr/bin/reboot` — we chose the lighter route per user decision.
- **Fix**: DMS `customPowerActionReboot = "sudo -n /usr/bin/reboot"` (SessionService 1.6.2 runs custom power commands via `sh -c` as the session user; verified source) + `/etc/sudoers.d/zz-power` (`%wheel ALL=(root) NOPASSWD: /usr/bin/reboot`) — both dinit-only. Poweroff/polkit/hook paths unchanged.
- **Live catch**: first debug_run attempt FAILED (`sudo: interactive authentication is required` despite `sudo -l` listing the NOPASSWD rule): sudo-rs applies the LAST matching rule and `/etc/sudoers.d/power` sorts BEFORE `/etc/sudoers.d/wheel` (`%wheel ALL=(ALL) ALL`, password) → the wheel rule overrode the NOPASSWD tag. Renamed to `zz-power` (sorts after wheel) — verified live: `sudo -n /usr/bin/reboot` as the session user → `reboot: Restarting system`, SSH returned ~20s, uptime fresh, 0 LUKS prompts (TPM).
- **Harness**: dinit_shutdown_commands extended (sudoers.d/zz-power + customPowerActionReboot seed).
- **Validated**: harness run `2026-10-11T23-06-13Z` **97/97 PASS** (dinit_shutdown_commands green with the zz-power assertions); live debug_run re-test of the exact command (`su - skea999 -s /bin/sh -c "sudo -n /usr/bin/reboot"`): "Issuing shutdown command..." → serial `reboot: Restarting system` + `reboot: machine restart`, SSH back in ~15s, fresh uptime, TPM auto-unlock.
- **Note**: CLI `loginctl reboot` remains broken (elogind path) — acceptable; elogind rebuild parked in backlog if it ever matters.

### 2026-10-12 03:00 - theme: system-wide dark preference (portal color-scheme fixed)
**Agent:** codebase
**Summary:** User-reported: some apps ignored the dark theme (light UI). Diagnosed LIVE via debug_run: apps following the xdg-desktop-portal color-scheme were light because Portals reported `v u 0` (no preference) — dconf was ABSENT and `org.gnome.desktop.interface color-scheme` was never set (`gsettings get` = 'default'). Also: `QT_QPA_PLATFORMTHEME=gtk3` was DEAD (Alpine ships NO QGtk3 platformtheme plugin — only `libqxdgdesktopportal.so`), and gtk-4.0 settings were never seeded.
- **Fix (installer)**: niri packages += `dconf`; system dconf default `/etc/dconf/db/local.d/00-feralos-theme` (`color-scheme='prefer-dark'`) + `dconf update` (compiles the system db without a session bus; every user inherits, user values still win) → libadwaita/GTK4 apps (gnome-maps/calendar), Firefox, Qt can all go dark; `gtk-4.0/settings.ini` seed added; niri env `QT_QPA_PLATFORMTHEME=xdgdesktopportal` (only Qt platformtheme plugin present — follows the portal).
- **Second live finding (first harness run)**: Alpine's `dconf` package ships NO `/etc/dconf/profile/user` (Debian ships it) → GLib never consulted the system db, `gsettings get` = `'default'`, `theme_dark` FAILED. Installer now writes the standard GNOME profile (`user-db:user` + `system-db:local`) before `dconf update`; re-verified live (profile written → `'prefer-dark'`).
- **Harness**: `theme_dark` check (dconf default file + GTK4 seed + effective `gsettings get` value as the user).
- **Validated**: harness run `2026-10-11T20-38-40Z` (niri full fixture) **97/97 PASS** — `theme_dark` green (`dark preference OK (dconf system db + gtk4 seed)`), 0 failed/skipped. First run `20-31-51Z` = 96/97 (the missing-profile catch).

### 2026-10-12 02:00 - niri: prefer-no-csd in the user config (frameless, Windows-like)
**Agent:** codebase
**Summary:** User request: also remove app titlebars (CSD). Research: the only lever is niri's GLOBAL `prefer-no-csd` (top-level config; no per-window toggle, no DMS setting — DMS's own embedded niri config ships it ACTIVE by default, but FeralOS writes its own niri config via the installer).
- **Installer** (`niri.go` user config): `prefer-no-csd` added top-level + comment. Greeter config untouched. Window controls after the change: DMS bar widget (dms-window-buttons, same-day round) + keybinds Mod+Q/F/Shift+F + runningApps context menu.
- **Caveats** (niri docs): floating windows lose resize handles (Mod+right-click resize); apps must be restarted; apps may still force their own CSD.
- **Validated**: harness run (niri_config_valid = `niri validate` on the new config) — see run dir in that round.
- **Follow-up (CSD per-app reality, researched)**: titlebars after prefer-no-csd: (a) GTK3/GTK4 windows WITHOUT a custom titlebar are frameless ✓; (b) **libadwaita/GNOME apps (gnome-maps/calendar/eog) and Firefox KEEP their header — it is app UI, the client requests CSD and niri HONORS it by design** (no compositor can strip it; `GTK_CSD=0` is dead code on Wayland — GTK source exits before reading it); (c) Qt drops decorations via xdg-decoration negotiation + now the hard override `QT_WAYLAND_DISABLE_WINDOWDECORATION=1` (niri env; numeric "1" required — "true" parses to 0 in Qt). Harness niri_config_valid extended (prefer-no-csd + Qt env asserts).

### 2026-10-12 01:00 - dms: window-buttons plugin wired system-wide (bar back on top)
**Agent:** codebase
**Summary:** User decisions: keep runningApps, put the plugin on the RIGHT, bar back on TOP; plugin developed in a NEW repo then imported to aports.
- **Plugin repo** `github.com/skea999/dms-window-buttons` (NEW, ISC): `plugin.json` + `WindowButtons.qml` (PluginComponent; maximize-to-edges + fullscreen + close for the focused window via NiriService IPC / `toplevel.close()`; hidden with no focused window; minimize intentionally absent — niri has none, DMS hides it too). Tag `v1.0.0` (`6af94ee`).
- **aports** `dms-window-buttons` 1.0.0 (`b8604c1`, CI green): installs to `/etc/xdg/quickshell/dms-plugins/dmsWindowButtons/` (DMS system plugin dir — no per-user seed needed for the files).
- **Installer** (`niri.go`): settings.json seed — `dankBarAtBottom` REMOVED (v1→v2 migration default = Top), `dankBarRightWidgets` pinned (defaults + `dmsWindowButtons` at the end); NEW `plugin_settings.json` seed `{"dmsWindowButtons":{"enabled":true}}` (widget plugins default-disabled; file shape verified against 1.6.2 `parsePluginSettings` — bare map, no wrapper). `internal_rules`: niri packages += `dms-window-buttons`.
- **Harness**: `dms_window_buttons` check (plugin files + enabled + right-widget wiring).
- **v1.0.1 (same day user report)**: v1.0.0 buttons 1-2 silently did nothing — they used `NiriService.send` behind a `Quickshell.env("NIRI_SOCKET")` guard, but DMS has NO NIRI_SOCKET in its env (starts from the user dinit, not as a niri child; only close worked = native `toplevel.close()`). Rework: buttons = ↓ `move-column-to-workspace-down` · ↑ `move-column-to-workspace-up` · ⛶ `maximize-column` (= Mod+F) · ✕ close; actions via `niri msg action` with self-resolved `$XDG_RUNTIME_DIR/niri*.sock`. Plugin v1.0.1 (`0bc7da7`) + aports bump (`1a80c51`, CI green).

### 2026-10-11 20:30 - update: feralos-update script wired to the DMS System Update button
**Agent:** codebase
**Summary:** User request: working update button in DMS. Research (DMS 1.6.2 = installed version, source-verified): the System Update tab supports a custom command — `updaterUseCustomCommand` + `updaterCustomCommand` run the command IN A TERMINAL (auto-detect alacritty ✓) as the session user with a sudo prompt hint; exit code surfaces as success/failure. DMS has NO native apk backend (pacman/dnf/apt/zypper/xbps/pkg/portage/flatpak only) → custom command is the right hook.
- **Script** (`misc.go` step 17 → `/usr/local/bin/feralos-update`): sequence apk (update + upgrade --available) → flatpak if installed (uninstall --unused user/system, update --user/--system) → nix if installed (nix-channel --update, nix profile upgrade '.*', nix-env --delete-generations +5, nix-collect-garbage; + root profile trim/GC if present). Best-effort error counter, exit code = errors (DMS reports failure).
- **DMS wiring** (niri settings.json seed): `updaterUseCustomCommand: true` + `updaterCustomCommand: "/usr/local/bin/feralos-update"`.
- **No harness check** (user decision): running the script in the VM hits the edge-package upgrade limitation (the niri stack comes from `extraPackagesEdge`, its repos are commented out after install — `apk upgrade` of those packages fails in-harness) → check dropped; validation = manual on real hardware.
- **Caveat**: DMS upgrade timeout = 30 min (server-side); very long nix updates could hit it — acceptable for now, upstream escalation if real cases appear.
- Also committed: system-config-printer dropped from niri packages (DMS has printer settings).
- **Fix (same round)**: the script write uses `ExecSilent` — the echoed script body landed in installer.log and tripped the harness `log_errors` sweep (its words "error"/"FAILED"/"failing" are legitimate script content; run `2026-10-11T17-45-21Z` exposed the false positive). Same rationale as firmware detection.

### 2026-10-11 18:30 - fix: DMS power off/reboot dead under dinit (elogind → busybox SIGUSR2)
**Agent:** codebase
**Summary:** User-reported: DMS "Power off"/"Reboot" did nothing. Research (DMS source + elogind source): DMS runs `loginctl poweroff|reboot` (elogind branch) → elogind BYPASSES PATH and execs the absolute `/sbin/poweroff` (busybox symlink) AFTER running `/etc/elogind/system-shutdown` hooks — busybox sends SIGUSR2, which dinit IGNORES (no systemd; no config override for the binary path in elogind — build-time only). Our /usr/bin/dinit-shutdown wrappers are never consulted.
- **Fix** (`services.go` `setupDinitShutdownCommands`): installer writes `/etc/elogind/system-shutdown/50-feralos-dinit.sh` — elogind's documented hook point, running BEFORE its busybox exec: `exec /usr/sbin/dinit-{poweroff,reboot,halt}` per `$1` verb (machine tears down; the subsequent busybox exec is a harmless no-op either way).
- **Harness**: `dinit_shutdown_commands` extended (hook executable + contains dinit-poweroff).
- **Second finding (same round, bundled)**: under dinit elogind cannot map PIDs to sessions (`NoSessionForPID` — no session cgroups) → polkit never sees allow_active → every login1 power action = auth challenge (silent timeout without an agent). Fix: `/etc/polkit-1/rules.d/49-feralos-power.rules` — wheel → login1 power-off/reboot/halt YES (wheel = admin already; standard desktop posture, no escalation). Harness check extended (rule file + content).
- Real-fix chain: DMS → loginctl → polkit (allow_active, in-session) → elogind → hook → dinit shutdown.

### 2026-10-11 16:30 - nix: flake registry pinned to stable 26.05 system-wide (unstable leak fixed)
**Agent:** codebase
**Summary:** User-reported: nix resolved the unstable branch. Root cause: the stable CHANNEL (nixos-26.05) was already added, but the FLAKE REGISTRY pin lived INSIDE `if len(u.NixPackages) > 0` — enableNix without nixPackages (current default: niri rules carry no nixPackages, harness fixtures neither) left the builtin default (nixpkgs → nixpkgs-unstable) active for every `nixpkgs#...` reference. Root also never pinned (user registry only, via su).
- **Fix** (`nix.go`): system-wide `/etc/nix/registry.json` written (nixpkgs → github:NixOS/nixpkgs/nixos-26.05 — applies to every user incl. root) + user registry pin unconditional (belt+braces, user wins over system). Runs right after `nix-channel --update`, outside the nixPackages branch.
- **Harness**: new `nix_registry_stable` check (cfg.EnableNix block): system registry file contains nixos-26.05 AND `nix registry list` as the user resolves nixos-26.05.

### 2026-10-11 15:00 - fix: pkexec refused the FeralOS login-shell (DMS user creation dead)
**Agent:** codebase
**Summary:** User-reported: DMS "Create user" said it couldn't find /etc/shells. Diagnosed LIVE via debug_run of run `2026-10-11T12-18-06Z`: pkexec validates the session user's passwd shell against `/etc/shells` and refuses RC=127 ("The value for the SHELL variable was not found in the /etc/shells file. This incident has been reported.") — the FeralOS login-shell path (`/usr/libexec/feralos/login-shell`) was deliberately NOT listed (the old upstream niri-session bash re-exec trap). So EVERY pkexec in a niri session failed: all DMS privileged ops (user management = `pkexec useradd/chpasswd/usermod`), not just Create User — global session bug, backend healthy (root useradd RC=0, shadow installed, bash present).
- **Trap is gone**: the FeralOS niri-session (niri-user-dinit, replaces="niri") has ZERO /etc/shells/bash re-exec logic (grep-verified) — only upstream Alpine's script did.
- **Fix** (`users.go`): installer lists `/usr/libexec/feralos/login-shell` in `/etc/shells` (idempotent grep -qxF || echo) right after the symlink re-point to fish. Comment documents the backward risk (reverting to upstream niri-session re-arms the trap).
- **Harness**: `login_shell_trick` extended — also asserts the /etc/shells listing (falsifiable).
- UsersService facts: `pkexec useradd -m -s /bin/bash` + `chpasswd` via `head -n1 | chpasswd`; isBSD branch never fires on Linux.
- **VALIDATED**: harness run `2026-10-11T14-56-07Z` **95/95** — `login_shell_trick` (extended /etc/shells assertion) green, full niri stack healthy.

### 2026-10-11 03:30 - niri: DMS theme seeded green in the installer
**Agent:** codebase
**Summary:** User request: always pre-set DMS's green theme on niri installs. Researched from DMS docs + source (GitHub): settings live in `~/.config/DankMaterialShell/settings.json`; "green" = a Generic static theme (ThemeColorsTab grid), stored as `currentThemeCategory: "generic"` + `currentThemeName: "green"` (SettingsSpec.js defaults; generic = static Material palette, matugen/wallpaper only in dynamic mode — user switching to Auto/custom later overrides, by design).
- **Installer** (`niri.go`, after the `dms setup` fragments): seeds `~/.config/DankMaterialShell/settings.json` with the 2 keys via printf-escaped JSON (file doesn't exist before DMS's first launch — pre-seeding = DMS loads and merges on startup) + per-file chown (the general chown -R runs earlier — GOTCHA avoided: without per-file chown the seed would be root-owned and DMS could never save settings).
- **Dark mode seed (same round, user request)**: `~/.local/state/DankMaterialShell/session.json` seeded `{"isLightMode": false}` + chown — belt+braces: fresh installs already default dark (SharedSessionSpec.js `isLightMode` def=false; PortalService applies mode changes only after the first read, never at startup), but the explicit seed guarantees dark portal-independently (session store = XDG state dir, NOT config dir).
- **Bar at the bottom (same round, user request)**: the seed uses DMS's own v1→v2 settings MIGRATION — `configVersion: 1` + legacy `dankBarAtBottom: true` make DMS build the FULL default `barConfigs` with `position: 1` (Bottom) on first launch. Why not seeding `barConfigs` directly: SettingsStore.parse assigns arrays RAW (no deep merge) — a partial barConfigs would wipe the default widget layout; duplicating the full default would drift with upstream DMS versions. DMS's migration path = zero default duplication, canonical output.
- **Running apps widget (same round)**: `dankBarLeftWidgets: ["launcherButton", "workspaceSwitcher", "runningApps"]` in the seed — the same v1→v2 migration applies it (fallback `|| defaults` per list in SettingsStore.js), swapping the focused-window widget for running apps (BarWidgetCatalog: `runningApps`, left section). Only the LEFT list is pinned; center/right keep following upstream defaults.
- **Default apps (same round)**: DMS Default Apps tab = freedesktop frontend — installer seeds `~/.config/xdg-terminals.list` (`Alacritty.desktop`, xdg-terminal-exec) + `~/.config/mimeapps.list` `[Default Applications]` (firefox → web mimes, `org.gnome.eog.desktop` → images, `com.system76.CosmicPlayer.desktop` → audio; MIME lists = DMS `DefaultAppsTab.mimeMapping` verbatim). Desktop IDs verified against Alpine pkgs contents (GOTCHA: eog is `org.gnome.eog.desktop`, NOT `eog.desktop`). Both chowned (general chown -R runs earlier). Plus (later user request): gnome-maps → `x-scheme-handler/geo`, gnome-calendar → `text/calendar` + `calendar/webcal(s)` schemes (`org.gnome.Maps.desktop`, `org.gnome.Calendar.desktop` — verified on Alpine contents); packages added to niri list too.
- **Icon themes (later user request)**: `adwaita-icon-theme` + `hicolor-icon-theme` now `dms-shell` deps (aports `14f124b`, 1.6.2-r3 — discovered missing on the live target: breeze/hicolor/Cosmic present only transitively, adwaita absent; DMS resolves icons via QT_QPA_PLATFORMTHEME=gtk3 → `~/.config/gtk-3.0/settings.ini` → `gtk-icon-theme-name`, unset on our install = hicolor generics). Installer seeds the GTK settings.ini with `gtk-icon-theme-name=Adwaita` + `gtk-application-prefer-dark-theme=true` (GTK apps dark = coherent with DMS dark mode). Empirical: dms.log had ZERO icon errors in QEMU (breeze fallback) — user reported missing icons on real hw.
- **Harness `dms_icons` check (same round)**: niri-block, 3 assertions — adwaita-icon-theme + hicolor-icon-theme installed (dms-shell deps), `/usr/share/icons/Adwaita/index.theme` on disk, seeded `gtk-3.0/settings.ini` points DMS at Adwaita. Falsifiable end-to-end guard of the icon chain. VALIDATED: run `2026-10-11T14-56-07Z` — dms_icons green (adwaita+hicolor installed from dms-shell 1.6.2-r3 deps, Adwaita on disk, GTK seed check passed), 95/95 total.
- Hardcoded per user request (no variables.yaml option). Docs: `docs/guides/niri.md` (DMS fragments paragraph).
- **VALIDATED**: harness run `2026-10-11T12-18-06Z` **94/94** — all seeds written on target (settings.json/session.json/xdg-terminals.list/mimeapps.list, exit 0 + chown skea999), cosmic-screenshot absent from the install (edit+player present), DMS runtime green (layers rendered, IPC OK, valid screenshot PNG, wayvnc :5900). 0 ignored errors. Commit chain: green `50a1246`, dark `d37c496`, bottom bar `e534c5a`, runningApps `bc6e5a3`, default apps `4f216c0`, cosmic-screenshot drop `0c47b5a`.

### 2026-10-11 02:30 - session apps bound to the niri session (add-wants model) — greeter class of bugs eliminated
**Agent:** codebase
**Summary:** User asked for the documented upstream model before fixing the greeter-DMS bug class permanently. Research: niri docs (session apps = `niri.service.wants`, "start when the niri session begins and terminate when the session exits"), DMS docs (`add-wants niri.service` — "won't run in other sessions"; explicit warning that user-manager enable starts DMS in ALL sessions = our exact bug), turnstile README (user manager = per-login spawn/last-logout teardown). Conclusion: boot.d = login scope, session apps must = compositor-session scope.
- **aports `niri-user-dinit` r3 (`9838d81`, CI green)**: `niri-session` starts `polkit-agent.user dms.user wayvnc.user` existence-filtered right after `dinitctl --user start niri` (before the dinit-monitor hold). Real logins only — greeter.sh never runs niri-session. The dms-session greeter guard (`0004567`) stays as belt+braces.
- **Installer** (`niri.go` step 13): boot.d list = infrastructure only (`dbus.user pipewire.user pipewire-pulse.user wireplumber.user wait-wayland.user`); session apps dropped with model explanation comment.
- **Harness** (`user_services_enabled`): now checks 5 links PRESENT in boot.d AND 3 session apps ABSENT; `niri_user_dinit_runtime` unchanged (stack started = `[[+]]` markers via explicit dinitctl). `dinit_user_services` unchanged (files exist).
- **Lifetime semantics match upstream**: start = when niri-session runs (real login); stop = session end → turnstile tears down user dinit at last logout.
- **Docs**: `docs/guides/niri.md` (turnstile paragraph + session services list). dms-shell split (`dms` + `dms-dinit`) still PENDING (user parked it). **2026-10-18 user decision: DROPPED — not planned, do not pursue.**
- **VALIDATED**: harness run `2026-10-11T02-30-17Z` **94/94** — niri-user-dinit r3 + dms-shell 1.6.2-r2 installed from Pages, `user_services_enabled` = 5 infra links present + 3 session apps absent, full user session stack running via niri-session (dms IPC + wayvnc :5900 + layers rendered + screenshot PNG). 0 ignored errors.

### 2026-10-11 01:30 - aports: DMS no longer starts on the greeter session (dms-shell guard)
**Agent:** codebase
**Summary:** User-reported: DMS started on the greetd/regreet login screen. Root cause: the installer links `dms.user` into system-wide `/usr/lib/dinit.d/user/boot.d/`, so the greeter user's own user-dinit (turnstile spawns one for every session, greeter included) started the full desktop stack; the `wait-wayland.user` gate opens because the greeter's niri (greeter.sh `exec niri -- regreet`) creates the socket.
- **Fix (aports `dms-shell`, commit `0004567`, pkgrel 1→2, sha512sums regenerated)**: `dms-session` guard — clean `exit 0` when `id -un = greeter` (fixed installer-created username, reliable) or `XDG_SESSION_CLASS = greeter` (belt+braces, env may not reach user dinit). `dms.user` dropped `restart = true` (crucial: restart=true would loop on the clean guard exit; smooth-recovery kept so real crashes still recover — ADR-009 style).
- **Scope note**: wayvnc/polkit-agent left as-is (wayvnc on greeter = remote unlock view, potentially useful; polkit-agent harmless).
- **Validation**: `sh -n` OK, guard logic tested locally (rc=0 as normal user, GUARD_HITS with class=greeter), aports CI green. Greeter UI NOT harness-checkable (harness autologin bypasses regreet) — final check is manual boot/real HW: login screen must show only regreet, no DMS bar.

### 2026-10-11 00:30 - config: edge-repos auto-enable + wizard on invalid config (2 UX fixes)
**Agent:** codebase
**Summary:** User-reported: `extraPackagesEdge` without `enableEdgeRepos: true` hard-errored BEFORE the wizard — user could not fix the value interactively.
- **Fix 1 (auto-enable)**: `applyImpliedFlags(conf)` in `LoadConfig` (right after YAML unmarshal) — non-empty `extraPackagesEdge` ⇒ `enableEdgeRepos = true` + `[INFO]` (single source: the package list implies the flag, same philosophy as Nix auto-enable). Hard error REMOVED from `validateConfig`.
- **Fix 2 (wizard pre-filled on invalid config)**: `LoadConfig` now returns `(conf, err)` on VALIDATION errors (yaml-parse/rules errors still `nil` conf → fatal). `main.go` restructured: not-exist → defaults+wizard (unchanged); loaded-but-invalid → `[WARN]` + wizard pre-filled; after wizard `config.ValidateConfig(conf)` re-checks — still invalid (e.g. unattended run) → fatal BEFORE any disk work.
- **Tests**: new `src/utils/config/config_test.go` — `TestApplyImpliedFlagsEdgeRepos` (5 subtests) + `TestValidateConfigEdgePackagesNoRepoFlagRequired`. Green.
- **Lint**: one govet shadow fix in main.go (mechanical, `rulesErr`).
- **Validated**: installer + createValues build, vet/gofmt/golangci-lint clean, `go test ./...` green.
- **Docs**: `docs/guides/variables.md` (edge rows + Compilation wizard-behavior note).

### 2026-10-10 23:30 - branches: developDinit → develop → main merged; work continues on develop
**Agent:** codebase
**Summary:** User-requested merge chain. developDinit → develop = **fast-forward** (`b50e04f` → `c3ef811`). develop → main = **merge commit `8568df0`** with 9 conflicts resolved — develop side everywhere.
- **Conflict cause**: main tip carried 2 dinit reverts (`148ef31` installer integration, `2393c6a` harness checks) + net-zero resolv pair (`73f9066` + its revert `cdee03a`). develop (146 ahead) carries the complete validated dinit work → resolution = reverts cancelled intentionally (take-develop on all 8 content conflicts + keep `btrfs_standard_dinit.yaml` on the modify/delete).
- **main now = develop content**: bootstrap URL fix (`51269f5` content) ON main → the pending cherry-pick item is RESOLVED by this merge; dinit integration + harness checks live on main; setup.sh repoUrl / README wget / `module feralos/src` all verified on main.
- **Note**: main `variables.yaml` template now defaults `enablePodman: true` + `enableDinit: true` (develop's tested template) — no-backward-compat rule, single-run installer.
- All 3 branches pushed, SHA match local == github. Work continues on **develop** (clean tree).
- Pre-commit govulncheck warnings = pre-existing stdlib findings (Go 1.22.2), non-blocking.

### 2026-10-10 22:45 - validation: tests + harness dinit/niri 94/94 + debug_run live; debug_run QEMU flag fix
**Agent:** codebase
**Summary:** Post-rename validation round (user request: memory recall → go test → harness dinit+niri → debug_run).
- **Tests**: `go test ./...` green in src (config + swap, 25 subtests), harness (service pkg), debug_run (no tests).
- **Harness** (fixture `btrfs_standard_niri_full_test.yaml`): run `2026-10-10T22-11-11Z` **94/94 PASS** — 0 ignored/hard errors, dinit stack + niri rendering + screenshot PNG + wayvnc :5900 + DMS + bluetooth org.bluez + NM wifi + TPM auto-unlock all green. INFRA FIXES needed (folder-rename fallout, host-side): runner qcow2 stale backing path (rebase -u to `~/Progetti/feralos/...`), runner disk 3G→8G (`prepare-runner -force`), `cloud-image-utils` (cloud-localds) reinstalled.
- **debug_run**: live boot of the run = 8/8 checks PASS (PID1 dinit, boot chain, user dinit niri/dms/wayvnc/pipewire started, greetd socket, less shim `-R` OK, wifi enabled).
- **BUG found+fixed** (`tests/debug_run/main.go`): target `-device virtio-vga-gl,venus=on,blob=true,hostmem=1G` — apt QEMU 8.2.2 rejects `venus` → QEMU exit 1; harness dropped these flags in the 14:30 round but debug_run was missed. Fix `dccb79a`: plain `virtio-vga-gl` + comment updated; validated with a NATIVE boot (no wrapper): dinit PID1, niri alive, wayvnc :5900.
- **debug_run gotchas (no code change)**: guest poweroff from ssh user needs password sudo → use QMP `system_powerdown` on `d-qmp.sock`; wrapper/nohup parent may linger after qemu exit (kill manually); root ssh shell = fish (wrap globs in `sh -c`).
- Commits: `dccb79a` pushed to `developDinit` (SHA match).

### 2026-10-10 17:00 - src: Go module alpineInstaller → feralos/src (completes the rename)
**Agent:** codebase
**Summary:** User request: rename the src module too (it carried the older `alpineInstaller` name — never `skeaAlpineInstaller`, hence untouched by the 16:30 round).
- **Module + imports**: `src/go.mod` → `module feralos/src`; ~50 `.go` files' imports `"alpineInstaller/..."` → `"feralos/src/..."`; createValues Fatalf template-path hint updated. Pure rename: 121 insertions / 121 deletions.
- **Docs**: CONTEXT.md + `docs/intent/vocabulary.md` (`Go module`), ADR-001 (`go.mod:`), ADR-003 + `docs/architecture/isolation.md` (internal-rule import examples).
- **KEPT**: git branch name `alpineInstallerGolang` (docs/audit.md + AGENTS.md migration history) — it's a branch, not a module ref.
- **Validated**: 5 binaries build (installer, createValues, secureboot, tpm, webmin), go vet + gofmt + golangci-lint clean, `go test ./...` green (config + swap), grep zero `alpineInstaller` left in src/. Pre-commit govulncheck warnings = pre-existing stdlib findings (Go 1.22.2 toolchain), non-blocking.
- Commit `5d3a7a8` pushed to `developDinit` (SHA match local == github).

### 2026-10-10 16:30 - repo: Go modules + paths renamed skeaAlpineInstaller → feralos
**Agent:** codebase
**Summary:** Follow-up to the GitHub migration + folder rename (`~/Progetti/feralos`): in-repo `skeaAlpineInstaller` identifiers renamed to `feralos` for coherence.
- **Go modules** (2 of 3 — `src/go.mod` was already `alpineInstaller`): `tests/harness/go.mod` → `module feralos/tests/harness`, `tests/debug_run/go.mod` → `module feralos/tests/debug_run`; imports updated in `cmd/skea_harness/main.go`, `internal/mcp/server.go`, `internal/service/service.go`.
- **Guest path** (`service.go:716`): runner-VM repo root `/home/<user>/skeaAlpineInstaller` → `/home/<user>/feralos` (tar name derives from it — consistent). SAFE for existing runner images: the repo is copied at runtime (tar → extract), nothing baked in the image; old runs + debug_run unaffected (target VM paths independent).
- **Docs**: `docs/harness/{debug_run,development,workflow,building}.md` (module names, run-dir example, `~/feralos.tar.gz`), `docs/guides/getting_started.md` (BONUS FIX: curl/clone URLs pointed to wrong owner `skem/skeaAlpineInstaller` + nonexistent `latest-stable` path → now real `raw.githubusercontent.com/skea999/feralos/main/setup.sh` + `github.com/skea999/feralos.git`), `docs/guides/development.md` + AGENTS.md harness example paths → `~/Progetti/feralos`.
- **Historical occurrences KEPT** (correct as-is): AGENTS.md migration-round summary, `state/session_state.json` decision record, `.agent-memory` entry — they describe the OLD gitea repo name.
- Validated: zero remaining functional `skeaAlpineInstaller` occurrences (grep); harness + debug_run builds green (go build + go vet).

### 2026-10-10 15:45 - repo: hosting migrated Gitea → GitHub (skea999/feralos)
**Agent:** codebase
**Summary:** User request: repo hosting migration gitea.com/sketa999/skeaAlpineInstaller → github.com/skea999/feralos. GitHub importer had already copied refs; parity verified BEFORE switching.
- **Refs parity**: `git ls-remote` both sides — main/develop/alpineInstallerGolang/oldDevelop identical SHAs; ZERO tags on both sides (nothing to migrate); `developDinit` behind on GitHub by exactly the 2 unpushed local commits → fast-forward push (no force).
- **Remotes**: old `origin` renamed `gitea` (kept during confidence period, remove + archive gitea repo later); new `origin` = `https://github.com/skea999/feralos.git`; `gh auth setup-git` credential helper; tracking re-pointed on main/develop/developDinit.
- **URLs fixed** (only 2 hardcoded refs repo-wide; other grep hits = Artix audit refs / historical memory): `setup.sh` repoUrl → `https://github.com/skea999/feralos.git`; README wget → `https://raw.githubusercontent.com/skea999/feralos/main/setup.sh` (GOTCHA: raw format differs — gitea `raw/branch/main/...` vs github `raw.githubusercontent.com/<owner>/<repo>/main/...`).
- **No CI to port**: `.github/` + `.githooks/` empty of tracked content; no `.gitea/workflows` either.

### 2026-10-10 14:30 - host: Nix QEMU stack removed, apt QEMU 8.2.2 validated 94/94
**Agent:** codebase
**Summary:** User request: swap the bug-3-era Nix QEMU for apt QEMU. Validated BEFORE believing the theory — full harness run on the apt stack.
- **Removed**: all 5 wrapper locations (`/usr/local/bin` + dpkg-unowned `/usr/bin`+`/bin` symlinks + `~/.local/bin` — the one actually resolving in PATH), nix profile `qemu` (11.1.1; swtpm/OVMF/virt-manager/mesa KEPT in profile), AppArmor nix overrides (4 local files: abstractions/libvirt-qemu, usr.sbin.libvirtd, virt-aa-helper, virtqemud — all `/nix/store/**` + `~/.local/bin/qemu*` lines deleted, `.pre-aptqemu-backup` backups kept, apparmor reloaded). Backup: `/tmp/qemu-nix-backup.txt`.
- **Installed**: apt `qemu-system-x86` + `qemu-system-gui` **8.2.2** + `libvirglrenderer1` **1.0.0** (Ubuntu noble — no CLI virglrenderer pkg needed, QEMU links the lib). Capability greps: `virtio-vga-gl` ✓ `egl-headless` ✓.
- **Insight recorded**: the GL path niri actually uses = guest mesa virgl (edge) ↔ host libvirglrenderer; QEMU version is NOT the discriminator (any ≥~7 with virtio-vga-gl + egl-headless works) — the bug-3 fix was the GUEST mesa virgl driver, not Nix QEMU. `venus=on,blob,hostmem` flags harmless leftovers (harness doesn't pass them anymore).
- **Validated**: run `2026-10-10T14-04-38Z` (apt QEMU) **94/94 PASS** — niri_layers/outputs/screenshot/wayvnc all green, zero GL regressions. Stale comments updated (service.go host requirements + debug_run: virgl is the path, venus optional).

### 2026-10-10 12:15 - DMS wifi/BT harness checks: 2 found+fixed real bugs, 94/94
**Agent:** codebase
**Summary:** Two new niri-block checks (`bluetooth_service` init-conditional, `nm_wifi_support` init-agnostic) — first run (94 checks, run `2026-10-10T11-37-52Z`) caught TWO real bugs; fixed; re-run `2026-10-10T12-12-01Z` = **94/94 PASS, zero failures**.
- **NEW `bluetooth_service`** (dinit/OpenRC conditional like console_keymap): started marker (dinitctl/rc-status) + `pgrep bluetoothd` + **org.bluez registered on the system bus** via `dbus-send ListNames` — the endpoint DMS BT widget actually talks to. No BT adapter in QEMU: org.bluez registers anyway; adapters = real-hw.
- **NEW `nm_wifi_support`**: `networkmanager-wifi` + `wireless-regdb` + `wpa_supplicant` packages + `regulatory.db` + `nmcli radio wifi` answers (NM daemon + radio subsystem live). Scan/connect = real hardware.
- **BUG 1 (aports, found by check)**: bluez-dinit service `command` = `/usr/libexec/bluetooth/bluetoothd` — Debian path; Alpine installs `/usr/lib/bluetooth/bluetoothd` → bluetoothd never ran, service STOPPED-failed, `log_errors` cascaded on the serial `[FAILED] bluetooth`. Fix aports `fb64d77` r3 (Pages live, CI green, docker abuild green).
- **BUG 2 (harness, found by check)**: my check tested `/usr/sbin/wpa_supplicant` — Alpine installs `/sbin/wpa_supplicant`. Path fixed; wifi stack itself was healthy (`nmcli radio wifi` = enabled).
- **Validated**: run `2026-10-10T12-12-01Z` 94/94 — `bluetooth_service` `[ {+} ] bluetooth (pid: 1562) ... org.bluez on bus`, `nm_wifi_support` `radio: enabled`, all sentinels green (niri_layers/greetd_socket/dms_ipc/wayvnc/less checks). Installer.log proves bluez/bluez-openrc/bluez-dinit r2(→r3 next run)/networkmanager-wifi/wireless-regdb all installed from rules.

### 2026-10-10 01:40 - bluetooth service unified + niri-only enablement
**Agent:** codebase
**Summary:** Correction round on the previous bluetooth wiring (user review): (a) the dinit service must NOT live in `defaultRunlevelsDinit.boot` — that's the BASE list, bluetooth would be enabled on every dinit install, not just niri; (b) since feralos OWNS `bluez-dinit`, the service name can be unified instead of hacked around.
- **aports rename** (`e9ac00c`, CI green, Pages `bluez-dinit 0.1.0-r2`): service file `/lib/dinit.d/bluez` → `/lib/dinit.d/bluetooth` (same name as the OpenRC initd). Published apk verified: contains ONLY `lib/dinit.d/bluetooth`.
- **internal_rules simplified**: `defaultRunlevelsDinit.boot` back to original (no bluez), `dinitSkipServices` back to original (no bluetooth — the service file now EXISTS under dinit, must not be skipped). Enablement = the single `"bluetooth": "default"` in the NIRI desktop services map only → dinit loop (Runlevels pass) links `boot.d/bluetooth` + OpenRC `rc-update add bluetooth default`. Desktop flows (server/minimal/kde/cosmic) never enable it.
- **wireless-regdb explained** (user asked): kernel regulatory database (`/lib/firmware/regulatory.db`, cfg80211) — legal channels/frequencies/power per country; without it real wifi cards refuse to scan or lock to wrong channels. Tiny data package, standard everywhere wifi ships.
- YAML assertions green (no bluez in dinit boot, no bluetooth in skip, niri services has bluetooth, base default clean), installer build/vet/gofmt clean.

### 2026-10-10 01:10 - DMS bluetooth + NM wifi completeness (data-only round)
**Agent:** codebase
**Summary:** DMS bluetooth widget dead + DMS network widget missing wifi — both diagnosed on the live target (`dms doctor`, debug_run of run `2026-10-09T22-30-58Z`) and fixed data-only in `internal_rules.yaml` (zero Go changes). DMS doctor also gave the definitive missing-packages list: matugen + cava = hard warnings (user handled separately), dgop = NOT a gap (embedded in the dms Go binary), kimageformats/khal/adw-gtk3 = user already added.
- **Bluetooth (was entirely absent)**: niri packages += `bluez`, `bluez-openrc`, `bluez-dinit` (feralos bluez-dinit = `/lib/dinit.d/bluez`, dbus-wait-for readiness, exists in repo+Pages — never wired into the installer until now). DMS BT widget speaks org.bluez dbus.
- **Service-name split (gotcha)**: OpenRC initd = `bluetooth`, feralos bluez-dinit service = `bluez`. Wiring: niri desktop `services: {"bluetooth": "default"}` (OpenRC enablement) + `defaultRunlevelsDinit.boot += "bluez"` (dinit boot.d link) + `dinitSkipServices += bluetooth` (prevents the dinit loop from trying the nonexistent /lib/dinit.d/bluetooth).
- **WiFi (incomplete)**: Alpine splits the NM wifi plugin — `networkmanager-wifi` was missing, so NM had NO wifi support at all (DMS network widget = no wifi section). niri packages += `networkmanager-wifi`, `wireless-regdb`. wpa_supplicant already present (NM spawns it); NM config (`10-manage.conf` managed=true) untouched; real-hw wifi firmware covered by the installer's hardware detection.
- Verified names against Alpine 3.24 (docker `apk add -s`: bluez 5.86-r2, networkmanager-wifi 1.52.2-r0, wireless-regdb 2025.10.07-r0 all resolve; obex subpackage = `bluez-obexd`, not used). YAML parse + assertions green, installer build/vet/gofmt clean.

### 2026-10-10 00:20 - wizard password double-entry confirmation
**Agent:** codebase
**Summary:** Generator UX hardening (`create_values.go` askSecret): password + LUKS passphrase now asked TWICE (entry + `Confirm` prompt, masked both); mismatch = BOTH re-asked from the first entry. Empty LUKS (reuse user password) skips confirmation (nothing to mistype). Non-tty path (pipes/automation) keeps plain readLine but confirmation still enforced. Min length UNCHANGED at `MinPasswordLen = 6` (validators.go single source, shared with validateConfig) — already stricter than the requested ">4". Lint: one govet shadow fix in the same function (mechanical, zero behavior change). Builds green: installer + createValues, vet/lint/test. Docs: `docs/guides/variables.md` wizard paragraph.

### 2026-10-09 23:50 - niri/DMS dinit runtime depth: 5 checks added/hardened, validated 91/91
**Agent:** codebase
**Summary:** Harness-only round — deeper "does it actually DISPLAY and WORK" coverage for the dinit niri stack (user asked to leave `dms_ipc`/`niri_wayvnc_port` OpenRC failures as-is and deepen dinit verification instead). Fixture for dinit niri validation = `btrfs_standard_niri_full_test.yaml` (`enableDinit: true`).
- **NEW `niri_layers`** (dinit): `niri msg layers` must list ≥1 `^Output` — layer-shell surfaces MAPPED = the DMS bar actually RENDERS (deeper than dms_runtime process-alive and dms_ipc IPC-answers). Output shape verified live: `Output "Virtual-1": Background layer: Namespace: "quickshell"`.
- **NEW `greetd_socket`** (BOTH inits): greetd IPC socket exists — complement to static greetd_config. GOTCHA verified live: greetd socket is PID-suffixed `/run/greetd-<pid>.sock` (clients get path via GREETD_SOCKET env), NOT the literal `/run/greetd.sock` → glob `/run/greetd-*.sock`. First run failed on the literal path (91/92), fixed, standalone re-verify 91/91.
- **NEW `session_dbus_socket` + `session_pipewire_socket`** (dinit): `/run/user/$uid/bus` + `/run/user/$uid/pipewire-0` — services "started" ≠ consumer endpoints exist.
- **`niri_screenshot` hardened**: PNG magic bytes asserted (`od -An -tx1` == `89 50 4e 47 0d 0a 1a 0a`) — fresh-file criterion now also proves a REAL PNG (validated: 220926-byte capture).
- **regreet runtime NOT checkable** on verify boots: harness test-only autologin starts the user session directly — greeter UI never runs there; greetd daemon + static config remain the greeter coverage.
- **Validated**: run `2026-10-09T22-30-58Z` (dinit+edge+niri) 91/92 → 91/91 after greetd glob fix; all existing dinit runtime checks green (user dinit stack, dms_runtime, dms_ipc, wayvnc :5900, windows shows DMS window `com.danklinux.dms` "Benvenuto"). Builds green. OpenRC niri fixture unchanged (2 known dinit-only FAILs remain by user decision).

### 2026-10-09 21:45 - less shim: aports less-dummy package replaces broken symlink dummy
**Agent:** codebase
**Summary:** Dummy `less`→bat symlink CONFIRMED BROKEN on live target (debug_run of run `2026-10-09T18-21-04Z`): bat with TTY stdout execs its HARDCODED pager `less -R -F -K --no-init` (ignores `$PAGER` entirely — only `cat` short-circuits), so the symlink made bat re-exec ITSELF and clap errored `unexpected argument '-R'` — fish `history`, `git log`, `man` all broke (anything piping to $PAGER on a TTY). busybox ash "worked" only because it has no pager-piping history builtin. Fix = aports package `less-dummy` (`f8dcee5`, CI green after ECR mirror fix, Pages `999.0.0-r2`): `/usr/bin/less` = POSIX-sh wrapper that DROPS all less flags (keeps file operands, post-`--` args kept), dumps once via `bat --color=always --paging=never` — child bat never re-pages, chain always terminates. `provides="less=999.0.0-r0"` + `replaces="less"` (file conflict = real C less can never sneak back). CI infra: `build.yml` container `alpine:latest` → `public.ecr.aws/docker/library/alpine:latest` (Docker Hub anonymous pull rate limit blocked 3 runs — permanent fix).
- **Installer**: `internal_rules.yaml` — `less-dummy` added to standardPackages (Memory-safe block), `dummyPackages: {}` (bat→less symlink rule REMOVED with explanatory comments); no Go changes (flows via AllPackages).
- **Harness**: `less_dummy` check made FALSIFIABLE — `apk info -e less-dummy && ! test -L /usr/bin/less && printf 'shim-test\n' | less -R && echo OK || exit 1` (the `-R` path = the exact formerly-broken flag); old Go post-processing block enforcing symlink→bat semantics DELETED (wrapper is a regular file now).
- **Tradeoff**: no interactive scroll/search in "less" — dumps. Future: moar/ov (Go pagers) via same package, zero installer change.
- Builds green: src gofmt/vet/lint/build; harness gofmt/vet/lint/build/test. NEXT: optional `BAT_PAGER=cat` belt+braces env.
- **VALIDATED LIVE** (run `2026-10-09T22-03-03Z`, fixture `btrfs_standard_niri`, 68/71): checks deepened BEFORE the run — `less_dummy` = 4 assertions (pkg + regular file + bat's EXACT flag set `-R -F -K --no-init` accepted + stdin/file/`--` content passthrough via markers) + NEW `less_fish_history` (fish `-ic history` output must contain no clap `unexpected argument` text). Both GREEN on the installed target — clap parses args before any TTY check, so the flag-set test is a real discriminator even on non-TTY verify SSH. 3 unrelated FAILs = pre-existing gating gap: fixture runs OpenRC (`enableDinit: false`) but `console_keymap`/`dms_ipc`/`niri_wayvnc_port` assume dinit (keymap actually loaded: `loadkmap [ started ]` in rc-status) — FOLLOW-UP: `console_keymap` is NOW init-conditional (dinit: dinitctl `[[+]]`/`[{+}]` marker; OpenRC: `rc-status --all` `loadkmap [  started  ]` marker, regex validated against the real run output); `dms_ipc` + `niri_wayvnc_port` still dinit-only (user-dinit services don't exist under OpenRC — need cfg.EnableDinit gating or fixture flip).

### 2026-10-09 - bug-3 DEAD: niri renders in QEMU (86/86 zero skips); edge repos = confined feature
**Agent:** codebase
**Summary:** Host + installer + harness round. **bug-3 (zero wl_outputs) RESOLVED**: host Nix QEMU 11.1.1 full build via dynamic `/usr/local/bin` wrappers (qemu-system-x86_64 + qemu-img exec the nix profile binaries — no dead copies, libvirt/virt-manager compatible; libvirt AppArmor local override for `/home/skem`) + `-device virtio-vga-gl,venus=on,blob=true,hostmem=1G` + `-display egl-headless,gl=on` + host kernel 7.0 HWE (venus-gpu module) + guest mesa 26.2.4 @edge (virgl gallium driver restored upstream) ⇒ niri has a REAL output (`Virtual-1` 1280x800), wayvnc listens :5900, screenshot writes a real PNG. Run `2026-10-09T18-21-04Z`: **86/86 PASS, zero skips** (first fully-real niri run in QEMU).
- **Installer**: `enableEdgeRepos` + `extraPackagesEdge` confined feature `chrootPhase/features/edgerepos.go` — 3-step at the tail of SetupPackages: append @edge repos → `apk add` the list → comment the repos out (apk v3 tagged-repo masking kills mid-install edge mixing — "masked in @edge" solver cascade verified live; the early chroot_env writer removed). Console keymap: aports `busybox-loadkmap-dinit` r1 (NO /etc/conf.d file — OpenRC owns it) + `misc.go` KEYMAP= from user variables (bkeymaps layout: keymap == layout dir) + internal_rules boot-list enablement. setup.sh: config path = repo `src/variables/variables.yaml` + cdrom apt-source fix for the live host.
- **Harness**: QEMU DMI skip-gates (niri_outputs_count/niri_wayvnc_port/niri_screenshot + wayvnc.user required-list) REMOVED — all checks real on every host; console_keymap regex fixed for the `[{+}]` dependency-started dinitctl marker; screenshot check = count-based fresh-PNG (newest-compare loses the rc-0-before-write race); GPU line: `virtio-vga-gl` + `egl-headless,gl=on` in service.go target args.
- **Fixtures**: 28/28 = full 33-key template mirror (niri fixtures carry the 5-pkg edge mesa stack in `extraPackagesEdge`).
- NEXT: user's real-hw validation (keymap + rendering on real GPU); dms windows depth; booster initramfs module policy parked (booster = host-specific by default; user force_load list sufficient — no universal mode). **2026-10-18 user: RESOLVED/closed (this parked behavior is final).**

### 2026-10-08 - all 28 fixtures mirror variables.yaml template structure
**Agent:** codebase
**Summary:** Every `tests/fixtures/variables/*.yaml` fixture (28) rewritten to carry the COMPLETE template field set (33 keys, template order, 1-line header comment). Missing fields filled with behavior-preserving defaults (Go zero values = prior effective harness behavior): `luksPassword` = fixture `password` (matches installer fallback config.go:130), `enableDinit: false` when absent (OpenRC regression fixtures keep OpenRC), podman/webmin/zswap/nix/hardening false when absent, `videoDriver: ""` (== "none": both miss the VideoDrivers map → no-op, logic.go:27).
- **niri fixtures** (`btrfs_standard_niri.yaml`, `btrfs_raid0_full.yaml`, `btrfs_standard_niri_full_test.yaml`): `enableEdgeRepos: true` + `videoDriver: virtio` + `extraPackagesEdge:` = 5-pkg edge mesa stack (mesa, mesa-dri-gallium, mesa-gl, mesa-egl, mesa-gbm @edge); no `@edge` entries left in `extraPackages`.
- **server/minimal fixtures**: `enableEdgeRepos: false`, `extraPackagesEdge: []`. Distinctive values preserved everywhere (diskLayout, disks count, sb/tpm flags, swapSize explicit values, amdgpu for kde/cosmic, incus/qemu extraPackages).
- Semantics verified by before/after JSON diff (only intended changes); 28/28 yaml parse; harness + installer builds green. No Go code changed — harness passthrough already handles all fields (loadVariablesFile/writeHarnessVariables).
- NEXT: niri harness run (edge mesa + virtio GL validation, bug-3 direction (a)).

### 2026-10-08 - extraPackagesEdge (edge-tagged packages installed LAST)
**Agent:** codebase
**Summary:** New user variable `extraPackagesEdge` ([]string, default empty): packages installed from the @edge tagged repos, installed AFTER all other package installs + FinalUpgrade (very tail of `SetupPackages`) so the base install completes from stable first. Requires `enableEdgeRepos: true` — hard validation error in `validateConfig` (config.go) otherwise; per-package `@tag` suffix accepted (`ValidateExtraPackages` pkgRe already allows it).
- **Installer**: `config.go` field + validation (edge list w/o flag = error); `packages.go` tail: `apk add <pkgs>` after FinalUpgrade, hard error on failure (NOT ignoreErrors-swallowed at call site — SKEA_IGNORE_ERRORS env still applies inside ExecuteCmdArgs). No wizard prompt (template-only; create_values.go has no extraPackages prompt either). Template `variables.yaml`: `extraPackagesEdge: []` documented near extraPackages.
- **Harness**: full passthrough (RunConfig field, VariablesYAML field, LoadVariablesFile copy, writeHarnessVariables conditional block mirroring flatpakPackages/nixPackages style). Fixture `btrfs_standard_niri_full_test.yaml`: 5 mesa@edge entries MOVED from extraPackages to new `extraPackagesEdge:` list (fixture already `enableEdgeRepos: true`). `enableEdgeRepos` was already wired end-to-end in the harness.
- Builds green: installer gofmt/vet/build; harness gofmt/vet/build. Docs: `docs/guides/variables.md` Extra Packages table row. NEXT: niri harness run to validate mesa-from-edge lands + GL-on-virtio (bug-3 zero-outputs fix direction (a)).

### 2026-10-07 - console keymap (loadkmap) restored under dinit
**Agent:** codebase
**Summary:** Console keymap regression fixed (OpenRC→dinit migration lost the busybox-initscripts loadkmap service). New aports package `busybox-loadkmap-dinit` 0.1.0-r0 (docker abuild green): scripted service `/lib/dinit.d/loadkmap` (depends-on local.target, before login.target) → `/usr/lib/dinit/loadkmap` wrapper (sources `/etc/conf.d/loadkmap`, default `us`, pipes `.bmap.gz` via `zcat | busybox loadkmap`) → conf default `/etc/conf.d/loadkmap` (apk-protected; installer rewrites KEYMAP= from user variables).
- **Installer**: `misc.go` keymap no longer hardcoded `it` — `keymapPath = /usr/share/bkeymaps/<keymap>/<keymap>.bmap.gz` from `u.Keymap` (kbd-bkeymaps layout: keymap == layout dir; `%q` quoting). `packages.go` dinit corePkgs += `busybox-loadkmap-dinit` (hard-verified install). `kbd-bkeymaps` was ALREADY in `standardPackages` (no change).
- **Enablement**: `internal_rules.yaml` `defaultRunlevelsDinit.boot` += `loadkmap` (boot.d symlink via services.go first loop); `dinitSkipServices` keeps `loadkmap` (prevents duplicate enable from the OpenRC boot list — same name in both lists). OpenRC `defaultRunlevels.boot` already had `loadkmap` (busybox-initscripts) — untouched.
- **Why not chimera early-console**: `dinit-console` reads `/etc/conf.d/keymaps` (short name) under `/usr/share/keymaps/...` — paths don't exist with kbd-bkeymaps; always no-op for keymaps. Font half still applies. Note in `docs/dinit/BOOT.md` step 24.
- **Harness**: `console_keymap` check (niri block, service.go): `grep -q 'bkeymaps' /etc/conf.d/loadkmap && dinitctl list | grep -E '\[.\] *loadkmap'` — conf + started marker, no polling (boot-time scripted).
- Builds green: installer gofmt/vet/golangci-lint/build; harness gofmt/vet/build. NEXT: harness dinit niri fixture run to validate check + keymap live.

### 2026-10-05 19:35 - check mechanics fixes validated on LIVE target (standalone verify 85/85)
**Agent:** codebase
**Summary:** Harness-only round. 5 output-dependent niri checks re-guarded with DMI product_name+sys_vendor concat; `dms_ipc` syntax fixed to an empirically-verified call. Re-verified against the LIVE debug_run boot of run `2026-10-05T00-42-34Z` — **85/85 PASS** (standalone `./skea_harness verify -run-dir ...`; writes JSON to stdout only, does NOT touch verify.md — the run-dir verify.md stays the install-time snapshot).
- **DMI QEMU detection (service.go, 5 checks)**: `niri_outputs_count`, `niri_wayvnc_port`, `niri_screenshot`, `niri_user_dinit_runtime` (wayvnc.user dropped from REQUIRED list under QEMU — output-starved crash-loop cascade, NOT a product bug), all now read BOTH `product_name` AND `sys_vendor` and match the concat `case "$p$v" in *QEMU*)` (q35: product_name `Standard PC (Q35 + ICH9, 2009)`, sys_vendor `QEMU` — verified verbatim on the live target). Guards exit 0 informational under QEMU; real hw unchanged (hard).
- **dms_ipc REAL syntax (service.go)**: `dms ipc call outputs status` — empirically verified on the live target: answers `auto: off / active: none / outputs: []` rc 0 (empty list = QEMU zero-output reality, call still proves IPC). GOTCHA: dms answers UNKNOWN targets rc 0 too (`widget list` also rc 0 despite no `widget` target in help) — only real target/function pairs are valid proof. dms REFUSES root → `su - <user> -s /bin/sh`; NIRI_SOCKET exported for symmetry; 60s poll, `timeout 10`.
- **Working read-only dms calls found live**: `audio status`, `lock status` (JSON), `outputs status`, `notifications getDoNotDisturb`.
- **Note**: run `2026-10-05T01-00-52Z` (later run in state) failed "context canceled" and its teardown killed all VMs; target for this round re-booted from run `00-42-34Z` disk via `tests/debug_run` (port 2223). Host key change on reconnect = fresh install, expected (ssh-keygen -R the stale entry).

### 2026-10-05 00:40 - greeter dinit log dir; 4 new niri/dms runtime checks; OpenRC minimal 49/49
**Agent:** codebase
**Summary:** Installer + harness round, OpenRC minimal regression green (`btrfs_standard_minimal`, run `2026-10-05T00-33-48Z`, **49/49 PASS**, verify.md in run dir).
- **Greeter user-dinit log dir (niri.go)**: greeter user services (dbus-session etc.) log under `$HOME/.local/share/dinit` — dir was missing for `/var/lib/greetd` → "opening log file" failures. Added `mkdir -p /var/lib/greetd/.local/share/dinit` in the greeter section, covered by the existing `chown -R greeter:greeter`.
- **Dead eth0 write (network.go)**: `setupInterfaces` eth0-dhcp write is dead ONLY in the niri flow (overwritten by niri.go lo-only write; NM manages eth0). NOT removed — function shared by all non-niri flows (their only network config). Both spots now carry DEAD-in-niri-flow comments (network.go func doc + niri.go step 7).
- **Harness runtime checks (service.go, gated niri)**: `niri_msg_version` (rc 0 + head -2 detail), `niri_windows` (state query, pass on rc 0), `niri_outputs_count` (THE ONE QEMU skip: `*QEMU*` product_name → exit 0 informational — venus/virgl upstream limitation gives niri ZERO outputs, bug-3; real hw enforces ≥1 `^Output`), `dms_ipc` (`dms ipc call widget list` — DankLinux docs syntax `dms ipc call <target> <function>`, NO `output list` target; client finds shell socket via XDG_RUNTIME_DIR; VERIFY EMPIRICALLY next niri run). All use the same socket discovery (`ls /run/user/$uid/niri*.sock | head -1`).
- **Realhw script** (`tests/harness/niri_realhw_check.sh`): same 4 checks added; `niri_outputs_count` HARD there (real hw MUST have ≥1), no QEMU skip.
- Builds green: harness gofmt/vet/golangci-lint/build OK; installer go vet + build OK. niri fixture run = next step (not this round).

### 2026-10-03 21:45 - caef07b fixes VALIDATED; bug-3 root cause = zero wl_outputs in QEMU (77/80 ×2)
**Agent:** codebase
**Summary:** Runs 2026-10-03T18-56-40Z + 2026-10-03T19-27-59Z, both 77/80. **All 3 aports caef07b fixes work**; 3 remaining FAILs share ONE root cause: niri boots with ZERO outputs under QEMU.
- **Fix 1 race poll WORKS**: run 1 verify boot missing-list was `niri wayvnc.user`; after a check-regex mechanics fix (service.go: dinitctl marks EXPLICITLY-started services `[[+]]`, DEPENDENCY-started `[{+}]`; niri started via `dinitctl --user start niri` renders `[[+]] niri` — old regex `'\{\+.*\] niri'` never matched), run 2 missing-list = `wayvnc.user` ONLY → niri IS supervised in user dinit on verify boots. Timing proof: compositor starts 1.3s after PAM open; the fallback path requires the 15s poll to expire first — impossible. Debug_run boot confirmed live: `[[+] ] niri (pid: 1619)`, `dinitctl --user status niri` = STARTED/explicitly started. Earlier "race lost 3/3" conclusion in the 19:35 round was a false negative from this regex.
- **Fix 2 niri.log WORKS**: `~/.local/share/dinit/niri.log` exists with full stderr; bug-3 root cause captured (see below).
- **Fix 3 launchers WORK (half-proven)**: polkit-agent.user GREEN both runs (was crash-looping every boot); dms.user green. wayvnc.user now fails one layer deeper: env resolves, wayvnc starts then exits 1 8× "No output found" (smooth-recovery loop) — because compositor has no outputs.
- **Bug-3 root cause (niri.log verbatim, verify boot)**: `WARN niri::backend::tty: error getting the render node for the primary GPU; proceeding anyway` → `using as the render node: "/dev/dri/card0"` → card0 = PCI 1234:1111 bochs (mesa `driver (null)`, `no allocator available for device`) → card1 virtio-gpu-gl: `virtio_gpu: driver missing` (guest mesa lacks venus/virgl pipe) → both rejected (`software EGL renderers are skipped`) → `INFO listening on Wayland socket: wayland-1` with ZERO devices → `WARN niri::handlers::layer_shell: no output for new layer surface, closing`. `niri msg outputs` = EMPTY (rc 0). Consequences: screenshot-screen exits 0 but writes nothing (no output to capture), wayvnc "No output found" crash-loop, :5900 never listens.
- **Fix directions (product/env, NOT harness-check)**: (a) guest userspace for virtio-gpu-gl (mesa venus/virgl in target install — check `mesa-dri-gallium` split), or (b) make virtio-gpu-gl the PRIMARY/boot VGA (only card; bochs card0 comes from QEMU defaults — consider `-nodefaults` + explicit console path) so niri picks it as primary. Real hardware unaffected (real GPU = real outputs).
- **Harness**: debug_run (tests/debug_run) boot of run 18-56-40Z used for live user-dinit inspection; old target QEMU must be killed before re-run (port 2223 hostfwd collision). Session lifetime re-proven both runs: `New session c1 of user skea999` @4.8-5.0s → `Removed session c1` only at ~298s STOPPD teardown.
- Runs kept: `runs/2026-10-03T18-56-40Z`, `runs/2026-10-03T19-27-59Z` (verify.md = report path).

### 2026-10-03 19:35 - niri-session fix VALIDATED; 3 product bugs diagnosed (77/80 ×2)
**Agent:** codebase
**Summary:** Harness runs 2026-10-03T16-34-07Z + 2026-10-03T17-09-19Z, both 77/80. **Session fix WORKS**: verify boots hold `New session c1 of user skea999` @4.9s → Removed only @~297s at STOPPD teardown (was ~350ms death); 3 of 6 runtime checks GREEN for the first time (`niri_session_user`, `niri_wayland_socket`, `niri_ipc`); 74 legacy checks green, zero regressions.
- **Package evidence**: `niri-user-dinit-989.0.1.0-r1` installed (installer.log:1779); `apk info --who-owns /usr/bin/niri-session` → niri-user-dinit; file header = FeralOS fix (aports 6dec818).
- **Bug 1 — pgrep race (aports niri-user-dinit)**: niri-session's single-shot `pgrep -u UID dinit` loses the turnstiled async user-dinit spawn on both harness verify boots (2/2) → fallback `exec niri --session` → compositor outside user dinit: verify checks report `niri` missing from `dinitctl --user list` while pgrep/IPC/wayland-1 all pass. Debug boots win the race (2/2, `[[+]] niri` in list) — nondeterministic. Fix: poll for user dinit (e.g. `dinitctl --user ping` retry loop) before the fallback decision.
- **Bug 2 — user dinit env lacks WAYLAND_DISPLAY (aports)**: wayvnc.user + polkit-agent.user crash-loop on EVERY boot incl. normal-path boots (`wayvnc.log`: "WAYLAND_DISPLAY is not set"; `polkit-agent.log`: "cannot open display"). niri-session's setenv imports greetd's login env, which cannot contain WAYLAND_DISPLAY (socket doesn't exist pre-start); nothing sets it after `dinitctl --user start niri` returns. `dms.user` works only because its wrapper self-resolves the socket. Fix: setenv `WAYLAND_DISPLAY=wayland-1` (+ DISPLAY/XDG_SESSION_TYPE) after start returns, or on-start hook in niri.service.
- **Bug 3 — screenshot writes nothing (niri 25.11 in QEMU)**: action exits 0 in ALL CLI shapes (no-arg, `--write-to-disk true`, `--path <abs>`; positional arg rejected) yet NO file ever appears (whole-FS find; decoy persists → not perms/deleter; niri unconfined by AppArmor; stderr → /dev/null — no logfile in niri.user service). Harness check mechanics fixed + verified correct (fires action, requires NEW non-empty PNG).
- **Harness fixes shipped (service.go)**: `niri_screenshot` — dropped the rejected positional path-arg attempt, fires `screenshot-screen` and requires a *fresh* non-empty PNG (before/after latest-png compare); `enableNiriTestAutologin` — `install -d -o <user> -g <user> -m 2700` on BOTH `~/Pictures` and `~/Pictures/Screenshots` (root `mkdir -p` left the parent root-owned mode 2700 → user could never write screenshots anywhere under ~/Pictures).
- **Known issue**: golangci-lint shadow at service.go:1149 pre-dates this round (earlier uncommitted work); go vet clean.
- Run dirs kept for debug_run: `runs/2026-10-03T16-34-07Z`, `runs/2026-10-03T17-09-19Z`.

### 2026-10-02 02:30 - earlyoom tiered victim selection
**Agent:** codebase
**Summary:** earlyoom-dinit tiered (prefer user apps 172 / ignore system core 48 / desktop session natural); aports `ce86d69` CI green first try.
- Tier 1 user apps killed FIRST via `--prefer` (+300): 172 apps (browsers, chat/mail, office/PDF, KDE/GNOME/XFCE, media, images, games/emu, torrents, dev tools, installer apps); flatpak apps keep comm names = same regex; truncation-correct 15-BYTE comms.
- Tier 3 system core NEVER chosen: `--ignore` (hard skip) not `--avoid` — key insight: `--avoid` only subtracts 300 oom_score (soft, can be outbid — manpage-verified) while `--ignore` = hard skip. Covers PID1 (dinit + init for OpenRC path), session mgr, bus, polkitd, network, logging, time, cron, ssh, login path, seat/hw daemons, printers, bt/modem/store, flatpak infra, kthreadd.
- Tier 2 desktop session (niri, pipewire, portals, terminals) absent from both lists = natural oom_score order.
- Wrapper POSIX sh (busybox-clean); config `/etc/dinit.d/config/earlyoom.conf` (ADR-008 pattern); regex values glob-safe (no `[ ] * ?`), case via repeated names (discord|Discord).

### 2026-10-02 00:45 - dinit services aligned to Artix/Chimera (ADR-009)
**Agent:** codebase
**Summary:** 48 -dinit pkgs aligned after full Artix/Chimera audit; aports `b69a573` pushed, CI green; 74/74 dinit niri + 49/49 OpenRC minimal.
- `restart=true` removed from ALL daemons (Artix/Chimera never restart — smooth-recovery only); scripted one-shots explicit `restart=false`; getty keeps `restart=true` + gained `termsignal=HUP` + config-driven baud/term (wrapper `/usr/lib/dinit/getty` + `/etc/dinit.d/config/getty.conf`, Artix agetty-default pattern, 115200 serial default).
- `before=login.target` added to 33 services (Artix+Chimera universal convention); network ordering: firewalld/nftables/iptables before `pre-network.target`, NetworkManager depends pre-network + before `network.target`, avahi/rsync depend networking, apparmor before `local.target` (profiles load BEFORE local services), fuse after `early-root-rw.target`.
- `dnsmasq` privilege drop `-u dnsmasq -g dnsmasq --enable-dbus` (Artix exact; was running as root); `dbus` `smooth-recovery=no` (nobody restarts the bus; ADR-007 stop guard kept, verified clean shutdown elogind→dbus→power down).
- bluez/elogind bus-name readiness via NEW package `dbus-wait-for-dinit` (BSD-2-Clause helper from Artix dinit-rc, libdbus, `/usr/lib/dinit/dbus-wait-for`, pipefd:4).
- `lxc` FIXED latent restart-loop bug — process+restart=true on one-shot lxc-autostart → scripted start/stop + stop-command; `sshd` gained scripted `ssh-keygen` prereq service (keys baked at install, Artix/Chimera pattern).
- Audit facts: Artix dbus-activates polkit/udisks2/fwupd (no init service by design); irqbalance/rasdaemon/incus/nix/vector/klogd/cosmic-greeter NOT packaged on Artix; turnstile-dinit on Artix = split subpackage (we already match its 2 seds); ADR-008 wrapper+config pattern retained as FeralOS extension (NOT in Artix/Chimera).

### 2026-10-01 23:59 - conf.d-equivalent config files (Artix pattern)
**Agent:** codebase
**Summary:** 22 -dinit packages converted to wrapper+config-file pattern (Artix); aports `cc107a1` pushed, CI green; harness `dinit_config` check added; 74/74 dinit niri + 46/46 OpenRC minimal.
- Every `-dinit` service now: service file → `/usr/lib/dinit/<svc>` wrapper → `[ -r ] && . /etc/dinit.d/config/<svc>.conf` → exec daemon `${VARS}` — user-editable, apk-protected (OpenRC `/etc/conf.d` convention restored; defaults match Alpine conf.d).
- `dnsmasq` `DNSMASQ_CFGFILE` defaults to `/etc/dnsmasq.conf` (empty would mean no config); `cpufrequtils` gained a `stop-command` consuming `CPUFREQ_STOP_OPTS`.
- Harness: permanent `dinit_config` check in `tests/harness/internal/service/service.go` (EnableDinit only: config dir + ≥10 `.conf` + wiring spot-checks).
- Verified: dinit niri full **74/74** (run `2026-10-01T22-19-23Z`), OpenRC minimal **46/46** (run `2026-10-01T22-27-05Z`, check correctly absent). ADR-008.

## Workflow

**Chain (complex)**: `context/recall` → `interview/refine` → `@planner` → *approve* → `@codebase` → PARALLEL(`@review`+`@code-reviewer`+`@vulnerability-scan`+`@information-scan`) → `@debugger/tests` → `@docs/docs` → save
**Direct (simple)**: `@codebase` or specialist
State: `state/session_state.json` | Handoff: `handoff/latest.md`

**Installer Workflow**: Understand Requirements (`CONTEXT.md`, `ROADMAP.md`, shell ref) → Plan (`hostPhase`/`chrootPhase` modules) → Implement (Go, naming conventions) → Test locally → Test harness (QEMU) → Review → Commit (conventional)

Key Decisions: `hostPhase` writes files (NO chroot), `chrootPhase` runs commands, module isolation, data-driven YAML/Go, security first (LUKS2, Secure Boot, AppArmor)

→ Details: `docs/guides/development.md` (how to code / separation), `docs/INSTALLER.md` (flow)

## Documentation Rule (MANDATORY)

**Every code change MUST update documentation.** After you change code, update `AGENTS.md` (primary context, keeps full info) **and** the matching `docs/` copy (architecture/adr/guides/harness/intent/ideas) to keep double documentation in sync. No code without doc update. `AGENTS.md` → `docs/` is single source duplication; keep both identical in substance. Update `Docs Index` pointers if you add/move files. Harness docs → `docs/harness/` (canonical) — `tests/harness/README.md` stays pointer only.

**ADRs:** If you make an architectural decision (new pattern, isolation rule, storage layout, boot method, logging, compatibility, etc.) and no ADR exists, **write one** in `docs/adr/ADR-XXX-title.md` (next number) + update `docs/adr/README.md` index. Template: Status / Date / Context / Decision / Consequences. No decision without ADR.

→ Details: `docs/architecture/`, `docs/adr/`, `docs/guides/`, `docs/harness/`, `docs/intent/`, `docs/ideas/`

## Session Memory

Storage `.agent-memory/`, scripts `.agents/skills/session-memory/scripts/`

**The memory must be pushed.** `.agent-memory/` is tracked in git — after saving, commit and push it with the session's changes.

```bash
.agents/skills/session-memory/scripts/memory.sh context --days 14
.agents/skills/session-memory/scripts/memory.sh recall "vector" --limit 5
.agents/skills/session-memory/scripts/memory.sh stats
.agents/skills/session-memory/scripts/memory.sh save "decision" "Chose X because Y" tag1 tag2
MEMORY_IMPORTANCE=high .agents/skills/session-memory/scripts/memory.sh save "insight" "Key finding" installer
.agents/skills/session-memory/scripts/memory.sh save "summary" "Today: implemented X. Next: Z" -Tags installer go feralos
```

## Build & Test

```bash
# Build Go installer
cd src && go build -o ../feralos_installer .

# Build Go generator
cd src/utils/createConfig/createValues && go build -o ../../../../feralos_create_config .

# Build first-boot programs
cd src && go build -o ../feralos_secureboot ./firstBoot/secureboot/
cd src && go build -o ../feralos_tpm ./firstBoot/tpm/

# Build harness
cd tests/harness && go build -o skea_harness ./cmd/skea_harness/

# Run installer (requires root + live)
sudo ./feralos_installer -config variables.yaml

# Run generator
./feralos_create_config

# Run harness tests
cd tests/harness
./skea_harness prepare-runner
./skea_harness run -f ~/Progetti/feralos/src/variables/variables.yaml
./skea_harness status
./skea_harness cleanup

# Run harness with fixture templates (covers all scenarios)
./skea_harness run -f tests/fixtures/variables/btrfs_raid0_minimal.yaml
./skea_harness run -f tests/fixtures/variables/btrfs_raid0_noswap.yaml
./skea_harness run -f tests/fixtures/variables/btrfs_standard_swap_explicit_8g.yaml
ls tests/fixtures/variables/  # 30 fixtures, full variables.yaml template mirror (all scenarios)

# Run tests (when available)
cd src && go test ./...
```

→ Details: `docs/guides/development.md` (build/test/lint + conventions), `docs/harness/commands.md` (CLI), `docs/harness/workflow.md` (3-phase flow)

## Test Artifacts

Output to `tests/artifacts/<tool-name>/<timestamp>/`:

```text
tests/artifacts/
├── feralos_create_config/
│   └── .gitkeep
└── harness/
    ├── cache/ (base-images, keys, runner-images, web/alpine)
    ├── prepare/ (logs)
    ├── runs/ (per-install logs + disks)
    └── state/ (active.json)
```

Rules: gitignored `tests/artifacts/**/*` except `.gitkeep`, never commit outputs, harness keeps last 2 runs.

→ Details: `docs/harness/artifacts.md` (where generated, gitignored, how to inspect)

## Lint (Native Linux)

```bash
cd src && go vet ./...
cd src && golangci-lint run ./...       # includes dupl (code clone detection)
cd tests/harness && go vet ./...
cd tests/harness && golangci-lint run ./...
```

**Dupl exclusions** (intentional duplication): `src/hostPhase/disks/` (layout packages), `src/chrootPhase/desktops/` (derived desktop flows — niri.go/niri_wayle.go are intentionally duplicated, same policy as disk layouts: do NOT merge or extract shared blocks), `src/utils/firstboot/` (standalone binaries). Config: `.golangci.yml` → `dupl.threshold: 100` + `exclude-rules`. Linter set also carries `wastedassign` + `nakedret` (both clean); `revive` is DISABLED — its var-naming rule flags every MixedCaps package name (`hostPhase`, `chrootPhase`, `btrfsStandard`, `createConfig`…), which is the documented project convention ("Folders camelCase"), so its findings are false positives (rationale comment in `.golangci.yml`). Full polish task history is in the git log.

**Static analysis suite (ursine pattern)** — configs at repo root: `.ecrc`, `.gitleaks.toml`, `.gitleaksignore`, `.markdownlint-cli2.jsonc`, `.shellcheckrc`, `typos.toml`, `.yamllint.yml`, `taplo.toml`; wired into `scripts/ci.sh` after the existing checks; tool versions pinned in `Dockerfile` lint-tools (gitleaks 8.30.1, typos 1.51.1, yamllint 1.38.0, markdownlint-cli2 0.24.0 + Node 22.23.4, editorconfig-checker 4.0.2; taplo optional/not in image):
- **blocking**: `yamllint -c .yamllint.yml` over every `*.yaml`/`*.yml` in the tree except artifacts/vendor dirs (broken YAML breaks the installer); `gitleaks detect --source . --config .gitleaks.toml --no-banner --redact` (git checkout = full commit history; known historical findings fingerprinted in `.gitleaksignore`; the docker image ships no `.git` → ci.sh falls back to `gitleaks dir .` working-tree scan; the old pattern-grep scan stays as the fallback for hosts without gitleaks)
- **warn-only**: `typos --config typos.toml --format brief .`, `markdownlint-cli2` (globs/ignores in `.markdownlint-cli2.jsonc`), `ec --config .ecrc .`, `taplo lint --config taplo.toml`
- `.editorconfig` exceptions: `*.md` keeps trailing whitespace + `indent_size = unset`; `*.go` indent rules unset — gofmt is the sole Go indentation authority (editorconfig-checker cannot tell code from space-indented raw-string content)

→ Details: `docs/guides/development.md` (lint/format)

## Naming Conventions

| Element | Convention | Example |
|---------|------------|---------|
| Files | snake_case | `bind_mounts.go`, `config.go` |
| Folders | camelCase | `hostPhase/`, `chrootPhase/` |
| Variables | camelCase | `targetDisk` |
| Functions | camelCase/PascalCase | `getMinDiskSize()`, `RunBtrfsSetup()` |
| Types | PascalCase | `Config`, `Subvolume` |

→ Details: `docs/guides/development.md` (module map + conventions)

## Architecture Principle

Standard core, pluggable data — installer logic never changes, storage layouts are Go packages, essential packages hardcoded, optional+rules in YAML, parser same code only data changes, booster replaces mkinitfs, UKI replaces GRUB, LUKS2 on RAID/Swap, Btrfs RAID1 on LUKS. **No backward compatibility — installer runs once, always fresh.**

→ Details: `docs/architecture/overview.md` (principle + tree + decisions), `docs/adr/ADR-001-go-installer.md`, `ADR-005-luks-btrfs-booster-uki.md`, `ADR-006-no-backward-compat.md`

## Entry Point & Phase Isolation

`main.go` is entry point ONLY. It collects/generates config then delegates:

- `cleanup.Cleanup(conf)` — pre-host cleanup (swapoff, unmount, LUKS close)
- `hostPhase.Run(conf)` — all host-side installation
- `chrootPhase.Run(conf)` — all chroot-side installation
- `firstboot.WriteInstallConfig(conf)` — reads cmdline, writes `/opt/feralos/config.yaml`
- `firstboot.SetupSecureboot(conf)` — compile → copy → chmod 755
- `firstboot.SetupTpm(conf)` — compile → copy → chmod 755
- `cleanup.Cleanup(conf)` — final unmount before reboot

`main` must NOT call disk layouts or chroot commands directly.

**Internal package rule**: `hostPhase/internal/*` is importable ONLY by `hostPhase` (and its subpackages). `main` cannot import it. This is the structural guarantee of host/chroot isolation.

**`hostPhase.Run()` fixed order**:

1. Disk layout switch (`hostPhase/disks/*`) by `conf.User.DiskLayout`
2. Mount virtual FS (`internal/bind_mounts.go`)
3. Alpine bootstrap (`internal/bootstrap.go`)
4. Chroot prep (`internal/chroot_env.go`)

Cleanup (before/after) and firstboot (install config, secureboot/tpm compile) are outside `hostPhase.Run()` — called directly by `main`.

→ Details: `docs/architecture/isolation.md` (entry-point + internal rule + order), `docs/INSTALLER.md#installation-flow`, `docs/adr/ADR-002-host-chroot-isolation.md`, `ADR-003-internal-package-rule.md`

## Logging Rule

**Commands: `shell.ExecuteCmd`; file writes: `shell.WriteFile`/`shell.WriteFileAppend` — in `hostPhase`/`chrootPhase`, never raw `os.*`.**

- **Commands** (installs, chmod, sed, chown, anything executed): `shell.ExecuteCmd` — logs CMD, stdout/stderr, exit, retry, `SKEA_NONINTERACTIVE` handling. Host: `shell.ExecuteCmd("", "mkdir -p /mnt/etc")`. Chroot: `shell.ExecuteCmd(chroot, "mkdir -p /etc")`.
- **File writes** (`shell.WriteFile(chroot, path, content, perm)` / `WriteFileAppend`, `src/utils/shell/write.go`): logged direct writes on the chroot mount — `[WRITE]`/`[APPEND]` line (path + size, content NEVER logged) + `[RESULT]`; MkdirAll of the parent (0755); perm forced via chmod on create (umask-independent); honors `SKEA_IGNORE_ERRORS=1` like ExecuteCmd. **The `echo '<escaped>' > path` shell pattern is forbidden for new code**: single-quote escaping of arbitrary content is fragile (one missed `'` corrupts the seed) and very large payloads can hit ARG_MAX — the escaped-echo call sites were converted to `WriteFile`. KEEP `ExecuteCmd` for: conditional appends (`grep -q … || echo … >> file` — the guard is part of the operation), writes computed at shell-runtime (e.g. awk over `/proc/meminfo`), and in-place edits (sed/awk). `WriteFileAppend` covers unconditional appends.
- `import "os"` forbidden in `hostPhase`/`chrootPhase` (exceptions: `firstBoot/` standalone, `utils/shell` impl).
- Documented exceptions (secrets must never be logged): passwd via stdin pipe (chrootPhase/internal/users.go) and cryptsetup stdin (hostPhase/disks/common/luks.go).

→ Details: `docs/architecture/logging.md`, `docs/adr/ADR-004-executecmd-logging.md`, `src/utils/shell/execute.go`, `src/utils/shell/write.go`

## Error Handling (`unattended` vs `ignoreErrors`)

**`unattended: true`** skips manual input prompts ONLY (wizard skip; unchanged semantics). **`ignoreErrors: true`** (default `false`) = best-effort install: command failures are logged (`[ERROR][IGNORED] command failed (ignoreErrors=true): ...`) and installation CONTINUES. Wiring: `LoadConfig` sets `SKEA_IGNORE_ERRORS=1` when `conf.User.IgnoreErrors`; `shell.ExecuteCmd`/`ExecuteCmdArgs` honor it after retries are exhausted (return nil error → callers continue). Used by the test harness: harness runs are ALWAYS best-effort (`LoadVariablesFile` forces `cfg.IgnoreErrors = true`, `writeHarnessVariables` writes `ignoreErrors: true`, all fixtures carry `ignoreErrors: true`) — a failing optional step must not abort the run; the verification phase judges what actually works.

→ Details: `docs/guides/variables.md` (Options table), `src/utils/config/config.go` (env wiring), `src/utils/shell/execute.go` (ignore path)

## Activatable Feature Confinement

**Rule: Every activatable feature MUST be confined to a single owner file.**

Each `Enable*` flag (and `DesktopEnvironment`) is owned by exactly ONE file under `src/chrootPhase/features/` or `src/utils/firstboot/` or `src/chrootPhase/desktops/`. All logic — packages, configuration, guards — lives in that file.

**Callers must NOT contain feature logic:**

- `packages.go` → calls `features.XxxPackages(conf)`, never contains inline feature packages
- `logic.go` → zero feature-specific logic, only generic rule application
- `chroot_phase.go` → calls `features.SetupXxx(conf)`, never contains feature configuration
- `host_phase.go` → calls `features.SetupXxx(conf)`, never contains feature compilation

**Pattern for new features:**

1. Create `src/chrootPhase/features/<name>.go` (or `src/utils/firstboot/` for compile+copy)
2. Export `XxxPackages(conf) []string` — returns APK packages needed
3. Export `SetupXxx(conf) error` — configures the feature inside chroot
4. Guard: `if !conf.User.EnableXxx { return nil }`
5. Call from `packages.go` (packages) and `chroot_phase.go` or `host_phase.go` (setup)
6. NEVER mutate flags from outside the feature file (exception: `NixPackages()` auto-enables internally)

**Current confinement map:**

| Feature | Owner file | Called from |
|---------|-----------|-------------|
| `EnableSecureboot` | `utils/firstboot/secureboot.go` | `main.go` |
| `EnableTPM` | `utils/firstboot/tpm.go` | `main.go` |
| `EnableWebmin` | `utils/firstboot/webmin.go` | `main.go` |
| `EnablePodman` | `chrootPhase/features/podman.go` | `packages.go` + `chroot_phase.go` |
| `EnableHardening` | `chrootPhase/features/hardening.go` | `packages.go` + `chroot_phase.go` |
| `EnableNix` | `chrootPhase/features/nix.go` | `packages.go` + `chroot_phase.go` |
| `EnableZswap` | `chrootPhase/features/zswap.go` + `zswap_server.go` | `packages.go` + `chroot_phase.go` |
| `EnableEdgeRepos` | `chrootPhase/features/edgerepos.go` | `packages.go` (tail of `SetupPackages`) |
| `EnableDebug` | `chrootPhase/features/hardening.go` | via `HardeningPackages()` |
| `EnableSwap` | `hostPhase/disks/*` | `host_phase.go` (disk layout layer) |
| `DesktopEnvironment` | `chrootPhase/desktops/desktops.go` | `chroot_phase.go` |

**User-session daemons = supervised services (Artix/Chimera model, user rule 2026-10-16):** EVERY user-session daemon runs as a supervised per-user dinit service (`.user`, started by the turnstile-spawned user dinit) — NOTHING is left to unsupervised D-Bus activation, autostart or daemon self-spawn. If Alpine lacks the `-user-dinit` package, one is created in feralos-aports (e.g. `oo7-user-dinit`, `xdg-desktop-portal-user-dinit`). Applies to BOTH inits (OpenRC gets the identical user-dinit stack via turnstile).

**Desktop environments supported:**
- `niri` — custom config (greetd, regreet, niri config, DMS, xdg-su) in `chrootPhase/desktops/niri.go`; session wiring = turnstile (PAM → user dinit) + `niri-session` via login-shell trick (`-s /usr/libexec/feralos/login-shell`, not in `/etc/shells`, symlink re-pointed to fish — busybox applet dispatch breaks the package's `/bin/ash` target; user `chsh` skipped) + `niri`/`niri-shutdown` user services (`niri-user-dinit`); installer enables `/etc/dinit.d/boot.d/turnstiled` + user `boot.d` stack in `/usr/lib/dinit.d/user/boot.d/`
  - Session startup: real path = greetd/regreet → `niri-session` (fixed dinit branch shipped by `niri-user-dinit`, `replaces="niri"`: full login-env setenv + dinit-monitor wait — upstream post-25.11 mechanism). greetd autologin exists ONLY as a harness test fixture (simulates the real login); the installer never writes `[initial_session]`. Harness = real-life simulation; installer = the product; harness-discovered fixes go to installer/aports, never the harness.
- `niriWayle` — **the flagship/recommended desktop** (the only DE developed and verified as a complete daily-driver experience; the others ship for user convenience): custom config in `chrootPhase/desktops/niri_wayle.go` (niri.go derivation minus DMS), Wayle shell + Veila + Raffi + oo7; launcher TOGGLES via `raffi-toggle` (feralos-aports — `/usr/bin/raffi-toggle`: close the running raffi window via niri IPC or spawn a new detached instance via setsid; wired into the Wayle bar custom module `left-click` + the niri Mod+D bind; one-way `raffi-toggle → raffi` dep — the raffi→raffi-toggle direction would be an unbuildable build-order cycle, the installer lists both packages explicitly; abuild forbids `/usr/local` in packages — the user spec's `/usr/local/bin` path corrected to `/usr/bin` during implementation, CI run 41153877924); **window switcher** Mod+W + bar window-list module (`[[modules.custom]] id = "windows"`, niri IPC watcher, `left-click = "raffi -i 'win '"` — raffi 0.26.0 initial query; harness `wayle_session_actions` asserts both); Key Rack secrets GUI via Flathub flatpak (`app.drey.KeyRack`, NOT an apk — seahorse conflicts with oo7-server on org.freedesktop.secrets.service), portals Secret interface pinned to `oo7-portal`; oo7 login auto-unlock via PAM (`oo7_pam_unlock` harness check — authenticated PAM proof); same turnstile/login-shell session wiring. Wayle session actions wired (installer seeds `~/.config/wayle/config.toml` — the dashboard button (far right) opens the dashboard dropdown whose lock/logout/reboot/poweroff buttons run FeralOS commands; `left-click = "dropdown:dashboard"` pinned explicitly (upstream default equals it — layered config keeps per-field defaults on partial tables, verified live incl. a real VNC click opening the panel, run `2026-10-28T00-10-10Z`); reboot via the `zz-power` NOPASSWD path; harness `wayle_session_actions`); `stasis` context-aware idle manager (supervised `stasis.user`; FeralOS system config `/etc/stasis/stasis.rune` via `-c` — veila lock → niri dpms → loginctl suspend; harness `stasis_idle`); veilad runs through the `veilad-session` env wrapper (harness `veila_daemon`); greeter session uniqueness — the installer appends `NoDisplay=true` to `/usr/share/wayland-sessions/niri.desktop` on niriWayle installs so regreet lists ONLY the niriWayle session (real-hw finding: the upstream bare `niri` session showed next to it; ReGreet 0.2.0 respects the Desktop Entry Hidden/NoDisplay fields; apk-safe — survives niri upgrades as `.apk-new`; harness `niriwayle_session_unique`). Ships the **full niri app set** (browser/office/utilities + printing/scanning support + the niri flatpaks alongside Key Rack) — user request 2026-10-19; DMS shell packages and seahorse/gnome-keyring are intentionally not copied.
- `kde` — packages only via `internal_rules.yaml`, no custom config
- `cosmic` — packages only via `internal_rules.yaml` (uses seatd, not elogind), no custom config

**Dinit services** ship `/etc/dinit.d/config/<svc>.conf` (Artix pattern, ADR-008): service → `/usr/lib/dinit/<svc>` wrapper → sources config → exec daemon. User-editable, apk-protected; service conventions per ADR-009 (no restart, before=login.target, pre-network ordering, dbus-wait-for readiness).

→ Details: `docs/architecture/feature_confinement.md`, `docs/guides/development.md` (how to add feature)

## D-Bus Activation Rule (ADR-010)

**Rule: every D-Bus-activatable daemon is a dinit service; activation only triggers dinit.** No exceptions, no per-application special cases (user directive 2026-10-18; decision `docs/adr/ADR-010-dbus-activation-dinit.md`, mechanism + checklist `docs/architecture/dbus-activation.md`).

- **Unit name follows WHO activates the service in FeralOS (R1, amended 2026-10-25):** user (session) services = canonical bus name + `.user` (`/etc/dinit.d/user/`) — dbus/turnstile-activated on demand, never boot-enabled by us; system services WE boot-enable (an OpenRC initd exists) = exactly the OpenRC initd name (`/lib/dinit.d/<initd-name>`) — the `services` map alone drives both inits (rc-update + boot.d symlink), no corrective mapping, and `dinitServices` stays EMPTY (mechanism kept for exceptional cases only); system services dbus-activated on demand (never boot-enabled — dbus decides when they run) = canonical bus name (`/lib/dinit.d/<Name>`) — fwupd, PPD, upower, udisks2, avahi, wpa_supplicant, nm_dispatcher, accountsservice, bolt, cups-pk-helper, flatpak SystemHelper, fprintd, geoclue, kcron, the Plasma KAuth helpers. fwupd/PPD were boot-enabled before the amendment — corrected to activation-only on both inits. Aliases (e.g. `net.hadess.PowerProfiles` → `org.freedesktop.UPower.PowerProfiles`) are separate activation files mapping onto the same canonical unit.
- Bus names stay on the D-Bus side in a rename: override filenames + `Name=`, `dbus-wait-for -n` arguments, polkit action ids never change — the override `Exec=` aliases the bus name onto the unit whatever it is called.
- The effective `.service` file carries `Exec=/usr/libexec/feralos/dbus-activate <unit> -- <upstream exec>`; overrides ship in `/usr/share/feralos/dbus-system-services` (system) + `/usr/share/feralos/dbus-services` (session) and are materialized into the effective servicedirs: `/usr/local/share/dbus-1/system-services` at boot via `dbus-services-setup` (the system bus activates through the setuid launch helper, which resolves names ONLY against the standard dirs — `/etc/dbus-1/system-services` is daemon-enumerated but NOT helper-activatable on dbus 1.16.2) and `$XDG_RUNTIME_DIR/dbus-1/services` at login via the `dbus-session` wrapper; upstream files untouched.
- Shim: forwards the activation environment to dinit (single `dinitctl setenv`) then `dinitctl start` (waits) and exits; OpenRC system-bus fallback = upstream exec when no system dinit; session bus always user dinit (both inits).
- Enforcement: harness `dbus_activation_uniform` (override sources + boot materialization + unit exists; fails if anything is left in the helper-ineligible `/etc/dbus-1/system-services`) + `dbus_activation_env` (real activation proof: unit started with no `DBUS_STARTER_*` in the daemon environ, plus `setenv` forwarding via a direct shim marker run) + `dbus_activation_migration` (forward-looking retirement gate: enumerates every activation file live, fails only when dbus-broker is installed AND every target is unit-covered) + `dbus_system_activation` (system-bus coverage on EVERY DE, dinit-only).
- Adding a new activatable service: follow the checklist in `docs/architecture/dbus-activation.md` (aports mirror: `DBUS-ACTIVATION.md` in `feralos-aports`); the same doc carries the verification/troubleshooting commands (`ListActivatableNames` ≠ activatable; the `DBUS_STARTER_*` check; raw-spawn diagnosis).
- `dbus-services-setup` is the reference boot-time oneshot (both inits, ordered before a consumer) — reusable pattern + gotchas: `docs/architecture/boot-oneshots.md`.

## Compatibility

No backward compat. Breaking changes allowed without fallback (e.g., breaking changes to `internal_rules.yaml` without deprecated path). Installer is single-run.

→ Details: `docs/adr/ADR-006-no-backward-compat.md`

## FirstBoot Architecture

Activatable features (Secure Boot, TPM, Webmin) use standalone binaries in `src/firstBoot/`:

- `src/firstBoot/secureboot/main.go` — autonomous: installs packages, generates EFI keys, enrolls sbctl, builds+signs UKI, writes hooks
- `src/firstBoot/tpm/main.go` — autonomous: checks TPM, installs clevis@edge + tpm2-tools, binds LUKS to TPM2, rebuilds initramfs+UKI
- `src/firstBoot/webmin/main.go` — autonomous: verifies podman, creates `/opt/webmin`, generates compose + module configs, seeds `/etc/webmin` from image (`webmin-seed` container), starts via `podman compose up -d`, creates `webmin-ensure` oneshot (dual-init: PID1 via `cat /proc/1/comm` → dinit `/etc/dinit.d/webmin-ensure` + `/usr/libexec/dinit-webmin-ensure` + boot.d link, OpenRC `/etc/init.d/webmin-ensure`), sets root password via `changepass.pl` + `chpasswd`

**Installer flow**:

1. `firstboot.SetupSecureboot(conf)` / `firstboot.SetupTpm(conf)` / `firstboot.SetupWebmin(conf)` compile binaries → copy to `/opt/feralos_*`
2. `firstboot.WriteInstallConfig(conf)` writes `/opt/feralos/config.yaml` (disk layout, LUKS UUIDs, user info)
3. Binaries are autonomous — installer does NOT run them

**3-boot harness flow**:

1. **Runner (Debian)**: installer runs, compiles binaries, writes config. swtpm always present.
2. **Target (Alpine) — first boot**: harness SSH runs `feralos_secureboot` + `feralos_tpm` + `feralos_webmin` (if enabled). Binaries install packages, configure features, bind LUKS to TPM2, start the Webmin container. swtpm always present.
3. **Target (Alpine) — verification**: harness checks sbctl status, clevis bindings, LUKS auto-unlock, `podman ps` + `wget https://localhost:10000/` (Webmin). swtpm always present.

**Key rules**:

- swtpm is always passed to runner + target VMs (even if EnableTPM=false) — simplifies harness
- firstBoot binaries read `/opt/feralos/config.yaml` for disk layout + LUKS UUIDs (fallback: kernel cmdline)
- On real hardware: user runs `/opt/feralos_secureboot`, `/opt/feralos_tpm` and `/opt/feralos_webmin` manually after first boot (webmin is idempotent — seeds config once, restarts container; podman must be installed)

**Known limitation**: Secure Boot enrollment cannot be confirmed in QEMU (no real efivarfs). TPM auto-unlock CAN be confirmed and works reliably.

→ Details: `docs/architecture/firstboot.md` (hostPhase/firstBoot design), `docs/guides/first_boot.md` (real hardware steps), `docs/guides/webmin.md` (webmin compose/seed), `docs/guides/podman.md` (btrfs driver), `docs/FIRST_BOOT.md` is deprecated → `guides/first_boot.md`

## Project Structure

Full tree → `docs/INSTALLER.md#project-structure` and `docs/architecture/overview.md#project-structure`. Pointer only here to keep file concise, but info kept (double documentation).

## Docs Index (double documentation — AGENTS.md keeps full info, copies live in `docs/`)

- **Architecture**: `docs/architecture/overview.md` (principle + tree), `isolation.md` (entry-point + internal rule + hostPhase order), `logging.md` (ExecuteCmd rule), `feature_confinement.md` (single-owner), `storage.md` (layouts: btrfsStandardServer, btrfsStandard, btrfsRaid0, btrfsUnencrypted), `firstboot.md` (secureboot/tpm 3-boot), `dbus-activation.md` (ADR-010 mechanism + new-service checklist + diagnostics), `boot-oneshots.md` (both-init boot oneshot pattern) — all double of AGENTS.md above
- **ADRs**: `docs/adr/README.md` → ADR-001 Go installer, 002 isolation, 003 internal rule, 004 ExecuteCmd, 005 LUKS+Btrfs+booster+UKI, 006 no compat, 007 dinit shutdown ordering (decisions from AGENTS.md)
- **Guides** (user guides only): `docs/guides/README.md` (index) → `getting_started.md` (real hardware), `first_boot.md` (secureboot/tpm post-install), `variables.md` (variables.yaml compilation), `niriwayle.md` (flagship niriWayle desktop — Wayle/Veila/Raffi/oo7 + PAM unlock), `niri.md` (niri + NetworkManager + wayvnc), `webmin.md` (podman-based web admin), `podman.md` (btrfs driver), `development.md` (how to code / separation)
- **Harness**: `docs/harness/README.md` (index) → `overview.md` (what it does), `workflow.md` (3-phase + 3-boot), `artifacts.md` (where generated, gitignored), `building.md` (where compiled binary goes), `development.md` (how to code), `commands.md` (CLI+MCP), `debug_run.md` (`tests/debug_run/` boot existing run, LUKS inject). `tests/harness/README.md` is pointer only.
- **Intent / Ideas**: `docs/intent/vision.md`, `docs/intent/goals.md`, `docs/intent/vocabulary.md`, `docs/ideas/backlog.md` ← `CONTEXT.md` / `ROADMAP.md`
- **Project context / inventory**: `docs/memory_safe_alternatives.md` (detailed per-package memory-safety inventory; condensed table → root `CONTEXT.md`), `docs/FERALOS_NAME.md` (name rationale)
- **AI tooling environment** (how this repo's agents/opencode setup is installed and updated — not FeralOS product docs): `docs/agents_guide.md` (agent roster + daily workflow), `docs/global_setup.md` (global opencode/caveman/cavecrew bootstrap), `docs/update.md` (update procedures)

→ All `docs/` files are double of AGENTS.md sections above. AI agents: read AGENTS.md first for full context, then follow `→` pointers for deep dives.

## Roadmap

| Phase | Milestone | Status |
|-------|-----------|--------|
| Phase 1 | btrfsStandardServer, btrfsStandard, btrfsRaid0 | Completed |
| Phase 1 | Generator, Test harness, Nix | Completed |
| Phase 1 | btrfsUnencrypted | Completed |
| Phase 1 | niri + NetworkManager + wayvnc + firewall + webmin/podman | Completed |
| Phase 1 | niriWayle flagship desktop (Wayle + Veila + Raffi + oo7) | Completed |
| Phase 1 | Refining | In progress |
| Phase 2 | Dinit (shipped, selectable via `enableDinit`), Custom repo (live) | In progress |
| Phase 2 | Full release (Dinit as default) | Planned |

See `ROADMAP.md` for pipeline priorities. Memory-safe alternatives table → `CONTEXT.md`. Backlog → `docs/ideas/backlog.md`.

## Known Issues

All prior swap/vector/dummy issues resolved. See `CHANGELOG.md` Fixed section. Current: `log_errors` shows QEMU-only noise (`smartd`/`fsck` whitelisted; netns `mount/umount /run/netns ... Invalid argument` QEMU early-chain noise allowlisted; udevd `/bin/false` module install errors — parport/psmouse/intel_gtt/input_leds/intel_pmc_bxt — from OpenRC udev path, harmless in QEMU, nix pivot fix landed). Resolved 2026-09-30: dinit shutdown ordering — `turnstiled` pam_elogind bus error at shutdown (dinit releases dependencies at dependent stop-start, not exit) fixed via feralos-aports `stop-command` guards on `dbus`/`elogind` (ADR-007, commit `75c566a`; harness 63/63 run 2026-09-30T00-08-04Z).
