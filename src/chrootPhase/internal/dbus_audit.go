package internal

import (
	"fmt"
	"strings"

	"feralos/src/utils/config"
	"feralos/src/utils/shell"
)

// SetupDbusAudit builds the feralos-dbus-audit binary host-side (same
// pattern as the firstboot buildBinary helper) into the chroot at
// /usr/libexec/feralos/feralos-dbus-audit, then runs the install-time
// audit INSIDE the chroot with --no-fail — AFTER all package installations
// (nix/flatpak included), so the auto-installer can pull supervision
// packages right here. --no-fail = the audit never blocks the install
// (report + MISS lines only); the post-install/boot signal stays strict
// (exit 1, the oneshot marks itself FAILED so the user notices aports
// gaps). The aports package (same name) ships the binary for post-install
// use plus the boot oneshot units; the installer build overwrites it with
// the freshest binary before the audit runs.
func SetupDbusAudit(conf *config.Config) error {
	shell.SetCaller("dbus_audit")
	chroot := conf.Internal.ChrootDir
	binPath := chroot + "/usr/libexec/feralos/feralos-dbus-audit"

	if _, err := shell.ExecuteCmd(chroot, "mkdir -p /usr/libexec/feralos"); err != nil {
		return fmt.Errorf("failed to create /usr/libexec/feralos: %w", err)
	}

	// CGO_ENABLED=0: the installer host is Debian (glibc); a CGO build
	// links glibc dynamically and the binary dies with exit 127 on the
	// musl Alpine target. GOOS/GOARCH pinned to the x86_64 Alpine target
	// (same arch as the feralos-aports repo). The binary is stdlib+shell
	// only — CGO adds nothing.
	buildCmd := fmt.Sprintf("cd src && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o %s ./firstBoot/dbus-audit/", binPath)
	if _, err := shell.ExecuteCmd("", buildCmd); err != nil {
		return fmt.Errorf("failed to build feralos-dbus-audit binary: %w", err)
	}
	if _, err := shell.ExecuteCmd("", fmt.Sprintf("chmod 755 %s", binPath)); err != nil {
		return fmt.Errorf("failed to chmod feralos-dbus-audit binary: %w", err)
	}
	fmt.Printf("[INFO] feralos-dbus-audit binary: %s\n", binPath)

	// FULL path (bare name is not on the chroot's PATH lookup). --no-fail:
	// the audit ALWAYS exits 0 in installer mode — an extra package without
	// a ready *-user-dinit must never stop an interactive install (the dbus
	// raw-exec fallback keeps the system working until the supervision
	// package lands in aports; the boot oneshot + CLI stay strict — exit 1,
	// dinit marks the oneshot FAILED at boot, so the user notices). The
	// trailing marker `; echo FERALOS_AUDIT_EXIT=$?` still captures the REAL
	// exit code from stdout for diagnostics and keeps the wrapper itself at
	// exit 0 (ExecuteCmd never sees a failure; the 3-retry loop never
	// re-runs the audit). The report is complete either way — MISS lines
	// name every gap.
	out, err := shell.ExecuteCmd(chroot, "/usr/libexec/feralos/feralos-dbus-audit --no-fail; echo FERALOS_AUDIT_EXIT=$?")
	if err != nil {
		return fmt.Errorf("failed to run feralos-dbus-audit: %w", err)
	}
	auditExit := -1
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "FERALOS_AUDIT_EXIT=") {
			if _, serr := fmt.Sscanf(line, "FERALOS_AUDIT_EXIT=%d", &auditExit); serr != nil {
				auditExit = -1
			}
			break
		}
	}
	switch auditExit {
	case 0:
		fmt.Println("[INFO] feralos-dbus-audit: all activation targets supervised")
	case 1:
		fmt.Println("[WARNING] feralos-dbus-audit: exit 1 — activation targets lack a supervision package (MISS lines above; create/rename it in feralos-aports, daemon apk name + -user-dinit/-dinit)")
	default:
		fmt.Printf("[WARNING] feralos-dbus-audit: exit %d — binary failed to run (check the report above)\n", auditExit)
	}
	// NOTE: no boot.d relink needed here — the desktop setup (user boot.d
	// stack, step 13) runs AFTER this audit in chroot_phase.Run, so units
	// the audit auto-installed are linked by the desktop step itself
	// (verified in the installer.log timeline, run 2026-11-04T01-08-02Z).
	return nil
}
