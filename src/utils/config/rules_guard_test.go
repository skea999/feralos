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
