# Niri — Wayland Compositor Desktop

> Niri desktop on FeralOS: greetd + regreet + niri + DMS (DankMaterialShell, native apk) + NetworkManager + wayvnc remote desktop.
> **Flagship desktop**: for the recommended experience (niri + Wayle shell + Veila + Raffi + oo7 secrets) see [`niriwayle.md`](./niriwayle.md).

## Overview

Niri is a scrollable-tiling Wayland compositor. FeralOS installs it via `desktopEnvironment: "niri"` in `variables.yaml`.

**DMS is native**: `dms-shell` 1.6.2 + `quickshell` 0.3.1 ship as musl apks from the `feralos-aports` repo. **No nix, no nixGL, no execfilter** in the desktop stack. The old `/etc/init.d/dms` OpenRC service is gone on purpose — it hung boot and duplicated the shell; DMS (`dms.user`) is supervised by the USER dinit instance turnstile starts at login (wayland-gated via `wait-wayland.user`).

Includes (from `internal_rules.yaml:desktopEnvironments.niri`):
- Compositor: `niri`, `niri-portalsconf`, `xwayland-satellite`, `xwayland`, `mesa-dri-gallium`, `wayland-libs-server`
- Shell: `dms-shell` (1.6.2, feralos-aports), `quickshell` (0.3.1, feralos-aports)
- Greeter: `greetd` + `greetd-regreet` (vt 7, greeter session via `/etc/greetd/greeter.sh`), `accountsservice`
- Session: `elogind`, `dbus`, `xdg-desktop-portal-wlr/gtk`, `polkit-elogind` + `polkit-gnome`, `power-profiles-daemon`, `cups-pk-helper`, `alacritty`
- Secrets: `gnome-keyring` (login keyring unlocked at greetd PAM — see [Secrets (gnome-keyring)](#secrets-gnome-keyring) below)
- Session services (user dinit): `turnstile` (feralos-aports, PAM session → user dinit), `niri-user-dinit` (feralos-aports: `niri` + `niri-shutdown` user services + `/usr/libexec/feralos/login-shell` + FIXED `/usr/bin/niri-session`, `replaces="niri"`), `dbus-user-dinit`, `pipewire-user-dinit`, `pipewire-pulse-user-dinit`, `wireplumber-user-dinit`, `wayland-wait-user-dinit`, `polkit-agent-user-dinit` (feralos-aports; pull pipewire/pipewire-pulse/wireplumber as runtime deps), `wayvnc-user-dinit` (feralos-aports) + `dms.user` (shipped by `dms-shell`) + `kglobalaccel-user-dinit` with the `kglobalacceld` daemon (KDE global shortcuts, activation-only per ADR-010)
- Network: `networkmanager`, `networkmanager-cli`, `networkmanager-tui` (service `default`; the dinit service arrives via the OpenRC→dinit conversion), `networkmanager-wifi`, `wireless-regdb`, `wpa_supplicant` (spawned/supervised by NM), `wireguard-tools` (WireGuard via NM — see below)
- Scanner stack: `sane`, `sane-backends`, `sane-utils`, `sane-airscan`, `simple-scan` + `qt6-qtmultimedia` (DMS optdepends), `i2c-tools`, `wtype` (DMS optdepends promoted)
- Apps: `cosmic-files/edit/screenshot/player/store/settings`, `okular`, `eog`, `cups/cups-filters/avahi/system-config-printer`, `libreoffice`, `galculator`, `firefox`, `firewalld-gui`, `gnome-firmware`, `thunderbird`, `gnome-disk-utility`, `kcron`, `ksystemlog`, `rclone`
- System: `xdg-user-dirs` (portal access), `firewalld` service, `iptables/ip6tables/ebtables` (firewalld direct-rule backends)
- Services: `greetd`, `elogind`, `firewalld`, `polkit`, `cupsd`, `networkmanager` (all `default`)
- Groups: `video`, `input`, `audio`, `plugdev`, `netdev` (plugdev: NM polkit rule allows GUI connection management)

> Notes: `cava` (DMS audio visualizer) not on Alpine 3.24 — re-add when packaged. `turnstile` ships via feralos-aports (Alpine stable does not have it). `matugen`/`rclone-ui` not on Alpine — DMS degrades gracefully. `render` group does not exist on Alpine (video covers DRM). No `nixPackages` — `enableNix` is an independent feature.

> RustDesk removed (round 43 cleanup philosophy — never in the validated stack). Remote desktop = **wayvnc** (user dinit service, loopback by default) — see below.

## Session Startup and Verification

> **"The installer is the product; the harness simulates real life."** The installer must produce a system that works on real hardware with zero test-only crutches; the harness verifies that behavior by simulating real usage in a VM. Any fix discovered via the harness belongs in the installer or `feralos-aports` — never in the harness.

**Real-life path** (all shipped by the installer + `feralos-aports`, no test-only crutches):

`greetd` → regreet login (`niri.desktop`, `Exec=niri-session`) → `/usr/bin/niri-session` (shipped FIXED by `niri-user-dinit`, `replaces="niri"` — upstream post-25.11 dinit mechanism) → `pam_turnstile.so` → per-user dinit instance → `niri` user service (`niri --session`, `chain-to = niri-shutdown`). The script imports the FULL login env into the user dinit (`dinitctl --user setenv`), starts the `niri` user service, then holds itself open for the whole session (`dinit-monitor` wait, fallback `is-started` poll) — script lifetime = greetd session lifetime.

**Why fixed** (maintainer note): upstream's script (niri ≤ 25.11, our base) started the user service and exited immediately → greetd tore the session down in ~300 ms (no session lifetime); and it never imported the login session env → niri died on `TakeControl` over turnstiled's seatless pseudo-session (ENOSYS). `niri-user-dinit` carries both upstream fixes, adapted to FeralOS; launch paths: greetd autologin, regreet, manual tty.

**Harness (QEMU)**: the autologin fixture (`enableNiriTestAutologin`) SIMULATES that login — greetd `[initial_session] command = "niri-session"` runs the exact same `/usr/bin/niri-session` a real login runs. The installer NEVER writes `[initial_session]` — real installs log in manually through regreet. Runtime checks verify the session REALLY runs: `niri_session_user`, `niri_wayland_socket`, `niri_ipc`, `niri_user_dinit_runtime`, `niri_wayvnc_port`, `niri_screenshot` (IPC screenshot → non-empty PNG = real-rendering proof). Details → `docs/harness/workflow.md`.

**Real hardware**: log in graphically (regreet), then run as root on the target:

```bash
sh tests/harness/niri_realhw_check.sh [username]
```

`tests/harness/niri_realhw_check.sh` (POSIX sh, busybox-clean) — same session checks + pre-session system checks (DRM card0/renderD128 + driver, greetd/elogind/turnstiled, wayland logind session type).

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

- **`niri-session` — FIXED, shipped by `niri-user-dinit`**: `/usr/bin/niri-session` comes from `niri-user-dinit` (`replaces="niri"`; upstream post-25.11 dinit mechanism — see [Session Startup and Verification](#session-startup-and-verification)). greetd sets `SHELL` from the passwd entry; the user is created with `-s /usr/libexec/feralos/login-shell` — a symlink shipped by `niri-user-dinit`, deliberately **not** listed in `/etc/shells`. The package default target is `/bin/ash`, but busybox dispatches applets on `argv[0]`: a `login-shell`-named symlink to `/bin/ash` fails with `login-shell: applet not found` for every passwd-shell consumer (sshd, `su -`, installer's `dms setup`), so the installer re-points it to `/usr/bin/fish` (the pre-turnstile user shell). The script's "re-exec through bash when `$SHELL` is in `/etc/shells`" check therefore fails, the bash re-exec is skipped, and it reaches its dinit branch: full login-env `setenv` + `dinitctl --user start niri` starts the `niri` user service (`command = niri --session`, `depends-on = dbus.user`, `chain-to = niri-shutdown`, `restart = false`), then `dinit-monitor` holds the script open as the session lifetime. The installer does **not** write `/usr/local/bin/niri-session` anymore (old wrapper removed; `user chsh` is also skipped for niri so it cannot clobber the passwd shell). NOTE: the login-shell path IS listed in `/etc/shells` (installer, `users.go`) — pkexec requires the session shell there (RC=127 otherwise, broke every privileged DMS op); the FeralOS niri-session has no /etc/shells re-exec trap, so listing it is safe
- **turnstile (PAM session → user dinit)**: `session optional pam_turnstile.so` already ships in Alpine's `base-session` (3.24-stable). At greetd login `pam_turnstile.so` asks the system `turnstiled` (symlinked into `/etc/dinit.d/boot.d/`) to spawn the per-user dinit instance (`/usr/libexec/turnstile/dinit`, backend `dinit`, `manage_rundir=false` — elogind owns `/run/user/<uid>`). The installer enables only the INFRASTRUCTURE services in the user dinit's `boot.d` (`/usr/lib/dinit.d/user/boot.d/`): `dbus.user pipewire.user pipewire-pulse.user wireplumber.user wait-wayland.user` (existence-filtered). **Session apps are NOT in boot.d** — upstream `add-wants niri.service` model (niri docs wants-dir + DMS docs): `niri-session` starts `polkit-agent.user`/`dms.user`/`wayvnc.user` explicitly after niri (aports `niri-user-dinit`), so they bind to the compositor session — the greetd greeter session (`greeter.sh` = `niri -- regreet`, never runs `niri-session`) never gets them. **Dual-init**: `turnstiled` runs as a system service on BOTH inits — dinit (`/etc/dinit.d/turnstiled`, ships in the main `turnstile` package, `before: login.target`) and OpenRC (`turnstile-openrc` subpackage, `rc-update add turnstiled default` — enabled on BOTH inits, never conditional: a dinit install must stay OpenRC-bootable; `depend(): need elogind; before greetd`). The `before greetd` ordering is load-bearing: `pam_turnstile.so` is optional/silent, so greetd's login session must not open before turnstiled's control socket listens, or no user dinit spawns (live-caught boot-order race under OpenRC). The per-user dinit instance and the entire `-user-dinit` stack are identical on both inits
- **Session env**: `/etc/environment` (`XDG_SESSION_DESKTOP=niri`, `XDG_DATA_DIRS` with flatpak exports first and `$HOME/.nix-profile/share` appended last — nix-installed desktop entries stay visible to launchers/gio; read by `pam_env` in `/etc/pam.d/turnstiled`) + the niri config `environment` block. No `DBUS_SESSION_BUS_ADDRESS` export needed: the session bus is the `dbus.user` service of the turnstile-started user dinit
- **User dinit session services** (Artix `dinit-userservd-services` model — `.user` suffix, sub-vars, `smooth-recovery`, dbus readiness via `pipefd:3` + `dinitctl setenv`): **standing rule (user directive)** — EVERY user-session daemon runs as a supervised per-user dinit service (`.user`, started by the turnstile-spawned user dinit); NOTHING is left to unsupervised D-Bus activation, autostart or daemon self-spawn (missing `-user-dinit` packages get created in feralos-aports; applies to BOTH inits — OpenRC gets the identical stack via turnstile)
  - `dbus.user`, `pipewire.user`, `pipewire-pulse.user`, `wireplumber.user`
  - `wait-wayland.user` — compositor-agnostic gate: resolves `WAYLAND_DISPLAY` from the compositor socket
  - `polkit-agent.user`, `wayvnc.user` (remote desktop, wayland-gated), `dms.user` (wayland-gated; shipped by `dms-shell`) — **session-scoped**: started by `niri-session` after niri (add-wants model), not in boot.d
  - Logs: `~/.local/share/dinit/`
- User niri config: `/home/<user>/.config/niri/config.kdl` — keymap from `variables.yaml`, wayland env (`XDG_SESSION_TYPE wayland`, `XDG_SESSION_DESKTOP niri`, `XDG_DATA_DIRS` with flatpak exports first, `QT_QPA_PLATFORM wayland`, `QT_QPA_PLATFORMTHEME gtk3`, `SDL_VIDEO_DRIVER wayland`, etc.), no session service spawns (supervision + ordering now native dinit):
  - `xwayland-satellite { path "xwayland-satellite" }` block — XWayland for X11 apps
  - `include "dms/binds.kdl"` + `dms/layout.kdl` + `dms/colors.kdl`
  - top-level `prefer-no-csd` — apps omit CSD titlebars (window controls = DMS bar widget + keys; floating resize = Mod+right-click); Qt also gets `QT_WAYLAND_DISABLE_WINDOWDECORATION=1` (hard override). Note: GTK apps with a custom headerbar (GNOME/libadwaita such as gnome-maps/calendar/eog and Firefox) KEEP their header — it is app UI, the client requests CSD and niri honors it by design (`GTK_CSD=0` has no effect on Wayland)
- **DMS fragments**: pre-created before first niri validation via `su - <user> -c 'dms setup binds|layout|colors'` (packaged CLI). Config ownership fixed BEFORE `dms setup` (installer mkdir/writes run as root — without `chown -R`, `dms setup` cannot create `.config/niri/dms/` and every fragment fails)
- **DMS theme seed**: installer writes `~/.config/DankMaterialShell/settings.json` with `currentThemeCategory: generic` + `currentThemeName: green` (Settings → Theme & Colors → Generic grid, static Material palette). File chowned to the user so DMS can persist changes; switching to Auto (wallpaper/matugen) or a registry theme later overrides normally. Dark mode: session.json (~/.local/state/DankMaterialShell/) seeded {"isLightMode": false} (default already dark — explicit guarantee). Bar position: v1→v2 migration default (position=Top) — dankBarAtBottom removed; seed uses configVersion:1 so DMS builds the complete default barConfigs itself (no fragile default duplication). Left bar widgets: launcherButton + workspaceSwitcher + runningApps (focusedWindow swapped out — runningApps shows all running apps with focus indication). Bar on TOP; right section pinned with the `dmsWindowButtons` window-controls plugin (maximize/fullscreen/close for the focused window — system-wide plugin, package `dms-window-buttons`)
- **Default apps**: `~/.config/xdg-terminals.list` = `Alacritty.desktop`; `~/.config/mimeapps.list` sets browser=firefox, images=`org.gnome.eog.desktop`, audio=`com.system76.CosmicPlayer.desktop` (DMS Default Apps tab reads both on first launch); maps=`org.gnome.Maps.desktop` (geo), calendar=`org.gnome.Calendar.desktop` (text/calendar + calendar/webcal schemes)
- **Update button**: DMS System Update uses `updaterCustomCommand = /usr/local/bin/feralos-update` — runs in a terminal: apk (update + upgrade) → flatpak if installed (unused user/system, update user/system) → nix if installed (channel update, profile upgrade, trim generations keep-5, GC; root profile too). Exit code = error count
- **Power/reboot (dinit only)**: DMS Power → Reboot uses `customPowerActionReboot = "sudo -n /usr/bin/reboot"` + `/etc/sudoers.d/zz-power` (`%wheel ALL=(root) NOPASSWD: /usr/bin/reboot`, 440; `zz-` so it sorts AFTER `/etc/sudoers.d/wheel` — sudo-rs last-match-wins, a plain `power` filename was overridden by `%wheel ALL=(ALL) ALL`, live-caught) because the loginctl/elogind path always execs busybox /sbin/reboot (SIGTERM → dinit overwrites REBOOT→HALT; CLI `loginctl reboot` broken by design of elogind) — plus the shutdown hook `/etc/elogind/system-shutdown/50-feralos-dinit.sh` (execs `dinit-{poweroff,reboot,halt}`) for the poweroff path
- **Theming — dms-theme-sync plugin (no manual seeds)**: the installer writes NO theme files — the third-party DMS plugin `dms-theme-sync` (GPL-3.0) is the theming authority: it propagates the DMS appearance (mode/colors/icons) to GTK, Qt, KDE and Flatpak. Removed seeds to avoid conflicts: `~/.config/kdeglobals` (BreezeDark [Colors:*] + [Icons] breeze-dark + [UiSettings] BreezeDark pin), `~/.config/gtk-3.0|4.0/settings.ini`, flatpak user-global override (`~/.local/share/flatpak/overrides/global`), dconf system default `/etc/dconf/db/local.d/00-feralos-theme`. Icon theme packages `adwaita-icon-theme` + `hicolor-icon-theme` remain dms-shell deps; the **matugen** backend (dynamic wallpaper palettes → `DankMatugen` KColorScheme export for the plugin) ships from feralos-aports (4.2.0, absent in Alpine) in the niri package list
- **gsettings infrastructure (not a theme override)**: `dconf` package + `/etc/dconf/profile/user` (standard GNOME profile `user-db:user` + `system-db:local` — Alpine's dconf ships none, without it GLib ignores the user/system dbs) + `dconf update` (compiles the now-empty system db so runtime user-level writes take effect; `/etc/dconf/db/local.d` is created explicitly — Alpine's dconf ships no `/etc/dconf` at all and `dconf update` errors without the dir, harness-caught run `2026-10-12T22-09-52Z`); Qt platform theme env `QT_QPA_PLATFORMTHEME=xdgdesktopportal` (only platformtheme plugin in Alpine — follows the portal/DMS). Greeter user created as system user (`addgroup -S greeter` + `adduser -S -G greeter` — busybox `-S` alone defaults the group to `nogroup` and creates no same-name group) so ReGreet's UID-filtered list hides it; the "Important Hotkeys" overlay is intentionally NOT skipped in the user niri config (user request — keybind cheatsheet at login); final step 14 `chown -R user:user ~/.config ~/.local` sweep fixes root-owned seed-created dirs (per-seed chowns cover files; mkdir dirs stayed root because the general chown runs before the seeds)
- **background-effect strip**: DMS 1.6 layout template emits `background-effect { xray false }` — niri node added after 25.11 (`ext-background-effect-v1`, unsupported by Alpine 3.24 niri) that breaks the whole include. Installer strips the block via awk (`blur` unsupported on this niri anyway)
- **User portals conf**: `/home/<user>/.config/xdg-desktop-portal/niri-portals.conf` — `[preferred] org.freedesktop.impl.portal.ScreenCast=wlr;` (system default `gnome;gtk` breaks ScreenCast)
- **Session logs**: `~/.local/share/dinit/` (user dinit service logs — dinit `log-type=file` per service)
- **Groups**: user `video,input` (rules) + `audio` (`/dev/snd` for PipeWire) via `usermod -aG`; `plugdev`+`netdev` (rules) for NetworkManager GUI management; seat access via elogind D-Bus
- `xdg-su` wrapper: `/usr/local/bin/xdg-su` — niri unsupported by xdg-su, uses `alacritty -e su -c` for `*niri*` desktops

## Secrets (gnome-keyring)

- Package: `gnome-keyring` (via `internal_rules.yaml` niri packages) — closes installer-gaps 4.5 (no `org.freedesktop.secrets` provider — gvfs/Thunderbird could not store passwords)
- **PAM unlock**: `auth optional pam_gnome_keyring.so` appended to `/etc/pam.d/greetd` (idempotent grep-guard); the `session ... auto_start` line is **not** added anymore — it already ships in Alpine's `base-session`. The login keyring is unlocked with the login password at greetd login, and the keyring daemon autostarts when the PAM session opens
- **Secrets service**: D-Bus activated on demand — `org.freedesktop.secrets` service file ships with the package; the session bus is `dbus.user`, supervised by the user dinit instance

## Network (NetworkManager)

NetworkManager replaces ifupdown as the niri network service (`FIX-niri-dms.md` rounds 30/44 — DMS network/VPN UI needs the NM backend):

- Service: `networkmanager` on `default` (via `internal_rules.yaml` niri `services`; the dinit service arrives via the OpenRC→dinit conversion)
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

- APK: `flatpak` added only when `flatpakPackages` non-empty (`chrootPhase/features/flatpak.go:FlatpakPackages`) — niri sets a default (`internal_rules.yaml` niri `flatpakPackages`: RClone Manager + Flatseal)
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

# turnstile + user dinit (PAM session → per-user dinit)
ls -la /etc/dinit.d/boot.d/turnstiled
pgrep -x turnstiled
getent passwd <user> | cut -d: -f7      # → /usr/libexec/feralos/login-shell
ls -la /usr/lib/dinit.d/user/boot.d/
ls -la /usr/libexec/feralos/login-shell

# Session bus
ls -la $XDG_RUNTIME_DIR/bus

# Logs (harness allowlist)
grep -i "error" /var/log/messages | grep -vE "commitpartial|bwrap.*pivot_root|To enable access to XDG directories"
```

> Harness result 63/63 PASS (niri+dinit fixture, run 2026-09-29T22-07-20Z, turnstile wiring): `dms_installed` = apk check (`dms-shell`/`quickshell` native), `dinit_user_services` = all 7 `.user` service files under `/etc/dinit.d/user/`, `niri_user_service` = `/etc/dinit.d/user/niri` runs `niri --session`, `login_shell_trick` = passwd shell `feralos/login-shell` + symlink present, `turnstile_pkg` = `/etc/dinit.d/turnstiled` + `boot.d` link + `pam_turnstile.so`, `user_services_enabled` = all 8 `boot.d` links under `/usr/lib/dinit.d/user/boot.d/` (`turnstiled` running as a dinit system service), `niri_greeter` = `niri -- regreet` running (polled, slow QEMU boots), `gnome_keyring` = pkg + `org.freedesktop.secrets` service file + `pam_gnome_keyring` in greetd PAM, wayvnc user service + NetworkManager active, niri config validation, no ignored errors in installer.log, allowlisted log errors. RustDesk checks removed (`flatpak_installed`/`firewall_rustdesk` only when a fixture explicitly lists a rustdesk flatpak). `/var/tmp` 1777 exempt in the chmod-777 log scan (O_TMPFILE). See `docs/harness/artifacts.md`.

## Upstream References

- `references/niri-alpine/FIX-niri-dms.md` — DMS migration rounds (OpenRC service hang, fragment setup, background-effect, NetworkManager 30/44, wayvnc 37, RustDesk removal 43, /var/tmp 47)
- `references/niri-alpine/niri-real-hardware.md` — real-hardware session notes
- `references/niri-alpine/installer-gaps.md` — gap list (secrets 4.5 CLOSED — gnome-keyring; remaining: session env 5.4, logs 5.5, portals 5.12, greeter user/config, regreet state)

## Code References

- `src/chrootPhase/desktops/niri.go` — greeter/regreet/niri-session wrapper (user dinit)/portals/xdg-su/dms setup + gnome-keyring PAM unlock (`pam_gnome_keyring` in `/etc/pam.d/greetd`) + NetworkManager (interfaces lo-only, 10-manage.conf) + CUPS printer admin via `lpadmin` group (SystemGroup resta il default Alpine)
- `src/chrootPhase/internal/packages.go` — `/lib/modules` before apk + `/var/tmp` 1777 + dinit conversion session/NM-managed skips (pipewire stack, wpa_supplicant, wireguard-tools)
- `src/variables/internal_rules.yaml:desktopEnvironments.niri` — packages/services/groups (NM, wayvnc-user-dinit, sane/scanner stack, plugdev/netdev)
- `src/hostPhase/disks/*/fstab.go` + `partitioning.go` — `/tmp` exec (nosuid kept)
- `src/utils/config/config.go:FlatpakPackages` + `src/utils/logic/logic.go:applyRule` — flatpakPackages propagation (generic, no niri default)
- `docs/guides/variables.md` — full variables reference
- `docs/harness/artifacts.md` — harness checks table
