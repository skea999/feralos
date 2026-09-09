# Niri — Wayland Compositor Desktop

> Niri desktop on FeralOS: greetd + regreet + niri + DMS (DankMaterialShell) + flatpak RustDesk.

## Overview

Niri is a scrollable-tiling Wayland compositor. FeralOS installs it via `desktopEnvironment: "niri"` in `variables.yaml`.

Includes (from `internal_rules.yaml:desktopEnvironments.niri`):
- Compositor: `niri`, `niri-portalsconf`, `xwayland-satellite`, `mesa-dri-gallium`, `wayland-libs-server`
- Greeter: `greetd` + `greetd-regreet` + `greetd-openrc` (vt 7, greeter session via `/etc/greetd/greeter.sh`)
- Session: `elogind` + `polkit-elogind`, `dbus`, `xdg-desktop-portal-wlr/gtk`, `pipewire`, `alacritty`
- Apps: `cosmic-files/edit/screenshot/player/store/settings`, `okular`, `eog`, `cups/cups-filters/avahi/system-config-printer`, `libreoffice`, `galculator`, `firefox`, `firewalld-gui`, `gnome-firmware`, `thunderbird`, `gnome-disk-utility`, `kcron`, `ksystemlog`
- Nix: `quickshell`, `dms-shell` (stable `nixos-26.05`), `rclone/rclone-ui`, `mission-center`, `cups-browsed` (6 packages, `nix-software-center` manual due to crates.io 403 on QEMU IPs)
- Flatpak: `flathub:com.rustdesk.RustDesk` (conditional — only when `flatpakPackages` contains rustdesk)
- System: `xdg-user-dirs` (portal access), `firewalld` service

> Wayvnc removed — per-session `spawn-at-startup "wayvnc"` dropped in favor of on-demand remote. Use flatpak RustDesk for remote.

## Installation

Niri installed automatically when `desktopEnvironment: niri` in `variables.yaml`:

```yaml
desktopEnvironment: "niri"
```

Desktop packages pulled via `utils/logic.ApplyRules` from `internal_rules.yaml` (`desktopEnvironments.niri.packages`, `nixPackages`, `flatpakPackages`) + `chrootPhase/desktops/niri.go:SetupNiri`.

Manual (chroot):
```bash
apk add niri niri-portalsconf greetd greetd-regreet xdg-user-dirs firewalld
rc-update add greetd default
rc-update add elogind default
rc-update add firewalld default
```

## Greeter + Display Manager

- `greetd` config: `/etc/greetd/config.toml` → `vt 7`, `command = "/etc/greetd/greeter.sh"`, `user = "greeter"`
- `greeter.sh`: handles `XDG_RUNTIME_DIR` fallback (`/tmp/greetd-runtime` when turnstile not yet), `LIBGL_ALWAYS_SOFTWARE=true`, `exec niri -- regreet`
- `regreet.toml`: `/etc/greetd/regreet.toml` — timezone from `variables.yaml` (`timezone`), reboot = `["reboot"]`, poweroff = `["poweroff"]` (not `systemctl` — Alpine OpenRC, fixed 2026-09-09)
- Greeter niri config: `/var/lib/greetd/.config/niri/config.kdl` — keymap from `variables.yaml` (`keymap`), `WLR_NO_HARDWARE_CURSORS 1`
- Greeter user: `greeter` (`/var/lib/greetd` home, `video,input,greetd` groups — `greetd` owns `/var/lib/greetd` 0750 at boot)

## User Session

- User niri config: `/home/<user>/.config/niri/config.kdl` — keymap `it`/`us`, wayland env (`XDG_SESSION_TYPE wayland`, `QT_QPA_PLATFORM wayland`, etc.), `WLR_NO_HARDWARE_CURSORS 1`
- **RustDesk autostart (conditional)**: when `flatpakPackages` contains `rustdesk` (default via `internal_rules.yaml` niri `flatpakPackages: ["flathub:com.rustdesk.RustDesk"]`), user config appends:
  ```
  spawn-at-startup "flatpak" "run" "com.rustdesk.RustDesk" "--service"
  ```
  Implemented in `chrootPhase/desktops/niri.go` — `hasRustdesk` check over `conf.User.FlatpakPackages`. No autostart when flatpakPackages empty.
- **Firewall ports for RustDesk direct IP**: `21115-21119/tcp` + `21116/udp` opened via `firewall-offline-cmd --zone=public --add-port=...` (not `firewall-cmd` — `firewalld` not running in chroot, use offline cmd). Applied inside `SetupNiri` when `hasRustdesk` true, with `|| true` fallback.
- `xdg-su` wrapper: `/usr/local/bin/xdg-su` — niri unsupported by xdg-su, uses `alacritty -e su -c` for `*niri*` desktops
- DMS service: `/etc/init.d/dms` OpenRC (adapted from DMS flake systemd user service) — waits for wayland socket (`/run/user/<uid>/wayland-*`, 60×5s), then `start-stop-daemon --user <user> --env WAYLAND_DISPLAY` launches `~/.local/state/nix/profiles/profile/bin/dms`. Enabled via `rc-update add dms default`. Config via `su - <user> -c 'dms setup binds|layout|colors'`.

## Flatpak + RustDesk

- APK: `flatpak` added when `flatpakPackages` non-empty (`chrootPhase/features/flatpak.go:FlatpakPackages`)
- Setup: `features.SetupFlatpak` — `flatpak remote-add --if-not-exists flathub`, then `flatpak install -y --noninteractive <remote> <appID>` per entry (`flathub:com.rustdesk.RustDesk` split on `:`)
- **/lib/modules handling**: `src/chrootPhase/internal/packages.go:SetupPackages` ensures `mkdir -p /lib/modules` **before** `apk update`/`apk add` (flatpak and others check kernel module path). Previously in `flatpak.go` after install — moved earlier to avoid transient errors. Flatpak install still creates it as fallback but early mkdir prevents apk warnings.
- **xdg-user-dirs**: added to `standardPackages` (always installed) for flatpak portal `XDG` directory access (`Documents`, `Downloads`, etc.) — required for RustDesk file transfer portal.
- Harness checks: `flatpak_installed` (`flatpak list | grep rustdesk`), `firewall_rustdesk` (`firewall-cmd --permanent --list-ports | grep 21115` or `grep -rq 21115 /etc/firewalld/`)

## Webmin + Podman Integration

Webmin is **podman-based** (not host package):

- Requires `enablePodman: true` + `enableWebmin: true` (compose needs podman)
- Podman: `btrfs` driver via `sed` on `/etc/containers/storage.conf`, `cgroups` + `podman` services, `iptables`, `fuse-overlayfs` fallback, `podman-compose`+`podman-docker`, `docker→podman` dummy
- Webmin binary: standalone `src/firstBoot/webmin/main.go` compiled to `/opt/feralos_webmin` (verify via `podman --version` only, podman already configured in chrootPhase)
- Container: `demosthenesk/webmin:noble` image, `read_only: true` + `tmpfs: /tmp /run /var/log /var/webmin`, seed `/etc/webmin` via `webmin-seed` container, `podman compose up -d`, oneshot `webmin-ensure` service, `changepass.pl` + `chpasswd`
- Nix tmp: `/nix/var/nix/build` 775 `root:nixbld`, `/nix/var/nix/build/tmp` 1777 `root:nixbld` (world-writable sticky, `noexec` `/tmp` workaround), `TMPDIR` scoped to daemon via `/etc/conf.d/nix-daemon` (not global — leaked into podman broke `containers-storage`)
- Locale: `it` → `it_IT.UTF-8` (`%s_%s.UTF-8` expansion) — bare `it` crashes `conmon` `setlocale` (musl)
- Harness: `webmin_binary`/`webmin_container`/`webmin_port`/`webmin_http` (8×5s wget), `trusted_users_clean`, `nix_build_tmp_perms` (stat 777), `nix_packages_missing` (unique count, 6 for niri stable), `chmod_77_check` excludes `/nix/var/nix/build/tmp`, `works_in_real_life` (no non-interactive warning)

See `docs/guides/podman.md` (btrfs sed, cgroups, iptables) and `docs/guides/webmin.md` (compose seed, tmpfs, demosthenesk).

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

# DMS
rc-service dms status
pgrep -x dms

# Logs (harness allowlist 51/51)
grep -i "error" /var/log/messages | grep -vE "commitpartial|bwrap.*pivot_root|To enable access to XDG directories"
```

> Harness result 51/51 PASS with `firewall_rustdesk` + `flatpak_installed` + `commitpartial`/`bwrap`/`XDG` allowlist, `/lib/modules` before apk, `xdg-user-dirs`, `it_IT.UTF-8`, `TMPDIR` scoped. See `docs/harness/artifacts.md`.

## References

- `src/chrootPhase/desktops/niri.go` — greeter/regreet/niri/rustdesk/firewall/xdg-su/dms
- `src/chrootPhase/features/flatpak.go` — flathub remote + flatpak install
- `src/utils/config/config.go:FlatpakPackages` + `src/utils/logic/logic.go:applyRule` — flatpakPackages propagation
- `src/variables/internal_rules.yaml:desktopEnvironments.niri` — packages/nixPackages/flatpakPackages/services/groups
- `src/chrootPhase/internal/packages.go` — `/lib/modules` before apk
- `docs/guides/variables.md` — full variables reference
- `docs/harness/artifacts.md` — harness checks table (firewall_rustdesk, flatpak_installed, etc.)
