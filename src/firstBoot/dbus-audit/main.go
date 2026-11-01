// feralos-dbus-audit — ADR-010 supervision audit (standalone Go binary).
//
// Rule (docs/adr/ADR-010-dbus-activation-dinit.md): every D-Bus-activatable
// daemon must run as a supervised dinit service — activation only triggers
// dinit. Session-bus targets are user units named <bus-name>.user in
// /etc/dinit.d/user (R1 naming; infra units may carry the service name
// instead, e.g. pipewire.user). This tool is the enforcement/reporting
// counterpart: it scans /usr/share/dbus-1/services/*.service (session-bus
// activation inventory), verifies a unit exists for every Exec binary and
// reports the core session infra boot.d links.
//
// Runs in three modes, same code:
//   - install-time: called by the installer at the tail of chrootPhase,
//     AFTER all package installations (nix/flatpak included) — the apk add
//     fallback can pull a missing *-user-dinit package right there
//   - boot-time: dinit oneshot / OpenRC initd (feralos-dbus-audit package)
//     writes the report to /var/log/feralos-dbus-audit.log via --log-file
//   - CLI manual: plain invocation, structured report on stdout
//
// Exit code: 0 = every activation target is covered (unit exists, or the
// matching *-user-dinit package was found and installed); 1 = at least one
// target has no *-user-dinit package at all (an aports gap — CI/CI-like
// consumers use this to catch regressions).
//
// Stdlib-only binary (the utils/shell import is part of the module): no new
// third-party dependencies.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"feralos/src/utils/shell"
)

const (
	servicesDir  = "/usr/share/dbus-1/services"
	userUnitsDir = "/etc/dinit.d/user"

	// user boot.d dirs: the turnstile-spawned user dinit waits for both —
	// /usr/lib/dinit.d/user/boot.d (packages + installer links) and
	// /etc/dinit.d/user/boot.d (admin dir).
	bootdLibDir  = "/usr/lib/dinit.d/user/boot.d"
	bootdEtcDir  = "/etc/dinit.d/user/boot.d"
	auditLogFile = "/var/log/feralos-dbus-audit.log"
)

// coreBootdUnits — core session INFRA units that must be linked into the
// user boot.d dirs when installed (they are NOT dbus-activated; without the
// link they would never start before the session needs them). Checked
// report-only: a unit file absent on this install (DE does not ship it) is
// n/a, a unit file present WITHOUT the boot.d link is a warning.
var coreBootdUnits = []string{
	"dbus",
	"pipewire",
	"pipewire-pulse",
	"wireplumber",
	"wait-wayland",
	"org.freedesktop.secrets",          // oo7 (niriWayle)
	"org.freedesktop.portal.Desktop",   // xdg-desktop-portal frontend
	"org.freedesktop.portal.Documents", // document portal
	"org.freedesktop.impl.portal.PermissionStore", // permission store
	"org.freedesktop.impl.portal.desktop.oo7",     // oo7 Secret portal
}

// sessionScopedUnits — units intentionally NOT in boot.d (session-apps
// model, 2026-10-11: they start with the compositor session via the session
// script, never at login scope; the greeter user must not start them).
// Reported for visibility only, never a warning.
var sessionScopedUnits = []string{
	"polkit-agent",
	"wayvnc",
}

// exemptNames — activation targets with NO unit possible upstream; the
// audit must not flag them (kept in sync with the harness dbus checks):
//   - org.kde.kcookiejar5: kded5 binary absent on Alpine 3.24 — no unit
//     can exist (documented harness exemption).
var exemptNames = map[string]bool{
	"org.kde.kcookiejar5": true,
}

// dbusService — one parsed activation file.
type dbusService struct {
	file string // basename of the .service file
	name string // Name= (canonical bus name)
	bin  string // binary basename from Exec=
	exec string // raw Exec= (kept for the report)
}

// reporter writes each line to stdout and, when --log-file is set, to the
// log file (boot mode).
type reporter struct {
	file io.Writer
}

func (r *reporter) line(format string, a ...any) {
	msg := fmt.Sprintf(format, a...)
	fmt.Println(msg)
	if r.file != nil {
		fmt.Fprintln(r.file, msg)
	}
}

func main() {
	logFile := flag.String("log-file", "", "also write the report to this file (boot mode)")
	userName := flag.String("user", "", "session user for the report (default: $USER; units are system-wide, this only annotates the report and the runtime override check)")
	flag.Parse()

	shell.SetCaller("dbus-audit")
	fmt.Println("=== FeralOS — D-Bus supervision audit (ADR-010) ===")

	var file io.Writer
	var logF *os.File
	if *logFile != "" {
		f, err := os.Create(*logFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[ERROR] cannot open log file %s: %v\n", *logFile, err)
			os.Exit(1)
		}
		file = f
		logF = f
	}
	rep := &reporter{file: file}

	if *userName == "" {
		*userName = os.Getenv("USER")
	}
	if *userName == "" {
		if u, err := user.Current(); err == nil {
			*userName = u.Username
		}
	}

	exitCode := auditActivationTargets(rep)
	checkCoreBootd(rep)
	reportSessionOverrides(rep, *userName)

	if logF != nil {
		_ = logF.Close()
	}
	os.Exit(exitCode)
}

// auditActivationTargets scans the session activation inventory and returns
// the exit code (0 covered, 1 gap).
func auditActivationTargets(rep *reporter) int {
	files, err := filepath.Glob(filepath.Join(servicesDir, "*.service"))
	if err != nil {
		rep.line("[ERROR] scanning %s: %v", servicesDir, err)
		return 1
	}
	rep.line("")
	rep.line("--- Activation targets (%s) ---", servicesDir)
	if len(files) == 0 {
		rep.line("no .service files found (base/server install — nothing to audit)")
		return 0
	}

	ok, installed, missing := 0, 0, 0
	for _, f := range files {
		svc, err := parseServiceFile(f)
		if err != nil {
			rep.line("SKIP  %s: %v", filepath.Base(f), err)
			continue
		}
		if svc.bin == "" {
			rep.line("SKIP  %s: no Exec", svc.file)
			continue
		}
		if isPlaceholderExec(svc.exec) {
			rep.line("SKIP  %s: placeholder Exec (%s) — systemd-only upstream trap", svc.file, svc.exec)
			continue
		}
		if exemptNames[svc.name] {
			rep.line("EXEMPT %s (%s): no unit possible upstream (documented)", svc.file, svc.name)
			continue
		}

		if unitExists(svc.name, svc.bin) {
			ok++
			rep.line("OK     %-45s unit=%s.user", svc.name, unitName(svc.name, svc.bin))
			continue
		}

		// Missing unit — try the *-user-dinit package fallback.
		pkg := ensurePackage(svc.bin, svc.name)
		if pkg != "" {
			if unitExists(svc.name, svc.bin) {
				installed++
				rep.line("FIXED  %-45s installed %s (unit now present)", svc.name, pkg)
				continue
			}
			rep.line("WARN   %-45s installed %s but unit still missing", svc.name, pkg)
			missing++
			continue
		}

		missing++
		rep.line("MISS   %-45s exec=%s — no *-user-dinit package for %s — create it in aports", svc.name, svc.exec, svc.bin)
	}

	rep.line("")
	rep.line("--- Summary: %d activation targets — %d covered, %d installed, %d missing ---",
		len(files), ok, installed, missing)
	if missing > 0 {
		return 1
	}
	return 0
}

// parseServiceFile parses a session D-Bus activation file (INI format,
// [D-BUS Service] section with Name=/Exec= keys).
func parseServiceFile(path string) (dbusService, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return dbusService{}, err
	}
	svc := dbusService{file: filepath.Base(path)}
	inSection := false
	for _, raw := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			inSection = strings.EqualFold(line, "[D-BUS Service]")
			continue
		}
		if !inSection {
			continue
		}
		key, val, found := strings.Cut(line, "=")
		if !found {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		switch key {
		case "Name":
			if svc.name == "" {
				svc.name = val
			}
		case "Exec":
			if svc.exec == "" {
				svc.exec = val
			}
		}
	}
	if svc.name == "" {
		return svc, fmt.Errorf("missing Name=")
	}
	svc.bin = binFromExec(svc.exec)
	return svc, nil
}

// binFromExec extracts the binary basename from an Exec= value (first
// whitespace-separated token, quotes stripped, dir prefix dropped).
func binFromExec(exec string) string {
	if exec == "" {
		return ""
	}
	fields := strings.Fields(exec)
	if len(fields) == 0 {
		return ""
	}
	first := strings.Trim(fields[0], `"'`)
	return filepath.Base(first)
}

// isPlaceholderExec — upstream systemd-only trap files (e.g. avahi's
// historical Exec=/bin/false): no real daemon to supervise.
func isPlaceholderExec(exec string) bool {
	bin := binFromExec(exec)
	return bin == "false" || bin == "true" || bin == ":" || bin == ""
}

// unitExists checks both unit-naming conventions for a target: the
// ADR-010 canonical bus name (org.freedesktop.secrets.user) and the
// service name used by core infra units (pipewire.user).
func unitExists(busName, bin string) bool {
	return unitName(busName, bin) != ""
}

// unitName returns the matching unit basename (without .user) or "".
func unitName(busName, bin string) string {
	for _, candidate := range []string{busName, bin} {
		if candidate == "" {
			continue
		}
		if fileExists(filepath.Join(userUnitsDir, candidate+".user")) {
			return candidate
		}
	}
	return ""
}

// ensurePackage tries `apk search` for <bin>-user-dinit and
// <busName>-user-dinit; on a hit installs it and returns the package name.
// Best-effort: any failure (offline, non-root, missing index) returns ""
// and the target counts as missing. ExecSilent — NOT ExecuteCmd: the
// retry/interactive prompt path would hang a boot-time oneshot (no tty at
// PID1) and spam the boot log; the audit logs its own outcome lines.
func ensurePackage(bin, busName string) string {
	for _, candidate := range []string{bin + "-user-dinit", busName + "-user-dinit"} {
		out, err := shell.ExecSilent("", "apk search -q -x "+candidate)
		if err != nil || !strings.Contains(out, candidate) {
			continue
		}
		if _, err := shell.ExecSilent("", "apk add "+candidate); err != nil {
			fmt.Printf("[WARNING] apk add %s failed: %v\n", candidate, err)
			return ""
		}
		return candidate
	}
	return ""
}

// checkCoreBootd verifies the core session infra boot.d links (report-only:
// absent unit files are n/a for this install, present-but-unlinked units
// warn; polkit-agent/wayvnc are session-scoped and never linked at boot).
func checkCoreBootd(rep *reporter) {
	rep.line("")
	rep.line("--- Core session infra (boot.d links) ---")
	for _, name := range coreBootdUnits {
		unit := filepath.Join(userUnitsDir, name+".user")
		if !fileExists(unit) {
			rep.line("N/A    %s (unit not installed on this system)", name)
			continue
		}
		if linkExists(name) {
			rep.line("OK     %s (linked in boot.d)", name)
			continue
		}
		rep.line("WARN   %s (unit installed but NOT linked in boot.d — infra would not start at login)", name)
	}
	for _, name := range sessionScopedUnits {
		unit := filepath.Join(userUnitsDir, name+".user")
		if !fileExists(unit) {
			rep.line("N/A    %s (unit not installed on this system)", name)
			continue
		}
		rep.line("OK     %s (session-scoped — started by the session script, no boot.d link by design)", name)
	}
}

// linkExists checks both boot.d dirs for the unit link.
func linkExists(name string) bool {
	return fileExists(filepath.Join(bootdLibDir, name)) || fileExists(filepath.Join(bootdEtcDir, name))
}

// reportSessionOverrides reports the user's materialized session activation
// overrides ($XDG_RUNTIME_DIR/dbus-1/services — the dbus-session wrapper
// copies the packaged overrides there at login). Informational: the dir
// only exists on a booted session, never at install time. Best-effort
// (permission-denied for other users is silently n/a).
func reportSessionOverrides(rep *reporter, userName string) {
	rep.line("")
	rep.line("--- Session overrides (user: %s) ---", userName)
	if userName == "" {
		rep.line("N/A    no user given")
		return
	}
	u, err := user.Lookup(userName)
	if err != nil {
		rep.line("N/A    unknown user %s", userName)
		return
	}
	dir := fmt.Sprintf("/run/user/%s/dbus-1/services", u.Uid)
	entries, err := os.ReadDir(dir)
	if err != nil {
		rep.line("N/A    %s not present (no live session)", dir)
		return
	}
	count := 0
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".service") {
			count++
		}
	}
	rep.line("OK     %s: %d materialized session activation overrides", dir, count)
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
