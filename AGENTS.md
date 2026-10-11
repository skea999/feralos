# Project Context

**Stack**: Go 1.22 (installer) + Shell (legacy reference) + YAML config
**Project**: FeralOS — Alpine-based distribution, memory safety + security, Nix for glibc, Dinit replaces OpenRC later, custom repo
**Status**: Installer Phase 1

> **Primary context for AI agents.** This file keeps full project info. Detailed copies live in `docs/` (double documentation). Read this file first, then follow `→` pointers for deep dives.

## Recent Rounds (latest first)

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

### 2026-10-12 01:00 - dms: window-buttons plugin wired system-wide (bar back on top)
**Agent:** codebase
**Summary:** User decisions: keep runningApps, put the plugin on the RIGHT, bar back on TOP; plugin developed in a NEW repo then imported to aports.
- **Plugin repo** `github.com/skea999/dms-window-buttons` (NEW, ISC): `plugin.json` + `WindowButtons.qml` (PluginComponent; maximize-to-edges + fullscreen + close for the focused window via NiriService IPC / `toplevel.close()`; hidden with no focused window; minimize intentionally absent — niri has none, DMS hides it too). Tag `v1.0.0` (`6af94ee`).
- **aports** `dms-window-buttons` 1.0.0 (`b8604c1`, CI green): installs to `/etc/xdg/quickshell/dms-plugins/dmsWindowButtons/` (DMS system plugin dir — no per-user seed needed for the files).
- **Installer** (`niri.go`): settings.json seed — `dankBarAtBottom` REMOVED (v1→v2 migration default = Top), `dankBarRightWidgets` pinned (defaults + `dmsWindowButtons` at the end); NEW `plugin_settings.json` seed `{"dmsWindowButtons":{"enabled":true}}` (widget plugins default-disabled; file shape verified against 1.6.2 `parsePluginSettings` — bare map, no wrapper). `internal_rules`: niri packages += `dms-window-buttons`.
- **Harness**: `dms_window_buttons` check (plugin files + enabled + right-widget wiring).

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
- **Docs**: `docs/guides/niri.md` (turnstile paragraph + session services list). dms-shell split (`dms` + `dms-dinit`) still PENDING (user parked it).
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
- NEXT: user's real-hw validation (keymap + rendering on real GPU); dms windows depth; booster initramfs module policy parked (booster = host-specific by default; user force_load list sufficient — no universal mode).

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
ls tests/fixtures/variables/  # 28 fixtures, full variables.yaml template mirror (all scenarios)

# Run tests (when available)
cd src && go test ./...
```

→ Details: `docs/guides/development.md` (build/test/lint + conventions), `docs/harness/commands.md` (CLI), `docs/harness/workflow.md` (3-phase flow)

## Test Artifacts

Output to `tests/artifacts/<tool-name>/<timestamp>/`:

```
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

**Dupl exclusions** (intentional duplication): `src/hostPhase/disks/` (layout packages), `src/utils/firstboot/` (standalone binaries). Config: `.golangci.yml` → `dupl.threshold: 100` + `exclude-rules`.

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

**Always use `shell.ExecuteCmd` in `hostPhase`/`chrootPhase` — never `os.*`.** File ops (`MkdirAll`, `Create`, `WriteFile`, `ReadFile`) and `exec.Command` bypass logging. `ExecuteCmd` logs CMD, stdout/stderr, exit, retry, and `SKEA_NONINTERACTIVE` handling. `import "os"` forbidden in those packages (exceptions: `firstBoot/` standalone, `utils/shell` impl). Host: `shell.ExecuteCmd("", "mkdir -p /mnt/etc")`. Chroot: `shell.ExecuteCmd(chroot, "mkdir -p /etc")`.

→ Details: `docs/architecture/logging.md`, `docs/adr/ADR-004-executecmd-logging.md`, `src/utils/shell/execute.go`

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

**Desktop environments supported:**
- `niri` — custom config (greetd, regreet, niri config, DMS, xdg-su) in `chrootPhase/desktops/niri.go`; session wiring = turnstile (PAM → user dinit) + `niri-session` via login-shell trick (`-s /usr/libexec/feralos/login-shell`, not in `/etc/shells`, symlink re-pointed to fish — busybox applet dispatch breaks the package's `/bin/ash` target; user `chsh` skipped) + `niri`/`niri-shutdown` user services (`niri-user-dinit`); installer enables `/etc/dinit.d/boot.d/turnstiled` + user `boot.d` stack in `/usr/lib/dinit.d/user/boot.d/`
  - Session startup: real path = greetd/regreet → `niri-session` (fixed dinit branch shipped by `niri-user-dinit`, `replaces="niri"`: full login-env setenv + dinit-monitor wait — upstream post-25.11 mechanism). greetd autologin exists ONLY as a harness test fixture (simulates the real login); the installer never writes `[initial_session]`. Harness = real-life simulation; installer = the product; harness-discovered fixes go to installer/aports, never the harness.
- `kde` — packages only via `internal_rules.yaml`, no custom config
- `cosmic` — packages only via `internal_rules.yaml` (uses seatd, not elogind), no custom config

**Dinit services** ship `/etc/dinit.d/config/<svc>.conf` (Artix pattern, ADR-008): service → `/usr/lib/dinit/<svc>` wrapper → sources config → exec daemon. User-editable, apk-protected; service conventions per ADR-009 (no restart, before=login.target, pre-network ordering, dbus-wait-for readiness).

→ Details: `docs/architecture/feature_confinement.md`, `docs/guides/development.md` (how to add feature)

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
2. **Target (Alpine) — first boot**: harness runs `feralos_secureboot` + `feralos_tpm` via SSH. Binaries install packages, configure features, bind LUKS to TPM2. swtpm always present.
3. **Target (Alpine) — verification**: harness checks sbctl status, clevis bindings, LUKS auto-unlock. swtpm always present.

**Key rules**:

- swtpm is always passed to runner + target VMs (even if EnableTPM=false) — simplifies harness
- firstBoot binaries read `/opt/feralos/config.yaml` for disk layout + LUKS UUIDs (fallback: kernel cmdline)
- On real hardware: user runs `/opt/feralos_secureboot` and `/opt/feralos_tpm` manually after first boot

**Known limitation**: Secure Boot enrollment cannot be confirmed in QEMU (no real efivarfs). TPM auto-unlock CAN be confirmed and works reliably.

→ Details: `docs/architecture/firstboot.md` (hostPhase/firstBoot design), `docs/guides/first_boot.md` (real hardware steps), `docs/guides/webmin.md` (webmin compose/seed), `docs/guides/podman.md` (btrfs driver), `docs/FIRST_BOOT.md` is deprecated → `guides/first_boot.md`

## Project Structure

Full tree → `docs/INSTALLER.md#project-structure` and `docs/architecture/overview.md#project-structure`. Pointer only here to keep file concise, but info kept (double documentation).

## Docs Index (double documentation — AGENTS.md keeps full info, copies live in `docs/`)

- **Architecture**: `docs/architecture/overview.md` (principle + tree), `isolation.md` (entry-point + internal rule + hostPhase order), `logging.md` (ExecuteCmd rule), `feature_confinement.md` (single-owner), `storage.md` (layouts: btrfsStandardServer, btrfsStandard, btrfsRaid0, btrfsUnencrypted), `firstboot.md` (secureboot/tpm 3-boot) — all double of AGENTS.md above
- **ADRs**: `docs/adr/README.md` → ADR-001 Go installer, 002 isolation, 003 internal rule, 004 ExecuteCmd, 005 LUKS+Btrfs+booster+UKI, 006 no compat, 007 dinit shutdown ordering (decisions from AGENTS.md)
- **Guides** (user guides only): `docs/guides/README.md` (index) → `getting_started.md` (real hardware), `first_boot.md` (secureboot/tpm post-install), `variables.md` (variables.yaml compilation), `niri.md` (niri + NetworkManager + wayvnc), `webmin.md` (podman-based web admin), `podman.md` (btrfs driver), `development.md` (how to code / separation)
- **Harness**: `docs/harness/README.md` (index) → `overview.md` (what it does), `workflow.md` (3-phase + 3-boot), `artifacts.md` (where generated, gitignored), `building.md` (where compiled binary goes), `development.md` (how to code), `commands.md` (CLI+MCP), `debug_run.md` (`tests/debug_run/` boot existing run, LUKS inject). `tests/harness/README.md` is pointer only.
- **Intent / Ideas**: `docs/intent/vision.md`, `docs/intent/goals.md`, `docs/intent/vocabulary.md`, `docs/ideas/backlog.md` ← `CONTEXT.md` / `ROADMAP.md` / `TASKS.md`

→ All `docs/` files are double of AGENTS.md sections above. AI agents: read AGENTS.md first for full context, then follow `→` pointers for deep dives.

## Roadmap

| Phase | Milestone | Status |
|-------|-----------|--------|
| Phase 1 | btrfsStandardServer, btrfsStandard, btrfsRaid0 | Completed |
| Phase 1 | Generator, Test harness, Nix | Completed |
| Phase 1 | btrfsUnencrypted | Completed |
| Phase 1 | niri + NetworkManager + wayvnc + firewall + webmin/podman | Completed |
| Phase 1 | Refining | Planned |
| Phase 2 | Dinit, Custom repo, Full release | Planned |

See `ROADMAP.md` for pipeline priorities. Memory-safe alternatives table → `CONTEXT.md`. Backlog → `docs/ideas/backlog.md`.

## Known Issues

All prior swap/vector/dummy issues resolved. See `CHANGELOG.md` Fixed section. Current: `log_errors` shows QEMU-only noise (`smartd`/`fsck` whitelisted; netns `mount/umount /run/netns ... Invalid argument` QEMU early-chain noise allowlisted; udevd `/bin/false` module install errors — parport/psmouse/intel_gtt/input_leds/intel_pmc_bxt — from OpenRC udev path, harmless in QEMU, nix pivot fix landed). Resolved 2026-09-30: dinit shutdown ordering — `turnstiled` pam_elogind bus error at shutdown (dinit releases dependencies at dependent stop-start, not exit) fixed via feralos-aports `stop-command` guards on `dbus`/`elogind` (ADR-007, commit `75c566a`; harness 63/63 run 2026-09-30T00-08-04Z).
