package desktops

import (
	"alpineInstaller/utils/config"
	"alpineInstaller/utils/shell"
	"fmt"
	"strings"
)

// SetupNiri configures the niri desktop environment:
// greetd (greeter), regreet, niri configs (greeter + user), the POSIX
// niri-session wrapper (session env + D-Bus session bus), DMS (native apk,
// spawned by the niri session), PipeWire audio, and the polkit agent.
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
	if _, err := shell.ExecuteCmd(chroot, "chown -R greeter:greeter /var/lib/greetd /var/lib/regreet && chmod 700 /var/lib/regreet"); err != nil {
		return fmt.Errorf("fix greeter/regreet ownership: %w", err)
	}

	// 5. niri-session wrapper (/usr/local/bin wins over /usr/bin): greetd
	// provides neither the session env nor a D-Bus session bus (gap 5.4).
	// The wrapper also starts the USER DINIT instance supervising the
	// session services (dbus/pipewire/pulse/wireplumber via *-user-dinit
	// packages; polkit-agent, dms — wayland-gated). Existence-filtered:
	// a service file exists only if its package is installed, so the same
	// wrapper works on any desktop. Idempotent across re-logins: if the
	// user manager control socket is alive, services are already running
	// (supervised — crashed ones were restarted by dinit).
	niriSession := `#!/bin/sh
export XDG_SESSION_TYPE=wayland
export XDG_CURRENT_DESKTOP=niri
export DBUS_SESSION_BUS_ADDRESS="unix:path=${XDG_RUNTIME_DIR}/bus"
if [ ! -S "$XDG_RUNTIME_DIR/dinit.ctl" ]; then
    svcs=""
    for s in dbus.user pipewire.user pipewire-pulse.user wireplumber.user \
             wait-wayland.user polkit-agent.user dms.user; do
        [ -f "/etc/dinit.d/user/$s" ] && svcs="$svcs $s"
    done
    [ -n "$svcs" ] && dinit -d /etc/dinit.d/user $svcs >/dev/null 2>&1 &
fi
if [ ! -S "$XDG_RUNTIME_DIR/bus" ]; then
    dbus-daemon --session --fork --address="unix:path=$XDG_RUNTIME_DIR/bus" 2>/dev/null || true
fi
exec /usr/bin/niri-session "$@"
`
	if err := writeFile(chroot, "/usr/local/bin/niri-session", niriSession); err != nil {
		return fmt.Errorf("write niri-session wrapper: %w", err)
	}
	if _, err := shell.ExecuteCmd(chroot, "chmod +x /usr/local/bin/niri-session"); err != nil {
		return fmt.Errorf("chmod niri-session wrapper: %w", err)
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

	// 7. user session logs dir — DMS/PipeWire spawns redirect here
	// (gap 5.5, rounds 3/19: missing dir = the spawn's >> redirect dies).
	if _, err := shell.ExecuteCmd(chroot, fmt.Sprintf("mkdir -p /home/%s/.local/log/user", u.UserName)); err != nil {
		return fmt.Errorf("mkdir user log dir: %w", err)
	}

	// 8. niri config for the real user — keymap, wayland env, session
	// services via spawn-at-startup (greetd starts none of them), DMS,
	// polkit agent (MUST be the Alpine/elogind build — gap 5.9), XWayland
	// (gap 5.10), portal includes. No software-rendering workarounds.
	hasRustdesk := false
	for _, f := range conf.User.FlatpakPackages {
		if strings.Contains(f, "rustdesk") {
			hasRustdesk = true
			break
		}
	}
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
    XDG_CURRENT_DESKTOP "niri"
    QT_QPA_PLATFORM "wayland"
    QT_QPA_PLATFORMTHEME "gtk3"
    SDL_VIDEO_DRIVER "wayland"
    CLUTTER_BACKEND "wayland"
    _JAVA_AWT_WM_NONREPARENTING "1"
    WINIT_WAYLAND_NO_AVOID_RESIZE "1"
}

// --- XWayland for X11 apps ---
xwayland-satellite {
    path "xwayland-satellite"
}

// Session services (dbus/pipewire/pulse/wireplumber/polkit-agent/dms) are
// supervised by the USER dinit instance started by the niri-session
// wrapper — not spawned here (crash-restart + ordering handled by dinit).

include "dms/binds.kdl"
include "dms/layout.kdl"
include "dms/colors.kdl"
`, u.Keymap)
	if hasRustdesk {
		userNiri += "spawn-at-startup \"flatpak\" \"run\" \"com.rustdesk.RustDesk\" \"--service\"\n"
		// Firewall for RustDesk direct IP (21115-21119/tcp + 21116/udp) — use offline cmd (firewalld not running in chroot)
		if _, err := shell.ExecuteCmd(chroot, "firewall-offline-cmd --zone=public --add-port=21115-21119/tcp 2>&1 || true"); err != nil {
			fmt.Printf("[WARNING] Failed to add rustdesk tcp ports to firewall: %v\n", err)
		}
		if _, err := shell.ExecuteCmd(chroot, "firewall-offline-cmd --zone=public --add-port=21116/udp 2>&1 || true"); err != nil {
			fmt.Printf("[WARNING] Failed to add rustdesk udp ports to firewall: %v\n", err)
		}
	}
	if _, err := shell.ExecuteCmd(chroot, fmt.Sprintf("mkdir -p /home/%s/.config/niri", u.UserName)); err != nil {
		return fmt.Errorf("mkdir user niri config dir: %w", err)
	}
	if err := writeFile(chroot, fmt.Sprintf("/home/%s/.config/niri/config.kdl", u.UserName), userNiri); err != nil {
		return fmt.Errorf("write user niri config: %w", err)
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

	fmt.Println("[INFO] niri desktop environment configured.")
	return nil
}

// writeFile writes a file inside the chroot via shell (echo pattern).
func writeFile(chroot, path, content string) error {
	escaped := strings.ReplaceAll(content, "'", "'\\''")
	_, err := shell.ExecuteCmd(chroot, fmt.Sprintf("mkdir -p \"$(dirname '%s')\" && echo '%s' > '%s'", path, escaped, path))
	return err
}
