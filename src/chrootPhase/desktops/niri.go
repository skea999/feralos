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
	// DE rules block (internal_rules desktopEnvironments.niri): greeter
	// groups + user dinit boot.d infrastructure units live there.
	deRules := conf.Internal.DesktopEnvironments[u.DesktopEnvironment]

	fmt.Println("[INFO] Configuring niri desktop environment...")

	// greetd config.toml + greeter.sh are package-shipped (ADR-013):
	// greetd-config (common) + dms-config/wayle-config (DE-specific script).

	// 3. ReGreet config ships in greetd-config 0.2.0 — 100% static, ONE
	// SHARED file for both DEs (owner rule 2026-11-25: aesthetic merges,
	// functional separates). The [widget.clock] timezone key is OMITTED
	// (regreet sample: "Remove to use the system time zone") and the
	// installer sets the system tz from variables. Nothing to write here.

	// 3a. Branding (logo + wallpapers) ships in niri-base-config — nothing
	// to install here.

	// 4. greeter user + dirs: greetd-config 0.2.0 post-install provisions
	// ALL of it (greeter user with the -S/-G rationale below, /var/lib
	// dirs, fontconfig USER-cache pre-build, ownership).
	// The greeter niri config stays installer-written: its ONLY variable
	// is the keyboard layout. The greeter's niri reads its config from its
	// HOME (per-home, not /etc/xdg — gap 5.7): overlay off at the login
	// screen (round 14).
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

	// 4b. Login-shell trick (moved from users.go — desktop-owned now, runs
	// AFTER user creation: chrootPhase step 5 users < step 9 desktops).
	// passwd shell = /usr/libexec/login-shell (niri-user-dinit) so
	// niri-session skips its bash re-exec and reaches the dinit branch.
	// The shipped symlink targets /bin/ash, but busybox invoked as
	// `login-shell` looks up an applet with that name and fails
	// ("login-shell: applet not found") — re-point it at fish. The path is
	// ALSO listed in /etc/shells so pkexec accepts the session shell
	// (RC=127 otherwise — every privileged DMS/polkit op died, run
	// 2026-10-11 debug_run). Listing it is safe: the FeralOS niri-session
	// (niri-user-dinit, replaces="niri") has NO /etc/shells bash re-exec
	// trap — only upstream Alpine's script did. Backward risk: reverting to
	// upstream niri-session would re-arm that trap. All three steps
	// idempotent.
	if _, err := shell.ExecuteCmd(chroot, fmt.Sprintf("usermod -s /usr/libexec/login-shell %s", u.UserName)); err != nil {
		fmt.Printf("[WARNING] Failed to set %s shell to login-shell: %v\n", u.UserName, err)
	}
	if _, err := shell.ExecuteCmd(chroot, "ln -sf /usr/bin/fish /usr/libexec/login-shell"); err != nil {
		fmt.Printf("[WARNING] Failed to re-point login-shell to fish: %v\n", err)
	}
	if _, err := shell.ExecuteCmd(chroot, "grep -qxF '/usr/libexec/login-shell' /etc/shells || echo '/usr/libexec/login-shell' >> /etc/shells"); err != nil {
		fmt.Printf("[WARNING] Failed to list login-shell in /etc/shells: %v\n", err)
	}

	// 5. Session startup — no wrapper: Alpine's /usr/bin/niri-session is used
	// UNMODIFIED. greetd sets SHELL from passwd; the user is created with
	// /usr/libexec/login-shell — the path IS listed in /etc/shells
	// (pkexec requirement, step 4b) but it is NOT the fish binary itself
	// (a symlink to fish), so niri-session skips its bash re-exec and
	// reaches the dinit branch: turnstile (PAM, base-session) starts the
	// user dinit at login and `dinitctl --user start niri` starts the
	// `niri` service (niri-user-dinit); niri-shutdown clears the session
	// env on exit. XDG session env comes from /etc/environment (pam_env in
	// the turnstiled PAM stack) + the niri config environment block below.
	environmentFile := fmt.Sprintf(`XDG_SESSION_DESKTOP=niri
XDG_DATA_DIRS=/home/%s/.local/share/flatpak/exports/share:/var/lib/flatpak/exports/share:/usr/local/share:/usr/share:/home/%s/.nix-profile/share
`, u.UserName, u.UserName)
	if err := writeFile(chroot, "/etc/environment", environmentFile); err != nil {
		return fmt.Errorf("write /etc/environment: %w", err)
	}

	// 6. xdg-desktop-portal backend — the ADMIN-path override ships in
	// dms-config 0.3.0 (/etc/xdg-desktop-portal/niri-portals.conf:
	// default=gtk, wlr Screenshot/ScreenCast, Secret=gnome-keyring — the
	// DMS flavor; XDG precedence user > /etc > /usr/share, with no user
	// file the admin conf wins). The Alpine system niri-portals.conf
	// (default=gnome;gtk) is still shadowed by the niri-portalsconf 999
	// dummy in the package list.

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
	userNiri := fmt.Sprintf(`// FeralOS niri (DMS) — user config (installer-written, user-editable).
// The STATIC base ships in dms-config and merges per-property:
//     include "/usr/share/dms/niri/base.kdl"
// Only the PER-INSTALL parts live here.

include "/usr/share/dms/niri/base.kdl"

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

include "dms/binds.kdl"
include "dms/layout.kdl"
include "dms/colors.kdl"
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
	// /etc/sudoers.d/zz-power ships in the feralos-reboot package (the DMS
	// custom reboot command = `sudo -n /usr/bin/reboot` under dinit).
	if _, err := shell.ExecuteCmd(chroot, fmt.Sprintf("chown %s:%s '%s/settings.json'", u.UserName, u.UserName, dmsSettingsDir)); err != nil {
		return fmt.Errorf("chown DMS settings.json: %w", err)
	}
	// DMS plugin seed: enable the window-buttons plugin. Widget plugins are
	// DISABLED by default (PluginService: getPluginSetting(id, "enabled",
	// false)); the file is a bare map { "<pluginId>": { ... } } (verified
	// against 1.6.2 SettingsData.parsePluginSettings).
	pluginSettings := "{\n  \"dmsWindowButtons\": {\n    \"enabled\": true\n  }\n}\n"
	if err := shell.WriteFile(chroot, dmsSettingsDir+"/plugin_settings.json", []byte(pluginSettings), 0644); err != nil {
		return fmt.Errorf("seed DMS plugin_settings.json (window buttons): %w", err)
	}
	if _, err := shell.ExecuteCmd(chroot, fmt.Sprintf("chown %s:%s '%s/plugin_settings.json'", u.UserName, u.UserName, dmsSettingsDir)); err != nil {
		return fmt.Errorf("chown DMS plugin_settings.json: %w", err)
	}
	// DMS dark mode seed (belt+braces): fresh installs already default dark
	// (SharedSessionSpec.js isLightMode def=false; PortalService applies mode
	// changes only after the first read), but the explicit seed guarantees
	// dark portal-independently. Session store = XDG state dir, NOT config.
	dmsStateDir := fmt.Sprintf("/home/%s/.local/state/DankMaterialShell", u.UserName)
	sessionJSON := "{\n  \"isLightMode\": false\n}\n"
	if err := shell.WriteFile(chroot, dmsStateDir+"/session.json", []byte(sessionJSON), 0644); err != nil {
		return fmt.Errorf("seed DMS session.json (dark mode): %w", err)
	}
	if _, err := shell.ExecuteCmd(chroot, fmt.Sprintf("chown %s:%s '%s/session.json'", u.UserName, u.UserName, dmsStateDir)); err != nil {
		return fmt.Errorf("chown DMS session.json: %w", err)
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
	// GLOBAL since dms-config 0.3.0: they ship as /etc/xdg/mimeapps.list +
	// /etc/xdg/xdg-terminals.list (the GLib / xdg-terminal-exec standard
	// system layers — every user inherits them live; the DMS Default Apps
	// tab reads/writes the USER files on first change). Nothing to seed here.
	// NOTE: no manual theme seeds here (no kdeglobals/gtk settings.ini/dconf
	// color-scheme/flatpak overrides) — the DMS plugin `dms-theme-sync`
	// (third-party, GPL-3.0) is the theming authority: it propagates the
	// DMS appearance (mode/colors/icons) to GTK, Qt, KDE, Flatpak. Manual
	// seeds would conflict with it. The plugin + enablement are installed
	// separately (see the plugin round).
	// gsettings INFRASTRUCTURE — FULLY package-materialized now:
	// /etc/dconf/profile/user ships in niri-base-config AND its post-install
	// compiles the (empty) system db at PACKAGE-install time — the edge/
	// flatpak install runs BEFORE the desktop setup and the dconf writes
	// need the compiled db (harness run 2026-10-12T22-09-52Z caught the
	// "Error opening directory" failure; proof 2026-11-23T23-15-08Z
	// caught the flatpak variant of the same race). Nothing left here —
	// DMS's theming plugin writes user-level dconf at runtime against the
	// package-materialized db.
	// mimeapps.list + xdg-terminals.list ship in dms-config 0.3.0 as the
	// GLOBAL system-layer defaults (/etc/xdg/... — every user inherits
	// them live). Nothing to seed here.
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
	// when the PAM session opens (greetd PAM). The pam_gnome_keyring AUTH
	// line ships in the dms-config takeover of /etc/pam.d/greetd (the
	// `session ... auto_start` line already ships in Alpine's
	// base-session). Secrets themselves are D-Bus activated on demand
	// (org.freedesktop.secrets service ships with the package).

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
