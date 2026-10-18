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
- Secrets: `oo7` + `oo7-pam` + `oo7-cli` + `oo7-portal` (Rust Secret Service — org.freedesktop.secrets daemon + PAM + portal + CLI); full-desktop unlock parity — see [Secrets unlocking (PAM)](#secrets-unlocking-pam); secrets GUI = **Key Rack** (`app.drey.KeyRack`, Flathub flatpak via `flatpakPackages` — NOT an apk: seahorse conflicts with oo7-server on `/usr/share/dbus-1/services/org.freedesktop.secrets.service`; nixpkgs only has 0.4.0, too old; Flathub ships current 0.6.x, Rust + native oo7 client)
- Theming: `matugen`; icons adwaita + hicolor; fonts (terminus/inconsolata/dejavu/noto/noto-cjk/font-awesome)
- Session glue: `wayle-session` (feralos-aports — /usr/bin/wayle-session session script, wayle.user + veila.user + stasis.user user dinit services, niriwayle.desktop session entry; depends niri-user-dinit + stasis)
- Network: same as niri (NetworkManager + wifi + wireless-regdb, bluez, wpa_supplicant, wireguard-tools)
- Extras: `wl-clipboard-rs` (Rust wl-copy/wl-paste), `dconf`, `xdg-user-dirs`, `wtype`, `i2c-tools`; Key Rack secrets GUI via Flathub (`flatpakPackages: ["app.drey.KeyRack"]` — installer adds the Flathub remote + `flatpak install -y --noninteractive`)
- Idle: `stasis` (context-aware idle manager — included via the `wayle-session` dependency as a supervised `stasis.user` session service; veila remains the lock screen)
- NOT included (add per-install): browser/office/apps

## Session startup

greetd → regreet (shared greeter) → user picks **niriWayle** → `wayle-session` (mirrors the FeralOS niri-session: login env, `dinitctl --user start niri`, then session apps `wayle.user`/`veila.user`/`stasis.user`/`polkit-agent.user`/`wayvnc.user`, dinit-monitor hold). The Wayle shell runs FOREGROUND via `wayle shell` under dinit supervision (`wayle panel start` daemonizes — breaks process supervision, live-caught). The session wrapper `wayle-shell-session` resolves `WAYLAND_DISPLAY`/`NIRI_SOCKET` and exports the session env before `exec wayle shell` (user-dinit services start before the compositor env exists — same wrapper pattern as wayvnc-session).

## Secrets unlocking (PAM)

The oo7 stack reaches full gnome-keyring parity: auto-unlock at login, no-prompt first use, and keyring password re-sync on `passwd` — with the daemon staying a supervised per-user dinit service (`secrets.user`).

- **Login auto-unlock** (`/etc/pam.d/greetd`, appended by the installer, idempotent): `auth optional pam_oo7.so` stashes the login password in the PAM handle; `session optional pam_oo7.so auto_start` (LAST line — after the base-session includes, so `pam_turnstile`/`pam_elogind` ran and `/run/user` exists) forks as the user and sends it to the daemon's PAM listener `/run/user/$UID/oo7-pam.sock` (0600, peers: root or same UID) with a 20×100 ms retry window. The supervised daemon (started after `dbus.user`) listens there and unlocks all locked collections with the login password. oo7 0.6.0's `auto_start` also tries to exec `/usr/bin/oo7-daemon` on a missing socket — a harmless no-op on Alpine (binary lives in `/usr/libexec`), so nothing is ever started unsupervised; the retry loop hits our daemon.
- **First use, no prompt** (daemon-side, oo7 0.6.0 source-verified): the daemon auto-creates an EMPTY "Login" keyring aliased `default`; an empty keyring accepts any password, so the PAM unlock binds it to the login password and the first stored secret persists it encrypted with exactly that.
- **Password re-sync** (`/etc/pam.d/passwd`, installer-written override — Alpine ships no such file): `password include base-password` + `password optional pam_oo7.so` — a user-initiated `passwd` re-encrypts every keyring with the new password (pam_oo7 reads PAM_OLDAUTHTOK/PAM_AUTHTOK set by base-password and ships both to the daemon).
- **Known limitation** (universal, same as GNOME): root-forced resets via `chpasswd` carry no old authtok → keyrings keep the old password → next login prompts once.
- Harness check `oo7_pam_unlock` proves the chain end-to-end (run `2026-10-18T15-10-54Z`, harness 102/102): statics (`.so` on disk + both greetd lines + passwd override + `python3`) → an AUTHENTICATED PAM login on the real `greetd` service (`python3`+`ctypes` libpam client: `pam_start("greetd") → authenticate → acct_mgmt → open_session` with the configured password — runs as a plain exec-SSH script, `PAM_LOGIN_OK` marker) → a NEW "Received unlock request" line in `secrets.log` (30 s grep-poll, count-delta — the count must increase after the login; the log half runs when `enableDebug: true`, which makes the installer set `RUST_LOG=info` in `secrets.conf`). The autologin session + live daemon are untouched by the proof (no second greetd session). Client loads `pam_oo7.so` under `RTLD_GLOBAL` (pam_oo7 0.6.0 has no `DT_NEEDED libpam.so.0` — the undefined `pam_*` symbols need the global scope, else musl `dlopen` fails and PAM silently skips the `optional` module with rc=0).

## Locking & idle

`veila daemon` (veila.user) + `veila lock` (bind: Super+Alt+L; the dashboard dropdown's lock button runs the same command — see next section). The niri session lock protocol: ext-session-lock-v1; `stasis` (stasis.user — context-aware idle manager, included via `wayle-session`) handles the idle behavior.

## Session actions

The Wayle bar carries a **power button** (right end — seeded into the layout: upstream's default bar has no session entry point) which opens the dashboard dropdown; the dropdown's user-row carries the lock/logout/reboot/poweroff buttons. v0.7.0 source-verified: each button runs `sh -c <command>`, installer-seeded in `~/.config/wayle/config.toml` because upstream defaults are systemd-only and dead on FeralOS:

| Button | Command | Why |
|---|---|---|
| Lock | `veila lock` | `loginctl lock-session` is a no-op on niri (no logind Lock handler) |
| Logout | `niri msg action quit --skip-confirmation` | exits the compositor → wayle-session's dinit-monitor hold ends → session tears down to the greeter. `loginctl terminate-session` would hit the dinit polkit gap (`NoSessionForPID` → auth prompt) |
| Reboot | `sudo -n /usr/bin/reboot` (dinit) · `loginctl reboot` (OpenRC) | under dinit the logind path is broken (elogind → busybox `/sbin/reboot` SIGTERM → dinit overwrites REBOOT→HALT); the dinit wrapper + `zz-power` NOPASSWD rule is the proven path (same as niri/DMS) |
| Power off | `loginctl poweroff` | works under both inits via the elogind system-shutdown hook (`50-feralos-dinit.sh` → `dinit-poweroff`) |

## Launcher

Raffi: config `~/.config/raffi/raffi.yaml` (falls back to scanning desktop entries).

## Config paths

- Wayle: `~/.config/wayle/config.toml` (installer-seeded session commands + power bar button — see [Session actions](#session-actions); auto-created otherwise; live reload; GUI `wayle-settings`)
- Veila: `~/.config/veila/config.toml` (auto-created; themes, widgets, idle)
- niri: `~/.config/niri/config.kdl` (installer-written: env block, prefer-no-csd, binds — launcher/lock/screenshots/window management; no DMS includes)

## Harness

Fixture `tests/fixtures/variables/btrfs_standard_niriwayle_test.yaml` (dinit + edge). Checks: wayle shell running, veilad running, user dinit stack (incl. wayle.user/veila.user/stasis.user), niri layers (Wayle bar), session sockets, wayvnc, screenshot, oo7 packages installed, Key Rack flatpak installed + Secret=oo7-portal pinned (`keyrack_flatpak`), oo7 PAM unlock chain (`oo7_pam_unlock` — static PAM wiring + authenticated PAM login on service greetd + count-delta "Received unlock request" runtime proof with `enableDebug: true`), Wayle session-action seed (`wayle_session_actions` — dashboard dropdown commands + power bar button), stasis idle manager (`stasis_idle` — package + stasis.user started marker).
