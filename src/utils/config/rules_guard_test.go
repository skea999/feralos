package config

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// supervisionAllowlist — the *-dinit packages that may stay in
// internal_rules.yaml. These are NOT dbus-activated (no activation file
// references them, so feralos-dbus-audit can never resolve or install
// them): core session infra, the session script, session-scoped daemons,
// the override materializer, and the override-only targets (no upstream
// activation file — the FeralOS override is the only activation route, so
// the audit is structurally blind to them).
//
// ADR-010 R1b: every OTHER supervision package must NOT be listed in the
// rules — feralos-dbus-audit discovers the activation target and
// apk-adds the daemon-named counterpart (<owner-of-Exec>-user-dinit /
// <owner-of-Exec>-dinit) at install time (chroot step 8a).
var supervisionAllowlist = map[string]bool{
	// core session infra (boot.d-linked, not activatable)
	"dbus-user-dinit":           true,
	"pipewire-user-dinit":       true,
	"pipewire-pulse-user-dinit": true,
	"wireplumber-user-dinit":    true,
	"wayland-wait-user-dinit":   true,
	// session script + niri user service (turnstile session wiring)
	"niri-user-dinit": true,
	// session-scoped daemons (started by the session script, not activation)
	"polkit-agent-user-dinit": true,
	"wayvnc-user-dinit":       true,
	// login-scope oneshot (materializes the session overrides at login)
	"dbus-session-materializer-user-dinit": true,
	// override-only targets: NO upstream activation file on Alpine — the
	// packaged FeralOS override is the only activation route, so the audit
	// cannot discover them
	"kglobalaccel-user-dinit":   true,
	"cosmic-applets-user-dinit": true,
}

// supervisionPkgRe matches quoted package tokens ending in -dinit inside
// internal_rules.yaml package lists.
var supervisionPkgRe = regexp.MustCompile(`"[A-Za-z0-9_+.@<>-]*-dinit"`)

// rulesCandidatePaths mirrors LoadInternalRules' search paths (relative to
// src/, the chdirSrc working directory).
var rulesCandidatePaths = []string{
	filepath.Join("variables", "internal_rules.yaml"),
	filepath.Join("..", "variables", "internal_rules.yaml"),
}

// TestInternalRulesCarryNoSupervisionPackages enforces the ADR-010 R1b
// auto-install contract: dbus-activated supervision packages are installed
// by feralos-dbus-audit at install time and must never be listed in the
// rules (a stale entry would silently mask an audit gap). Only the
// non-activatable infra allowlist may appear.
func TestInternalRulesCarryNoSupervisionPackages(t *testing.T) {
	chdirSrc(t)

	var data []byte
	var err error
	for _, p := range rulesCandidatePaths {
		data, err = os.ReadFile(p)
		if err == nil {
			break
		}
	}
	if err != nil {
		t.Fatalf("cannot read internal_rules.yaml: %v", err)
	}

	lines := strings.Split(string(data), "\n")
	bad := []string{}
	for i, line := range lines {
		for _, m := range supervisionPkgRe.FindAllString(line, -1) {
			pkg := strings.Trim(m, `"`)
			if supervisionAllowlist[pkg] {
				continue
			}
			bad = append(bad, filepath.Base(rulesCandidatePaths[0])+":"+
				strconv.Itoa(i+1)+": "+pkg)
		}
	}
	if len(bad) > 0 {
		t.Errorf("internal_rules.yaml lists supervision packages that feralos-dbus-audit must install (ADR-010 R1b) — remove them or justify via the allowlist:\n%s",
			strings.Join(bad, "\n"))
	}
}

// TestRulesCarryRequiredTunables pins the stage-2 rules-ification contract:
// the always-consumed tunables must be non-empty in the shipped rules (the
// installer is tunable by editing ONLY internal_rules.yaml — losing one of
// these would break every install), the niri family must enable turnstiled
// via its services map, and standardPackages must keep the packaged ADR-010
// entries (the installer RUNS the packaged audit binary — it does not
// compile it).
func TestRulesCarryRequiredTunables(t *testing.T) {
	chdirSrc(t)

	conf := &Config{}
	if err := LoadInternalRules(conf); err != nil {
		t.Fatalf("LoadInternalRules: %v", err)
	}
	i := conf.Internal

	if i.Kernel.Package == "" {
		t.Error("kernel.package is empty (hooks.go EFI sync hook watches it)")
	}
	if i.Kernel.Flavor == "" {
		t.Error("kernel.flavor is empty (uki.go: vmlinuz-<flavor>/booster-<flavor>.img/linux-<flavor>.efi)")
	}
	if i.FeralosRepo.KeyringURL == "" {
		t.Error("feralosRepo.keyringUrl is empty (packages.go repo bootstrap)")
	}
	if len(i.ResolvNameservers) == 0 {
		t.Error("resolvNameservers is empty (chroot resolv.conf)")
	}
	for name, c := range map[string]string{
		"apkAudit":  i.MaintenanceCron.ApkAudit,
		"trim":      i.MaintenanceCron.Trim,
		"scrub":     i.MaintenanceCron.Scrub,
		"snapshots": i.MaintenanceCron.Snapshots,
	} {
		if c == "" {
			t.Errorf("maintenanceCron.%s is empty (crontab schedule)", name)
		}
	}
	if i.Limits.NoFileDefault == 0 || i.Limits.NoFileUser == 0 {
		t.Error("limits.nofileDefault/nofileUser are empty/zero (limits.conf)")
	}
	if i.BoosterConfig.Compression == "" || i.BoosterConfig.ExtraFiles == "" || i.BoosterConfig.MountTimeout == "" {
		t.Error("boosterConfig compression/extraFiles/mountTimeout are empty (/etc/booster.yaml)")
	}
	if len(i.StandardUserGroups) == 0 {
		t.Error("standardUserGroups is empty (users.go supplementary groups)")
	}
	if len(i.DinitSkipSessionManaged) == 0 {
		t.Error("dinitSkipSessionManaged is empty (packages.go conversion skips)")
	}

	for _, de := range []string{"niri", "niriWayle"} {
		rules, ok := i.DesktopEnvironments[de]
		if !ok {
			t.Fatalf("desktopEnvironments.%s missing", de)
		}
		if rules.Services["turnstiled"] == "" {
			t.Errorf("desktopEnvironments.%s.services missing \"turnstiled\" enablement (services.go drives both inits)", de)
		}
	}

	for _, pkg := range []string{"feralos-dbus-activate", "feralos-dbus-audit"} {
		found := false
		for _, p := range i.StandardPackages {
			if p == pkg {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("standardPackages missing %s (the installer depends on the packaged binary)", pkg)
		}
	}
}

// TestDiskLayoutsPinned pins the diskLayouts ↔ DiskCountForLayout pairing:
// the rules file keys and the validator's switch were hand-synced with no
// test — a layout added to one side only would either orphan rules entries
// (never applied) or hit the unknown-layout runtime error. Both directions
// must match exactly and every layout must require a positive disk count.
func TestDiskLayoutsPinned(t *testing.T) {
	chdirSrc(t)

	conf := &Config{}
	if err := LoadInternalRules(conf); err != nil {
		t.Fatalf("LoadInternalRules: %v", err)
	}
	rulesLayouts := conf.Internal.DiskLayouts

	validatorLayouts := []string{"btrfsStandardServer", "btrfsStandard", "btrfsRaid0", "btrfsUnencrypted"}

	// rules → validator: every diskLayouts key must be a known layout
	for name := range rulesLayouts {
		count, err := DiskCountForLayout(name)
		if err != nil {
			t.Errorf("diskLayouts key %q is not handled by DiskCountForLayout (orphaned rules entry)", name)
			continue
		}
		if count <= 0 {
			t.Errorf("diskLayouts key %q maps to a non-positive disk count (%d)", name, count)
		}
	}

	// validator → rules: every handled layout must have a rules entry
	for _, name := range validatorLayouts {
		if _, ok := rulesLayouts[name]; !ok {
			t.Errorf("DiskCountForLayout handles %q but diskLayouts has no entry for it", name)
		}
	}

	// exact set: no extra rules keys beyond the validator's set
	if len(rulesLayouts) != len(validatorLayouts) {
		t.Errorf("diskLayouts has %d keys, DiskCountForLayout handles %d layouts — the sets drifted",
			len(rulesLayouts), len(validatorLayouts))
	}
}

// chimeraCoveredServices — OpenRC service names that must stay in the rules
// dinitSkipServices list (one-directional pin: extra entries allowed, a
// missing entry fails).
//
// Invariant: these services are enabled via defaultRunlevels /
// packageLogic.services on OpenRC, but NO -dinit unit exists for them to
// boot.d-link:
//   - udev/udev-trigger/udev-settle/udev-postmount + btrfs-scan + dmcrypt +
//     swap + cgroups — the chimera early chain provides them (the package
//     bases are in dinitReplacedByChimera, so packages.go never converts
//     them to -dinit either)
//   - vector-setup — replaced under dinit by vector-dinit's tmpfiles file
//   - loadkmap — provided dinit-natively (busybox-loadkmap-dinit) and
//     enabled via defaultRunlevelsDinit; the skip keeps the OpenRC boot-list
//     entry (same name in both lists) from creating a duplicate boot.d link.
//
// If one of these dropped out of dinitSkipServices, services.go's dinit
// enablement loop would link a unit whose service file is never installed —
// a dangling boot.d entry on every dinit install. The list is hand-synced
// (no machine-readable source); this guard is the sync proof.
var chimeraCoveredServices = []string{
	"udev",
	"udev-trigger",
	"udev-settle",
	"udev-postmount",
	"btrfs-scan",
	"dmcrypt",
	"vector-setup",
	"swap",
	"cgroups",
	"loadkmap",
}

// TestDinitSkipServicesCoverChimera enforces the chimera↔skip hand-sync
// described on chimeraCoveredServices.
func TestDinitSkipServicesCoverChimera(t *testing.T) {
	chdirSrc(t)

	conf := &Config{}
	if err := LoadInternalRules(conf); err != nil {
		t.Fatalf("LoadInternalRules: %v", err)
	}
	skip := make(map[string]bool, len(conf.Internal.DinitSkipServices))
	for _, s := range conf.Internal.DinitSkipServices {
		skip[s] = true
	}

	for _, s := range chimeraCoveredServices {
		if !skip[s] {
			t.Errorf("dinitSkipServices is missing %q — chimera-covered (or dinit-native) service present in the OpenRC enablement lists; without the skip, services.go boot.d-links a unit that no -dinit package provides", s)
		}
	}
}
