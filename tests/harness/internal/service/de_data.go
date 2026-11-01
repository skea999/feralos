package service

import (
	"fmt"
	"strings"
)

// P2-4 data-driven per-DE verification data. The literals here are pinned
// byte-for-byte by golden_test.go (captured mechanically from the
// pre-refactor inline code) — edit the data, the goldens catch semantic
// drift.

// isNiriFamilyDE — niri AND niriWayle share the compositor (niri), greeter
// (greetd/regreet), session stack (turnstile + per-user dinit), pipewire
// stack, portals, polkit agent and wayvnc. DMS-specific checks are
// niri-only; Wayle/Veila/Raffi/oo7 checks are niriWayle-only.
func isNiriFamilyDE(de string) bool {
	return de == desktopNiri || de == desktopNiriWayle
}

// deSessionStack — the per-DE user-session service lists:
//
//	dinitUserSvcFiles    — user service FILES shipped by the packages
//	                       (dinit_user_services check)
//	sessionAppsAbsent    — session apps that must NOT sit in boot.d
//	                       (user_services_enabled check, second loop)
//	userDinitRuntimeSvcs — services that must hold a started marker
//	                       (niri_user_dinit_runtime check)
//	userSvcBootLinks     — infrastructure boot.d links
//	                       (user_services_enabled check, first loop)
type deSessionStack struct {
	dinitUserSvcFiles    string
	sessionAppsAbsent    string
	userDinitRuntimeSvcs string
	userSvcBootLinks     string
}

// joinUnits renders a space-separated unit list (the check shells loop over
// the words).
func joinUnits(units ...string) string {
	return strings.Join(units, " ")
}

// concatUnits concatenates unit segments (niriWayle = niri base + wayle
// additions).
func concatUnits(groups ...[]string) []string {
	n := 0
	for _, g := range groups {
		n += len(g)
	}
	out := make([]string, 0, n)
	for _, g := range groups {
		out = append(out, g...)
	}
	return out
}

// Shared unit segments (order significant — the goldens pin the joined
// strings): boot.d/user-stack infrastructure shared by both niri-family
// DEs, the GTK/portal tail and the KDE-services tail of dinitUserSvcFiles,
// and the runtime-stack infrastructure (no wait-wayland gate, compositor
// first).
var (
	sessionStackInfra = []string{
		"dbus.user", "pipewire.user", "pipewire-pulse.user", "wireplumber.user",
		"wait-wayland.user",
		"org.freedesktop.portal.Desktop.user", "org.freedesktop.portal.Documents.user",
		"org.freedesktop.impl.portal.PermissionStore.user",
	}
	sessionStackGTKTail = []string{
		"ca.desrt.dconf.user", "org.a11y.Bus.user",
		"org.kde.kwalletd6.user", "org.kde.kded6.user",
		"com.google.code.AccountsSSO.SingleSignOn.user",
		"org.freedesktop.LocalSearch3.user", "org.freedesktop.LocalSearch3.Control.user",
		"org.freedesktop.LocalSearch3.Writeback.user",
		"org.freedesktop.Flatpak.user", "org.freedesktop.portal.Flatpak.user",
		"org.flatpak.Authenticator.Oci.user",
		"org.freedesktop.impl.portal.desktop.gtk.user", "org.freedesktop.impl.portal.desktop.wlr.user",
	}
	sessionStackKDETail = []string{
		"org.kde.kiod6.user", "org.kde.kioexecd6.user", "org.kde.kpasswdserver6.user",
		"org.kde.kssld6.user", "org.gnome.Nautilus.user", "org.gtk.GLib.PACRunner.user",
		"com.canonical.indicators.webcredentials.user", "com.nokia.singlesignonui.user",
		"org.freedesktop.portal.Tracker.user", "org.kde.secretservicecompat.user",
		"org.freedesktop.impl.portal.desktop.kwallet.user",
	}
	sessionStackRuntimeInfra = []string{
		"niri", "dbus.user", "pipewire.user", "pipewire-pulse.user", "wireplumber.user",
		"org.freedesktop.portal.Desktop.user", "org.freedesktop.portal.Documents.user",
		"org.freedesktop.impl.portal.PermissionStore.user",
	}
	// wayle additions over the niri base: oo7 secrets (org.freedesktop
	// .secrets + the oo7 desktop portal), the Wayle session apps and the
	// gnome desktop portal.
	wayleSecretsUnits    = []string{"org.freedesktop.secrets.user", "org.freedesktop.impl.portal.desktop.oo7.user"}
	wayleSessionAppUnits = []string{"wayle.user", "veila.user", "stasis.user"}
	wayleGnomePortalUnit = []string{"org.freedesktop.impl.portal.desktop.gnome.user"}
	niriSessionAppUnits  = []string{"polkit-agent.user", "dms.user"}
	wayvncUnit           = []string{"wayvnc.user"}
)

var deSessionStacks = map[string]deSessionStack{
	desktopNiri: {
		dinitUserSvcFiles: joinUnits(concatUnits(
			sessionStackInfra, niriSessionAppUnits, sessionStackGTKTail, sessionStackKDETail)...),
		sessionAppsAbsent:    joinUnits(concatUnits(niriSessionAppUnits, wayvncUnit)...),
		userDinitRuntimeSvcs: joinUnits(concatUnits(sessionStackRuntimeInfra, niriSessionAppUnits, wayvncUnit)...),
		userSvcBootLinks:     joinUnits(sessionStackInfra...),
	},
	desktopNiriWayle: {
		dinitUserSvcFiles: joinUnits(concatUnits(
			sessionStackInfra, wayleSecretsUnits, []string{"polkit-agent.user"}, wayleSessionAppUnits,
			sessionStackGTKTail, wayleGnomePortalUnit, sessionStackKDETail)...),
		sessionAppsAbsent:    joinUnits(concatUnits(wayleSessionAppUnits, []string{"polkit-agent.user"})...),
		userDinitRuntimeSvcs: joinUnits(concatUnits(sessionStackRuntimeInfra, wayleSecretsUnits, []string{"polkit-agent.user"}, wayleSessionAppUnits, wayvncUnit)...),
		userSvcBootLinks:     joinUnits(concatUnits(sessionStackInfra, wayleSecretsUnits)...),
	},
}

// sessionStackForDE returns the session stack for the DE. Only niri-family
// DEs consume it; kde/cosmic/none keep the niri default (mirrors the
// pre-refactor locals, which held the niri values for those DEs but were
// never read there).
func sessionStackForDE(de string) deSessionStack {
	if de == desktopNiriWayle {
		return deSessionStacks[desktopNiriWayle]
	}
	return deSessionStacks[desktopNiri]
}

// deSessionProbe — the parameters of the kde/cosmic DE session runtime
// check (one parameterized payload, P2-4). pgrep -f anchored on the binary
// paths: busybox `pgrep -x` has a false-negative class on these processes
// (2026-10-24 run 2026-10-24T23-06-35Z: kwin_wayland/plasmashell proven
// RUNNING by the check's own process dump while `pgrep -x` polled 90 s
// without matching — same class as the turnstile_pkg fix). The ^ anchor
// also avoids self-matching this check's own cmdline.
type deSessionProbe struct {
	checkName         string // "kde_session" / "cosmic_session"
	procPatternA      string // compositor process pattern
	procPatternB      string // shell/session process pattern
	notRunningWording string // failure wording (parenthesized process names)
	diagPgrepPattern  string // failure diagnostics pgrep -a -f pattern
	okWording         string // success wording (ends with ", wayland socket $w)")
}

var deSessionProbes = map[string]deSessionProbe{
	desktopKDE: {
		checkName:         "kde_session",
		procPatternA:      "^/usr/bin/kwin_wayland",
		procPatternB:      "^/usr/bin/plasmashell",
		notRunningWording: "KDE session not running (kwin_wayland/plasmashell)",
		diagPgrepPattern:  "kwin|plasma|sddm",
		okWording:         "KDE session OK (kwin_wayland + plasmashell, wayland socket $w)",
	},
	desktopCosmic: {
		checkName:         "cosmic_session",
		procPatternA:      "^cosmic-comp",
		procPatternB:      "^/usr/bin/cosmic-session",
		notRunningWording: "COSMIC session not running (cosmic-comp/cosmic-session)",
		diagPgrepPattern:  "cosmic",
		okWording:         "COSMIC session OK (cosmic-comp + cosmic-session, wayland socket $w)",
	},
}

// buildDESessionCheck renders the DE session runtime check payload (poll
// ≤90 s like the niri runtime checks; falsifiable, exit 1 on failure).
// The goldens in golden_test.go pin the exact rendered payload per DE.
func buildDESessionCheck(cfg *VerifyConfig, probe *deSessionProbe) string {
	return fmt.Sprintf(`
i=0; ok=0
while [ $i -lt 90 ]; do
    if pgrep -f '%s' >/dev/null 2>&1 && pgrep -f '%s' >/dev/null 2>&1; then ok=1; break; fi
    sleep 1; i=$((i+1))
done
[ "$ok" = 1 ] || { echo '%s'; pgrep -a -f '%s' 2>/dev/null | head -10; exit 1; }
uid=$(id -u %s)
w=''
for s in /run/user/$uid/wayland-*; do [ -S "$s" ] && w="$s"; done
[ -n "$w" ] || { echo 'no wayland socket in XDG_RUNTIME_DIR'; exit 1; }
echo "%s"
`, probe.procPatternA, probe.procPatternB, probe.notRunningWording, probe.diagPgrepPattern, cfg.InstalledUser, probe.okWording)
}

// dinitExpectedServicesBase — services that must hold a started marker in
// `dinitctl list` on every dinit install (services_status evaluation).
var dinitExpectedServicesBase = []string{
	// entry points + targets
	"boot", "system", "local.target", "pre-local.target",
	"login.target", "network.target", "pre-network.target",
	// console
	"getty@tty1", "getty@ttyS0",
	// daemons (rules-driven boot.d enablement; ADR-010 R1
	// amendment: explicitly-activated system units carry the
	// OpenRC initd name; fwupd/PPD are dbus-activated on demand
	// — never boot-enabled, so not expected here)
	"networking", "incusd", "vector", "cronie", "chronyd", "dbus",
	"sshd", "acpid", "irqbalance", "earlyoom",
}

// dinitExpectedServicesNiriFamily — the niri-family desktop services
// (polkit/elogind/bluetooth; rules-driven boot.d enablement).
var dinitExpectedServicesNiriFamily = []string{"polkit", "elogind", "bluetooth"}

// openrcExpectedServicesBase — services rc-status must report started on
// every OpenRC install (services_status evaluation).
var openrcExpectedServicesBase = []string{
	// sysinit
	"devfs", "dmesg", "sysctl", "sysfs",
	// boot
	"hwclock", "hostname", "bootmisc", "binfmt", "root", "procfs",
	"loadkmap", "localmount", "btrfs-scan", "dmcrypt",
	"udev", "udev-trigger", "vector", "vector-setup",
	// default
	"cgroups", "networking", "cronie", "chronyd", "dbus", "sshd",
	"acpid", "irqbalance", "earlyoom", "incusd", "udev-postmount",
	// fwupd NOT expected: dbus-activated on demand (ADR-010 R1
	// amendment) — never in a runlevel on either init
}

// openrcExpectedServicesNiriFamily — greetd + elogind are only expected
// when a niri-family desktop is enabled (niri, niriWayle — both enable
// them in their rules services map).
var openrcExpectedServicesNiriFamily = []string{"greetd", "elogind"}

// dinitExpectedServices assembles the expected started-marker list for a
// dinit target (golden-tested against the pre-refactor slices).
func dinitExpectedServices(cfg *VerifyConfig) []string {
	expected := append([]string{}, dinitExpectedServicesBase...)
	if isNiriFamilyDE(cfg.DesktopEnvironment) {
		expected = append(expected, dinitExpectedServicesNiriFamily...)
	}
	return expected
}

// openrcExpectedServices assembles the expected rc-status list for an
// OpenRC target (golden-tested against the pre-refactor slices).
func openrcExpectedServices(cfg *VerifyConfig) []string {
	expected := append([]string{}, openrcExpectedServicesBase...)
	if isNiriFamilyDE(cfg.DesktopEnvironment) {
		expected = append(expected, openrcExpectedServicesNiriFamily...)
	}
	// swap service is conditional: only when swap exists (enableSwap for
	// btrfsStandard/btrfsRaid0, always for btrfsStandardServer LUKS swap).
	if cfg.EnableSwap || cfg.DiskLayout == layoutBtrfsStandardServer {
		expected = append(expected, "swap")
	}
	return expected
}
