package desktops

import (
	"feralos/src/utils/config"
	"feralos/src/utils/shell"
	"fmt"
	"strings"
)

// SetupNiriWayle: niri compositor + Wayle shell + Veila lock + Raffi
// launcher + oo7 secrets. Shares greeter/turnstile/pipewire/portals with
// the niri DE; no DMS. User niri config = upstream default binds with
// alacritty/raffi/veila. Secrets via oo7 (PAM login-unlock + passwd
// re-sync; daemon = supervised user dinit). Wayle session actions seeded
// in ~/.config/wayle/config.toml (upstream defaults are systemd-only).
func SetupNiriWayle(conf *config.Config) error {
	chroot := conf.Internal.ChrootDir
	u := conf.User
	// greeter groups + user dinit boot.d units live in the rules.
	deRules := conf.Internal.DesktopEnvironments[u.DesktopEnvironment]

	fmt.Println("[INFO] Configuring niriWayle desktop environment...")

	// greetd config.toml + greeter.sh are package-shipped (ADR-013):
	// greetd-config (common) + dms-config/wayle-config (DE-specific script).

	// regreet.toml (greetd-config): static, no timezone key — regreet uses
	// the system tz. Branding: niri-base-config.

	// hide the bare upstream `niri` session from regreet (NoDisplay; the
	// local edit survives niri upgrades as .apk-new).
	if _, err := shell.ExecuteCmd(chroot, `grep -q '^NoDisplay=true' /usr/share/wayland-sessions/niri.desktop 2>/dev/null || printf 'NoDisplay=true\n' >> /usr/share/wayland-sessions/niri.desktop`); err != nil {
		return fmt.Errorf("hide bare niri session from regreet: %w", err)
	}

	// greeter provisioning = greetd-config post-install. The greeter niri
	// config stays installer-written (only variable: the layout).
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
	// it to fish + list in /etc/shells (pkexec requirement).
	if _, err := shell.ExecuteCmd(chroot, fmt.Sprintf("usermod -s /usr/libexec/login-shell %s", u.UserName)); err != nil {
		fmt.Printf("[WARNING] Failed to set %s shell to login-shell: %v\n", u.UserName, err)
	}
	if _, err := shell.ExecuteCmd(chroot, "ln -sf /usr/bin/fish /usr/libexec/login-shell"); err != nil {
		fmt.Printf("[WARNING] Failed to re-point login-shell to fish: %v\n", err)
	}
	if _, err := shell.ExecuteCmd(chroot, "grep -qxF '/usr/libexec/login-shell' /etc/shells || echo '/usr/libexec/login-shell' >> /etc/shells"); err != nil {
		fmt.Printf("[WARNING] Failed to list login-shell in /etc/shells: %v\n", err)
	}

	// session env via /etc/environment (pam_env). XDG_*_DESKTOP stays
	// "niri": portal resolution + the xdg-su wrapper match on it.
	environmentFile := fmt.Sprintf(`XDG_SESSION_DESKTOP=niri
XDG_DATA_DIRS=/home/%s/.local/share/flatpak/exports/share:/var/lib/flatpak/exports/share:/usr/local/share:/usr/share:/home/%s/.nix-profile/share
`, u.UserName, u.UserName)
	if err := writeFile(chroot, "/etc/environment", environmentFile); err != nil {
		return fmt.Errorf("write /etc/environment: %w", err)
	}

	// portals: the ADMIN-path override ships in wayle-config
	// (/etc/xdg-desktop-portal/niri-portals.conf: default=gtk, wlr
	// Screenshot/ScreenCast, Secret=oo7-portal — no gnome-keyring here, so
	// the org.freedesktop.secrets D-Bus name must be owned by oo7's
	// portal). Precedence: user > /etc > /usr/share. The vendor
	// /usr/share conf (Alpine's niri-portalsconf subpackage:
	// gnome;gtk + gnome-keyring) is shadowed for the interfaces we pin;
	// the xdg-desktop-portal-gnome 999 dummy keeps the real gnome backend
	// out.

	// niri config = 3 files, 3 owners (owner decision 2026-11-26):
	//   base.kdl   (wayle-config)     static bulk — apk owns it, upgrades
	//                                  replace it, nobody edits it
	//   install.kdl (installer, ONCE) per-install values (layout +
	//                                  XDG_DATA_DIRS) — never re-asserted
	//   config.kdl (installer once,   the USER file: include lines only —
	//    then USER)                   edits here merge per-key and survive
	//                                  apk + installer forever
	if _, err := shell.ExecuteCmd(chroot, fmt.Sprintf("mkdir -p /home/%s/.local/share/dinit", u.UserName)); err != nil {
		return fmt.Errorf("mkdir user dinit log dir: %w", err)
	}
	// Ownership BEFORE the seeds below: the mkdir/writes above ran as root.
	if _, err := shell.ExecuteCmd(chroot, fmt.Sprintf("chown -R %s:%s /home/%s/.config /home/%s/.local", u.UserName, u.UserName, u.UserName, u.UserName)); err != nil {
		return fmt.Errorf("chown user config: %w", err)
	}

	// wayle config.toml: the SKEL copy (wayle-config 0.1.4-r1+) lands at
	// adduser — identical content, nothing to write here.

	// NO DMS block in this DE (niri.go seeds settings.json/plugin_settings
	// here): no dms-shell, no customPowerActionReboot, no session.json, no
	// xdg-terminals.list/mimeapps.list seeds. Apps and defaults are
	// per-install concerns for niriWayle v1.

	// The sudoers NOPASSWD rule (/etc/sudoers.d/zz-power) ships in the
	// feralos-reboot package (the wrapper's dinit sudo path needs it).

	// gsettings INFRASTRUCTURE — FULLY package-materialized now:
	// /etc/dconf/profile/user ships in niri-base-config AND its post-install
	// compiles the (empty) system db at PACKAGE-install time — the edge/
	// flatpak install runs BEFORE the desktop setup and the flatpak
	// dconf writes need the compiled db (proof 2026-11-23T23-15-08Z:
	// Key Rack hit "unable to open file /etc/dconf/db/local" when the
	// materialization lived here). Nothing left in the installer.

	// 8. NetworkManager is the network service for the niriWayle desktop
	// (Wayle network module talks to NM): both statics ship in
	// niri-base-config — /etc/NetworkManager/conf.d/10-manage.conf
	// (managed=true) and /etc/network/interfaces reduced to loopback (it
	// OVERWRITES the eth0-dhcp file internal.setupInterfaces wrote earlier;
	// that write is dead in the niri-family flow).

	// 10. groups: the session user's groups come from the rules (DE `groups`
	// — incl. audio for /dev/snd, PipeWire); the greeter joins the DE
	// `greeterGroups` (greetd re-owns /var/lib/greetd 0750 at boot —
	// traversal needed). seat access via elogind D-Bus; render group absent
	// on Alpine.
	if _, err := shell.ExecuteCmd(chroot, fmt.Sprintf("usermod -aG %s greeter", strings.Join(deRules.GreeterGroups, ","))); err != nil {
		fmt.Printf("[WARNING] Failed to add greeter to greeterGroups: %v\n", err)
	}

	// oo7 PAM wiring (the gnome-keyring analogue): greetd `auth` stashes
	// the password, `session auto_start` (appended LAST so /run/user
	// exists) hands it to the daemon's socket listener and unlocks the
	// keyrings; a missing socket → the module execs /usr/bin/oo7-daemon =
	// OUR recovery wrapper (dinitctl --user only — never unsupervised).
	// /etc/pam.d/passwd override: a user `passwd` re-encrypts all keyrings
	// (root chpasswd resets cannot sync — universal limit).
	// first use: the daemon auto-creates an empty default keyring — any
	// password binds it, the first secret persists it encrypted.
	pamGreetd := "/etc/pam.d/greetd"
	if _, err := shell.ExecuteCmd(chroot, "grep -q 'auth.*pam_oo7.so' "+pamGreetd+" || echo 'auth       optional pam_oo7.so' >> "+pamGreetd); err != nil {
		fmt.Printf("[WARNING] Failed to add pam_oo7 auth to greetd PAM: %v\n", err)
	}
	if _, err := shell.ExecuteCmd(chroot, "grep -q 'session.*pam_oo7.so' "+pamGreetd+" || echo 'session    optional pam_oo7.so auto_start' >> "+pamGreetd); err != nil {
		fmt.Printf("[WARNING] Failed to add pam_oo7 session to greetd PAM: %v\n", err)
	}
	// /etc/pam.d/passwd ships in wayle-config (Alpine ships none).
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
