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
// DMS = the feralos-aports native package (musl, no nix). Spawned by the
// niri session (needs WAYLAND_DISPLAY + seat) — an OpenRC service hung
// boot and duplicated the shell, so none exists.
func SetupNiri(conf *config.Config) error {
	chroot := conf.Internal.ChrootDir
	u := conf.User
	// DE rules block (internal_rules desktopEnvironments.niri): greeter
	// groups + user dinit boot.d infrastructure units live there.
	deRules := conf.Internal.DesktopEnvironments[u.DesktopEnvironment]

	fmt.Println("[INFO] Configuring niri desktop environment...")

	// greetd config.toml + greeter.sh are package-shipped (ADR-013):
	// greetd-config (common) + dms-config/wayle-config (DE-specific script).

	// regreet.toml (greetd-config): ONE shared static file, no timezone
	// key — regreet uses the system tz. Branding + greeter provisioning:
	// niri-base-config / greetd-config post-install.

	// greeter niri config stays installer-written (only variable: the
	// layout; per-home config, not /etc/xdg — overlay off at login).
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
	if err := writeFile(chroot, "/var/lib/greetd/.config/niri/config.kdl", greeterNiri); err != nil {
		return fmt.Errorf("write greeter niri config: %w", err)
	}

	// login-shell trick (per-user, idempotent): passwd shell = the
	// login-shell proxy so niri-session skips its bash re-exec; re-point
	// the shipped ash symlink to fish ("applet not found" otherwise) +
	// list in /etc/shells (pkexec RC=127 without it).
	if _, err := shell.ExecuteCmd(chroot, fmt.Sprintf("usermod -s /usr/libexec/login-shell %s", u.UserName)); err != nil {
		fmt.Printf("[WARNING] Failed to set %s shell to login-shell: %v\n", u.UserName, err)
	}
	if _, err := shell.ExecuteCmd(chroot, "ln -sf /usr/bin/fish /usr/libexec/login-shell"); err != nil {
		fmt.Printf("[WARNING] Failed to re-point login-shell to fish: %v\n", err)
	}
	if _, err := shell.ExecuteCmd(chroot, "grep -qxF '/usr/libexec/login-shell' /etc/shells || echo '/usr/libexec/login-shell' >> /etc/shells"); err != nil {
		fmt.Printf("[WARNING] Failed to list login-shell in /etc/shells: %v\n", err)
	}

	// session: upstream /usr/bin/niri-session unmodified (login-shell proxy
	// skips its bash re-exec; turnstile starts the user dinit, `dinitctl
	// --user start niri` starts the compositor). Session env via
	// /etc/environment (pam_env).
	environmentFile := fmt.Sprintf(`XDG_SESSION_DESKTOP=niri
XDG_DATA_DIRS=/home/%s/.local/share/flatpak/exports/share:/var/lib/flatpak/exports/share:/usr/local/share:/usr/share:/home/%s/.nix-profile/share
`, u.UserName, u.UserName)
	if err := writeFile(chroot, "/etc/environment", environmentFile); err != nil {
		return fmt.Errorf("write /etc/environment: %w", err)
	}

	// per-user portals conf stays installer-written: the Secret pin
	// differs per variant (gnome-keyring here, oo7-portal in wayle-config)
	// and the smoke CI would collide same paths (one accumulated root).
	// default=gtk; wlr owns Screenshot/ScreenCast; the niri-portalsconf
	// 999 dummy shadows the vendor gnome backend.
	userPortals := `[preferred]
default=gtk;
org.freedesktop.impl.portal.ScreenCast=wlr;
org.freedesktop.impl.portal.Screenshot=wlr;
org.freedesktop.impl.portal.Secret=gnome-keyring;
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
	// DMS static base (prefer-no-csd, environment minus the per-install
	// XDG_DATA_DIRS, xwayland-satellite, the xdg-user-dirs spawn) ships in
	// dms-config (/usr/share/dms/niri/base.kdl) and merges PER-PROPERTY
	// (niri >=25.11 include). The fragment below = the per-install parts
	// ONLY (input keymap + the user-path XDG_DATA_DIRS) + the per-version
	// DMS fragments (`dms setup` generates them per user — relative
	// includes, NOT static, NOT package content).
	// niri config = 3 files, 3 owners (owner decision 2026-11-26):
	//   base.kdl    (dms-config)        static bulk — apk owns it
	//   install.kdl (installer, ONCE)   per-install values — never re-asserted
	//   config.kdl  (installer once,    the USER file: includes only — edits
	//    then USER)                       merge per-key, survive everything
	// The dms/*.kdl fragments stay `dms setup`-generated (per-version).
	userNiri := `// FeralOS niri (DMS) — YOUR niri config. The includes below load:
//   /usr/share/dms/niri/base.kdl — package static base (apk owns it)
//   install.kdl                  — per-install values (installer, once)
//   dms/*.kdl                    — DMS fragments (generated by ` + "`dms setup`" + `)
// Your edits HERE merge per-key over all of them.

include "/usr/share/dms/niri/base.kdl"
include "install.kdl"
include "dms/binds.kdl"
include "dms/layout.kdl"
include "dms/colors.kdl"
`
	installNiri := fmt.Sprintf(`// per-install values — written ONCE by the installer, safe to edit
// (nothing re-asserts this file).
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
	if err := writeFile(chroot, fmt.Sprintf("/home/%s/.config/niri/install.kdl", u.UserName), installNiri); err != nil {
		return fmt.Errorf("write user niri install config: %w", err)
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
	// DMS reboot fix: plain `loginctl reboot` under dinit halts (elogind
	// execs busybox /sbin/reboot, dinit overwrites REBOOT→HALT). Wire
	// customPowerActionReboot = `sudo -n /usr/bin/reboot` (zz-power), dinit
	// only; OpenRC keeps loginctl.
	settingsJSON := "{\n  \"configVersion\": 1,\n  \"currentThemeCategory\": \"generic\",\n  \"currentThemeName\": \"green\",\n  \"dankBarLeftWidgets\": [\"launcherButton\", \"workspaceSwitcher\", \"runningApps\"],\n  \"dankBarRightWidgets\": [\"systemTray\", \"clipboard\", \"cpuUsage\", \"memUsage\", \"notificationButton\", \"battery\", \"controlCenterButton\", \"dmsWindowButtons\"],\n  \"updaterUseCustomCommand\": true,\n  \"updaterCustomCommand\": \"/usr/local/bin/feralos-update\""
	if u.EnableDinit {
		settingsJSON += ",\n  \"customPowerActionReboot\": \"sudo -n /usr/bin/reboot\""
	}
	settingsJSON += "\n}\n"
	if err := writeFile(chroot, fmt.Sprintf("%s/settings.json", dmsSettingsDir), settingsJSON); err != nil {
		return fmt.Errorf("seed DMS settings.json: %w", err)
	}
	// /etc/sudoers.d/zz-power ships in the feralos-reboot package (the DMS
	// custom reboot command = `sudo -n /usr/bin/reboot` under dinit).
	if _, err := shell.ExecuteCmd(chroot, fmt.Sprintf("chown %s:%s '%s/settings.json'", u.UserName, u.UserName, dmsSettingsDir)); err != nil {
		return fmt.Errorf("chown DMS settings.json: %w", err)
	}
	// DMS plugin seed: widget plugins are disabled by default — enable
	// dmsWindowButtons (bare { "<pluginId>": ... } map).
	pluginSettings := "{\n  \"dmsWindowButtons\": {\n    \"enabled\": true\n  }\n}\n"
	if err := shell.WriteFile(chroot, dmsSettingsDir+"/plugin_settings.json", []byte(pluginSettings), 0644); err != nil {
		return fmt.Errorf("seed DMS plugin_settings.json (window buttons): %w", err)
	}
	if _, err := shell.ExecuteCmd(chroot, fmt.Sprintf("chown %s:%s '%s/plugin_settings.json'", u.UserName, u.UserName, dmsSettingsDir)); err != nil {
		return fmt.Errorf("chown DMS plugin_settings.json: %w", err)
	}
	// dark mode seed: belt+braces (installs default dark; the seed
	// guarantees it portal-independently). State store = XDG state dir.
	dmsStateDir := fmt.Sprintf("/home/%s/.local/state/DankMaterialShell", u.UserName)
	sessionJSON := "{\n  \"isLightMode\": false\n}\n"
	if err := shell.WriteFile(chroot, dmsStateDir+"/session.json", []byte(sessionJSON), 0644); err != nil {
		return fmt.Errorf("seed DMS session.json (dark mode): %w", err)
	}
	if _, err := shell.ExecuteCmd(chroot, fmt.Sprintf("chown %s:%s '%s/session.json'", u.UserName, u.UserName, dmsStateDir)); err != nil {
		return fmt.Errorf("chown DMS session.json: %w", err)
	}
	// default apps: GLOBAL since dms-config 0.3.0 (/etc/xdg/mimeapps.list +
	// /etc/xdg/xdg-terminals.list — every user inherits; the DMS Default
	// Apps tab reads/writes the USER files on first change). Nothing to
	// seed. Desktop IDs verified against Alpine.
	// no theme seeds: dms-theme-sync owns theming (it propagates to GTK/Qt/
	// KDE/Flatpak — manual seeds conflict). dconf: fully package-
	// materialized (profile + compiled db in niri-base-config post-install
	// — the edge/flatpak install runs BEFORE the desktop setup and the
	// dconf writes need the compiled db). mimeapps/terminals: /etc/xdg
	// globals in dms-config. Nothing to seed here.
	// strip `background-effect { xray false }`: a niri node newer than the
	// Alpine 3.24 niri — it breaks the whole include (blur unsupported here
	// anyway).
	layoutKdl := fmt.Sprintf("/home/%s/.config/niri/dms/layout.kdl", u.UserName)
	awk := `awk '/background-effect[[:space:]]*\{/{skip=1; next} skip{ if (/\}/) skip=0; next } {print}'`
	if _, err := shell.ExecuteCmd(chroot, fmt.Sprintf("%s '%s' > '%s.tmp' && mv '%s.tmp' '%s'", awk, layoutKdl, layoutKdl, layoutKdl, layoutKdl)); err != nil {
		fmt.Printf("[WARNING] failed to strip background-effect from layout.kdl: %v\n", err)
	}

	// 7. NetworkManager is the network service (the DMS widget talks to NM,
	// rounds 30/44 — DMS network/VPN UI needs the NM backend): both statics
	// ship in niri-base-config — /etc/NetworkManager/conf.d/10-manage.conf
	// (managed=true) and /etc/network/interfaces reduced to loopback (it
	// OVERWRITES the eth0-dhcp file internal.setupInterfaces wrote earlier;
	// that write is dead in the niri-family flow).

	// 8. CUPS printer admin: SystemGroup stays at the Alpine default
	// (lpadmin, in cups-files.conf) — the session user is added to the
	// lpadmin group instead of widening CUPS admin to all of wheel
	// (round 24: without membership, CUPS answers 403 on CUPS-Get-Devices).

	// 10. groups: the session user's groups come from the rules (DE `groups`
	// — incl. audio for /dev/snd, PipeWire); the greeter joins the DE
	// `greeterGroups` (greetd re-owns /var/lib/greetd 0750 at boot —
	// traversal needed). seat access via elogind D-Bus; render group absent
	// on Alpine.
	if _, err := shell.ExecuteCmd(chroot, fmt.Sprintf("usermod -aG %s greeter", strings.Join(deRules.GreeterGroups, ","))); err != nil {
		fmt.Printf("[WARNING] Failed to add greeter to greeterGroups: %v\n", err)
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
	// verbatim `.user` unit names). Session apps (polkit-agent/dms/wayvnc)
	// are NOT enabled here — upstream "add-wants niri.service" model:
	// niri-session starts them after niri (aports niri-user-dinit), so the
	// greetd greeter session (greeter.sh, which never runs niri-session)
	// never gets desktop apps. Existence-filtered — only installed packages
	// get enabled.
	userServices := strings.Join(deRules.UserServices, " ")
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

// writeFile writes a file inside the chroot (logged direct write — logging
// rule amendment P2-5: the escaped `echo '<content>' > path` pattern is gone;
// the log carries path + size, never the content). The trailing newline
// reproduces the old `echo` byte-for-byte. Files needing another mode are
// chmod'ed by the caller right after (unchanged behavior).
func writeFile(chroot, path, content string) error {
	return shell.WriteFile(chroot, path, []byte(content+"\n"), 0644)
}
