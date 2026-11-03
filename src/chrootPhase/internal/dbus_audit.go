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
// audit INSIDE the chroot — AFTER all package installations (nix/flatpak
// included), so the apk fallback can pull a missing *-user-dinit package
// right here. The aports package (same name) ships the binary for
// post-install use plus the boot oneshot units; the installer build
// overwrites it with the freshest binary before the audit runs.
// Non-fatal by design: exit 1 (activation targets with no *-user-dinit
// package) is a WARNING — the tool's job is to surface aports gaps, not
// abort installs.
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

	// FULL path (bare name is not on the chroot's PATH lookup). The
	// trailing marker `; echo FERALOS_AUDIT_EXIT=$?` captures the REAL
	// exit code from stdout: ExecuteCmd's error return cannot gate
	// success here — the harness always sets SKEA_IGNORE_ERRORS=1 (and
	// the interactive [i]gnore / noninteractive skip paths do the same),
	// so a failing binary (exit 127 glibc-on-musl, or the designed
	// exit-1 aports gap) returned nil and the old code printed the false
	// "all activation targets supervised". The wrapper command itself
	// always exits 0, which also keeps the 3-retry loop from re-running
	// the audit.
	out, err := shell.ExecuteCmd(chroot, "/usr/libexec/feralos/feralos-dbus-audit; echo FERALOS_AUDIT_EXIT=$?")
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
	return nil
}
