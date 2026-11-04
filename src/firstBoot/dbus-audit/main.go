// feralos-dbus-audit — ADR-010 supervision audit (standalone Go binary).
//
// Rule (docs/adr/ADR-010-dbus-activation-dinit.md): every D-Bus-activatable
// daemon must run as a supervised dinit service — activation only triggers
// dinit. This tool is the AUTO-INSTALLER and enforcement/reporting
// counterpart (R1b, 2026-11-03): it discovers every activation target and
// installs its aports supervision package, so internal_rules.yaml carries
// NO *-dinit supervision packages at all.
//
// Discovery scans the upstream activation inventory of BOTH buses (the same
// enumeration as the harness dbus_activation_migration check):
//   - session: /usr/share/dbus-1/services/*.service
//   - system:  /usr/share/dbus-1/system-services/*.service
//
// and parses Name= (bus name) + Exec= (binary path). Placeholder upstream
// traps (Exec=/bin/false — systemd-only) and documented exemptions
// (org.kde.kcookiejar5 — no unit possible on Alpine 3.24) are skipped.
//
// Resolution — unit present → OK; missing unit → apk add the supervision
// package, candidates in order (R1b naming = the daemon's apk package):
//  1. <who-owns(Exec)>-user-dinit / <who-owns(Exec)>-dinit
//     (apk info --who-owns on the Exec path = the daemon's apk package;
//     disambiguates same-API daemons like org.freedesktop.secrets —
//     gnome-keyring on niri vs oo7 on niriWayle — and multi-target
//     daemons like plasma-workspace/kio/xdg-desktop-portal with zero
//     package splits)
//  2. <bus-name>-user-dinit / <bus-name>-dinit
//     (resolves provides= aliases, e.g. the plasma-kauth bus names)
//
// First successful install is re-verified by the unit-existence check.
// No package at all → MISS (an aports gap) and exit code 1.
//
// The unit-existence check honors both unit-naming conventions (bus name
// and binary basename) AND the packaged FeralOS overrides
// (/usr/share/feralos/dbus-services session, /usr/share/feralos/
// dbus-system-services system): an override aliases the bus name onto the
// unit it starts, e.g. org.freedesktop.login1 → elogind (R1 amended).
//
// Runs in three modes, same code:
//   - install-time: called by the installer at the tail of chrootPhase
//     (step 8a, AFTER all package installations incl. nix/flatpak) — the
//     apk fallback installs the supervision packages right there
//   - boot-time: dinit oneshot / OpenRC initd (feralos-dbus-audit package)
//     writes the report to /var/log/feralos-dbus-audit.log via --log-file
//   - CLI manual: plain invocation, structured report on stdout
//
// Exit code: the INSTALLER passes --no-fail → always 0 (an extra package
// without a ready *-user-dinit must never stop an interactive install; the
// dbus raw-exec fallback keeps the system working until the supervision
// package lands in aports). Every other mode (boot oneshot, CLI, CI
// consumers) stays STRICT: 0 = every activation target is covered, 1 = at
// least one target has no supervision package (an aports gap — the
// post-install signal the user must notice: the boot oneshot marks itself
// FAILED and the report lands in /var/log/feralos-dbus-audit.log).
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
	sessionServicesDir = "/usr/share/dbus-1/services"
	systemServicesDir  = "/usr/share/dbus-1/system-services"

	// FeralOS override staging dirs (packaged files): consulted ONLY for the
	// override-aware unit-existence check — an override aliases the bus name
	// onto the unit it starts (never for discovery; discovery is the upstream
	// inventory so new daemons are found without a static list).
	sessionOverridesDir = "/usr/share/feralos/dbus-services"
	systemOverridesDir  = "/usr/share/feralos/dbus-system-services"

	// unit dirs: session units are <name>.user under /etc/dinit.d/user;
	// system units are plain names under /lib/dinit.d (packages), /usr/lib/
	// dinit.d and the admin dir /etc/dinit.d.
	userUnitsDir = "/etc/dinit.d/user"

	// user boot.d dirs: the turnstile-spawned user dinit waits for both —
	// /usr/lib/dinit.d/user/boot.d (packages + installer links) and
	// /etc/dinit.d/user/boot.d (admin dir).
	bootdLibDir  = "/usr/lib/dinit.d/user/boot.d"
	bootdEtcDir  = "/etc/dinit.d/user/boot.d"
	auditLogFile = "/var/log/feralos-dbus-audit.log"
)

// systemUnitDirs — where system dinit units may live (first match wins).
var systemUnitDirs = []string{"/lib/dinit.d", "/usr/lib/dinit.d", "/etc/dinit.d"}

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
	"org.freedesktop.secrets",
	"org.freedesktop.portal.Desktop",
	"org.freedesktop.portal.Documents",
	"org.freedesktop.impl.portal.PermissionStore",
	"org.freedesktop.impl.portal.desktop.oo7",
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

// dbusScope — which bus a target activates on (drives the unit dirs and the
// supervision-package suffix).
type dbusScope string

const (
	scopeSession dbusScope = "session"
	scopeSystem  dbusScope = "system"
)

// suffix returns the supervision-package suffix for the scope
// (R1b: <daemon-apk-pkg>-user-dinit / <daemon-apk-pkg>-dinit).
func (s dbusScope) suffix() string {
	if s == scopeSession {
		return "-user-dinit"
	}
	return "-dinit"
}

// dbusService — one parsed activation file.
type dbusService struct {
	scope    dbusScope
	file     string // basename of the .service file
	name     string // Name= (canonical bus name)
	execPath string // Exec binary path (first token, quotes stripped)
	bin      string // binary basename from Exec=
	exec     string // raw Exec= (kept for the report)
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
	noFail := flag.Bool("no-fail", false, "report only: always exit 0 (installer mode) — without it a missing supervision package exits 1")
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
	if *noFail {
		exitCode = 0
	}

	if logF != nil {
		_ = logF.Close()
	}
	os.Exit(exitCode)
}

// auditActivationTargets scans both activation inventories and returns the
// exit code (0 covered, 1 gap).
func auditActivationTargets(rep *reporter) int {
	session := scanTargets(scopeSession, sessionServicesDir)
	system := scanTargets(scopeSystem, systemServicesDir)

	rep.line("")
	if len(session) == 0 && len(system) == 0 {
		rep.line("no .service files found (base/server install — nothing to audit)")
		return 0
	}

	ok, installed, missing := 0, 0, 0
	process := func(scope dbusScope, dir string, targets []dbusService) {
		rep.line("--- Activation targets — %s (%s) ---", scope, dir)
		for i := range targets {
			t := &targets[i]
			if unitExists(t) {
				ok++
				rep.line("OK     %-45s unit=%s", t.name, firstUnitPath(t))
				continue
			}

			// Missing unit — resolve + install the supervision package.
			pkg := ensurePackage(t)
			if pkg != "" {
				if unitExists(t) {
					installed++
					rep.line("FIXED  %-45s installed %s (unit now present)", t.name, pkg)
					continue
				}
				missing++
				rep.line("GAP    %-45s installed %s but unit still missing", t.name, pkg)
				continue
			}

			missing++
			rep.line("MISS   %-45s exec=%s — no supervision package (tried %s) — create it in aports (package = daemon apk name + %s)",
				t.name, t.execPath, strings.Join(candidateNames(t), ", "), t.scope.suffix())
		}
	}

	process(scopeSession, sessionServicesDir, session)
	process(scopeSystem, systemServicesDir, system)

	rep.line("")
	rep.line("--- Summary: %d activation targets (%d session, %d system) — %d covered, %d installed, %d missing ---",
		len(session)+len(system), len(session), len(system), ok, installed, missing)
	if missing > 0 {
		return 1
	}
	return 0
}

// scanTargets parses every activation file of one scope, skipping
// unparseable files, placeholder upstream traps and documented exemptions.
// Dedupes by bus name (first file wins).
func scanTargets(scope dbusScope, dir string) []dbusService {
	files, err := filepath.Glob(filepath.Join(dir, "*.service"))
	if err != nil {
		fmt.Printf("[ERROR] scanning %s: %v\n", dir, err)
		return nil
	}
	seen := map[string]bool{}
	out := make([]dbusService, 0, len(files))
	for _, f := range files {
		svc, err := parseServiceFile(f)
		if err != nil {
			fmt.Printf("SKIP   %s: %v\n", filepath.Base(f), err)
			continue
		}
		svc.scope = scope
		if svc.bin == "" {
			fmt.Printf("SKIP   %s: no Exec\n", svc.file)
			continue
		}
		if isPlaceholderExec(svc.exec) {
			// systemd-only trap (Exec=/bin/false). NOT skippable when WE
			// ship the activation route: the packaged FeralOS override
			// replaces the trap at login/boot (e.g. oo7-server's
			// org.freedesktop.secrets trap on niriWayle — our override
			// execs the real daemon through the shim; niri's gnome-keyring
			// ships a real upstream file, which is why that DE never hit
			// this). Re-derive the target from the override's upstream
			// exec so who-owns resolution works.
			if upstream := overrideUpstreamExec(&svc); upstream != "" {
				svc.exec = upstream
				svc.execPath = execPathFromExec(upstream)
				svc.bin = filepath.Base(svc.execPath)
			} else {
				fmt.Printf("SKIP   %s: placeholder Exec (%s) — systemd-only upstream trap\n", svc.file, svc.exec)
				continue
			}
		}
		if exemptNames[svc.name] {
			fmt.Printf("EXEMPT %s (%s): no unit possible upstream (documented)\n", svc.file, svc.name)
			continue
		}
		if seen[svc.name] {
			continue
		}
		seen[svc.name] = true
		out = append(out, svc)
	}
	return out
}

// parseServiceFile parses a D-Bus activation file (INI format,
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
	svc.execPath = execPathFromExec(svc.exec)
	svc.bin = filepath.Base(svc.execPath)
	return svc, nil
}

// execPathFromExec extracts the binary path from an Exec= value (first
// whitespace-separated token, quotes stripped).
func execPathFromExec(exec string) string {
	if exec == "" {
		return ""
	}
	fields := strings.Fields(exec)
	if len(fields) == 0 {
		return ""
	}
	return strings.Trim(fields[0], `"'`)
}

// binFromExec extracts the binary basename from an Exec= value.
func binFromExec(exec string) string {
	p := execPathFromExec(exec)
	if p == "" {
		return ""
	}
	return filepath.Base(p)
}

// isPlaceholderExec — upstream systemd-only trap files (e.g. avahi's
// historical Exec=/bin/false): no real daemon to supervise.
func isPlaceholderExec(exec string) bool {
	bin := binFromExec(exec)
	return bin == "false" || bin == "true" || bin == ":" || bin == ""
}

// shimUnit extracts the <unit> argument from a FeralOS override Exec=
// (/usr/libexec/feralos/dbus-activate <unit> -- <upstream exec>).
func shimUnit(exec string) string {
	fields := strings.Fields(exec)
	if len(fields) < 2 {
		return ""
	}
	if filepath.Base(fields[0]) != "dbus-activate" {
		return ""
	}
	return strings.Trim(fields[1], `"'`)
}

// overrideUnit reads the packaged FeralOS override for a target's bus name
// and returns the unit it starts ("" when no override / not a shim Exec).
func overrideUnit(t *dbusService) string {
	dir := sessionOverridesDir
	if t.scope == scopeSystem {
		dir = systemOverridesDir
	}
	svc, err := parseServiceFile(filepath.Join(dir, t.name+".service"))
	if err != nil {
		return ""
	}
	return shimUnit(svc.exec)
}

// overrideUpstreamExec returns the upstream exec stored in the packaged
// FeralOS override for a target (the part after the shim's `--`) or "".
// Placeholder-trap targets (Exec=/bin/false upstream) use this: the
// override is the real activation route and its upstream exec identifies
// the daemon binary for who-owns resolution.
func overrideUpstreamExec(t *dbusService) string {
	dir := sessionOverridesDir
	if t.scope == scopeSystem {
		dir = systemOverridesDir
	}
	return upstreamExecFromOverride(dir, t.name)
}

// upstreamExecFromOverride reads <dir>/<busName>.service and returns the
// exec stored after the shim's `--` separator ("" when absent / not a shim).
func upstreamExecFromOverride(dir string, busName string) string {
	svc, err := parseServiceFile(filepath.Join(dir, busName+".service"))
	if err != nil {
		return ""
	}
	fields := strings.Fields(svc.exec)
	for i, f := range fields {
		if f == "--" && i+1 < len(fields) {
			return strings.Trim(strings.Join(fields[i+1:], " "), `"'`)
		}
	}
	return ""
}

// unitPathsFor lists every on-disk unit path that covers a target: both
// unit-naming conventions (bus name + binary basename) plus the override-
// resolved unit (R1 amended: the override aliases the bus name onto the
// unit it starts, e.g. org.freedesktop.login1 → elogind). Pure — the fs
// check happens in unitExists.
func unitPathsFor(t *dbusService, overrideUnitName string) []string {
	var paths []string
	add := func(base string, suffix string) {
		if base == "" {
			return
		}
		if t.scope == scopeSession {
			paths = append(paths, filepath.Join(userUnitsDir, base+suffix))
			return
		}
		for _, d := range systemUnitDirs {
			paths = append(paths, filepath.Join(d, base+suffix))
		}
	}
	add(t.name, unitSuffix(t))
	add(t.bin, unitSuffix(t))
	add(overrideUnitName, "")
	return paths
}

// unitSuffix returns the unit-file suffix for a target's scope
// (session units carry .user).
func unitSuffix(t *dbusService) string {
	if t.scope == scopeSession {
		return ".user"
	}
	return ""
}

// firstUnitPath returns the first existing unit path for a target (report
// display) or "?" when none exists.
func firstUnitPath(t *dbusService) string {
	for _, p := range unitPathsFor(t, overrideUnit(t)) {
		if fileExists(p) {
			return p
		}
	}
	return "?"
}

// unitExists checks every candidate path for a target.
func unitExists(t *dbusService) bool {
	for _, p := range unitPathsFor(t, overrideUnit(t)) {
		if fileExists(p) {
			return true
		}
	}
	return false
}

// candidateNames lists the supervision-package candidates for a target in
// resolution order (R1b: the daemon's apk package first, then the bus name
// which resolves provides= aliases).
func candidateNames(t *dbusService) []string {
	var cands []string
	if owner := packageOwner(t.execPath); owner != "" {
		cands = append(cands, owner+t.scope.suffix())
	}
	if t.name != "" {
		cands = append(cands, t.name+t.scope.suffix())
	}
	return cands
}

// ensurePackage resolves and installs the supervision package for a target.
// Direct apk add — provider-aware (resolves provides= aliases). Best-effort:
// any failure returns "" and the target counts as missing. ExecSilent — NOT
// ExecuteCmd: the retry/interactive prompt path would hang a boot-time
// oneshot (no tty at PID1) and spam the boot log; the audit logs its own
// outcome lines.
func ensurePackage(t *dbusService) string {
	for _, c := range candidateNames(t) {
		if _, err := shell.ExecSilent("", "apk add "+c); err != nil {
			continue
		}
		return c
	}
	return ""
}

// packageOwner returns the apk package owning the Exec binary (name only,
// version stripped) or "". Output shape: "/usr/bin/x is owned by pkg-1.2.3-r0".
func packageOwner(execPath string) string {
	if execPath == "" || !strings.ContainsRune(execPath, '/') {
		return ""
	}
	out, err := shell.ExecSilent("", "apk info --who-owns "+execPath)
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		marker := " is owned by "
		i := strings.Index(line, marker)
		if i < 0 {
			continue
		}
		owner := strings.TrimSpace(line[i+len(marker):])
		if owner == "" {
			continue
		}
		return stripPkgVersion(owner)
	}
	return ""
}

// stripPkgVersion cuts "<pkg>-<version>-r<rel>" down to "<pkg>". The apkrel
// is a trailing -r<digits>; the version segment starts with a digit.
func stripPkgVersion(owner string) string {
	s := owner
	if i := strings.LastIndex(s, "-r"); i >= 0 && allDigits(s[i+2:]) {
		s = s[:i]
	}
	for i := len(s) - 2; i >= 0; i-- {
		if s[i] == '-' && isDigitByte(s[i+1]) {
			return s[:i]
		}
	}
	return s
}

func isDigitByte(b byte) bool { return b >= '0' && b <= '9' }

func allDigits(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if !isDigitByte(s[i]) {
			return false
		}
	}
	return true
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
