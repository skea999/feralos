package service

import (
	"reflect"
	"testing"
)

// Code generated golden literals for the P2-4 data-driven refactor.
// Captured MECHANICALLY from service.go BEFORE the refactor (generator
// in /tmp/opencode/gen_golden.go): the 4 per-DE session-stack lists for
// niri + niriWayle, the rendered kde_session/cosmic_session payloads
// (InstalledUser fixed to the golden user below) and the expected-services
// slices for both inits. These goldens are the proof of no-semantic-change:
// the refactored accessors/builders must reproduce them byte-for-byte.

const goldenInstalledUser = "skea999"

var goldenStackNiri = deSessionStack{
	dinitUserSvcFiles:    "dbus.user pipewire.user pipewire-pulse.user wireplumber.user wait-wayland.user org.freedesktop.portal.Desktop.user org.freedesktop.portal.Documents.user org.freedesktop.impl.portal.PermissionStore.user polkit-agent.user dms.user ca.desrt.dconf.user org.a11y.Bus.user org.kde.kwalletd6.user org.kde.kded6.user com.google.code.AccountsSSO.SingleSignOn.user org.freedesktop.LocalSearch3.user org.freedesktop.LocalSearch3.Control.user org.freedesktop.LocalSearch3.Writeback.user org.freedesktop.Flatpak.user org.freedesktop.portal.Flatpak.user org.flatpak.Authenticator.Oci.user org.freedesktop.impl.portal.desktop.gtk.user org.freedesktop.impl.portal.desktop.wlr.user org.kde.kiod6.user org.kde.kioexecd6.user org.kde.kpasswdserver6.user org.kde.kssld6.user org.gnome.Nautilus.user org.gtk.GLib.PACRunner.user com.canonical.indicators.webcredentials.user com.nokia.singlesignonui.user org.freedesktop.portal.Tracker.user org.kde.secretservicecompat.user org.freedesktop.impl.portal.desktop.kwallet.user",
	sessionAppsAbsent:    "polkit-agent.user dms.user wayvnc.user",
	userDinitRuntimeSvcs: "niri dbus.user pipewire.user pipewire-pulse.user wireplumber.user org.freedesktop.portal.Desktop.user org.freedesktop.portal.Documents.user org.freedesktop.impl.portal.PermissionStore.user polkit-agent.user dms.user wayvnc.user",
	userSvcBootLinks:     "dbus.user pipewire.user pipewire-pulse.user wireplumber.user wait-wayland.user org.freedesktop.portal.Desktop.user org.freedesktop.portal.Documents.user org.freedesktop.impl.portal.PermissionStore.user",
}

var goldenStackNiriWayle = deSessionStack{
	dinitUserSvcFiles:    "dbus.user pipewire.user pipewire-pulse.user wireplumber.user wait-wayland.user org.freedesktop.portal.Desktop.user org.freedesktop.portal.Documents.user org.freedesktop.impl.portal.PermissionStore.user org.freedesktop.secrets.user org.freedesktop.impl.portal.desktop.oo7.user polkit-agent.user wayle.user veila.user stasis.user ca.desrt.dconf.user org.a11y.Bus.user org.kde.kwalletd6.user org.kde.kded6.user com.google.code.AccountsSSO.SingleSignOn.user org.freedesktop.LocalSearch3.user org.freedesktop.LocalSearch3.Control.user org.freedesktop.LocalSearch3.Writeback.user org.freedesktop.Flatpak.user org.freedesktop.portal.Flatpak.user org.flatpak.Authenticator.Oci.user org.freedesktop.impl.portal.desktop.gtk.user org.freedesktop.impl.portal.desktop.wlr.user org.freedesktop.impl.portal.desktop.gnome.user org.kde.kiod6.user org.kde.kioexecd6.user org.kde.kpasswdserver6.user org.kde.kssld6.user org.gnome.Nautilus.user org.gtk.GLib.PACRunner.user com.canonical.indicators.webcredentials.user com.nokia.singlesignonui.user org.freedesktop.portal.Tracker.user org.kde.secretservicecompat.user org.freedesktop.impl.portal.desktop.kwallet.user",
	sessionAppsAbsent:    "wayle.user veila.user stasis.user polkit-agent.user",
	userDinitRuntimeSvcs: "niri dbus.user pipewire.user pipewire-pulse.user wireplumber.user org.freedesktop.portal.Desktop.user org.freedesktop.portal.Documents.user org.freedesktop.impl.portal.PermissionStore.user org.freedesktop.secrets.user org.freedesktop.impl.portal.desktop.oo7.user polkit-agent.user wayle.user veila.user stasis.user wayvnc.user",
	userSvcBootLinks:     "dbus.user pipewire.user pipewire-pulse.user wireplumber.user wait-wayland.user org.freedesktop.portal.Desktop.user org.freedesktop.portal.Documents.user org.freedesktop.impl.portal.PermissionStore.user org.freedesktop.secrets.user org.freedesktop.impl.portal.desktop.oo7.user",
}

var goldenKDESessionPayload = "\ni=0; ok=0\nwhile [ $i -lt 90 ]; do\n    if pgrep -f '^/usr/bin/kwin_wayland' >/dev/null 2>&1 && pgrep -f '^/usr/bin/plasmashell' >/dev/null 2>&1; then ok=1; break; fi\n    sleep 1; i=$((i+1))\ndone\n[ \"$ok\" = 1 ] || { echo 'KDE session not running (kwin_wayland/plasmashell)'; pgrep -a -f 'kwin|plasma|sddm' 2>/dev/null | head -10; exit 1; }\nuid=$(id -u skea999)\nw=''\nfor s in /run/user/$uid/wayland-*; do [ -S \"$s\" ] && w=\"$s\"; done\n[ -n \"$w\" ] || { echo 'no wayland socket in XDG_RUNTIME_DIR'; exit 1; }\necho \"KDE session OK (kwin_wayland + plasmashell, wayland socket $w)\"\n"

var goldenCosmicSessionPayload = "\ni=0; ok=0\nwhile [ $i -lt 90 ]; do\n    if pgrep -f '^cosmic-comp' >/dev/null 2>&1 && pgrep -f '^/usr/bin/cosmic-session' >/dev/null 2>&1; then ok=1; break; fi\n    sleep 1; i=$((i+1))\ndone\n[ \"$ok\" = 1 ] || { echo 'COSMIC session not running (cosmic-comp/cosmic-session)'; pgrep -a -f 'cosmic' 2>/dev/null | head -10; exit 1; }\nuid=$(id -u skea999)\nw=''\nfor s in /run/user/$uid/wayland-*; do [ -S \"$s\" ] && w=\"$s\"; done\n[ -n \"$w\" ] || { echo 'no wayland socket in XDG_RUNTIME_DIR'; exit 1; }\necho \"COSMIC session OK (cosmic-comp + cosmic-session, wayland socket $w)\"\n"

func TestGoldenSessionStacks(t *testing.T) {
	golden := map[string]deSessionStack{
		desktopNiri:      goldenStackNiri,
		desktopNiriWayle: goldenStackNiriWayle,
		desktopKDE:       goldenStackNiri, // niri default (unused)
		desktopCosmic:    goldenStackNiri, // niri default (unused)
	}
	for _, de := range []string{desktopNiri, desktopNiriWayle, desktopKDE, desktopCosmic} {
		got := sessionStackForDE(de)
		want := golden[de]
		if got != want {
			t.Errorf("session stack drift for DE %q:\n got dinitUserSvcFiles=%q\nwant dinitUserSvcFiles=%q\n got sessionAppsAbsent=%q\nwant sessionAppsAbsent=%q\n got userDinitRuntimeSvcs=%q\nwant userDinitRuntimeSvcs=%q\n got userSvcBootLinks=%q\nwant userSvcBootLinks=%q", de, got.dinitUserSvcFiles, want.dinitUserSvcFiles, got.sessionAppsAbsent, want.sessionAppsAbsent, got.userDinitRuntimeSvcs, want.userDinitRuntimeSvcs, got.userSvcBootLinks, want.userSvcBootLinks)
		}
	}
}

func TestGoldenDESessionPayloads(t *testing.T) {
	cfg := &VerifyConfig{InstalledUser: goldenInstalledUser}
	kdeProbe := deSessionProbes[desktopKDE]
	if got := buildDESessionCheck(cfg, &kdeProbe); got != goldenKDESessionPayload {
		t.Errorf("kde_session payload drift:\nGOT:\n%s\nWANT:\n%s", got, goldenKDESessionPayload)
	}
	cosmicProbe := deSessionProbes[desktopCosmic]
	if got := buildDESessionCheck(cfg, &cosmicProbe); got != goldenCosmicSessionPayload {
		t.Errorf("cosmic_session payload drift:\nGOT:\n%s\nWANT:\n%s", got, goldenCosmicSessionPayload)
	}
}

var goldenDinitExpectedBase = []string{
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

var goldenDinitExpectedNiriFamily = []string{"polkit", "elogind", "bluetooth"}

var goldenOpenRCExpectedBase = []string{
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

var goldenOpenRCExpectedNiriFamily = []string{"greetd", "elogind"}

// TestGoldenExpectedServices asserts the refactored expected-services data
// reproduces the pre-refactor slices for every DE (dinit: niri-family gains
// polkit/elogind/bluetooth; OpenRC: greetd/elogind + the swap conditional).
func TestGoldenExpectedServices(t *testing.T) {
	niriFamily := []string{desktopNiri, desktopNiriWayle}
	other := []string{desktopKDE, desktopCosmic}

	for _, de := range niriFamily {
		cfg := &VerifyConfig{DesktopEnvironment: de}
		want := append(append([]string{}, goldenDinitExpectedBase...), goldenDinitExpectedNiriFamily...)
		if got := dinitExpectedServices(cfg); !reflect.DeepEqual(got, want) {
			t.Errorf("dinit expected services drift for %q:\ngot:  %v\nwant: %v", de, got, want)
		}
		want = append(append([]string{}, goldenOpenRCExpectedBase...), goldenOpenRCExpectedNiriFamily...)
		if got := openrcExpectedServices(cfg); !reflect.DeepEqual(got, want) {
			t.Errorf("openrc expected services drift for %q:\ngot:  %v\nwant: %v", de, got, want)
		}
	}
	for _, de := range other {
		cfg := &VerifyConfig{DesktopEnvironment: de}
		if got := dinitExpectedServices(cfg); !reflect.DeepEqual(got, goldenDinitExpectedBase) {
			t.Errorf("dinit expected services drift for %q:\ngot:  %v\nwant: %v", de, got, goldenDinitExpectedBase)
		}
		if got := openrcExpectedServices(cfg); !reflect.DeepEqual(got, goldenOpenRCExpectedBase) {
			t.Errorf("openrc expected services drift for %q:\ngot:  %v\nwant: %v", de, got, goldenOpenRCExpectedBase)
		}
	}

	// swap conditional (OpenRC only; dinit has no swap entry)
	cfg := &VerifyConfig{DesktopEnvironment: desktopNiri, EnableSwap: true}
	want := append(append(append([]string{}, goldenOpenRCExpectedBase...), goldenOpenRCExpectedNiriFamily...), "swap")
	if got := openrcExpectedServices(cfg); !reflect.DeepEqual(got, want) {
		t.Errorf("openrc expected services drift (EnableSwap):\ngot:  %v\nwant: %v", got, want)
	}
	cfg = &VerifyConfig{DesktopEnvironment: desktopKDE, DiskLayout: layoutBtrfsStandardServer}
	want = append(append([]string{}, goldenOpenRCExpectedBase...), "swap")
	if got := openrcExpectedServices(cfg); !reflect.DeepEqual(got, want) {
		t.Errorf("openrc expected services drift (StandardServer):\ngot:  %v\nwant: %v", got, want)
	}
}
