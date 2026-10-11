package desktops

import (
	"feralos/src/utils/config"
	"feralos/src/utils/shell"
	"fmt"
	"strings"
)

// SetupNiri configures the niri desktop environment:
// greetd (greeter), regreet, niri configs (greeter + user), session startup
// through turnstile + Alpine's unmodified niri-session (user dinit service),
// DMS (native apk, spawned by the niri session), PipeWire audio, and the
// polkit agent.
//
// Real hardware only — the VirtualBox/Hyper-V software-rendering workarounds
// (LIBGL_ALWAYS_SOFTWARE, QT_QUICK_BACKEND=software, WLR_NO_HARDWARE_CURSORS,
// eglfilter, fixed custom-mode spawns) are intentionally absent. Reference:
// references/niri-alpine/niri-real-hardware.md + installer-gaps.md.
//
// DMS is the feralos-aports native package (dms-shell, musl): no nix, no
// nixGL, no execfilter. The old /etc/init.d/dms OpenRC service is gone on
// purpose — DMS must be spawned by the niri session (spawn-at-startup):
// it needs WAYLAND_DISPLAY and the seat session; an OpenRC service hung
// boot and duplicated the shell (FIX-niri-dms.md rounds 1-2).
func SetupNiri(conf *config.Config) error {
	chroot := conf.Internal.ChrootDir
	u := conf.User

	fmt.Println("[INFO] Configuring niri desktop environment...")

	// 1. greetd config — vt 7, greeter session via greeter.sh
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

	// 4. greeter user + dirs. greetd apk does not create the 'greeter' user.
	// The greeter's niri reads its config from its HOME (per-home, not
	// /etc/xdg — gap 5.7): overlay off at the login screen (round 14).
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
	if _, err := shell.ExecuteCmd(chroot, "getent passwd greeter >/dev/null 2>&1 || adduser -D -s /bin/sh -h /var/lib/greetd greeter"); err != nil {
		return fmt.Errorf("create greeter user: %w", err)
	}
	if _, err := shell.ExecuteCmd(chroot, "mkdir -p /var/lib/greetd/.config/niri"); err != nil {
		return fmt.Errorf("mkdir greeter niri config dir: %w", err)
	}
	if err := writeFile(chroot, "/var/lib/greetd/.config/niri/config.kdl", greeterNiri); err != nil {
		return fmt.Errorf("write greeter niri config: %w", err)
	}
	// ReGreet state dir must be writable by the greeter user, else last
	// user/session is never remembered (gap 5.8, round 15). mkdir first:
	// the dir only exists after greetd-regreet installs, and a missing dir
	// would make chown -R fail on fresh installs.
	if _, err := shell.ExecuteCmd(chroot, "mkdir -p /var/lib/regreet"); err != nil {
		return fmt.Errorf("mkdir /var/lib/regreet: %w", err)
	}
	// Greeter user-dinit log dir: the greeter's user dinit services
	// (dbus-session etc.) log under $HOME/.local/share/dinit (dinit logfile),
	// same layout as the session user below — without it they fail with
	// "opening log file". Covered by the chown -R greeter:greeter below.
	if _, err := shell.ExecuteCmd(chroot, "mkdir -p /var/lib/greetd/.local/share/dinit"); err != nil {
		return fmt.Errorf("mkdir greeter dinit log dir: %w", err)
	}
	if _, err := shell.ExecuteCmd(chroot, "chown -R greeter:greeter /var/lib/greetd /var/lib/regreet && chmod 700 /var/lib/regreet"); err != nil {
		return fmt.Errorf("fix greeter/regreet ownership: %w", err)
	}

	// 5. Session startup — no wrapper: Alpine's /usr/bin/niri-session is used
	// UNMODIFIED. greetd sets SHELL from passwd; the user is created with
	// /usr/libexec/feralos/login-shell (NOT in /etc/shells), so niri-session
	// skips its bash re-exec and reaches the dinit branch: turnstile (PAM,
	// base-session) starts the user dinit at login and `dinitctl --user start
	// niri` starts the `niri` service (niri-user-dinit); niri-shutdown clears
	// the session env on exit. XDG session env comes from /etc/environment
	// (pam_env in the turnstiled PAM stack) + the niri config environment
	// block below.
	environmentFile := fmt.Sprintf(`XDG_SESSION_DESKTOP=niri
XDG_DATA_DIRS=/home/%s/.local/share/flatpak/exports/share:/var/lib/flatpak/exports/share:/usr/local/share:/usr/share
`, u.UserName)
	if err := writeFile(chroot, "/etc/environment", environmentFile); err != nil {
		return fmt.Errorf("write /etc/environment: %w", err)
	}

	// 6. xdg-desktop-portal portal backend for the user — niri-portalsconf
	// ships system default=gnome;gtk but ScreenCast needs the explicit wlr
	// backend (gap 5.12; round 43 used a patched portal on the reference VM).
	userPortals := `[preferred]
org.freedesktop.impl.portal.ScreenCast=wlr;
`
	if _, err := shell.ExecuteCmd(chroot, fmt.Sprintf("mkdir -p /home/%s/.config/xdg-desktop-portal", u.UserName)); err != nil {
		return fmt.Errorf("mkdir user portals dir: %w", err)
	}
	if err := writeFile(chroot, fmt.Sprintf("/home/%s/.config/xdg-desktop-portal/niri-portals.conf", u.UserName), userPortals); err != nil {
		return fmt.Errorf("write user portals conf: %w", err)
	}

	// 8. niri config for the real user — keymap, wayland env, XWayland.
	// Session services live in the user dinit instance started by turnstile
	// at login; RustDesk removed — remote desktop = wayvnc (user dinit
	// service, round 37/43).
	userNiri := fmt.Sprintf(`input {
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
    XDG_DATA_DIRS "/home/%s/.local/share/flatpak/exports/share:/var/lib/flatpak/exports/share:/usr/local/share:/usr/share"
    QT_QPA_PLATFORM "wayland"
    QT_QPA_PLATFORMTHEME "gtk3"
    ELECTRON_OZONE_PLATFORM_HINT "auto"
    SDL_VIDEO_DRIVER "wayland"
    CLUTTER_BACKEND "wayland"
    _JAVA_AWT_WM_NONREPARENTING "1"
    WINIT_WAYLAND_NO_AVOID_RESIZE "1"
}

// --- XWayland for X11 apps ---
xwayland-satellite {
    path "xwayland-satellite"
}

// Session services (dbus/pipewire/pulse/wireplumber/polkit-agent/dms/wayvnc)
// are supervised by the USER dinit instance turnstile starts at login — not
// spawned here (crash-restart + ordering handled by dinit).

// XDG user dirs (Documents/Downloads/...) — idempotent, no-op after first run
spawn-at-startup "sh" "-c" "command -v xdg-user-dirs-update >/dev/null 2>&1 && xdg-user-dirs-update"

include "dms/binds.kdl"
include "dms/layout.kdl"
include "dms/colors.kdl"
`, u.Keymap, u.UserName)
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
	// Ownership BEFORE dms setup: the mkdir/writes above ran as root —
	// without this, `dms setup` (su <user>) cannot create
	// .config/niri/dms/ and every fragment setup fails (run 2026-09-26).
	if _, err := shell.ExecuteCmd(chroot, fmt.Sprintf("chown -R %s:%s /home/%s/.config /home/%s/.local", u.UserName, u.UserName, u.UserName, u.UserName)); err != nil {
		return fmt.Errorf("chown user config: %w", err)
	}
	// dms fragments (binds/layout/colors) must exist before niri validates
	// the includes — pre-create with the packaged CLI (chroot, root is fine:
	// fragments land in the invoking user's config only via su; use su).
	for _, setup := range []string{"binds", "layout", "colors"} {
		if _, err := shell.ExecuteCmd(chroot, fmt.Sprintf("su - %s -c 'dms setup %s'", u.UserName, setup)); err != nil {
			fmt.Printf("[WARNING] dms setup %s failed: %v\n", setup, err)
		}
	}
	// DMS theme seed: green (Settings → Theme & Colors → Generic grid) +
	// bar at the bottom. settings.json does not exist until DMS's first
	// launch — writing it pre-seeds the keys DMS loads and merges on
	// startup. Bar position uses the v1→v2 MIGRATION path (configVersion 1
	// + legacy dankBarAtBottom): DMS builds the FULL default barConfigs with
	// position=1 itself — seeding a partial barConfigs would replace the
	// whole default array (raw assignment, no deep merge) and break the bar,
	// and duplicating the full default would drift with DMS versions.
	// dankBarLeftWidgets swaps the focused-window widget for running apps
	// (same migration path; center/right keep following upstream defaults).
	dmsSettingsDir := fmt.Sprintf("/home/%s/.config/DankMaterialShell", u.UserName)
	if _, err := shell.ExecuteCmd(chroot, fmt.Sprintf("mkdir -p '%s' && printf '{\\n  \"configVersion\": 1,\\n  \"currentThemeCategory\": \"generic\",\\n  \"currentThemeName\": \"green\",\\n  \"dankBarAtBottom\": true,\\n  \"dankBarLeftWidgets\": [\"launcherButton\", \"workspaceSwitcher\", \"runningApps\"],\\n  \"updaterUseCustomCommand\": true,\\n  \"updaterCustomCommand\": \"/usr/local/bin/feralos-update\"\\n}\\n' > '%s/settings.json' && chown %s:%s '%s/settings.json'", dmsSettingsDir, dmsSettingsDir, u.UserName, u.UserName, dmsSettingsDir)); err != nil {
		return fmt.Errorf("seed DMS settings.json (green theme, bottom bar): %w", err)
	}
	// DMS dark mode seed (belt+braces): fresh installs already default dark
	// (SharedSessionSpec.js isLightMode def=false; PortalService applies mode
	// changes only after the first read), but the explicit seed guarantees
	// dark portal-independently. Session store = XDG state dir, NOT config.
	dmsStateDir := fmt.Sprintf("/home/%s/.local/state/DankMaterialShell", u.UserName)
	if _, err := shell.ExecuteCmd(chroot, fmt.Sprintf("mkdir -p '%s' && printf '{\\n  \"isLightMode\": false\\n}\\n' > '%s/session.json' && chown %s:%s '%s/session.json'", dmsStateDir, dmsStateDir, u.UserName, u.UserName, dmsStateDir)); err != nil {
		return fmt.Errorf("seed DMS session.json (dark mode): %w", err)
	}
	// DMS default apps: the Settings → Default Apps tab is a frontend for the
	// freedesktop defaults — terminal via ~/.config/xdg-terminals.list
	// (xdg-terminal-exec; DMS writes `<Id>.desktop` lines), everything else
	// via ~/.config/mimeapps.list [Default Applications] (DMS reads/writes it
	// through DesktopService). Seeding the standard files = DMS shows the
	// values as set on first launch. Desktop IDs verified against Alpine
	// (pkgs.alpinelinux.org contents): firefox.desktop, org.gnome.eog.desktop
	// (NOT eog.desktop), com.system76.CosmicPlayer.desktop, Alacritty.desktop.
	// MIME lists = DMS DefaultAppsTab.mimeMapping verbatim.
	if err := writeFile(chroot, fmt.Sprintf("/home/%s/.config/xdg-terminals.list", u.UserName), "Alacritty.desktop"); err != nil {
		return fmt.Errorf("seed default terminal (Alacritty): %w", err)
	}
	// DMS reads icons through QT_QPA_PLATFORMTHEME=gtk3 → GTK settings —
	// without gtk-icon-theme-name apps fall back to hicolor generics (the
	// "missing icons" effect). Adwaita = GNOME default, coherent with the
	// GNOME apps shipped; prefer-dark keeps GTK apps coherent with DMS dark
	// mode. Icon themes = dms-shell deps (aports 14f124b).
	gtkSettings := `[Settings]
gtk-icon-theme-name=Adwaita
gtk-application-prefer-dark-theme=true`
	if err := writeFile(chroot, fmt.Sprintf("/home/%s/.config/gtk-3.0/settings.ini", u.UserName), gtkSettings); err != nil {
		return fmt.Errorf("seed GTK settings (icon theme + dark): %w", err)
	}
	mimeapps := `[Default Applications]
x-scheme-handler/https=firefox.desktop
x-scheme-handler/http=firefox.desktop
text/html=firefox.desktop
application/xhtml+xml=firefox.desktop
image/png=org.gnome.eog.desktop
image/jpeg=org.gnome.eog.desktop
image/gif=org.gnome.eog.desktop
image/bmp=org.gnome.eog.desktop
image/webp=org.gnome.eog.desktop
image/avif=org.gnome.eog.desktop
image/svg+xml=org.gnome.eog.desktop
image/tiff=org.gnome.eog.desktop
image/heic=org.gnome.eog.desktop
image/heif=org.gnome.eog.desktop
image/jxl=org.gnome.eog.desktop
	audio/mpeg=com.system76.CosmicPlayer.desktop
audio/x-flac=com.system76.CosmicPlayer.desktop
audio/wav=com.system76.CosmicPlayer.desktop
audio/ogg=com.system76.CosmicPlayer.desktop
audio/aac=com.system76.CosmicPlayer.desktop
audio/webm=com.system76.CosmicPlayer.desktop
x-scheme-handler/geo=org.gnome.Maps.desktop
text/calendar=org.gnome.Calendar.desktop
x-scheme-handler/calendar=org.gnome.Calendar.desktop
x-scheme-handler/webcal=org.gnome.Calendar.desktop
x-scheme-handler/webcals=org.gnome.Calendar.desktop`
	if err := writeFile(chroot, fmt.Sprintf("/home/%s/.config/mimeapps.list", u.UserName), mimeapps); err != nil {
		return fmt.Errorf("seed default apps (mimeapps.list): %w", err)
	}
	if _, err := shell.ExecuteCmd(chroot, fmt.Sprintf("chown %s:%s '/home/%s/.config/xdg-terminals.list' '/home/%s/.config/mimeapps.list' '/home/%s/.config/gtk-3.0/settings.ini'", u.UserName, u.UserName, u.UserName, u.UserName, u.UserName)); err != nil {
		return fmt.Errorf("chown default app seeds: %w", err)
	}
	// DMS 1.6 layout template emits `background-effect { xray false }` — a
	// niri node added after 25.11 (ext-background-effect-v1, unsupported by
	// Alpine 3.24 niri) that breaks the whole include (run 2026-09-26,
	// niri_config_valid). Strip the block: blur is unsupported on this niri
	// anyway (FIX-niri-dms.md round 6).
	layoutKdl := fmt.Sprintf("/home/%s/.config/niri/dms/layout.kdl", u.UserName)
	awk := `awk '/background-effect[[:space:]]*\{/{skip=1; next} skip{ if (/\}/) skip=0; next } {print}'`
	if _, err := shell.ExecuteCmd(chroot, fmt.Sprintf("%s '%s' > '%s.tmp' && mv '%s.tmp' '%s'", awk, layoutKdl, layoutKdl, layoutKdl, layoutKdl)); err != nil {
		fmt.Printf("[WARNING] failed to strip background-effect from layout.kdl: %v\n", err)
	}

	// 7. NetworkManager is the network service for the niri desktop
	// (rounds 30/44 — DMS network/VPN UI needs the NM backend):
	//  - NM may manage interfaces configured by ifupdown (managed=true)
	//  - /etc/network/interfaces reduced to lo (eth0 belongs to NM; the
	//    stale DHCP ifupdown config caused double default routes).
	//    This OVERWRITES the eth0-dhcp file written by internal.setupInterfaces
	//    earlier in the chroot run — that write is dead in the niri flow
	//    (kept there for non-niri flows; see network.go setupInterfaces).
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

	// 8. CUPS printer admin: SystemGroup stays at the Alpine default
	// (lpadmin, in cups-files.conf) — the session user is added to the
	// lpadmin group instead of widening CUPS admin to all of wheel
	// (round 24: without membership, CUPS answers 403 on CUPS-Get-Devices).

	// 9. xdg-su wrapper — niri unsupported by xdg-su, use alacritty + su
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
	// (/dev/snd — PipeWire, round 19); greeter gets video/input/greetd
	// (greetd re-owns /var/lib/greetd 0750 at boot — traversal needed).
	// seat access via elogind D-Bus; render group absent on Alpine.
	if _, err := shell.ExecuteCmd(chroot, "usermod -aG video,input,greetd greeter"); err != nil {
		fmt.Printf("[WARNING] Failed to add greeter to video/input/greetd: %v\n", err)
	}
	if _, err := shell.ExecuteCmd(chroot, fmt.Sprintf("usermod -aG audio %s", u.UserName)); err != nil {
		fmt.Printf("[WARNING] Failed to add %s to audio group: %v\n", u.UserName, err)
	}

	// 11. gnome-keyring: unlock the login keyring with the login password
	// when the PAM session opens (greetd PAM). Only the auth line is added
	// here — the `session ... auto_start` line already ships in Alpine's
	// base-session. Secrets themselves are D-Bus activated on demand
	// (org.freedesktop.secrets service ships with the package).
	pamGreetd := "/etc/pam.d/greetd"
	if _, err := shell.ExecuteCmd(chroot, "grep -q 'auth.*pam_gnome_keyring.so' "+pamGreetd+" || echo 'auth       optional pam_gnome_keyring.so' >> "+pamGreetd); err != nil {
		fmt.Printf("[WARNING] Failed to add pam_gnome_keyring to greetd PAM: %v\n", err)
	}

	// 12. turnstiled — system service: the PAM module talks to it over its
	// socket; it spawns the per-user dinit at login. Symlink into boot.d
	// (admin dir; FeralOS dinit runs as PID1 with enableDinit).
	if _, err := shell.ExecuteCmd(chroot, "mkdir -p /etc/dinit.d/boot.d && test -f /etc/dinit.d/turnstiled && ln -sf /etc/dinit.d/turnstiled /etc/dinit.d/boot.d/turnstiled || true"); err != nil {
		fmt.Printf("[WARNING] Failed to enable turnstiled: %v\n", err)
	}

	// 13. user session services: enable INFRASTRUCTURE services in boot.d of
	// the primary user service dir (/usr/lib/dinit.d/user; turnstile's dinit
	// backend also scans /etc/dinit.d/user, where packages install their
	// files). Session apps (polkit-agent/dms/wayvnc) are NOT enabled here —
	// upstream "add-wants niri.service" model: niri-session starts them
	// after niri (aports niri-user-dinit), so the greetd greeter session
	// (greeter.sh, which never runs niri-session) never gets desktop apps.
	// Existence-filtered — only installed packages get enabled.
	userServices := "dbus.user pipewire.user pipewire-pulse.user wireplumber.user wait-wayland.user"
	if _, err := shell.ExecuteCmd(chroot, fmt.Sprintf("mkdir -p /usr/lib/dinit.d/user/boot.d && for s in %s; do [ -f /etc/dinit.d/user/$s ] && ln -sf /etc/dinit.d/user/$s /usr/lib/dinit.d/user/boot.d/$s; done", userServices)); err != nil {
		fmt.Printf("[WARNING] Failed to enable user dinit services: %v\n", err)
	}

	fmt.Println("[INFO] niri desktop environment configured.")
	return nil
}

// writeFile writes a file inside the chroot via shell (echo pattern).
func writeFile(chroot, path, content string) error {
	escaped := strings.ReplaceAll(content, "'", "'\\''")
	_, err := shell.ExecuteCmd(chroot, fmt.Sprintf("mkdir -p \"$(dirname '%s')\" && echo '%s' > '%s'", path, escaped, path))
	return err
}
