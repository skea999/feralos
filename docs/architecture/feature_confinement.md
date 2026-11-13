# Activatable Feature Confinement

**Canonical detail source.** `AGENTS.md` carries the rule summary + confinement table and points here; this file holds the full detail (callers, pattern, map, user-session daemon rule, desktop environment inventory, dinit service conventions).

## Rule

**Every activatable feature MUST be confined to a single owner file.**

Each `Enable*` flag (and `DesktopEnvironment`) is owned by exactly ONE file under `src/chrootPhase/features/` or `src/utils/firstboot/` or `src/chrootPhase/desktops/`. All logic — packages, configuration, guards — lives in that file.

## Callers Must NOT Contain Feature Logic

- `packages.go` → calls `features.XxxPackages(conf)`, never inline feature packages
- `logic.go` → zero feature-specific logic, only generic rule application
- `chroot_phase.go` → calls `features.SetupXxx(conf)`, never feature configuration
- `host_phase.go` → calls `features.SetupXxx(conf)`, never feature compilation

Violation = feature logic leaked outside owner.

## Pattern for New Features

1. Create `src/chrootPhase/features/<name>.go` (or `src/utils/firstboot/` for compile+copy)
2. Export `XxxPackages(conf) []string` — returns APK packages needed
3. Export `SetupXxx(conf) error` — configures feature inside chroot
4. Guard: `if !conf.User.EnableXxx { return nil }`
5. Call from `packages.go` (packages) and `chroot_phase.go` or `host_phase.go` (setup)
6. NEVER mutate flags from outside the feature file (exception: `NixPackages()` auto-enables internally)

## Confinement Map

| Feature | Owner file | Called from |
|---------|-----------|-------------|
| `EnableSecureboot` | `utils/firstboot/secureboot.go` | `main.go` |
| `EnableTPM` | `utils/firstboot/tpm.go` | `main.go` |
| `EnablePodman` | `chrootPhase/features/podman.go` | `packages.go` + `chroot_phase.go` |
| `EnableHardening` | `chrootPhase/features/hardening.go` | `packages.go` + `chroot_phase.go` |
| `EnableNix` | `chrootPhase/features/nix.go` | `packages.go` + `chroot_phase.go` |
| `EnableZswap` | `chrootPhase/features/zswap.go` + `zswap_server.go` | `packages.go` + `chroot_phase.go` |
| `EnableEdgeRepos` | `chrootPhase/features/edgerepos.go` | `chroot_phase.go` (step 17, chroot tail — immediately after `SetupDbusAudit`, never inside `SetupPackages`) |
| `EnableDebug` | `chrootPhase/features/hardening.go` | via `HardeningPackages()` |
| `EnableSwap` | `hostPhase/disks/*` | `host_phase.go` (disk layout layer) |
| `DesktopEnvironment` | `chrootPhase/desktops/desktops.go` | `chroot_phase.go` |

## User-Session Daemons = Supervised Services (Artix/Chimera model, user rule 2026-10-16)

EVERY user-session daemon runs as a supervised per-user dinit service (`.user`, started by the turnstile-spawned user dinit) — NOTHING is left to unsupervised D-Bus activation, autostart or daemon self-spawn. If Alpine lacks the `-user-dinit` package, one is created in feralos-aports (e.g. `oo7-user-dinit`, `xdg-desktop-portal-user-dinit`). Applies to BOTH inits (OpenRC gets the identical user-dinit stack via turnstile).

## Desktop Environments Supported

- `niri` — custom config (greetd, regreet, niri config, DMS, xdg-su) in `chrootPhase/desktops/niri.go`; session wiring = turnstile (PAM → user dinit) + `niri-session` via login-shell trick (`-s /usr/libexec/login-shell`, not in `/etc/shells`, symlink re-pointed to fish — busybox applet dispatch breaks the package's `/bin/ash` target; user `chsh` skipped) + `niri`/`niri-shutdown` user services (`niri-user-dinit`); installer enables `/etc/dinit.d/boot.d/turnstiled` + user `boot.d` stack in `/usr/lib/dinit.d/user/boot.d/`
  - Session startup: real path = greetd/regreet → `niri-session` (fixed dinit branch shipped by `niri-user-dinit`, `replaces="niri"`: full login-env setenv + dinit-monitor wait — upstream post-25.11 mechanism). greetd autologin exists ONLY as a harness test fixture (simulates the real login); the installer never writes `[initial_session]`. Harness = real-life simulation; installer = the product; harness-discovered fixes go to installer/aports, never the harness. Admin GUIs are `ursine` panels; `kcron`/`ksystemlog`/`gnome-disk-utility`/`firewalld-gui` are not installed (Flatseal stays — Ursine does not edit Flatpak session-bus policies).
- `niriWayle` — **the flagship/recommended desktop** (the only DE developed and verified as a complete daily-driver experience; the others ship for user convenience): custom config in `chrootPhase/desktops/niri_wayle.go` (niri.go derivation minus DMS), Wayle shell + Veila + Raffi + oo7; launcher TOGGLES via `raffi-toggle` (feralos-aports — `/usr/bin/raffi-toggle`: close the running raffi window via niri IPC or spawn a new detached instance via setsid; wired into the Wayle bar custom module `left-click` + the niri Mod+D bind; one-way `raffi-toggle → raffi` dep — the raffi→raffi-toggle direction would be an unbuildable build-order cycle, the installer lists both packages explicitly; abuild forbids `/usr/local` in packages — the user spec's `/usr/local/bin` path corrected to `/usr/bin` during implementation, CI run 41153877924); **window switcher** Mod+W + bar window-list module (`[[modules.custom]] id = "windows"`, niri IPC watcher, `left-click = "raffi -i 'win '"` — raffi 0.26.0 initial query; harness `wayle_session_actions` asserts both); Key Rack secrets GUI via Flathub flatpak (`app.drey.KeyRack`, NOT an apk — seahorse conflicts with oo7-server on org.freedesktop.secrets.service), portals Secret interface pinned to `oo7-portal`; oo7 login auto-unlock via PAM (`oo7_pam_unlock` harness check — authenticated PAM proof); same turnstile/login-shell session wiring. Admin GUIs are `ursine` panels; `kcron`/`ksystemlog`/`gnome-disk-utility`/`firewalld-gui` are not installed (Flatseal stays — Ursine does not edit Flatpak session-bus policies). Daily-driver apps (2026-11-10): `evince`, `file-roller`, `thunderbird`, `gnome-maps`, `gnome-firmware`, `obs-studio`, `gst-libav` (H.265), `font-noto-emoji`, `dconf`, `xdg-desktop-portal-gtk` (the `default=gtk;` generic portal backend — rules list, see the 2026-11-10 round); the gnome portal backend is shadowed by an empty 999 aports dummy (`xdg-desktop-portal-gnome` — nautilus never installs; the installer seeds `default=gtk;` + wlr screenshot/screencast into the user portals conf); Ursine 0.27 is D-Bus-free (`depends="parted shadow util-linux"` — accountsservice/udisks2 gone from the install); supervision for the new activation targets = `file-roller-user-dinit` + `evince-user-dinit`. Wayle session actions wired (installer seeds `~/.config/wayle/config.toml` — the dashboard button (far right) opens the dashboard dropdown whose lock/logout/reboot/poweroff buttons run FeralOS commands; `left-click = "dropdown:dashboard"` pinned explicitly (upstream default equals it — layered config keeps per-field defaults on partial tables, verified live incl. a real VNC click opening the panel, run `2026-10-28T00-10-10Z`); reboot via the `zz-power` NOPASSWD path; harness `wayle_session_actions`); `stasis` context-aware idle manager (supervised `stasis.user`; FeralOS system config `/etc/stasis/stasis.rune` via `-c` — veila lock → niri dpms → loginctl suspend; harness `stasis_idle`); veilad runs through the `veilad-session` env wrapper (harness `veila_daemon`); greeter session uniqueness — the installer appends `NoDisplay=true` to `/usr/share/wayland-sessions/niri.desktop` on niriWayle installs so regreet lists ONLY the niriWayle session (real-hw finding: the upstream bare `niri` session showed next to it; ReGreet 0.2.0 respects the Desktop Entry Hidden/NoDisplay fields; apk-safe — survives niri upgrades as `.apk-new`; harness `niriwayle_session_unique`). Ships the **full niri app set** (browser/office/utilities + printing/scanning support + the niri flatpaks alongside Key Rack) — user request 2026-10-19; DMS shell packages and seahorse/gnome-keyring are intentionally not copied. **Branding** (2026-11-01): installer installs the FeralOS logo as a system hicolor icon (`/usr/share/icons/hicolor/scalable/apps/feralos-logo.svg` — the Wayle launcher custom module carries `icon-name = "feralos-logo"`; Adwaita inherits hicolor) + default wallpapers from the tracked repo-root `branding/` dir (`/usr/share/wallpapers/feralos-default.png` 1080p = the regreet background + `feralos-default-3440x1440.png` ultrawide) via `desktops/branding.go` (multi-candidate repo-path lookup, same pattern as createValues templatePaths; new logged `shell.ReadFile` host-side read counterpart of WriteFile) — shared by niri.go (regreet path was a dangling `/usr/share/wallpapers/default.png`, now real); missing assets = non-fatal WARNING. README top carries the logo.
- `kde` — packages only via `internal_rules.yaml`, no custom config
- `cosmic` — packages only via `internal_rules.yaml` (uses seatd, not elogind), no custom config

## Dinit Service Conventions

Dinit services ship `/etc/dinit.d/config/<svc>.conf` (Artix pattern, ADR-008): service → `/usr/lib/dinit/<svc>` wrapper → sources config → exec daemon. User-editable, apk-protected; service conventions per ADR-009 + Amendment 2026-11-08 (restart = on-failure default, smooth-recovery only on resident daemons, before=login.target, pre-network ordering, dbus-wait-for readiness).

## References

- `AGENTS.md#Activatable Feature Confinement`
- `docs/adr/README.md` → ADR-002 isolation
