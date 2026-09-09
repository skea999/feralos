package desktops

import (
	"alpineInstaller/utils/config"
	"alpineInstaller/utils/shell"
	"fmt"
	"strings"
)

// SetupNiri configures the niri desktop environment:
// greetd (greeter), regreet, niri configs (greeter + user),
// xdg-su wrapper, and the OpenRC dms service (adapted from the systemd
// user service shipped by the DankMaterialShell flake).
// Reference: references/niri/install-niri.sh (openSUSE/systemd → Alpine/OpenRC).
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

	// 2. greeter.sh — XDG_RUNTIME_DIR (fallback to /tmp when turnstile
	// hasn't created /run/user/<uid> yet), software GL, niri + regreet.
	greeterSh := `#!/bin/sh
if [ ! -d "/run/user/$(id -u greeter)" ]; then
    mkdir -p /tmp/greetd-runtime
    chmod 700 /tmp/greetd-runtime
    export XDG_RUNTIME_DIR=/tmp/greetd-runtime
else
    export XDG_RUNTIME_DIR="/run/user/$(id -u greeter)"
fi
export LIBGL_ALWAYS_SOFTWARE=true
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

	// 4. niri config for the greeter session (keymap from user variables)
	greeterNiri := fmt.Sprintf(`input {
    keyboard {
        xkb {
            layout "%s"
        }
    }
}

environment {
    WLR_NO_HARDWARE_CURSORS "1"
}
`, u.Keymap)
	// greetd needs a 'greeter' user for the greeter session (greetd apk
	// does not create one). Home = /var/lib/greetd so niri/regreet pick up
	// the configs written there.
	if _, err := shell.ExecuteCmd(chroot, "getent passwd greeter >/dev/null 2>&1 || adduser -D -s /bin/sh -h /var/lib/greetd greeter"); err != nil {
		return fmt.Errorf("create greeter user: %w", err)
	}
	if _, err := shell.ExecuteCmd(chroot, "mkdir -p /var/lib/greetd/.config/niri"); err != nil {
		return fmt.Errorf("mkdir greeter niri config dir: %w", err)
	}
	if err := writeFile(chroot, "/var/lib/greetd/.config/niri/config.kdl", greeterNiri); err != nil {
		return fmt.Errorf("write greeter niri config: %w", err)
	}
	if _, err := shell.ExecuteCmd(chroot, "chown -R greeter:greeter /var/lib/greetd"); err != nil {
		return fmt.Errorf("chown /var/lib/greetd: %w", err)
	}

	// 5. niri config for the real user — keymap, wayland environment.
	userNiriBase := fmt.Sprintf(`input {
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
    SDL_VIDEO_DRIVER "wayland"
    CLUTTER_BACKEND "wayland"
    _JAVA_AWT_WM_NONREPARENTING "1"
    WLR_NO_HARDWARE_CURSORS "1"
    WINIT_WAYLAND_NO_AVOID_RESIZE "1"
}
`, u.Keymap)
	// RustDesk autostart — only if flatpak com.rustdesk.RustDesk is installed
	hasRustdesk := false
	for _, f := range conf.User.FlatpakPackages {
		if strings.Contains(f, "rustdesk") {
			hasRustdesk = true
			break
		}
	}
	userNiri := userNiriBase
	if hasRustdesk {
		userNiri += "spawn-at-startup \"flatpak\" \"run\" \"com.rustdesk.RustDesk\" \"--service\"\n"
		// Firewall for RustDesk direct IP (21115-21119/tcp + 21116/udp)
		if _, err := shell.ExecuteCmd(chroot, "firewall-cmd --permanent --add-port=21115-21119/tcp 2>/dev/null || true"); err != nil {
			fmt.Printf("[WARNING] Failed to add rustdesk tcp ports to firewall: %v\n", err)
		}
		if _, err := shell.ExecuteCmd(chroot, "firewall-cmd --permanent --add-port=21116/udp 2>/dev/null || true"); err != nil {
			fmt.Printf("[WARNING] Failed to add rustdesk udp port to firewall: %v\n", err)
		}
	}
	if _, err := shell.ExecuteCmd(chroot, fmt.Sprintf("mkdir -p /home/%s/.config/niri", u.UserName)); err != nil {
		return fmt.Errorf("mkdir user niri config dir: %w", err)
	}
	if err := writeFile(chroot, fmt.Sprintf("/home/%s/.config/niri/config.kdl", u.UserName), userNiri); err != nil {
		return fmt.Errorf("write user niri config: %w", err)
	}
	if _, err := shell.ExecuteCmd(chroot, fmt.Sprintf("chown -R %s:%s /home/%s/.config/niri", u.UserName, u.UserName, u.UserName)); err != nil {
		return fmt.Errorf("chown user niri config: %w", err)
	}

	// 6. xdg-su wrapper — niri unsupported by xdg-su, use alacritty + su
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

	// 7. OpenRC dms service — adapted from the systemd user service shipped
	// by the DMS flake. dms needs a running wayland session: wait for the
	// user's wayland socket, then start dms with WAYLAND_DISPLAY set.
	dmsPath := fmt.Sprintf("/home/%s/.local/state/nix/profiles/profile/bin/dms", u.UserName)
	dmsInit := fmt.Sprintf(`#!/sbin/openrc-run
name="dms"
description="Dank Material Shell (wayland shell)"
command="%s"
command_user="%s"
command_background=true
pidfile="/run/dms.pid"

depend() {
    need localmount
    after bootmisc
}

start() {
    ebegin "Starting DMS"
    # dms needs the real user's wayland session, which only exists after
    # login — so start asynchronously: wait for the socket in the
    # background and launch dms when it appears. Never blocks boot.
    (
        local sock
        local i=0
        while [ $i -lt 60 ]; do
            sock=$(ls /run/user/$(id -u "$command_user")/wayland-* 2>/dev/null | head -1)
            [ -n "$sock" ] && break
            sleep 5
            i=$((i + 1))
        done
        if [ -z "$sock" ]; then
            ewarn "No wayland session found for user %s after 5 minutes"
            return 0
        fi
        start-stop-daemon --start --background --make-pidfile --pidfile "$pidfile" \
            --user "$command_user" --env WAYLAND_DISPLAY="$(basename "$sock")" \
            --exec "$command"
    ) &
    eend 0
}

stop() {
    ebegin "Stopping DMS"
    start-stop-daemon --stop --pidfile "$pidfile"
    eend $?
}
`, dmsPath, u.UserName, u.UserName)
	if err := writeFile(chroot, "/etc/init.d/dms", dmsInit); err != nil {
		return fmt.Errorf("write dms init script: %w", err)
	}
	if _, err := shell.ExecuteCmd(chroot, "chmod +x /etc/init.d/dms"); err != nil {
		return fmt.Errorf("chmod dms init: %w", err)
	}
	if _, err := shell.ExecuteCmd(chroot, "rc-update add dms default"); err != nil {
		fmt.Printf("[WARNING] Failed to add dms to default runlevel: %v\n", err)
	}

	// 8. greeter needs video/input groups for the greeter session, plus the
	// greetd group: greetd's init script re-owns /var/lib/greetd to
	// greetd:greetd 0750 at every boot, so greeter (home there) must be able
	// to traverse it to reach its niri config.
	// 9. greeter needs input/video groups for the greeter session
	// (render group does not exist on Alpine — video covers DRM access;
	// seat access is provided by elogind via D-Bus, no group needed)
	// greetd group: /var/lib/greetd is 750 greetd:greetd — greeter must
	// traverse it to read its niri config.
	if _, err := shell.ExecuteCmd(chroot, "usermod -aG video,input,greetd greeter"); err != nil {
		fmt.Printf("[WARNING] Failed to add greeter to video/input/greetd: %v\n", err)
	}

	// 9. Generate DMS user config (binds/layout/colors) — non-fatal.
	// Use the full path: the nix profile bin dir is not on the login PATH.
	dmsBin := dmsPath
	for _, setup := range []string{"binds", "layout", "colors"} {
		if _, err := shell.ExecuteCmd(chroot, fmt.Sprintf("su - %s -c '%s setup %s'", u.UserName, dmsBin, setup)); err != nil {
			fmt.Printf("[WARNING] dms setup %s failed: %v\n", setup, err)
		}
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
