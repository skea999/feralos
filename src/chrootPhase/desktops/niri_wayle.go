package desktops

import (
	"feralos/src/utils/config"
	"feralos/src/utils/shell"
	"fmt"
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
//     (secrets.user); the PAM module talks to its unix-socket listener.
//   - The /etc/sudoers.d/zz-power reboot NOPASSWD rule is KEPT (dinit-gated):
//     Wayle's power module likely goes through the same logind/loginctl path
//     that broke DMS reboots under dinit; a Wayle power-action equivalent of
//     DMS customPowerActionReboot is a follow-up once Wayle's power action
//     config key is identified.
func SetupNiriWayle(conf *config.Config) error {
	chroot := conf.Internal.ChrootDir
	u := conf.User

	fmt.Println("[INFO] Configuring niriWayle desktop environment...")

	// 1. greetd config — vt 7, greeter session via greeter.sh (shared with niri)
	greetdConf := `[terminal]
vt = 7

[default_session]
command = "/etc/greetd/greeter.sh"
user = "greeter"
`
	if err := writeFile(chroot, "/etc/greetd/config.toml", greetdConf); err != nil {
		return fmt.Errorf("write greetd config: %w", err)
	}

	// 2. greeter.sh — XDG_RUNTIME_DIR fallback (greetd starts before elogind
	// creates /run/user/<uid>), then niri + regreet. Real hardware: system
	// mesa + real GPU, no software-rendering env vars.
	greeterSh := `#!/bin/sh
if [ ! -d "/run/user/$(id -u greeter)" ]; then
    mkdir -p /tmp/greetd-runtime
    chmod 700 /tmp/greetd-runtime
    export XDG_RUNTIME_DIR=/tmp/greetd-runtime
else
    export XDG_RUNTIME_DIR="/run/user/$(id -u greeter)"
fi
exec niri -- regreet
`
	if err := writeFile(chroot, "/etc/greetd/greeter.sh", greeterSh); err != nil {
		return fmt.Errorf("write greeter.sh: %w", err)
	}
	if _, err := shell.ExecuteCmd(chroot, "chmod +x /etc/greetd/greeter.sh"); err != nil {
		return fmt.Errorf("chmod greeter.sh: %w", err)
	}

	// 3. regreet config — timezone from user variables
	regreetConf := fmt.Sprintf(`[background]
path = "/usr/share/wallpapers/default.png"
fit = "Contain"

[GTK]
application_prefer_dark_theme = true
cursor_theme_name = "Adwaita"
font_name = "Cantarell 16"
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

	// 6. xdg-desktop-portal portal backend for the user — niri-portalsconf
	// ships system default=gnome;gtk but ScreenCast needs the explicit wlr
	// backend (shared with niri). The Secret interface is pinned to
	// oo7-portal (exact config the oo7 README prescribes; without the pin
	// Secret would resolve implicitly to the gtk backend — niriWayle has
	// NO gnome-keyring, so the org.freedesktop.secrets D-Bus name must be
	// owned by oo7's portal).
	userPortals := `[preferred]
org.freedesktop.impl.portal.ScreenCast=wlr;
org.freedesktop.impl.portal.Secret=oo7-portal;
`
	if _, err := shell.ExecuteCmd(chroot, fmt.Sprintf("mkdir -p /home/%s/.config/xdg-desktop-portal", u.UserName)); err != nil {
		return fmt.Errorf("mkdir user portals dir: %w", err)
	}
	if err := writeFile(chroot, fmt.Sprintf("/home/%s/.config/xdg-desktop-portal/niri-portals.conf", u.UserName), userPortals); err != nil {
		return fmt.Errorf("write user portals conf: %w", err)
	}

	// 7. niri config for the real user — keymap, wayland env, XWayland,
	// default binds (terminal=alacritty, launcher=raffi, lock=veila).
	// Session services (dbus/pipewire/pulse/wireplumber/polkit-agent/
	// wayle/veila/wayvnc) are supervised by the USER dinit instance
	// turnstile starts at login — not spawned here.
	userNiri := fmt.Sprintf(`// CSD off (Windows-like): niri asks apps to omit client-side titlebars —
// window controls live on the Wayle bar plus keys (Mod+Q close / Mod+F
// maximize / Mod+Shift+F fullscreen). Caveats (niri docs): floating windows
// lose resize handles — resize with Mod+right-click; apps must be restarted
// for the change to apply.
prefer-no-csd

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
    XDG_SESSION_TYPE "wayland"
    XDG_SESSION_DESKTOP "niri"
    XDG_CURRENT_DESKTOP "niri"
    XDG_DATA_DIRS "/home/%s/.local/share/flatpak/exports/share:/var/lib/flatpak/exports/share:/usr/local/share:/usr/share:/home/%s/.nix-profile/share"
    QT_QPA_PLATFORM "wayland"
    QT_QPA_PLATFORMTHEME "xdgdesktopportal"
    // Hard override: Qt drops client-side decorations (niri's prefer-no-csd
    // also works via xdg-decoration negotiation; this env guarantees it —
    // value must be numeric "1", "true" parses to 0 and does NOT disable).
    QT_WAYLAND_DISABLE_WINDOWDECORATION "1"
    ELECTRON_OZONE_PLATFORM_HINT "auto"
    SDL_VIDEO_DRIVER "wayland"
    CLUTTER_BACKEND "wayland"
    _JAVA_AWT_WM_NONREPARENTING "1"
    WINIT_WAYLAND_NO_AVOID_RESIZE "1"
}

// NOTE: no hotkey-overlay skip-at-startup here — the "Important Hotkeys"
// overlay is wanted (user request; shows the keybind cheatsheet at login).
// Do not add a skip block.

// --- XWayland for X11 apps ---
xwayland-satellite {
    path "xwayland-satellite"
}

// Session services (dbus/pipewire/pulse/wireplumber/polkit-agent/wayle/
// veila/wayvnc) are supervised by the USER dinit instance turnstile starts
// at login — not spawned here (crash-restart + ordering handled by dinit).

// XDG user dirs (Documents/Downloads/...) — idempotent, no-op after first run
spawn-at-startup "sh" "-c" "command -v xdg-user-dirs-update >/dev/null 2>&1 && xdg-user-dirs-update"

// NO background-effect awk strip here (niri.go has one): the DMS layout.kdl
// include does not exist in this DE — nothing to strip.
`, u.Keymap, u.UserName, u.UserName)
	// Binds: niri's upstream default-config.kdl binds section, embedded
	// verbatim with TWO adjustments (raw string — NOT passed through
	// Sprintf, it contains % width arguments):
	//   - Mod+D launcher: fuzzel -> raffi (native iced GUI launcher)
	//   - Super+Alt+L lock: swaylock -> `veila lock` (veila daemon subcommand)
	// Mod+L keeps its default focus-column-right (the lock lives on
	// Super+Alt+L, niri's stock lock position). No wlogout references exist
	// in the upstream defaults. Runtime-only spawns kept as-is (wpctl,
	// playerctl, brightnessctl): missing binaries only error on keypress.
	userNiri += niriWayleBinds
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

	// NO DMS block in this DE (niri.go seeds settings.json/plugin_settings
	// here): no dms-shell, no customPowerActionReboot, no session.json, no
	// xdg-terminals.list/mimeapps.list seeds. Apps and defaults are
	// per-install concerns for niriWayle v1.

	// Wayle power follow-up (dinit): DMS reboots broke under dinit because
	// elogind execs the ABSOLUTE busybox /sbin/reboot (SIGTERM → dinit
	// overwrites REBOOT→HALT). DMS got customPowerActionReboot
	// (sudo -n /usr/bin/reboot). Wayle's power module config key is UNKNOWN
	// for now — the sudoers rule below is written so the wrapper works the
	// moment Wayle's power action is pointed at it (or the user runs it
	// manually); wiring it into Wayle settings is a documented follow-up.
	// Reboot wrapper NOPASSWD (wheel = admin already; reboot is no
	// privilege escalation). sudo-rs reads /etc/sudoers.d. FILENAME MUST
	// SORT AFTER "wheel": sudo-rs applies the LAST matching rule, and
	// /etc/sudoers.d/wheel (`%wheel ALL=(ALL) ALL`, password) would
	// otherwise override the NOPASSWD tag (live-caught via debug_run
	// 2026-10-11). Wired only under dinit (the wrapper is dinit's; OpenRC
	// keeps loginctl default).
	if u.EnableDinit {
		if err := writeFile(chroot, "/etc/sudoers.d/zz-power", "%wheel ALL=(root) NOPASSWD: /usr/bin/reboot\n"); err != nil {
			return fmt.Errorf("write sudoers power rule: %w", err)
		}
		if _, err := shell.ExecuteCmd(chroot, "chmod 440 /etc/sudoers.d/zz-power"); err != nil {
			return fmt.Errorf("chmod sudoers power rule: %w", err)
		}
	}

	// gsettings INFRASTRUCTURE (not a theme override; shared with niri):
	// Alpine's dconf ships NO /etc/dconf/profile/user and NO /etc/dconf at
	// all — without the profile GLib never consults the user/system dbs and
	// `dconf update` errors "Error opening directory /etc/dconf/db".
	dconfProfile := "user-db:user\nsystem-db:local\n"
	if err := writeFile(chroot, "/etc/dconf/profile/user", dconfProfile); err != nil {
		return fmt.Errorf("seed dconf profile: %w", err)
	}
	if _, err := shell.ExecuteCmd(chroot, "mkdir -p /etc/dconf/db/local.d"); err != nil {
		return fmt.Errorf("create dconf system db dir: %w", err)
	}
	if _, err := shell.ExecuteCmd(chroot, "dconf update"); err != nil {
		return fmt.Errorf("dconf update: %w", err)
	}

	// 8. NetworkManager is the network service for the niriWayle desktop
	// (Wayle network module talks to NM, same as the niri DMS widget did):
	//  - NM may manage interfaces configured by ifupdown (managed=true)
	//  - /etc/network/interfaces reduced to lo (eth0 belongs to NM).
	//    This OVERWRITES the eth0-dhcp file written by internal.setupInterfaces
	//    earlier in the chroot run — that write is dead in the niri/niriWayle
	//    flow (kept there for non-niri flows; see network.go setupInterfaces).
	//  - NM polkit rule allows the plugdev group (user added there via
	//    internal_rules groups)
	nmManage := `[ifupdown]
managed=true
`
	if err := writeFile(chroot, "/etc/NetworkManager/conf.d/10-manage.conf", nmManage); err != nil {
		return fmt.Errorf("write NM manage conf: %w", err)
	}
	if _, err := shell.ExecuteCmd(chroot, "mkdir -p /etc/NetworkManager/conf.d"); err != nil {
		return fmt.Errorf("mkdir NM conf.d: %w", err)
	}
	interfacesLo := `# NetworkManager manages wired/wireless interfaces (see
# /etc/NetworkManager/conf.d/10-manage.conf). Keep loopback here only.
auto lo
iface lo inet loopback
`
	if err := writeFile(chroot, "/etc/network/interfaces", interfacesLo); err != nil {
		return fmt.Errorf("write interfaces (lo only): %w", err)
	}

	// 9. xdg-su wrapper — niri unsupported by xdg-su, use alacritty + su
	// (shared with niri; matches *niri* so niriwayle sessions hit it too).
	xdgSu := `#!/bin/sh
# xdg-su wrapper — for niri uses alacritty+su, other DEs call the original
# Not overwritten by updates (in /usr/local/bin)

case "$XDG_CURRENT_DESKTOP" in
    *niri*)
        cmd=
        user=
        while [ $# -gt 0 ]; do
            case "$1" in
                -c) cmd="$2"; shift 2 ;;
                -u) user="$2"; shift 2 ;;
                *) shift ;;
            esac
        done
        if [ -n "$cmd" ] && command -v alacritty >/dev/null 2>&1; then
            if [ -n "$user" ]; then
                exec alacritty -e su -c "$cmd" "$user"
            else
                exec alacritty -e su -c "$cmd"
            fi
        fi
        exit 1
        ;;
esac

exec /usr/bin/xdg-su "$@"
`
	if err := writeFile(chroot, "/usr/local/bin/xdg-su", xdgSu); err != nil {
		return fmt.Errorf("write xdg-su wrapper: %w", err)
	}
	if _, err := shell.ExecuteCmd(chroot, "chmod +x /usr/local/bin/xdg-su"); err != nil {
		return fmt.Errorf("chmod xdg-su: %w", err)
	}

	// 10. groups: session user gets video/input (DRM/seat) + audio
	// (/dev/snd — PipeWire); greeter gets video/input/greetd (greetd re-owns
	// /var/lib/greetd 0750 at boot — traversal needed). seat access via
	// elogind D-Bus; render group absent on Alpine.
	if _, err := shell.ExecuteCmd(chroot, "usermod -aG video,input,greetd greeter"); err != nil {
		fmt.Printf("[WARNING] Failed to add greeter to video/input/greetd: %v\n", err)
	}
	if _, err := shell.ExecuteCmd(chroot, fmt.Sprintf("usermod -aG audio %s", u.UserName)); err != nil {
		fmt.Printf("[WARNING] Failed to add %s to audio group: %v\n", u.UserName, err)
	}

	// 11. oo7 PAM wiring — the gnome-keyring analogue for this DE.
	//   greetd stack: `auth` stashes the login password in the PAM handle;
	//   `session auto_start` (appended LAST, after the base-session
	//   includes, so pam_turnstile/pam_elogind have run and /run/user
	//   exists) forks as the user and connects to the daemon's PAM
	//   listener /run/user/$UID/oo7-pam.sock (0600, peer must be root or
	//   same UID) with a 20x100ms retry window — the supervised
	//   secrets.user daemon (started after dbus.user) listens there and
	//   unlocks all locked collections with the login password.
	//   oo7 0.6.0's auto_start also execs /usr/bin/oo7-daemon on a missing
	//   socket — harmless no-op on Alpine (binary lives in /usr/libexec);
	//   the retry loop then hits our supervised daemon. Nothing is left
	//   unsupervised (no D-Bus activation, no daemon self-spawn).
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

	// 12. turnstiled — system service: the PAM module talks to it over its
	// socket; it spawns the per-user dinit at login.
	//   dinit  — boot.d symlink (active-init enablement; FeralOS dinit = PID1)
	//   OpenRC — ALWAYS enabled, never conditional: a dinit install must stay
	//            bootable under OpenRC (fallback). The service ships in the
	//            turnstile-openrc subpackage (same daemon, same dinit user
	//            backend — turnstile is service-manager-agnostic).
	if u.EnableDinit {
		if _, err := shell.ExecuteCmd(chroot, "mkdir -p /etc/dinit.d/boot.d && test -f /etc/dinit.d/turnstiled && ln -sf /etc/dinit.d/turnstiled /etc/dinit.d/boot.d/turnstiled || true"); err != nil {
			fmt.Printf("[WARNING] Failed to enable turnstiled: %v\n", err)
		}
	}
	if _, err := shell.ExecuteCmd(chroot, "rc-update add turnstiled default"); err != nil {
		fmt.Printf("[WARNING] Failed to enable turnstiled (OpenRC): %v\n", err)
	}

	// 13. user session services: enable INFRASTRUCTURE services in boot.d of
	// the primary user service dir (/usr/lib/dinit.d/user; turnstile's dinit
	// backend also scans /etc/dinit.d/user, where packages install their
	// files). Session apps (polkit-agent/wayle/veila/wayvnc) are NOT enabled
	// here — the session script (wayle-session, via niri-session's
	// add-wants model) starts them after niri, so the greetd greeter session
	// (greeter.sh, which never runs wayle-session) never gets desktop apps.
	// Existence-filtered — only installed packages get enabled.
	userServices := "dbus.user pipewire.user pipewire-pulse.user wireplumber.user wait-wayland.user secrets.user oo7-portal.user xdg-desktop-portal.user xdg-document-portal.user xdg-permission-store.user"
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

// niriWayleBinds is the binds section of niri's upstream default-config.kdl
// (https://raw.githubusercontent.com/YaLTeR/niri/main/resources/default-config.kdl),
// embedded with two spawn adjustments: fuzzel → raffi, swaylock → veila lock.
// Raw string on purpose: the content contains % width arguments and must not
// go through Sprintf.
const niriWayleBinds = `binds {
    // Keys consist of modifiers separated by + signs, followed by an XKB key name
    // in the end. To find an XKB name for a particular key, you may use a program
    // like wev.
    //
    // "Mod" is a special modifier equal to Super when running on a TTY, and to Alt
    // when running as a winit window.
    //
    // Most actions that you can bind here can also be invoked programmatically with
    // ` + "`niri msg action do-something`" + `.

    // Mod-Shift-/, which is usually the same as Mod-?,
    // shows a list of important hotkeys.
    Mod+Shift+Slash { show-hotkey-overlay; }

    // Suggested binds for running programs: terminal, app launcher, screen locker.
    // FeralOS niriWayle: launcher = raffi, locker = veila (veila lock).
    Mod+T hotkey-overlay-title="Open a Terminal: alacritty" { spawn "alacritty"; }
    Mod+D hotkey-overlay-title="Run an Application: raffi" { spawn "raffi"; }
    Super+Alt+L hotkey-overlay-title="Lock the Screen: veila" { spawn "veila" "lock"; }

    // Use spawn-sh to run a shell command. Do this if you need pipes, multiple commands, etc.
    // Note: the entire command goes as a single argument. It's passed verbatim to ` + "`sh -c`" + `.
    // For example, this is a standard bind to toggle the screen reader (orca).
    Super+Alt+S allow-when-locked=true hotkey-overlay-title=null { spawn-sh "pkill orca || exec orca"; }

    // Example volume keys mappings for PipeWire & WirePlumber.
    // The allow-when-locked=true property makes them work even when the session is locked.
    // Using spawn-sh allows to pass multiple arguments together with the command.
    // "-l 1.0" limits the volume to 100%.
    XF86AudioRaiseVolume allow-when-locked=true { spawn-sh "wpctl set-volume @DEFAULT_AUDIO_SINK@ 0.1+ -l 1.0"; }
    XF86AudioLowerVolume allow-when-locked=true { spawn-sh "wpctl set-volume @DEFAULT_AUDIO_SINK@ 0.1-"; }
    XF86AudioMute        allow-when-locked=true { spawn-sh "wpctl set-mute @DEFAULT_AUDIO_SINK@ toggle"; }
    XF86AudioMicMute     allow-when-locked=true { spawn-sh "wpctl set-mute @DEFAULT_AUDIO_SOURCE@ toggle"; }

    // Example media keys mapping using playerctl.
    // This will work with any MPRIS-enabled media player.
    XF86AudioPlay        allow-when-locked=true { spawn-sh "playerctl play-pause"; }
    XF86AudioPause       allow-when-locked=true { spawn-sh "playerctl play-pause"; }
    XF86AudioStop        allow-when-locked=true { spawn-sh "playerctl stop"; }
    XF86AudioPrev        allow-when-locked=true { spawn-sh "playerctl previous"; }
    XF86AudioNext        allow-when-locked=true { spawn-sh "playerctl next"; }

    // Example brightness key mappings for brightnessctl.
    // You can use regular spawn with multiple arguments too (to avoid going through "sh"),
    // but you need to manually put each argument in separate "" quotes.
    XF86MonBrightnessUp allow-when-locked=true { spawn "brightnessctl" "--class=backlight" "set" "+10%"; }
    XF86MonBrightnessDown allow-when-locked=true { spawn "brightnessctl" "--class=backlight" "set" "10%-"; }

    // Open/close the Overview: a zoomed-out view of workspaces and windows.
    // You can also move the mouse into the top-left hot corner,
    // or do a four-finger swipe up on a touchpad.
    Mod+O repeat=false { toggle-overview; }

    Mod+Q repeat=false { close-window; }
    Alt+F4 repeat=false { close-window; }

    Mod+Left  { focus-column-left; }
    Mod+Down  { focus-window-down; }
    Mod+Up    { focus-window-up; }
    Mod+Right { focus-column-right; }
    Mod+H     { focus-column-left; }
    Mod+J     { focus-window-down; }
    Mod+K     { focus-window-up; }
    Mod+L     { focus-column-right; }

    Mod+Ctrl+Left  { move-column-left; }
    Mod+Ctrl+Down  { move-window-down; }
    Mod+Ctrl+Up    { move-window-up; }
    Mod+Ctrl+Right { move-column-right; }
    Mod+Ctrl+H     { move-column-left; }
    Mod+Ctrl+J     { move-window-down; }
    Mod+Ctrl+K     { move-window-up; }
    Mod+Ctrl+L     { move-column-right; }

    Mod+Home { focus-column-first; }
    Mod+End  { focus-column-last; }
    Mod+Ctrl+Home { move-column-to-first; }
    Mod+Ctrl+End  { move-column-to-last; }

    Mod+Shift+Left  { focus-monitor-left; }
    Mod+Shift+Down  { focus-monitor-down; }
    Mod+Shift+Up    { focus-monitor-up; }
    Mod+Shift+Right { focus-monitor-right; }
    Mod+Shift+H     { focus-monitor-left; }
    Mod+Shift+J     { focus-monitor-down; }
    Mod+Shift+K     { focus-monitor-up; }
    Mod+Shift+L     { focus-monitor-right; }

    Mod+Shift+Ctrl+Left  { move-column-to-monitor-left; }
    Mod+Shift+Ctrl+Down  { move-column-to-monitor-down; }
    Mod+Shift+Ctrl+Up    { move-column-to-monitor-up; }
    Mod+Shift+Ctrl+Right { move-column-to-monitor-right; }
    Mod+Shift+Ctrl+H     { move-column-to-monitor-left; }
    Mod+Shift+Ctrl+J     { move-column-to-monitor-down; }
    Mod+Shift+Ctrl+K     { move-column-to-monitor-up; }
    Mod+Shift+Ctrl+L     { move-column-to-monitor-right; }

    Mod+Page_Down      { focus-workspace-down; }
    Mod+Page_Up        { focus-workspace-up; }
    Mod+U              { focus-workspace-down; }
    Mod+I              { focus-workspace-up; }
    Mod+Ctrl+Page_Down { move-column-to-workspace-down; }
    Mod+Ctrl+Page_Up   { move-column-to-workspace-up; }
    Mod+Ctrl+U         { move-column-to-workspace-down; }
    Mod+Ctrl+I         { move-column-to-workspace-up; }

    Mod+Shift+Page_Down { move-workspace-down; }
    Mod+Shift+Page_Up   { move-workspace-up; }
    Mod+Shift+U         { move-workspace-down; }
    Mod+Shift+I         { move-workspace-up; }

    // You can bind mouse wheel scroll ticks using the following syntax.
    // These binds will change direction based on the natural-scroll setting.
    //
    // To avoid scrolling through workspaces really fast, you can use
    // the cooldown-ms property. The bind will be rate-limited to this value.
    Mod+WheelScrollDown      cooldown-ms=150 { focus-workspace-down; }
    Mod+WheelScrollUp        cooldown-ms=150 { focus-workspace-up; }
    Mod+Ctrl+WheelScrollDown cooldown-ms=150 { move-column-to-workspace-down; }
    Mod+Ctrl+WheelScrollUp   cooldown-ms=150 { move-column-to-workspace-up; }

    Mod+WheelScrollRight      { focus-column-right; }
    Mod+WheelScrollLeft       { focus-column-left; }
    Mod+Ctrl+WheelScrollRight { move-column-right; }
    Mod+Ctrl+WheelScrollLeft  { move-column-left; }

    // You can refer to workspaces by index. However, keep in mind that
    // niri is a dynamic workspace system, so these commands are kind of
    // "best effort".
    Mod+1 { focus-workspace 1; }
    Mod+2 { focus-workspace 2; }
    Mod+3 { focus-workspace 3; }
    Mod+4 { focus-workspace 4; }
    Mod+5 { focus-workspace 5; }
    Mod+6 { focus-workspace 6; }
    Mod+7 { focus-workspace 7; }
    Mod+8 { focus-workspace 8; }
    Mod+9 { focus-workspace 9; }
    Mod+Ctrl+1 { move-column-to-workspace 1; }
    Mod+Ctrl+2 { move-column-to-workspace 2; }
    Mod+Ctrl+3 { move-column-to-workspace 3; }
    Mod+Ctrl+4 { move-column-to-workspace 4; }
    Mod+Ctrl+5 { move-column-to-workspace 5; }
    Mod+Ctrl+6 { move-column-to-workspace 6; }
    Mod+Ctrl+7 { move-column-to-workspace 7; }
    Mod+Ctrl+8 { move-column-to-workspace 8; }
    Mod+Ctrl+9 { move-column-to-workspace 9; }

    // The following binds move the focused window in and out of a column.
    // If the window is alone, they will consume it into the nearby column to the side.
    // If the window is already in a column, they will expel it out.
    Mod+BracketLeft  { consume-or-expel-window-left; }
    Mod+BracketRight { consume-or-expel-window-right; }

    // Consume one window from the right to the bottom of the focused column.
    Mod+Comma  { consume-window-into-column; }
    // Expel the bottom window from the focused column to the right.
    Mod+Period { expel-window-from-column; }

    // Cycle through widths set in preset-column-widths.
    Mod+R { switch-preset-column-width; }
    // Cycling through the presets in reverse order is also possible.
    Mod+Shift+R { switch-preset-column-width-back; }

    Mod+Ctrl+Shift+R { switch-preset-window-height; }
    Mod+Ctrl+R { reset-window-height; }

    Mod+F { maximize-column; }
    Mod+Shift+F { fullscreen-window; }

    // While maximize-column leaves gaps and borders around the window,
    // maximize-window-to-edges doesn't: the window expands to the edges of the screen.
    Mod+M { maximize-window-to-edges; }

    // Expand the focused column to space not taken up by other fully visible columns.
    Mod+Ctrl+F { expand-column-to-available-width; }

    Mod+C { center-column; }

    // Center all fully visible columns on screen.
    Mod+Ctrl+C { center-visible-columns; }

    // Finer width adjustments.
    // This command can also:
    // * set width in pixels: "1000"
    // * adjust width in pixels: "-5" or "+5"
    // * set width as a percentage of screen width: "25%"
    // * adjust width as a percentage of screen width: "-10%" or "+10%"
    Mod+Minus { set-column-width "-10%"; }
    Mod+Equal { set-column-width "+10%"; }

    // Finer height adjustments when in column with other windows.
    Mod+Shift+Minus { set-window-height "-10%"; }
    Mod+Shift+Equal { set-window-height "+10%"; }

    // Move the focused window between the floating and the tiling layout.
    Mod+V       { toggle-window-floating; }
    Mod+Shift+V { switch-focus-between-floating-and-tiling; }

    // Toggle tabbed column display mode.
    Mod+W { toggle-column-tabbed-display; }

    Print { screenshot; }
    Ctrl+Print { screenshot-screen; }
    Alt+Print { screenshot-window; }

    // Applications such as remote-desktop clients and software KVM switches may
    // request that niri stops processing the keyboard shortcuts defined here
    // so they may, for example, forward the key presses as-is to a remote machine.
    // It is a good idea to bind an escape hatch to toggle the inhibitor,
    // so a buggy application cannot hold your session hostage.
    Mod+Escape allow-inhibiting=false { toggle-keyboard-shortcuts-inhibit; }

    // The quit action will show a confirmation dialog to avoid accidental exits.
    Mod+Shift+E { quit; }
    Ctrl+Alt+Delete { quit; }

    // Powers off the monitors. To turn them back on, do any input like
    // moving the mouse or pressing any other key.
    Mod+Shift+P { power-off-monitors; }
}
`
