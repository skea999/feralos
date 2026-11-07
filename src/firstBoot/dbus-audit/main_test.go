package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func writeTempService(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "org.example.Daemon.service")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write temp service: %v", err)
	}
	return path
}

func TestParseServiceFileUpstream(t *testing.T) {
	path := writeTempService(t, `[D-BUS Service]
Name=org.example.Daemon
Exec=/usr/bin/example-daemon --replace --flag
`)

	svc, err := parseServiceFile(path)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if svc.name != "org.example.Daemon" {
		t.Errorf("name = %q", svc.name)
	}
	if svc.execPath != "/usr/bin/example-daemon" {
		t.Errorf("execPath = %q", svc.execPath)
	}
	if svc.bin != "example-daemon" {
		t.Errorf("bin = %q", svc.bin)
	}
}

func TestParseServiceFileShim(t *testing.T) {
	path := writeTempService(t, `[D-BUS Service]
Name=org.example.Daemon
Exec=/usr/libexec/feralos/dbus-activate org.example.Daemon.user -- /usr/bin/example-daemon --flag
`)

	svc, err := parseServiceFile(path)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got := shimUnit(svc.exec); got != "org.example.Daemon.user" {
		t.Errorf("shimUnit = %q", got)
	}
	if got := binFromExec(svc.exec); got != "dbus-activate" {
		// bin from a shim Exec is the shim itself — irrelevant for resolution
		// because the override-aware unit check uses shimUnit instead.
		t.Errorf("binFromExec = %q", got)
	}
}

func TestParseServiceFileMissingName(t *testing.T) {
	path := writeTempService(t, `[D-BUS Service]
Exec=/usr/bin/example-daemon
`)
	if _, err := parseServiceFile(path); err == nil {
		t.Fatal("expected error for missing Name=")
	}
}

func TestExecPathFromExecQuotes(t *testing.T) {
	if got := execPathFromExec(`"/usr/bin/plain" --arg`); got != "/usr/bin/plain" {
		t.Errorf("quoted path = %q", got)
	}
	if got := execPathFromExec("/usr/bin/plain --x"); got != "/usr/bin/plain" {
		t.Errorf("plain path = %q", got)
	}
	if got := execPathFromExec(""); got != "" {
		t.Errorf("empty = %q", got)
	}
}

func TestIsPlaceholderExec(t *testing.T) {
	cases := map[string]bool{
		"/bin/false":        true,
		"/bin/true":         true,
		":":                 true,
		"":                  true,
		"/usr/bin/daemon":   false,
		"/usr/bin/false -x": true, // basename = false → placeholder
	}
	for exec, want := range cases {
		if got := isPlaceholderExec(exec); got != want {
			t.Errorf("isPlaceholderExec(%q) = %v, want %v", exec, got, want)
		}
	}
}

func TestStripPkgVersion(t *testing.T) {
	cases := map[string]string{
		"gnome-keyring-42.0-r0":   "gnome-keyring",
		"wpa_supplicant-2.11-r1":  "wpa_supplicant",
		"oo7-server-0.6.0-r4":     "oo7-server",
		"kactivitymanagerd-6.2.0": "kactivitymanagerd",
		"a-b-c-1.2.3_pre-r0":      "a-b-c",
		"plainpkg":                "plainpkg",
	}
	for in, want := range cases {
		if got := stripPkgVersion(in); got != want {
			t.Errorf("stripPkgVersion(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestUnitPathsForSession(t *testing.T) {
	target := dbusService{scope: scopeSession, name: "org.example.Daemon", bin: "example-daemon"}
	got := unitPathsFor(&target, "org.example.Alias.user")
	want := []string{
		"/etc/dinit.d/user/org.example.Daemon.user",
		"/etc/dinit.d/user/example-daemon.user",
		"/etc/dinit.d/user/org.example.Alias.user",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("unitPathsFor = %v, want %v", got, want)
	}
}

func TestUnitPathsForSystem(t *testing.T) {
	target := dbusService{scope: scopeSystem, name: "org.example.Daemon", bin: "example-daemon"}
	got := unitPathsFor(&target, "elogind")
	want := []string{
		"/lib/dinit.d/org.example.Daemon",
		"/usr/lib/dinit.d/org.example.Daemon",
		"/etc/dinit.d/org.example.Daemon",
		"/lib/dinit.d/example-daemon",
		"/usr/lib/dinit.d/example-daemon",
		"/etc/dinit.d/example-daemon",
		"/lib/dinit.d/elogind",
		"/usr/lib/dinit.d/elogind",
		"/etc/dinit.d/elogind",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("unitPathsFor = %v, want %v", got, want)
	}
}

func TestCandidateNames(t *testing.T) {
	session := dbusService{scope: scopeSession, name: "org.example.Daemon", execPath: "/usr/bin/example-daemon"}
	if got := candidateNames(&session); !reflect.DeepEqual(got, []string{"example-daemon-user-dinit", "org.example.Daemon-user-dinit"}) {
		// packageOwner shells out to apk — absent on the dev host it returns
		// "", so only the bus-name candidate is expected there.
		if len(got) != 1 || got[0] != "org.example.Daemon-user-dinit" {
			t.Errorf("candidateNames(session) = %v", got)
		}
	}
	system := dbusService{scope: scopeSystem, name: "org.freedesktop.UPower"}
	got := candidateNames(&system)
	if len(got) == 0 || got[len(got)-1] != "org.freedesktop.UPower-dinit" {
		t.Errorf("candidateNames(system) last = %v", got)
	}
}

func TestUpstreamExecFromOverride(t *testing.T) {
	dir := t.TempDir()
	shim := `[D-BUS Service]
Name=org.freedesktop.secrets
Exec=/usr/libexec/feralos/dbus-activate org.freedesktop.secrets.user -- /usr/bin/oo7-daemon --replace
`
	if err := os.WriteFile(filepath.Join(dir, "org.freedesktop.secrets.service"), []byte(shim), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if got := upstreamExecFromOverride(dir, "org.freedesktop.secrets"); got != "/usr/bin/oo7-daemon --replace" {
		t.Errorf("upstreamExecFromOverride = %q", got)
	}
	if got := upstreamExecFromOverride(dir, "org.example.Missing"); got != "" {
		t.Errorf("missing override = %q", got)
	}
	noSep := `[D-BUS Service]
Name=org.example.Plain
Exec=/usr/bin/plain-daemon
`
	if err := os.WriteFile(filepath.Join(dir, "org.example.Plain.service"), []byte(noSep), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if got := upstreamExecFromOverride(dir, "org.example.Plain"); got != "" {
		t.Errorf("non-shim override = %q", got)
	}
}

func TestScopeSuffix(t *testing.T) {
	if scopeSession.suffix() != "-user-dinit" {
		t.Errorf("session suffix = %q", scopeSession.suffix())
	}
	if scopeSystem.suffix() != "-dinit" {
		t.Errorf("system suffix = %q", scopeSystem.suffix())
	}
	session := dbusService{scope: scopeSession}
	if unitSuffix(&session) != ".user" {
		t.Error("session unit suffix should be .user")
	}
	system := dbusService{scope: scopeSystem}
	if unitSuffix(&system) != "" {
		t.Error("system unit suffix should be empty")
	}
}
