package desktops

import (
	"feralos/src/utils/config"
	"feralos/src/utils/shell"
	"fmt"
	"strings"
)

// SetupNiriWayle configures the niriWayle desktop: niri compositor + Wayle
// shell + Veila lock screen + Raffi launcher + oo7 secrets. Shares
// greetd/regreet greeter, turnstile user session, pipewire stack, portals
// and polkit agent with the niri DE.
//
// Derived from SetupNiri. Differences:
//   - NO DMS at all (no dms setup fragments, no DMS settings/plugin/session
//     seeds, no default-app seeds, no background-effect awk strip — there are
//     no dms/ includes to strip).
//   - The user niri config embeds niri's DEFAULT binds (upstream
//     default-config.kdl) with terminal=alacritty, launcher=raffi and
//     lock=veila instead of fuzzel/swaylock.
//   - oo7 PAM login-unlock (greetd stack) + passwd-stack keyring password
//     re-sync — the gnome-keyring analogue for this DE (step 11). The
//     secrets daemon itself stays a supervised per-user dinit service
//     (org.freedesktop.secrets.user); the PAM module talks to its unix-socket listener.
//   - Wayle session actions ARE wired (step 7b): the dashboard dropdown's
//     user-row buttons (lock/logout/reboot/poweroff — upstream v0.7.0
//     source-verified) run installer-seeded commands from
//     ~/.config/wayle/config.toml (upstream defaults are systemd-only and
//     dead on FeralOS). Reboot goes through the dinit wrapper
//     (`sudo -n /usr/bin/reboot`, the proven DMS path) via the zz-power
//     NOPASSWD rule below; the power bar button is seeded into the bar
//     layout (upstream's default bar has no session entry point) and opens
//     the dashboard dropdown where those buttons live.
func SetupNiriWayle(conf *config.Config) error {
	chroot := conf.Internal.ChrootDir
	u := conf.User
	// DE rules block (internal_rules desktopEnvironments.niriWayle): greeter
	// groups + user dinit boot.d infrastructure units live there.
	deRules := conf.Internal.DesktopEnvironments[u.DesktopEnvironment]

	fmt.Println("[INFO] Configuring niriWayle desktop environment...")

	// greetd config.toml + greeter.sh are package-shipped (ADR-013):
	// greetd-config (common) + dms-config/wayle-config (DE-specific script).

	// 3. regreet config — timezone from user variables
	// Font: DejaVu Sans (font-dejavu installed) — Cantarell is NOT in the
	// package list; a missing font forces full fontconfig fallback resolution
	// on every greeter start (slow-boot contributor).
	// Wallpaper: the FeralOS default (installed by installBrandingAssets in
	// step 3a below — the path must match its 1080p target).
	regreetConf := fmt.Sprintf(`[background]
path = "/usr/share/wallpapers/feralos-default.png"

[GTK]
application_prefer_dark_theme = true
cursor_theme_name = "Adwaita"
font_name = "DejaVu Sans 16"
icon_theme_name = "Adwaita"
theme_name = "Adwaita"

	[commands]
reboot = ["reboot"]
poweroff = ["poweroff"]

[appearance]
greeting_msg = "Welcome back!"

[widget.clock]
format = "%%a %%H:%%M"
resolution = "500ms"
timezone = "%s"
`, u.Timezone)
	if err := writeFile(chroot, "/etc/greetd/regreet.toml", regreetConf); err != nil {
		return fmt.Errorf("write regreet config: %w", err)
	}

	// 3a. Branding (logo + wallpapers) ships in niri-base-config — nothing
	// to install here.

	// 3b. Hide the upstream bare `niri` session from regreet (real-hardware
	// fix): the greeter showed BOTH the upstream niri session and the FeralOS
	// niriWayle session; NoDisplay hides it (ReGreet 0.2.0 respects the
	// Desktop Entry Hidden/NoDisplay fields). apk-safe: the local modification
	// survives niri upgrades as .apk-new.
	if _, err := shell.ExecuteCmd(chroot, `grep -q '^NoDisplay=true' /usr/share/wayland-sessions/niri.desktop 2>/dev/null || printf 'NoDisplay=true\n' >> /usr/share/wayland-sessions/niri.desktop`); err != nil {
		return fmt.Errorf("hide bare niri session from regreet: %w", err)
	}

	// 4. greeter user + dirs (shared with niri; full rationale in niri.go):
	// -S system user hides the greeter from ReGreet's UID-filtered list;
	// explicit -G avoids busybox adduser -S's nogroup primary-group trap.
	if _, err := shell.ExecuteCmd(chroot, "getent passwd greeter >/dev/null 2>&1 || { getent group greeter >/dev/null 2>&1 || addgroup -S greeter; adduser -S -G greeter -D -s /bin/sh -h /var/lib/greetd greeter; }"); err != nil {
		return fmt.Errorf("create greeter user: %w", err)
	}
	greeterNiri := fmt.Sprintf(`input {
    keyboard {
        xkb {
            layout "%s"
        }
    }
}

hotkey-overlay {
    skip-at-startup
}
`, u.Keymap)
	if _, err := shell.ExecuteCmd(chroot, "mkdir -p /var/lib/greetd/.config/niri"); err != nil {
		return fmt.Errorf("mkdir greeter niri config dir: %w", err)
	}
	if err := writeFile(chroot, "/var/lib/greetd/.config/niri/config.kdl", greeterNiri); err != nil {
		return fmt.Errorf("write greeter niri config: %w", err)
	}
	if _, err := shell.ExecuteCmd(chroot, "mkdir -p /var/lib/regreet"); err != nil {
		return fmt.Errorf("mkdir /var/lib/regreet: %w", err)
	}
	// Greeter user-dinit log dir (same layout as the session user below).
	if _, err := shell.ExecuteCmd(chroot, "mkdir -p /var/lib/greetd/.local/share/dinit"); err != nil {
		return fmt.Errorf("mkdir greeter dinit log dir: %w", err)
	}
	if _, err := shell.ExecuteCmd(chroot, "chown -R greeter:greeter /var/lib/greetd /var/lib/regreet && chmod 700 /var/lib/regreet"); err != nil {
		return fmt.Errorf("fix greeter/regreet ownership: %w", err)
	}

	// 4b. Pre-build fontconfig cache for the greeter user — GTK4's first
	// text render with a cold cache scans every system font (noto-cjk +
	// noto-extra = thousands of files): the consistent 30-60s gray screen
	// before the regreet login (real-install report 2026-11-15).
	// SEQUENCE MATTERS: (1) create the greeter USER cache dir and hand it
	// to greeter BEFORE fc-cache; (2) fc-cache as root with HOME pointed
	// there — fontconfig then writes the USER cache into the writable dir
	// (running fc-cache as the greeter user failed permission-denied on
	// the system cache, and root fc-cache alone never populates the user
	// cache — run17 "No such file"). Result: the greeter finds a valid
	// prebuilt user cache at first boot — no scan, fast login. The
	// login_readiness harness check measures whether it actually helps.
	if _, err := shell.ExecuteCmd(chroot, "mkdir -p /var/lib/greetd/.cache/fontconfig && chown -R greeter:greeter /var/lib/greetd/.cache"); err != nil {
		fmt.Printf("[WARNING] failed to prepare greeter fontconfig cache dir: %v\n", err)
	}
	if _, err := shell.ExecuteCmd(chroot, "env HOME=/var/lib/greetd fc-cache -f"); err != nil {
		fmt.Printf("[WARNING] fontconfig cache pre-build for greeter failed (non-fatal): %v\n", err)
	}
	if _, err := shell.ExecuteCmd(chroot, "chown -R greeter:greeter /var/lib/greetd/.cache"); err != nil {
		fmt.Printf("[WARNING] failed to chown greeter fontconfig cache: %v\n", err)
	}

	// 4c. Login-shell trick (moved from users.go — desktop-owned now, runs
	// AFTER user creation: chrootPhase step 5 users < step 9 desktops).
	// wayle-session depends on niri-user-dinit, so the login-shell binary
	// arrives via that dependency; same rationale as niri.go 4b (niri-
	// session bash re-exec skip + pkexec /etc/shells acceptance). The three
	// steps are idempotent.
	if _, err := shell.ExecuteCmd(chroot, fmt.Sprintf("usermod -s /usr/libexec/login-shell %s", u.UserName)); err != nil {
		fmt.Printf("[WARNING] Failed to set %s shell to login-shell: %v\n", u.UserName, err)
	}
	if _, err := shell.ExecuteCmd(chroot, "ln -sf /usr/bin/fish /usr/libexec/login-shell"); err != nil {
		fmt.Printf("[WARNING] Failed to re-point login-shell to fish: %v\n", err)
	}
	if _, err := shell.ExecuteCmd(chroot, "grep -qxF '/usr/libexec/login-shell' /etc/shells || echo '/usr/libexec/login-shell' >> /etc/shells"); err != nil {
		fmt.Printf("[WARNING] Failed to list login-shell in /etc/shells: %v\n", err)
	}

	// 5. Session startup — wayle-session (wayle-session package) replaces
	// Alpine's niri-session as the greetd session command; it depends on
	// niri-user-dinit, so the login-shell trick prerequisites (login-shell +
	// the `niri` user service) arrive via that dependency. turnstile (PAM)
	// starts the user dinit at login; wayle-session then starts the session
	// stack. XDG session env comes from /etc/environment (pam_env in the
	// turnstiled PAM stack) + the niri config environment block below.
	// NOTE: XDG_*_DESKTOP stays "niri" (not "niriwayle") — xdg-desktop-portal
	// resolves niri-portals.conf from XDG_CURRENT_DESKTOP, and the xdg-su
	// wrapper matches *niri*; the compositor IS niri either way.
	environmentFile := fmt.Sprintf(`XDG_SESSION_DESKTOP=niri
XDG_DATA_DIRS=/home/%s/.local/share/flatpak/exports/share:/var/lib/flatpak/exports/share:/usr/local/share:/usr/share:/home/%s/.nix-profile/share
`, u.UserName, u.UserName)
	if err := writeFile(chroot, "/etc/environment", environmentFile); err != nil {
		return fmt.Errorf("write /etc/environment: %w", err)
	}

	// 6. xdg-desktop-portal backend — the ADMIN-path override ships in
	// wayle-config (/etc/xdg-desktop-portal/niri-portals.conf: default=gtk,
	// wlr Screenshot/ScreenCast, Secret=oo7-portal — the oo7 README pin;
	// niriWayle has NO gnome-keyring so the org.freedesktop.secrets D-Bus
	// name must be owned by oo7's portal). XDG precedence: $XDG_CONFIG_HOME
	// > /etc > /usr/share — with no user file the admin conf wins, same
	// effective state as the per-user seed it replaces. The Alpine system
	// niri-portals.conf (default=gnome;gtk) is still shadowed by the
	// niri-portalsconf 999 dummy in the package list.

	// 7. niri config for the real user — the per-install parts ONLY (input
	// keymap + the user-path XDG_DATA_DIRS): the STATIC base (prefer-no-csd,
	// environment minus DATA_DIRS, xwayland-satellite, the full binds set —
	// upstream defaults + raffi/veila adjustments) ships in wayle-config as
	// /usr/share/feralos/wayle/niri/base.kdl and MERGES per-property (niri
	// >=25.11 include). User customization = edit THIS file: a binds block
	// here overrides individual keys over the package base, and the home
	// file is never package-re-asserted on updates (owner decision
	// 2026-11-23). Session services (dbus/pipewire/pulse/wireplumber/
	// polkit-agent/wayle/veila/wayvnc) are supervised by the USER dinit
	// instance turnstile starts at login — not spawned here.
	userNiri := fmt.Sprintf(`// FeralOS niriWayle — user config (installer-written, user-editable).
// The STATIC base ships in wayle-config and merges per-property:
//     include "/usr/share/feralos/wayle/niri/base.kdl"
// (prefer-no-csd, wayland env, xwayland-satellite, the full binds set —
// terminal=alacritty, launcher=raffi, lock=veila, window switcher=raffi).
// Only the PER-INSTALL parts live here. To override a single bind, add
// your own binds block below — only the keys you set change.

include "/usr/share/feralos/wayle/niri/base.kdl"

input {
    keyboard {
        xkb {
            rules "evdev"
            model "pc105"
            layout "%s"
        }
    }
}

environment {
    XDG_DATA_DIRS "/home/%s/.local/share/flatpak/exports/share:/var/lib/flatpak/exports/share:/usr/local/share:/usr/share:/home/%s/.nix-profile/share"
}
`, u.Keymap, u.UserName, u.UserName)
	if _, err := shell.ExecuteCmd(chroot, fmt.Sprintf("mkdir -p /home/%s/.config/niri", u.UserName)); err != nil {
		return fmt.Errorf("mkdir user niri config dir: %w", err)
	}
	if err := writeFile(chroot, fmt.Sprintf("/home/%s/.config/niri/config.kdl", u.UserName), userNiri); err != nil {
		return fmt.Errorf("write user niri config: %w", err)
	}
	// user dinit log files live under ~/.local/share/dinit (dinit logfile)
	if _, err := shell.ExecuteCmd(chroot, fmt.Sprintf("mkdir -p /home/%s/.local/share/dinit", u.UserName)); err != nil {
		return fmt.Errorf("mkdir user dinit log dir: %w", err)
	}
	// Ownership BEFORE the seeds below: the mkdir/writes above ran as root.
	if _, err := shell.ExecuteCmd(chroot, fmt.Sprintf("chown -R %s:%s /home/%s/.config /home/%s/.local", u.UserName, u.UserName, u.UserName, u.UserName)); err != nil {
		return fmt.Errorf("chown user config: %w", err)
	}

	// 7b. Wayle config seed — session actions. v0.7.0 source-verified: the
	// dashboard dropdown's user-row icon buttons (lock/logout/reboot/
	// poweroff) run `sh -c <dashboard.dropdown_*_command>`; upstream
	// defaults are systemd-only and DEAD on FeralOS:
	//   - reboot: `systemctl reboot`. Even the logind path is broken under
	//     dinit — elogind execs the ABSOLUTE busybox /sbin/reboot after its
	//     hooks and the SIGTERM makes dinit overwrite REBOOT -> HALT
	//     (2026-10-12 diagnosis, "system halted" instead of a restart). The
	//     proven path is the dinit wrapper via the zz-power rule below:
	//     `sudo -n /usr/bin/reboot` (same command DMS got via
	//     customPowerActionReboot). OpenRC keeps `loginctl reboot`
	//     (busybox reboot is fine when dinit does not trap the signal).
	//   - poweroff: `loginctl poweroff` works under BOTH inits — the
	//     elogind system-shutdown hook (50-feralos-dinit.sh) execs
	//     dinit-poweroff and dinit ignores the busybox follow-up SIGUSR2.
	//   - lock: default `loginctl lock-session` is a no-op on niri (no
	//     logind Lock handler) — veila owns the session lock.
	//   - logout: `niri msg action quit --skip-confirmation` — exits the
	//     compositor; wayle-session's dinit-monitor hold ends and the
	//     session tears down to the greeter (the Mod+Shift+E path without
	//     the interactive confirmation prompt). NOT loginctl
	//     terminate-session: under dinit polkit cannot attribute the
	//     session (NoSessionForPID) so it would hit an auth prompt, while
	//     niri's quit action needs no polkit.
	// Windows-style bottom bar (novice-friendly, user request 2026-10-27,
	// refined 2026-10-31 with the user's wayle-settings customizations):
	//   - location=bottom, exclusive=true (default — reserves space, no
	//     floating), background-opacity 0 (fully transparent bar — only
	//     buttons visible), rounding md; buttons: variant basic, rounding
	//     lg, gap 0.25; styling rounding lg (dropdowns/dialogs).
	//   - left: custom-launcher (Raffi via Mod+D or click) + niri-workspaces
	//     + window-title (focused window). center: empty. right: systray,
	//     icon-only status (battery/bluetooth/network/mic/volume),
	//     notifications, custom-windows (live window list — click opens the
	//     Raffi window switcher, mirrored by Mod+W), clock (HH:MM),
	//     dashboard (far right — lock/logout/reboot/poweroff dropdown,
	//     borders shown).
	//   - theme: theme-provider = "matugen" — the palette is generated from
	//     the wallpaper by matugen (installed); no static palette seeded.
	//     NOTE: matugen needs a wallpaper set (wayle wallpaper config) —
	//     without one the palette falls back to wayle defaults.
	//   - notifications popup: bottom-right.
	//   - dashboard dropdown = session actions (see step 7b rationale above).
	//     left-click is seeded EXPLICITLY even though v0.7.0's default equals
	//     it: wayle's config is a layered system (default < config.toml <
	//     runtime) applied per-leaf-path, so a partial table does NOT lose
	//     the default (proven live: `wayle config get
	//     modules.dashboard.left-click` = "dropdown:dashboard" AND a real
	//     VNC click opened the panel on run 2026-10-28T00-10-10Z) — the
	//     explicit line only pins the intent against upstream default drift.
	// wayleConf = 100% static since wayle-config 0.1.1: the dashboard's
	// reboot command is the package-shipped /usr/bin/feralos-reboot wrapper
	// (init-agnostic — dinit picks `sudo -n /usr/bin/reboot` via
	// /run/dinitctl, OpenRC keeps `loginctl reboot`), so the template no
	// longer branches on EnableDinit. The seed stays installer-written:
	// /home files are installer-owned (owner decision 2026-11-23) and the
	// config must remain user-editable — a package-owned seed re-asserted
	// by every update would override manual changes.
	wayleConf := `[bar]
location = "bottom"
background-opacity = 0
rounding = "md"
button-variant = "basic"
button-rounding = "lg"
button-gap = 0.25

[styling]
theme-provider = "matugen"
rounding = "lg"

[modules.notifications]
popup-position = "bottom-right"

[[modules.custom]]
id = "launcher"
icon-name = "feralos-logo"
label-show = false
left-click = "/usr/bin/raffi-toggle"

# Window list (user request 2026-10-31): live count + app-id summary from
# niri IPC, refreshed on every niri event (mode = "watch" — the command
# loops on the event stream itself; restart-policy = "on-exit" so a dead
# watcher comes back). Click = Raffi window switcher (same as Mod+W):
# -i 'win ' pre-fills the query so the window source is what you get.
[[modules.custom]]
id = "windows"
mode = "watch"
restart-policy = "on-exit"
command = '''
niri msg event-stream | while read -r _; do
  niri msg --json windows | jq -r '{
    count: length,
    text: ([.[] | .app_id] | unique | join(" · ")),
    alt: (if any(.[]; .is_focused) then "active" else "idle" end)
  }'
done
'''
format = "{{ text }}"
tooltip-format = "{{ count }} windows — click to switch"
icon-name = "ld-app-window-symbolic"
left-click = "raffi -i 'win '"

[modules.window-title]
label-max-length = 50

[modules.clock]
format = "%H:%M"

[modules.battery]
label-show = false

[modules.bluetooth]
label-show = false

[modules.network]
label-show = false

[modules.microphone]
label-show = false

[modules.volume]
label-show = false

[modules.dashboard]
left-click = "dropdown:dashboard"
border-show = true
dropdown-lock-command = "veila lock"
dropdown-logout-command = "niri msg action quit --skip-confirmation"
dropdown-reboot-command = "feralos-reboot"
dropdown-poweroff-command = "loginctl poweroff"

[[bar.layout]]
monitor = "*"
left = ["custom-launcher", "niri-workspaces", "window-title"]
center = []
	right = ["systray", "battery", "bluetooth", "network", "microphone", "volume", "notifications", "custom-windows", "clock", "dashboard"]
`
	if _, err := shell.ExecuteCmd(chroot, fmt.Sprintf("mkdir -p /home/%s/.config/wayle", u.UserName)); err != nil {
		return fmt.Errorf("mkdir wayle config dir: %w", err)
	}
	if err := writeFile(chroot, fmt.Sprintf("/home/%s/.config/wayle/config.toml", u.UserName), wayleConf); err != nil {
		return fmt.Errorf("write wayle config: %w", err)
	}

	// NO DMS block in this DE (niri.go seeds settings.json/plugin_settings
	// here): no dms-shell, no customPowerActionReboot, no session.json, no
	// xdg-terminals.list/mimeapps.list seeds. Apps and defaults are
	// per-install concerns for niriWayle v1.

	// Reboot NOPASSWD rule backing the reboot path: under dinit the seeded
	// Wayle command resolves (via the package-shipped /usr/bin/feralos-reboot
	// wrapper) to `sudo -n /usr/bin/reboot`, which must run without a
	// password prompt. wheel = admin already; reboot is no privilege
	// escalation. sudo-rs reads /etc/sudoers.d. FILENAME MUST SORT AFTER
	// "wheel": sudo-rs applies the LAST matching rule, and
	// /etc/sudoers.d/wheel (`%wheel ALL=(ALL) ALL`, password) would
	// otherwise override the NOPASSWD tag (live-caught via debug_run
	// 2026-10-11). Wired only under dinit (the wrapper's sudo path is
	// dinit's; OpenRC keeps `loginctl reboot`, no sudo involved).
	if u.EnableDinit {
		if err := writeFile(chroot, "/etc/sudoers.d/zz-power", "%wheel ALL=(root) NOPASSWD: /usr/bin/reboot\n"); err != nil {
			return fmt.Errorf("write sudoers power rule: %w", err)
		}
		if _, err := shell.ExecuteCmd(chroot, "chmod 440 /etc/sudoers.d/zz-power"); err != nil {
			return fmt.Errorf("chmod sudoers power rule: %w", err)
		}
	}

	// gsettings INFRASTRUCTURE: /etc/dconf/profile/user ships in
	// niri-base-config (Alpine's dconf ships NO /etc/dconf at all — without
	// the profile GLib never consults the user/system dbs; no keys exist so
	// no `dconf update` is needed).

	// 8. NetworkManager is the network service for the niriWayle desktop
	// (Wayle network module talks to NM): both statics ship in
	// niri-base-config — /etc/NetworkManager/conf.d/10-manage.conf
	// (managed=true) and /etc/network/interfaces reduced to loopback (it
	// OVERWRITES the eth0-dhcp file internal.setupInterfaces wrote earlier;
	// that write is dead in the niri-family flow).

	// 9. xdg-su wrapper — niri unsupported by xdg-su, use alacritty + su
	// (shared with niri; matches *niri* so niriwayle sessions hit it).
	// STAYS installer-side: packages must not write under /usr/local
	// (abuild rule) and the /usr/local location is what survives vendor
	// xdg-su updates.
	if err := installXdgSuWrapper(chroot); err != nil {
		return err
	}

	// 10. groups: the session user's groups come from the rules (DE `groups`
	// — incl. audio for /dev/snd, PipeWire); the greeter joins the DE
	// `greeterGroups` (greetd re-owns /var/lib/greetd 0750 at boot —
	// traversal needed). seat access via elogind D-Bus; render group absent
	// on Alpine.
	if _, err := shell.ExecuteCmd(chroot, fmt.Sprintf("usermod -aG %s greeter", strings.Join(deRules.GreeterGroups, ","))); err != nil {
		fmt.Printf("[WARNING] Failed to add greeter to greeterGroups: %v\n", err)
	}

	// 11. oo7 PAM wiring — the gnome-keyring analogue for this DE.
	//   greetd stack: `auth` stashes the login password in the PAM handle;
	//   `session auto_start` (appended LAST, after the base-session
	//   includes, so pam_turnstile/pam_elogind have run and /run/user
	//   exists) forks as the user and connects to the daemon's PAM
	//   listener /run/user/$UID/oo7-pam.sock (0600, peer must be root or
	//   same UID) with a 20x100ms retry window — the supervised
	//   org.freedesktop.secrets.user daemon (started after dbus.user) listens there and
	//   unlocks all locked collections with the login password.
	//   On a missing socket the module also execs /usr/bin/oo7-daemon —
	//   that path is OUR recovery wrapper (aports oo7-user-dinit), which
	//   delegates to `dinitctl --user` (start, or restart when the service
	//   is up but its socket was lost with /run/user): the daemon may only
	//   ever run as a supervised user-dinit service (user rule — nothing
	//   unsupervised, ever). Nothing is left to D-Bus activation or
	//   daemon self-spawn.
	//   passwd stack: the /etc/pam.d/passwd override adds pam_oo7 after
	//   base-password so a user-initiated `passwd` re-encrypts every
	//   keyring with the new password (root resets via chpasswd carry no
	//   old authtok and cannot sync — universal limitation, same as
	//   GNOME).
	//   First use needs no prompt: the daemon auto-creates an EMPTY
	//   "Login" keyring aliased default; an empty keyring accepts any
	//   password, so the PAM unlock binds it to the login password and
	//   the first stored secret persists it encrypted with exactly that.
	pamGreetd := "/etc/pam.d/greetd"
	if _, err := shell.ExecuteCmd(chroot, "grep -q 'auth.*pam_oo7.so' "+pamGreetd+" || echo 'auth       optional pam_oo7.so' >> "+pamGreetd); err != nil {
		fmt.Printf("[WARNING] Failed to add pam_oo7 auth to greetd PAM: %v\n", err)
	}
	if _, err := shell.ExecuteCmd(chroot, "grep -q 'session.*pam_oo7.so' "+pamGreetd+" || echo 'session    optional pam_oo7.so auto_start' >> "+pamGreetd); err != nil {
		fmt.Printf("[WARNING] Failed to add pam_oo7 session to greetd PAM: %v\n", err)
	}
	// Alpine ships NO /etc/pam.d/passwd (passwd falls back to the vendor
	// `other` stack); this override shadows ONLY the passwd service.
	//nolint:gosec // G101: PAM stack config text, not a credential
	passwdPam := `#%PAM-1.0
#
# FeralOS (niriWayle): oo7 keyring password re-sync. The password stack
# runs for user-initiated passwd changes; pam_oo7 (after base-password,
# which sets PAM_OLDAUTHTOK/PAM_AUTHTOK) sends old+new to the oo7 daemon,
# which re-encrypts all matching keyrings. Root resets (chpasswd) carry
# no old authtok and cannot sync - universal limitation, same as GNOME.
password include base-password
password optional pam_oo7.so
`
	if err := writeFile(chroot, "/etc/pam.d/passwd", passwdPam); err != nil {
		return fmt.Errorf("write /etc/pam.d/passwd override: %w", err)
	}
	// Daemon log level — the INFO line "Received unlock request" is the
	// end-to-end delivery proof (the harness oo7_pam_unlock runtime check
	// greps it), but per user directive only enableDebug installs carry
	// INFO logging. Written into the ADR-008 config the secrets wrapper
	// (aports oo7-user-dinit) sources and exports before exec'ing the
	// daemon, so it survives user-dinit respawns (dinitctl setenv would
	// apply too late — boot.d starts the daemon before the session
	// scripts run). Idempotent: set in place or append.
	if u.EnableDebug {
		if _, err := shell.ExecuteCmd(chroot, "mkdir -p /etc/dinit.d/config && f=/etc/dinit.d/config/secrets.conf; touch \"$f\"; if grep -q '^RUST_LOG=' \"$f\"; then sed -i 's/^RUST_LOG=.*/RUST_LOG=info/' \"$f\"; else echo 'RUST_LOG=info' >> \"$f\"; fi"); err != nil {
			fmt.Printf("[WARNING] Failed to set RUST_LOG=info for the oo7 daemon: %v\n", err)
		}
	}

	// 12. turnstiled enablement is RULES-DRIVEN: the DE `services` map
	// carries "turnstiled": "default" (internal_rules.yaml) — services.go
	// enables it on both inits (OpenRC: rc-update default, unconditional;
	// dinit: boot.d symlink resolving /lib/dinit.d first, /etc/dinit.d
	// fallback — turnstiled ships in /etc/dinit.d). Nothing desktop-specific
	// left here.

	// 13. user session services: enable INFRASTRUCTURE services in boot.d of
	// the primary user service dir (/usr/lib/dinit.d/user; turnstile's dinit
	// backend also scans /etc/dinit.d/user, where packages install their
	// files). The unit list comes from the rules (DE `userServices` —
	// verbatim `.user` unit names). Session apps (polkit-agent/wayle/veila/
	// wayvnc) are NOT enabled here — the session script (wayle-session, via
	// niri-session's add-wants model) starts them after niri, so the greetd
	// greeter session (greeter.sh, which never runs wayle-session) never
	// gets desktop apps. Existence-filtered — only installed packages get
	// enabled.
	userServices := strings.Join(deRules.UserServices, " ")
	if _, err := shell.ExecuteCmd(chroot, fmt.Sprintf("mkdir -p /usr/lib/dinit.d/user/boot.d && for s in %s; do [ -f /etc/dinit.d/user/$s ] && ln -sf /etc/dinit.d/user/$s /usr/lib/dinit.d/user/boot.d/$s; done", userServices)); err != nil {
		fmt.Printf("[WARNING] Failed to enable user dinit services: %v\n", err)
	}

	// 14. Final ownership sweep — seeded configs must never stay root-owned
	// (per-seed chowns cover FILES; the mkdir-created DIRECTORIES need this
	// sweep AFTER every seed).
	if _, err := shell.ExecuteCmd(chroot, fmt.Sprintf("chown -R %s:%s /home/%s/.config /home/%s/.local", u.UserName, u.UserName, u.UserName, u.UserName)); err != nil {
		return fmt.Errorf("final chown user config: %w", err)
	}

	fmt.Println("[INFO] niriWayle desktop environment configured.")
	return nil
}
