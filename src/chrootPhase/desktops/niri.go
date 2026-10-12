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
	// -S (system user, UID 100-999): ReGreet 0.2.0 lists /etc/passwd users
	// with UID in [UID_MIN, UID_MAX] (1000-60000 per /etc/login.defs) —
	// without -S the greeter account (UID 1001) shows up as a login option.
	// System UIDs are filtered out. GOTCHA: busybox adduser -S without -G
	// defaults the primary group to "nogroup" and creates NO same-name
	// group (loginutils/adduser.c: usegroup = "nogroup"), breaking the
	// later chown greeter:greeter — so create the system group explicitly
	// and pass -G. Side effects checked: turnstile spawns the user dinit
	// for any non-root login (greeter included); explicit -s /bin/sh
	// overrides the busybox system-user /bin/false default.
	if _, err := shell.ExecuteCmd(chroot, "getent passwd greeter >/dev/null 2>&1 || { getent group greeter >/dev/null 2>&1 || addgroup -S greeter; adduser -S -G greeter -D -s /bin/sh -h /var/lib/greetd greeter; }"); err != nil {
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
	userNiri := fmt.Sprintf(`// CSD off (Windows-like): niri asks apps to omit client-side titlebars —
// window controls live on the DMS bar (dms-window-buttons widget) plus keys
// (Mod+Q close / Mod+F maximize / Mod+Shift+F fullscreen). DMS's own embedded
// niri config ships prefer-no-csd active too. Caveats (niri docs): floating
// windows lose resize handles — resize with Mod+right-click; apps must be
// restarted for the change to apply.
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
    XDG_DATA_DIRS "/home/%s/.local/share/flatpak/exports/share:/var/lib/flatpak/exports/share:/usr/local/share:/usr/share"
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
	// DMS theme seed: green (Settings → Theme & Colors → Generic grid), bar
	// on TOP. settings.json does not exist until DMS's first launch —
	// writing it pre-seeds the keys DMS loads and merges on startup. Bar
	// position uses the v1→v2 MIGRATION path (configVersion 1): DMS builds
	// the FULL default barConfigs itself — seeding a partial barConfigs
	// would replace the whole default array (raw assignment, no deep merge)
	// and break the bar, and duplicating the full default would drift with
	// DMS versions. dankBarLeftWidgets swaps the focused-window widget for
	// running apps; dankBarRightWidgets pins the right section + appends the
	// dmsWindowButtons plugin (Windows-style maximize/fullscreen/close for
	// the focused window — system-wide plugin under
	// /etc/xdg/quickshell/dms-plugins, package dms-window-buttons).
	dmsSettingsDir := fmt.Sprintf("/home/%s/.config/DankMaterialShell", u.UserName)
	// DMS reboot fix (no elogind rebuild): DMS runs `loginctl reboot`
	// (isElogind branch) → elogind execs the ABSOLUTE /sbin/reboot (busybox)
	// → SIGTERM to dinit → dinit overwrites the shutdown type REBOOT→HALT =
	// machine halts, never reboots (poweroff survives: busybox SIGUSR2 is
	// ignored by dinit; diagnosis: run 2026-10-11T20-38-40Z live test).
	// SessionService (1.6.2) runs `customPowerActionReboot` as the session
	// user via `sh -c` → `sudo -n /usr/bin/reboot` = our dinit wrapper,
	// NOPASSWD-granted to wheel for this exact binary (below). Wired only
	// under dinit (the wrapper is dinit's; OpenRC keeps loginctl default).
	settingsJSON := "{\n  \"configVersion\": 1,\n  \"currentThemeCategory\": \"generic\",\n  \"currentThemeName\": \"green\",\n  \"dankBarLeftWidgets\": [\"launcherButton\", \"workspaceSwitcher\", \"runningApps\"],\n  \"dankBarRightWidgets\": [\"systemTray\", \"clipboard\", \"cpuUsage\", \"memUsage\", \"notificationButton\", \"battery\", \"controlCenterButton\", \"dmsWindowButtons\"],\n  \"updaterUseCustomCommand\": true,\n  \"updaterCustomCommand\": \"/usr/local/bin/feralos-update\""
	if u.EnableDinit {
		settingsJSON += ",\n  \"customPowerActionReboot\": \"sudo -n /usr/bin/reboot\""
	}
	settingsJSON += "\n}\n"
	if err := writeFile(chroot, fmt.Sprintf("%s/settings.json", dmsSettingsDir), settingsJSON); err != nil {
		return fmt.Errorf("seed DMS settings.json: %w", err)
	}
	if u.EnableDinit {
		// NOPASSWD for the dinit reboot wrapper only (wheel = admin already;
		// reboot is no privilege escalation — same posture as the polkit
		// login1 rule). sudo-rs reads /etc/sudoers.d. FILENAME MUST SORT
		// AFTER "wheel": sudo-rs applies the LAST matching rule, and
		// /etc/sudoers.d/wheel (`%wheel ALL=(ALL) ALL`, password) would
		// otherwise override the NOPASSWD tag (live-caught via debug_run
		// 2026-10-11: `sudo -l` showed the rule but execution asked auth).
		if err := writeFile(chroot, "/etc/sudoers.d/zz-power", "%wheel ALL=(root) NOPASSWD: /usr/bin/reboot\n"); err != nil {
			return fmt.Errorf("write sudoers power rule: %w", err)
		}
		if _, err := shell.ExecuteCmd(chroot, "chmod 440 /etc/sudoers.d/zz-power"); err != nil {
			return fmt.Errorf("chmod sudoers power rule: %w", err)
		}
	}
	if _, err := shell.ExecuteCmd(chroot, fmt.Sprintf("chown %s:%s '%s/settings.json'", u.UserName, u.UserName, dmsSettingsDir)); err != nil {
		return fmt.Errorf("chown DMS settings.json: %w", err)
	}
	// DMS plugin seed: enable the window-buttons plugin. Widget plugins are
	// DISABLED by default (PluginService: getPluginSetting(id, "enabled",
	// false)); the file is a bare map { "<pluginId>": { ... } } (verified
	// against 1.6.2 SettingsData.parsePluginSettings).
	if _, err := shell.ExecuteCmd(chroot, fmt.Sprintf("printf '{\\n  \"dmsWindowButtons\": {\\n    \"enabled\": true\\n  }\\n}\\n' > '%s/plugin_settings.json' && chown %s:%s '%s/plugin_settings.json'", dmsSettingsDir, u.UserName, u.UserName, dmsSettingsDir)); err != nil {
		return fmt.Errorf("seed DMS plugin_settings.json (window buttons): %w", err)
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
	// GTK settings (3.0 + 4.0): without gtk-icon-theme-name apps fall back to
	// hicolor generics (the "missing icons" effect); prefer-dark keeps GTK
	// apps coherent with DMS dark mode. Adwaita = GNOME default. Icon themes
	// = dms-shell deps (aports 14f124b). GTK4 file added because libadwaita/
	// GTK4 apps read gtk-4.0 — but they primarily follow the PORTAL color
	// scheme (see the dconf system default below).
	gtkSettings := `[Settings]
gtk-icon-theme-name=Adwaita
gtk-application-prefer-dark-theme=true`
	if err := writeFile(chroot, fmt.Sprintf("/home/%s/.config/gtk-3.0/settings.ini", u.UserName), gtkSettings); err != nil {
		return fmt.Errorf("seed GTK3 settings (icon theme + dark): %w", err)
	}
	if err := writeFile(chroot, fmt.Sprintf("/home/%s/.config/gtk-4.0/settings.ini", u.UserName), gtkSettings); err != nil {
		return fmt.Errorf("seed GTK4 settings (icon theme + dark): %w", err)
	}
	// KDE globals: KDE apps (okular, ksystemlog, kcron...) read colors from
	// KColorScheme → ~/.config/kdeglobals — NOT from the portal, so they
	// stayed light. No plasma/breeze package needed: the values are
	// self-contained here (upstream BreezeDark.colors, KDE Breeze,
	// LGPL-2.0-or-later; translations dropped, color groups verbatim).
	// [Icons] pins breeze-dark (breeze-icons is installed).
	// [UiSettings] ColorScheme=BreezeDark: KColorSchemeManager apps (okular)
	// BYPASS [Colors:*] — the manager resolves the scheme by id from the
	// EMBEDDED BreezeDark/BreezeLight Qt resources (kcolorscheme ships them)
	// and, without a pin, follows styleHints colorScheme which is not dark
	// in their process → embedded LIGHT applied. The pin forces embedded
	// BreezeDark. Consequence: pinned apps stop following portal light/dark
	// toggles (fixed-dark box; remove the pin if auto-switching is wanted).
	// The [Colors:*] seed still covers non-manager consumers (ksystemlog).
	kdeGlobals := `[ColorEffects:Disabled]
Color=56,56,56
ColorAmount=0
ColorEffect=0
ContrastAmount=0.65
ContrastEffect=1
IntensityAmount=0.1
IntensityEffect=2

[ColorEffects:Inactive]
ChangeSelectionColor=true
Color=112,111,110
ColorAmount=0.025
ColorEffect=2
ContrastAmount=0.1
ContrastEffect=2
Enable=false
IntensityAmount=0
IntensityEffect=0

[Colors:Button]
BackgroundAlternate=30,87,116
BackgroundNormal=41,44,48
DecorationFocus=61,174,233
DecorationHover=61,174,233
ForegroundActive=61,174,233
ForegroundInactive=161,169,177
ForegroundLink=29,153,243
ForegroundNegative=218,68,83
ForegroundNeutral=246,116,0
ForegroundNormal=252,252,252
ForegroundPositive=39,174,96
ForegroundVisited=155,89,182

[Colors:Complementary]
BackgroundAlternate=30,87,116
BackgroundNormal=32,35,38
DecorationFocus=61,174,233
DecorationHover=61,174,233
ForegroundActive=61,174,233
ForegroundInactive=161,169,177
ForegroundLink=29,153,243
ForegroundNegative=218,68,83
ForegroundNeutral=246,116,0
ForegroundNormal=252,252,252
ForegroundPositive=39,174,96
ForegroundVisited=155,89,182

[Colors:Header]
BackgroundAlternate=32,35,38
BackgroundNormal=41,44,48
DecorationFocus=61,174,233
DecorationHover=61,174,233
ForegroundActive=61,174,233
ForegroundInactive=161,169,177
ForegroundLink=29,153,243
ForegroundNegative=218,68,83
ForegroundNeutral=246,116,0
ForegroundNormal=252,252,252
ForegroundPositive=39,174,96
ForegroundVisited=155,89,182

[Colors:Header][Inactive]
BackgroundAlternate=41,44,48
BackgroundNormal=32,35,38
DecorationFocus=61,174,233
DecorationHover=61,174,233
ForegroundActive=61,174,233
ForegroundInactive=161,169,177
ForegroundLink=29,153,243
ForegroundNegative=218,68,83
ForegroundNeutral=246,116,0
ForegroundNormal=252,252,252
ForegroundPositive=39,174,96
ForegroundVisited=155,89,182

[Colors:Selection]
BackgroundAlternate=30,87,116
BackgroundNormal=61,174,233
DecorationFocus=61,174,233
DecorationHover=61,174,233
ForegroundActive=252,252,252
ForegroundInactive=161,169,177
ForegroundLink=253,188,75
ForegroundNegative=176,55,69
ForegroundNeutral=198,92,0
ForegroundNormal=252,252,252
ForegroundPositive=23,104,57
ForegroundVisited=155,89,182

[Colors:Tooltip]
BackgroundAlternate=32,35,38
BackgroundNormal=41,44,48
DecorationFocus=61,174,233
DecorationHover=61,174,233
ForegroundActive=61,174,233
ForegroundInactive=161,169,177
ForegroundLink=29,153,243
ForegroundNegative=218,68,83
ForegroundNeutral=246,116,0
ForegroundNormal=252,252,252
ForegroundPositive=39,174,96
ForegroundVisited=155,89,182

[Colors:View]
BackgroundAlternate=29,31,34
BackgroundNormal=20,22,24
DecorationFocus=61,174,233
DecorationHover=61,174,233
ForegroundActive=61,174,233
ForegroundInactive=161,169,177
ForegroundLink=29,153,243
ForegroundNegative=218,68,83
ForegroundNeutral=246,116,0
ForegroundNormal=252,252,252
ForegroundPositive=39,174,96
ForegroundVisited=155,89,182

[Colors:Window]
BackgroundAlternate=41,44,48
BackgroundNormal=32,35,38
DecorationFocus=61,174,233
DecorationHover=61,174,233
ForegroundActive=61,174,233
ForegroundInactive=161,169,177
ForegroundLink=29,153,243
ForegroundNegative=218,68,83
ForegroundNeutral=246,116,0
ForegroundNormal=252,252,252
ForegroundPositive=39,174,96
ForegroundVisited=155,89,182

[Icons]
Theme=breeze-dark

[UiSettings]
ColorScheme=BreezeDark
`
	if err := writeFile(chroot, fmt.Sprintf("/home/%s/.config/kdeglobals", u.UserName), kdeGlobals); err != nil {
		return fmt.Errorf("seed kdeglobals (BreezeDark): %w", err)
	}
	// Flatpak user-global override: the sandbox does NOT see host config —
	// KDE flatpaks stayed light. Bind the host config read-only (KDE flatpaks
	// read the bound kdeglobals: BreezeDark [Colors:*] + the [UiSettings]
	// BreezeDark pin; GTK3 flatpaks read the bound gtk settings.ini). GTK4/
	// libadwaita + portal-aware flatpaks already follow the portal
	// color-scheme (host portal reports prefer-dark via the dconf system
	// default below). BINDS ONLY — no [Environment] forcing (user request:
	// apps must FOLLOW the theme chosen in DMS, not be force-themed): the
	// read-only binds let flatpak KDE apps read the host kdeglobals
	// (BreezeDark + pin) and GTK apps the host gtk settings. Format verified
	// against flatpak sources: file =
	// <user base dir>/overrides/global (flatpak-dir.c: override file for
	// app_id == NULL), [Context] filesystems parsed as a GLib keyfile string
	// list (`;`-separated, trailing `;`) (flatpak-context.c). Harmless
	// without flatpak (plain files).
	flatpakOverride := `[Context]
filesystems=xdg-config/kdeglobals:ro;xdg-config/gtk-3.0:ro;xdg-config/gtk-4.0:ro;
`
	if err := writeFile(chroot, fmt.Sprintf("/home/%s/.local/share/flatpak/overrides/global", u.UserName), flatpakOverride); err != nil {
		return fmt.Errorf("seed flatpak user-global override: %w", err)
	}
	// writeFile ran as root: also the intermediate flatpak dirs are root-owned.
	// flatpak (running as the user) must be able to write in ~/.local/share/flatpak.
	if _, err := shell.ExecuteCmd(chroot, fmt.Sprintf("chown -R %s:%s /home/%s/.local/share/flatpak", u.UserName, u.UserName, u.UserName)); err != nil {
		return fmt.Errorf("chown flatpak overrides: %w", err)
	}
	// System-wide dark preference for the xdg-desktop-portal color-scheme.
	// Verified live: apps following the portal (libadwaita/GTK4 such as
	// gnome-maps/calendar, Firefox, Qt via xdgdesktopportal) stayed LIGHT
	// because dconf was absent and the color-scheme key was never set — the
	// portal reported "no preference" (`v u 0`). The system dconf db is
	// compiled with `dconf update` (no session bus needed in the chroot) and
	// is inherited by every user (a user-set value still wins).
	// Alpine's dconf package ships NO /etc/dconf/profile/user (Debian ships
	// it) — without the profile GLib never consults the system db and
	// `gsettings get` returns the schema default (`'default'`), verified
	// live. The standard GNOME profile is written explicitly.
	dconfProfile := "user-db:user\nsystem-db:local\n"
	if err := writeFile(chroot, "/etc/dconf/profile/user", dconfProfile); err != nil {
		return fmt.Errorf("seed dconf profile: %w", err)
	}
	dconfDefault := `[org/gnome/desktop/interface]
color-scheme='prefer-dark'`
	if err := writeFile(chroot, "/etc/dconf/db/local.d/00-feralos-theme", dconfDefault); err != nil {
		return fmt.Errorf("seed dconf color-scheme default: %w", err)
	}
	if _, err := shell.ExecuteCmd(chroot, "dconf update"); err != nil {
		return fmt.Errorf("dconf update: %w", err)
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
	if _, err := shell.ExecuteCmd(chroot, fmt.Sprintf("chown %s:%s '/home/%s/.config/xdg-terminals.list' '/home/%s/.config/mimeapps.list' '/home/%s/.config/gtk-3.0/settings.ini' '/home/%s/.config/gtk-4.0/settings.ini' '/home/%s/.config/kdeglobals'", u.UserName, u.UserName, u.UserName, u.UserName, u.UserName, u.UserName, u.UserName)); err != nil {
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

	// 14. Final ownership sweep — user-reported: seeded configs were
	// root-owned in places. The per-seed chowns cover the FILES, but the
	// DIRECTORIES created by the seeds' mkdir run as root (the general
	// chown -R at step 13 runs BEFORE the seeds), leaving e.g.
	// ~/.config/DankMaterialShell, ~/.config/gtk-4.0 and
	// ~/.local/state/DankMaterialShell root-owned → DMS (user) could not
	// create/save files there. Sweep AFTER every seed.
	if _, err := shell.ExecuteCmd(chroot, fmt.Sprintf("chown -R %s:%s /home/%s/.config /home/%s/.local", u.UserName, u.UserName, u.UserName, u.UserName)); err != nil {
		return fmt.Errorf("final chown user config: %w", err)
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
