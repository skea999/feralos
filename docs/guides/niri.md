# Niri — Wayland Compositor Desktop

> Niri desktop on FeralOS: greetd + regreet + niri + DMS (DankMaterialShell, native apk) + NetworkManager + wayvnc remote desktop.

## Overview

Niri is a scrollable-tiling Wayland compositor. FeralOS installs it via `desktopEnvironment: "niri"` in `variables.yaml`.

**DMS is native**: `dms-shell` 1.6.2 + `quickshell` 0.3.1 ship as musl apks from the `feralos-aports` repo. **No nix, no nixGL, no execfilter** in the desktop stack. The old `/etc/init.d/dms` OpenRC service is gone on purpose — it hung boot and duplicated the shell; DMS (`dms.user`) is supervised by the USER dinit instance started by the niri-session wrapper (wayland-gated via `wait-wayland.user`).

Includes (from `internal_rules.yaml:desktopEnvironments.niri`):
- Compositor: `niri`, `niri-portalsconf`, `xwayland-satellite`, `xwayland`, `mesa-dri-gallium`, `wayland-libs-server`
- Shell: `dms-shell` (1.6.2, feralos-aports), `quickshell` (0.3.1, feralos-aports)
- Greeter: `greetd` + `greetd-regreet` + `greetd-openrc` (vt 7, greeter session via `/etc/greetd/greeter.sh`), `accountsservice`
- Session: `elogind`, `dbus`, `xdg-desktop-portal-wlr/gtk`, `polkit-elogind` + `polkit-gnome`, `power-profiles-daemon`, `cups-pk-helper`, `alacritty`
- Session services (user dinit): `dbus-user-dinit`, `pipewire-user-dinit`, `pipewire-pulse-user-dinit`, `wireplumber-user-dinit`, `wayland-wait-user-dinit`, `polkit-agent-user-dinit` (feralos-aports; pull pipewire/pipewire-pulse/wireplumber as runtime deps), `wayvnc-user-dinit` (feralos-aports) + `dms.user` (shipped by `dms-shell`)
- Network: `networkmanager`, `networkmanager-cli`, `networkmanager-tui`, `networkmanager-dinit` (service `default`), `wpa_supplicant` (spawned/supervised by NM), `wireguard-tools` (WireGuard via NM — see below)
- Scanner stack: `sane`, `sane-backends`, `sane-utils`, `sane-airscan`, `simple-scan` + `qt6-qtmultimedia` (DMS optdepends), `i2c-tools`, `wtype` (DMS optdepends promoted)
- Apps: `cosmic-files/edit/screenshot/player/store/settings`, `okular`, `eog`, `cups/cups-filters/avahi/system-config-printer`, `libreoffice`, `galculator`, `firefox`, `firewalld-gui`, `gnome-firmware`, `thunderbird`, `gnome-disk-utility`, `kcron`, `ksystemlog`, `rclone`
- System: `xdg-user-dirs` (portal access), `firewalld` service, `iptables/ip6tables/ebtables` (firewalld direct-rule backends)
- Services: `greetd`, `elogind`, `firewalld`, `polkit`, `cupsd`, `networkmanager` (all `default`)
- Groups: `video`, `input`, `audio`, `plugdev`, `netdev` (plugdev: NM polkit rule allows GUI connection management)

> Notes: `cava` (DMS audio visualizer) not on Alpine 3.24 — re-add when packaged. `turnstile` edge-only. `matugen`/`rclone-ui` not on Alpine — DMS degrades gracefully. `render` group does not exist on Alpine (video covers DRM). No `nixPackages` — `enableNix` is an independent feature.

> RustDesk removed (round 43 cleanup philosophy — never in the validated stack). Remote desktop = **wayvnc** (user dinit service, loopback by default) — see below.

## Installation

Niri installed automatically when `desktopEnvironment: niri` in `variables.yaml`:

```yaml
desktopEnvironment: "niri"
```

Desktop packages pulled via `utils/logic.ApplyRules` from `internal_rules.yaml` (`desktopEnvironments.niri.packages`, `services`, `groups`) + `chrootPhase/desktops/niri.go:SetupNiri`.

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

- **niri-session wrapper**: `/usr/local/bin/niri-session` (wins over `/usr/bin`) — greetd provides neither the session env nor a D-Bus session bus. POSIX script: exports `XDG_SESSION_TYPE=wayland` + `XDG_CURRENT_DESKTOP=niri` + `DBUS_SESSION_BUS_ADDRESS`, starts `dbus-daemon --session` on `$XDG_RUNTIME_DIR/bus` if missing, starts the USER dinit instance, then hands off to `/usr/bin/niri-session`
- **User dinit session services**: wrapper runs `dinit -d /etc/dinit.d/user <services>` (existence-filtered — a service file exists only if its package is installed, so the same wrapper works on any desktop). Idempotent across re-logins: skipped when `$XDG_RUNTIME_DIR/dinit.ctl` is already alive (supervised services survive; crashed ones were restarted by dinit). Services (Artix `dinit-userservd-services` model — `.user` suffix, sub-vars, `smooth-recovery`, dbus readiness via `pipefd:3` + `dinitctl setenv`):
  - `dbus.user`, `pipewire.user`, `pipewire-pulse.user`, `wireplumber.user`
  - `wait-wayland.user` — compositor-agnostic gate: resolves `WAYLAND_DISPLAY` from the compositor socket
  - `polkit-agent.user`, `wayvnc.user` (remote desktop, wayland-gated), `dms.user` (wayland-gated; `dms.user` shipped by `dms-shell`)
  - Logs: `~/.local/share/dinit/`
- User niri config: `/home/<user>/.config/niri/config.kdl` — keymap from `variables.yaml`, wayland env (`XDG_SESSION_TYPE wayland`, `QT_QPA_PLATFORM wayland`, `QT_QPA_PLATFORMTHEME gtk3`, `SDL_VIDEO_DRIVER wayland`, etc.), no session service spawns (supervision + ordering now native dinit):
  - `xwayland-satellite { path "xwayland-satellite" }` block — XWayland for X11 apps
  - `include "dms/binds.kdl"` + `dms/layout.kdl` + `dms/colors.kdl`
- **DMS fragments**: pre-created before first niri validation via `su - <user> -c 'dms setup binds|layout|colors'` (packaged CLI). Config ownership fixed BEFORE `dms setup` (installer mkdir/writes run as root — without `chown -R`, `dms setup` cannot create `.config/niri/dms/` and every fragment fails)
- **background-effect strip**: DMS 1.6 layout template emits `background-effect { xray false }` — niri node added after 25.11 (`ext-background-effect-v1`, unsupported by Alpine 3.24 niri) that breaks the whole include. Installer strips the block via awk (`blur` unsupported on this niri anyway)
- **User portals conf**: `/home/<user>/.config/xdg-desktop-portal/niri-portals.conf` — `[preferred] org.freedesktop.impl.portal.ScreenCast=wlr;` (system default `gnome;gtk` breaks ScreenCast)
- **Session logs**: `~/.local/share/dinit/` (user dinit service logs; `~/.local/log/user` dir still pre-created)
- **Groups**: user `video,input` (rules) + `audio` (`/dev/snd` for PipeWire) via `usermod -aG`; `plugdev`+`netdev` (rules) for NetworkManager GUI management; seat access via elogind D-Bus
- `xdg-su` wrapper: `/usr/local/bin/xdg-su` — niri unsupported by xdg-su, uses `alacritty -e su -c` for `*niri*` desktops

## Network (NetworkManager)

NetworkManager replaces ifupdown as the niri network service (`FIX-niri-dms.md` rounds 30/44 — DMS network/VPN UI needs the NM backend):

- Service: `networkmanager-dinit` → `networkmanager` on `default` (via `internal_rules.yaml` niri `services`)
- `/etc/network/interfaces` reduced to **lo only** (`auto lo` + `iface lo inet loopback`) — NM manages wired/wireless interfaces; the stale DHCP ifupdown config caused double default routes
- `/etc/NetworkManager/conf.d/10-manage.conf`: `[ifupdown] managed=true` — NM may take over interfaces configured by ifupdown
- User groups: `plugdev` (NM polkit rule allows GUI connection management) + `netdev`, added via `internal_rules.yaml` niri `groups`
- DMS network + VPN applets active (NM backend); wifi via `wpa_supplicant` (NM spawns and supervises it — no system dinit service, see dinit conversion skips)
- WireGuard: native NM support (`nmcli connection add type wireguard`) + `wireguard-tools` for manual `wg` inspection; `wg-quick` is per-interface, no system dinit service

## Remote Desktop (wayvnc)

- `wayvnc` apk + `wayvnc-user-dinit` (feralos-aports) — runs as the session user under the user dinit instance, wayland-gated (starts after the compositor socket exists)
- Default bind: loopback `127.0.0.1:5900` — safe against LAN exposure out of the box
- LAN use: configure TLS + auth in the user's wayvnc config (`~/.config/wayvnc/config`, keys via `wayvnc --generate-certs`) per `FIX-niri-dms.md` round 37 — do not expose plain VNC to the network
- No RustDesk: flatpak + `spawn-at-startup` + firewall ports `21115-21119/tcp`/`21116/udp` all removed (round 43 cleanup — RustDesk was never in the validated reference stack)

## /tmp + /var/tmp

- `/tmp` subvolume mounts **exec** (nosuid kept): OpenTUI/Bun extract `.so` files to `/tmp` and `dlopen` them — `noexec` broke DMS tooling. All four btrfs layouts updated (`fstab.go` + `partitioning.go`)
- `/var/tmp` created `1777` (sticky world-writable): fresh btrfs subvolume mounts come up `0755`, which breaks `O_TMPFILE` and unprivileged temp writes (`flatpak --user`, podman) — round 47

## Flatpak (generic feature)

- APK: `flatpak` added only when `flatpakPackages` non-empty (`chrootPhase/features/flatpak.go:FlatpakPackages`) — niri no longer sets a default (`internal_rules.yaml` niri `flatpakPackages` removed)
- Setup: `features.SetupFlatpak` — `flatpak remote-add --if-not-exists flathub`, then `flatpak install -y --noninteractive <remote> <appID>` per entry
- **/lib/modules handling**: `src/chrootPhase/internal/packages.go:SetupPackages` ensures `mkdir -p /lib/modules` **before** `apk update`/`apk add` (flatpak and others check kernel module path)
- **xdg-user-dirs**: added to `standardPackages` (always installed) for flatpak portal `XDG` directory access

## Webmin + Podman Integration

Webmin is **podman-based** (not host package):

- Requires `enablePodman: true` + `enableWebmin: true` (compose needs podman)
- Podman: `btrfs` driver via `sed` on `/etc/containers/storage.conf`, `cgroups` + `podman` services, `iptables`, `fuse-overlayfs` fallback, `podman-compose`+`podman-docker`, `docker→podman` dummy
- Webmin binary: standalone `src/firstBoot/webmin/main.go` compiled to `/opt/feralos_webmin`
- Container: `demosthenesk/webmin:noble` image, `read_only: true` + `tmpfs`, seed `/etc/webmin` via `webmin-seed` container, `podman compose up -d`, oneshot `webmin-ensure`, `changepass.pl` + `chpasswd`
- Nix (only when `enableNix: true` — independent from niri): upstream default build dir (`/tmp`, exec)
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

# wayvnc (user dinit, loopback default)
ls /etc/dinit.d/user/wayvnc.user
dinitctl -u "$XDG_RUNTIME_DIR/dinit.ctl" status wayvnc.user
ss -tlnp | grep 5900

# NetworkManager
rc-status | grep networkmanager || dinitctl status networkmanager
nmcli device status
cat /etc/network/interfaces        # lo only
cat /etc/NetworkManager/conf.d/10-manage.conf
id | grep -E 'plugdev|netdev'

# DMS (native apk — supervised by the user dinit instance, no OpenRC service)
apk info -e dms-shell quickshell
pgrep -x dms
tail -50 ~/.local/share/dinit/dms.user.log

# User dinit session services
ls /etc/dinit.d/user/
dinitctl -u "$XDG_RUNTIME_DIR/dinit.ctl" status dbus.user
tail -50 ~/.local/share/dinit/pipewire.user.log

# Session bus
ls -la $XDG_RUNTIME_DIR/bus

# Logs (harness allowlist)
grep -i "error" /var/log/messages | grep -vE "commitpartial|bwrap.*pivot_root|To enable access to XDG directories"
```

> Harness result 59/59 PASS (niri+dinit fixture): `dms_installed` = apk check (`dms-shell`/`quickshell` native), `dinit_user_services` = all 7 `.user` service files under `/etc/dinit.d/user/`, `niri_session_wrapper` = wrapper starts the user dinit instance, wayvnc user service + NetworkManager active, niri config validation, allowlisted log errors. RustDesk checks removed (`flatpak_installed`/`firewall_rustdesk` only when a fixture explicitly lists a rustdesk flatpak). `/var/tmp` 1777 exempt in the chmod-777 log scan (O_TMPFILE). See `docs/harness/artifacts.md`.

## Upstream References

- `references/niri-alpine/FIX-niri-dms.md` — DMS migration rounds (OpenRC service hang, fragment setup, background-effect, NetworkManager 30/44, wayvnc 37, RustDesk removal 43, /var/tmp 47)
- `references/niri-alpine/niri-real-hardware.md` — real-hardware session notes
- `references/niri-alpine/installer-gaps.md` — gap list (session env 5.4, logs 5.5, portals 5.12, greeter user/config, regreet state)

## Code References

- `src/chrootPhase/desktops/niri.go` — greeter/regreet/niri-session wrapper (user dinit)/portals/xdg-su/dms setup + NetworkManager (interfaces lo-only, 10-manage.conf) + CUPS printer admin via `lpadmin` group (SystemGroup resta il default Alpine)
- `src/chrootPhase/internal/packages.go` — `/lib/modules` before apk + `/var/tmp` 1777 + dinit conversion session/NM-managed skips (pipewire stack, wpa_supplicant, wireguard-tools)
- `src/variables/internal_rules.yaml:desktopEnvironments.niri` — packages/services/groups (NM, wayvnc-user-dinit, sane/scanner stack, plugdev/netdev)
- `src/hostPhase/disks/*/fstab.go` + `partitioning.go` — `/tmp` exec (nosuid kept)
- `src/utils/config/config.go:FlatpakPackages` + `src/utils/logic/logic.go:applyRule` — flatpakPackages propagation (generic, no niri default)
- `docs/guides/variables.md` — full variables reference
- `docs/harness/artifacts.md` — harness checks table
