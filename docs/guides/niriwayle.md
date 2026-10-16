# niriWayle Desktop

Second desktop environment: the **niri compositor** with the **Wayle shell** (bar, notifications, OSD, wallpaper built in), **Veila** screen locker, **Raffi** launcher and **oo7** secrets daemon (gnome-keyring replacement). Shares the session infrastructure with the niri desktop.

## Packages

- Compositor: `niri`, `niri-portalsconf`, `xwayland-satellite`, `xwayland`, `mesa-dri-gallium`, `wayland-libs-server`
- Shell: `wayle` (0.7.0, feralos-aports — bar/notifications/OSD/wallpaper; niri + Hyprland/Mango modules; GTK4 + gtk4-layer-shell)
- Greeter: `greetd` + `greetd-regreet` (shared with niri; regreet lists both sessions — niri + niriWayle), `accountsservice`
- Session: `elogind`, `dbus`, portals (wlr/gtk), `polkit-elogind` + `polkit-gnome` agent, `power-profiles-daemon`, `upower` (Wayle battery/power module), `turnstile` + per-user dinit infra (`dbus-user-dinit`, `pipewire-user-dinit`, `pipewire-pulse-user-dinit`, `wireplumber-user-dinit`, `wayland-wait-user-dinit`, `oo7-user-dinit`, `xdg-desktop-portal-user-dinit`, `polkit-agent-user-dinit`); supervised boot.d stack (existence-filtered): `dbus.user pipewire.user pipewire-pulse.user wireplumber.user wait-wayland.user secrets.user oo7-portal.user xdg-desktop-portal.user xdg-document-portal.user xdg-permission-store.user` — standing rule: ALL user-session daemons are supervised per-user dinit services, nothing left to unsupervised D-Bus activation
- Portals: the user `niri-portals.conf` pins `org.freedesktop.impl.portal.Secret=oo7-portal` (oo7 README config — niriWayle has NO gnome-keyring, so the Secret interface must not fall through to the gtk backend)
- Session env: `/etc/environment` + the niri config `environment` block carry `XDG_DATA_DIRS` with flatpak exports first and `$HOME/.nix-profile/share` appended last (nix-installed desktop entries visible to Raffi/gio)
- Lock screen: `veila` (0.4.4 — ext-session-lock-v1, PAM password unlock via /etc/pam.d/veila, daemon + idle config)
- Launcher: `raffi` (0.24.0 — native iced GUI, YAML config at ~/.config/raffi/raffi.yaml)
- Secrets: `oo7` + `oo7-pam` + `oo7-cli` + `oo7-portal` (Rust Secret Service — org.freedesktop.secrets daemon + PAM + portal + CLI); secrets GUI = **Key Rack** (`app.drey.KeyRack`, Flathub flatpak via `flatpakPackages` — NOT an apk: seahorse conflicts with oo7-server on `/usr/share/dbus-1/services/org.freedesktop.secrets.service`; nixpkgs only has 0.4.0, too old; Flathub ships current 0.6.x, Rust + native oo7 client)
- Theming: `matugen`; icons adwaita + hicolor; fonts (terminus/inconsolata/dejavu/noto/noto-cjk/font-awesome)
- Session glue: `wayle-session` (feralos-aports — /usr/bin/wayle-session session script, wayle.user + veila.user user dinit services, niriwayle.desktop session entry)
- Network: same as niri (NetworkManager + wifi + wireless-regdb, bluez, wpa_supplicant, wireguard-tools)
- Extras: `wl-clipboard-rs` (Rust wl-copy/wl-paste), `dconf`, `xdg-user-dirs`, `wtype`, `i2c-tools`; Key Rack secrets GUI via Flathub (`flatpakPackages: ["app.drey.KeyRack"]` — installer adds the Flathub remote + `flatpak install -y --noninteractive`)
- NOT included (add per-install): browser/office/apps; `stasis` idle manager (v2 — veila daemon covers lock-on-idle + lock-before-sleep via elogind)

## Session startup

greetd → regreet (shared greeter) → user picks **niriWayle** → `wayle-session` (mirrors the FeralOS niri-session: login env, `dinitctl --user start niri`, then session apps `wayle.user`/`veila.user`/`polkit-agent.user`/`wayvnc.user`, dinit-monitor hold). The Wayle shell runs FOREGROUND via `wayle shell` under dinit supervision (`wayle panel start` daemonizes — breaks process supervision, live-caught). The session wrapper `wayle-shell-session` resolves `WAYLAND_DISPLAY`/`NIRI_SOCKET` and exports the session env before `exec wayle shell` (user-dinit services start before the compositor env exists — same wrapper pattern as wayvnc-session).

## Locking

`veila daemon` (veila.user) + `veila lock` (bind: Super+Alt+L). Built-in idle config (config.toml). The niri session lock protocol: ext-session-lock-v1.

## Launcher

Raffi: config `~/.config/raffi/raffi.yaml` (falls back to scanning desktop entries).

## Config paths

- Wayle: `~/.config/wayle/config.toml` (auto-created; live reload; GUI `wayle-settings`)
- Veila: `~/.config/veila/config.toml` (auto-created; themes, widgets, idle)
- niri: `~/.config/niri/config.kdl` (installer-written: env block, prefer-no-csd, binds — launcher/lock/screenshots/window management; no DMS includes)

## Harness

Fixture `tests/fixtures/variables/btrfs_standard_niriwayle_test.yaml` (dinit + edge). Checks: wayle shell running, veilad running, user dinit stack (incl. wayle.user/veila.user), niri layers (Wayle bar), session sockets, wayvnc, screenshot, oo7 packages installed, Key Rack flatpak installed + Secret=oo7-portal pinned (`keyrack_flatpak`).
