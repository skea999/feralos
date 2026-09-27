# Niri — Wayland Compositor Desktop

> Niri desktop on FeralOS: greetd + regreet + niri + DMS (DankMaterialShell, native apk) + flatpak RustDesk.

## Overview

Niri is a scrollable-tiling Wayland compositor. FeralOS installs it via `desktopEnvironment: "niri"` in `variables.yaml`.

**DMS is native**: `dms-shell` 1.6.2 + `quickshell` 0.3.1 ship as musl apks from the `feralos-aports` repo. **No nix, no nixGL, no execfilter** in the desktop stack. The old `/etc/init.d/dms` OpenRC service is gone on purpose — it hung boot and duplicated the shell; DMS is spawned by the niri session (`spawn-at-startup`).

Includes (from `internal_rules.yaml:desktopEnvironments.niri`):
- Compositor: `niri`, `niri-portalsconf`, `xwayland-satellite`, `xwayland`, `mesa-dri-gallium`, `wayland-libs-server`
- Shell: `dms-shell` (1.6.2, feralos-aports), `quickshell` (0.3.1, feralos-aports)
- Greeter: `greetd` + `greetd-regreet` + `greetd-openrc` (vt 7, greeter session via `/etc/greetd/greeter.sh`), `accountsservice`
- Session: `elogind`, `dbus`, `xdg-desktop-portal-wlr/gtk`, `pipewire` + `pipewire-pulse` + `wireplumber`, `polkit-elogind` + `polkit-gnome`, `power-profiles-daemon`, `cups-pk-helper`, `alacritty`
- Apps: `cosmic-files/edit/screenshot/player/store/settings`, `okular`, `eog`, `cups/cups-filters/avahi/system-config-printer`, `libreoffice`, `galculator`, `firefox`, `firewalld-gui`, `gnome-firmware`, `thunderbird`, `gnome-disk-utility`, `kcron`, `ksystemlog`, `rclone`
- Flatpak: `flathub:com.rustdesk.RustDesk` (conditional — only when `flatpakPackages` contains rustdesk)
- System: `xdg-user-dirs` (portal access), `firewalld` service, `iptables/ip6tables/ebtables` (firewalld direct-rule backends)
- Services: `greetd`, `elogind`, `firewalld`, `polkit`, `cupsd` (all `default`)
- Groups: `video`, `input`, `audio`

> Notes: `cava` (DMS audio visualizer) not on Alpine 3.24 — re-add when packaged. `turnstile` edge-only. `matugen`/`rclone-ui` not on Alpine — DMS degrades gracefully. `render` group does not exist on Alpine (video covers DRM). No `nixPackages` — `enableNix` is an independent feature.

> Wayvnc removed — per-session `spawn-at-startup "wayvnc"` dropped in favor of on-demand remote. Use flatpak RustDesk for remote.

## Installation

Niri installed automatically when `desktopEnvironment: niri` in `variables.yaml`:

```yaml
desktopEnvironment: "niri"
```

Desktop packages pulled via `utils/logic.ApplyRules` from `internal_rules.yaml` (`desktopEnvironments.niri.packages`, `flatpakPackages`, `services`, `groups`) + `chrootPhase/desktops/niri.go:SetupNiri`.

The feralos repo bootstrap is **unconditional** (desktop packages like `dms-shell`/`quickshell` live there too, not only nix).

Manual (chroot):
```bash
apk add niri niri-portalsconf dms-shell quickshell greetd greetd-regreet xdg-user-dirs firewalld
rc-update add greetd default
rc-update add elogind default
rc-update add firewalld default
```

## Greeter + Display Manager

- `greetd` config: `/etc/greetd/config.toml` → `vt 7`, `command = "/etc/greetd/greeter.sh"`, `user = "greeter"`
- `greeter.sh`: handles `XDG_RUNTIME_DIR` fallback (`/tmp/greetd-runtime` when elogind not yet up), then `exec niri -- regreet`. **No software-GL**: real hardware uses system mesa + real GPU (no `LIBGL_ALWAYS_SOFTWARE`, no `QT_QUICK_BACKEND`, no `WLR_NO_HARDWARE_CURSORS`)
- `regreet.toml`: `/etc/greetd/regreet.toml` — timezone from `variables.yaml` (`timezone`), reboot = `["reboot"]`, poweroff = `["poweroff"]` (not `systemctl` — no systemd)
- Greeter niri config: `/var/lib/greetd/.config/niri/config.kdl` — keymap from `variables.yaml` (`keymap`), hotkey overlay skipped at startup
- Greeter user: `greeter` (`/var/lib/greetd` home — greetd apk does not create it, installer does), groups `video,input,greetd` (greetd re-owns `/var/lib/greetd` 0750 at boot — traversal needed)
- `/var/lib/regreet`: `700 greeter:greeter` — ReGreet state dir, without it last user/session never remembered

## User Session

- **niri-session wrapper**: `/usr/local/bin/niri-session` (wins over `/usr/bin`) — greetd provides neither the session env nor a D-Bus session bus. POSIX script: exports `XDG_SESSION_TYPE=wayland` + `XDG_CURRENT_DESKTOP=niri`, starts `dbus-daemon --session` on `$XDG_RUNTIME_DIR/bus` if missing, exports `DBUS_SESSION_BUS_ADDRESS`, then hands off to `/usr/bin/niri-session`
- User niri config: `/home/<user>/.config/niri/config.kdl` — keymap from `variables.yaml`, wayland env (`XDG_SESSION_TYPE wayland`, `QT_QPA_PLATFORM wayland`, `QT_QPA_PLATFORMTHEME gtk3`, `SDL_VIDEO_DRIVER wayland`, etc.), session spawns (greetd starts none of them):
  - `spawn-at-startup "pipewire"`
  - `spawn-at-startup "pipewire-pulse"`
  - `spawn-at-startup` wait-loop: waits for `pipewire` pgrep, then `wireplumber >>$HOME/.local/log/user/wireplumber.log 2>&1`
  - `spawn-at-startup`: `mkdir -p $HOME/.local/log/user; exec dms run >>$HOME/.local/log/user/dms.log 2>&1`
  - `spawn-at-startup "/usr/lib/polkit-gnome/polkit-gnome-authentication-agent-1"` — MUST be the Alpine/elogind build
  - `xwayland-satellite { path "xwayland-satellite" }` block — XWayland for X11 apps
  - `include "dms/binds.kdl"` + `dms/layout.kdl` + `dms/colors.kdl`
  - RustDesk autostart (conditional, see below)
- **DMS fragments**: pre-created before first niri validation via `su - <user> -c 'dms setup binds|layout|colors'` (packaged CLI). Config ownership fixed BEFORE `dms setup` (installer mkdir/writes run as root — without `chown -R`, `dms setup` cannot create `.config/niri/dms/` and every fragment fails)
- **background-effect strip**: DMS 1.6 layout template emits `background-effect { xray false }` — niri node added after 25.11 (`ext-background-effect-v1`, unsupported by Alpine 3.24 niri) that breaks the whole include. Installer strips the block via awk (`blur` unsupported on this niri anyway)
- **User portals conf**: `/home/<user>/.config/xdg-desktop-portal/niri-portals.conf` — `[preferred] org.freedesktop.impl.portal.ScreenCast=wlr;` (system default `gnome;gtk` breaks ScreenCast)
- **Session logs**: `~/.local/log/user/dms.log` + `~/.local/log/user/wireplumber.log` (dir pre-created — missing dir = spawn redirect dies)
- **Groups**: user `video,input` (rules) + `audio` (`/dev/snd` for PipeWire) via `usermod -aG`; seat access via elogind D-Bus
- **RustDesk autostart (conditional)**: when `flatpakPackages` contains `rustdesk` (default via `internal_rules.yaml` niri `flatpakPackages: ["flathub:com.rustdesk.RustDesk"]`), user config appends:
  ```
  spawn-at-startup "flatpak" "run" "com.rustdesk.RustDesk" "--service"
  ```
  Implemented in `chrootPhase/desktops/niri.go` — `hasRustdesk` check over `conf.User.FlatpakPackages`. No autostart when flatpakPackages empty.
- **Firewall ports for RustDesk direct IP**: `21115-21119/tcp` + `21116/udp` opened via `firewall-offline-cmd --zone=public --add-port=...` (not `firewall-cmd` — `firewalld` not running in chroot). Applied inside `SetupNiri` when `hasRustdesk` true, with `|| true` fallback.
- `xdg-su` wrapper: `/usr/local/bin/xdg-su` — niri unsupported by xdg-su, uses `alacritty -e su -c` for `*niri*` desktops

## Flatpak + RustDesk

- APK: `flatpak` added when `flatpakPackages` non-empty (`chrootPhase/features/flatpak.go:FlatpakPackages`)
- Setup: `features.SetupFlatpak` — `flatpak remote-add --if-not-exists flathub`, then `flatpak install -y --noninteractive <remote> <appID>` per entry (`flathub:com.rustdesk.RustDesk` split on `:`)
- **/lib/modules handling**: `src/chrootPhase/internal/packages.go:SetupPackages` ensures `mkdir -p /lib/modules` **before** `apk update`/`apk add` (flatpak and others check kernel module path)
- **xdg-user-dirs**: added to `standardPackages` (always installed) for flatpak portal `XDG` directory access — required for RustDesk file transfer portal
- Harness checks: `flatpak_installed` (`flatpak list | grep rustdesk`), `firewall_rustdesk` (`firewall-cmd --permanent --list-ports | grep 21115` or `grep -rq 21115 /etc/firewalld/`)

## Webmin + Podman Integration

Webmin is **podman-based** (not host package):

- Requires `enablePodman: true` + `enableWebmin: true` (compose needs podman)
- Podman: `btrfs` driver via `sed` on `/etc/containers/storage.conf`, `cgroups` + `podman` services, `iptables`, `fuse-overlayfs` fallback, `podman-compose`+`podman-docker`, `docker→podman` dummy
- Webmin binary: standalone `src/firstBoot/webmin/main.go` compiled to `/opt/feralos_webmin`
- Container: `demosthenesk/webmin:noble` image, `read_only: true` + `tmpfs`, seed `/etc/webmin` via `webmin-seed` container, `podman compose up -d`, oneshot `webmin-ensure`, `changepass.pl` + `chpasswd`
- Nix (only when `enableNix: true` — independent from niri): `/nix/var/nix/build` tmpfs perms workaround, `TMPDIR` scoped to daemon
- Locale: `it` → `it_IT.UTF-8` — bare `it` crashes `conmon` `setlocale` (musl)

See `docs/guides/podman.md` and `docs/guides/webmin.md`.

## Troubleshooting

```bash
# Validate niri configs (harness niri_config_valid)
niri validate --config /var/lib/greetd/.config/niri/config.kdl
niri validate --config ~/.config/niri/config.kdl

# Check greeter session
cat /etc/greetd/config.toml
cat /etc/greetd/regreet.toml
ls -la /var/lib/greetd/.config/niri/

# RustDesk autostart & firewall
grep -q "rustdesk" ~/.config/niri/config.kdl && echo "rustdesk autostart enabled"
firewall-cmd --permanent --list-ports 2>/dev/null | grep 21115 || grep -rq 21115 /etc/firewalld/
flatpak list | grep rustdesk

# DMS (native apk — spawned by the niri session, no OpenRC service)
apk info -e dms-shell quickshell
pgrep -x dms
tail -50 ~/.local/log/user/dms.log
tail -50 ~/.local/log/user/wireplumber.log

# Session bus
ls -la $XDG_RUNTIME_DIR/bus

# Logs (harness allowlist)
grep -i "error" /var/log/messages | grep -vE "commitpartial|bwrap.*pivot_root|To enable access to XDG directories"
```

> Harness result 59/59 PASS (niri+dinit fixture): `dms_installed` = apk check (`dms-shell`/`quickshell` native), `firewall_rustdesk` + `flatpak_installed`, niri config validation, allowlisted log errors. See `docs/harness/artifacts.md`.

## Upstream References

- `references/niri-alpine/FIX-niri-dms.md` — DMS migration rounds (OpenRC service hang, fragment setup, background-effect)
- `references/niri-alpine/niri-real-hardware.md` — real-hardware session notes
- `references/niri-alpine/installer-gaps.md` — gap list (session env 5.4, logs 5.5, portals 5.12, greeter user/config, regreet state)

## Code References

- `src/chrootPhase/desktops/niri.go` — greeter/regreet/niri-session wrapper/portals/spawns/xdg-su/dms setup
- `src/chrootPhase/features/flatpak.go` — flathub remote + flatpak install
- `src/utils/config/config.go:FlatpakPackages` + `src/utils/logic/logic.go:applyRule` — flatpakPackages propagation
- `src/variables/internal_rules.yaml:desktopEnvironments.niri` — packages/flatpakPackages/services/groups
- `src/chrootPhase/internal/packages.go` — `/lib/modules` before apk
- `docs/guides/variables.md` — full variables reference
- `docs/harness/artifacts.md` — harness checks table
