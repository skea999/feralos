package features

import (
	"feralos/src/utils/config"
	"feralos/src/utils/shell"
	"fmt"
	"strings"
)

// CompleteSecurity — the completeSecurity variable (ADR-014): the
// enterprise observability stack (auditd, AIDE, fail2ban, Tracee, lynis —
// ADR-012), opt-in instead of always-on since the 2026-11-22 split.
// ONE package: complete-security-config pulls the whole family as apk
// dependencies AND ships the static configs the installer used to write
// (audit rules, AIDE config + cron — ADR-013 R2).
//
// There is no Setup work in Go left: the static configs are package-shipped
// and the service enablement (auditd/fail2ban/tracee: default) rides the
// complete-security-config packageLogic key in internal_rules.yaml —
// dep-installed packages have no reachable rules key of their own. This
// file remains the feature's single owner (feature confinement): the
// packages list, the INFO trail, and BaselineAide (the one remaining
// installer-side action — it MUST run at step 18, after the edge install,
// because the baseline has to capture the FINAL filesystem state).
func CompleteSecurityPackages(conf *config.Config) []string {
	if !conf.User.EnableCompleteSecurity {
		return nil
	}
	return []string{"complete-security-config"}
}

// SetupCompleteSecurity applies the complete-security feature inside the
// chroot. The static configs are package-shipped (complete-security-config:
// audit.rules, aide.conf, aide cron); enablement rides the rules key.
func SetupCompleteSecurity(conf *config.Config) error {
	if !conf.User.EnableCompleteSecurity {
		fmt.Println("[INFO] Complete security disabled, skipping...")
		return nil
	}
	fmt.Println("[INFO] Complete security: the enterprise stack is package-shipped (complete-security-config).")
	return nil
}

// BaselineAide generates the AIDE baseline database IN-CHROOT at the very end
// of ChrootPhase.Run (step 18, after the edge install — the last step that
// can touch watched paths): the baseline must capture the FINAL filesystem
// state or every install-time change would surface as first-boot drift.
// Outcome is verified by DB-FILE EXISTENCE, not by the command exit: in
// best-effort runs ExecuteCmd swallows failures (error=nil) — the harness
// check on /var/lib/aide/aide.db is the strict signal, this log is honest.
func BaselineAide(conf *config.Config, pkgList string) {
	if !conf.User.EnableCompleteSecurity || !pkgInstalled(pkgList, "aide") {
		fmt.Println("[INFO] AIDE not part of this install, skipping baseline database")
		return
	}
	chroot := conf.Internal.ChrootDir
	if _, err := shell.ExecuteCmd(chroot, "aide --init && test -f /var/lib/aide/aide.db.new && mv /var/lib/aide/aide.db.new /var/lib/aide/aide.db"); err != nil {
		fmt.Printf("[WARNING] AIDE baseline generation failed: %v\n", err)
	}
	// Outcome via ls OUTPUT, not exit code: best-effort runs (harness) swallow
	// command failures — even a `test -f` reports error=nil there, and an
	// exit-based check would print a false success (run7, line 5074).
	out, _ := shell.ExecuteCmd(chroot, "ls /var/lib/aide/aide.db 2>/dev/null")
	if !strings.Contains(out, "aide.db") {
		fmt.Printf("[WARNING] AIDE baseline database MISSING (integrity monitoring inactive until the daily check writes one)\n")
		return
	}
	fmt.Println("[INFO] AIDE baseline database generated.")
}

// pkgInstalled reports whether pkg appears in the `apk info` listing (the
// line-delimited gate pattern — formerly internal/security_stack.go's
// installed(), which moved here with its only consumer).
func pkgInstalled(pkgList, pkg string) bool {
	return strings.Contains(pkgList, "\n"+pkg+"\n") ||
		strings.HasPrefix(pkgList, pkg+"\n") ||
		strings.HasSuffix(pkgList, "\n"+pkg)
}
