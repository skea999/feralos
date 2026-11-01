package internal

import (
	"fmt"

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

	buildCmd := fmt.Sprintf("cd src && go build -o %s ./firstBoot/dbus-audit/", binPath)
	if _, err := shell.ExecuteCmd("", buildCmd); err != nil {
		return fmt.Errorf("failed to build feralos-dbus-audit binary: %w", err)
	}
	if _, err := shell.ExecuteCmd("", fmt.Sprintf("chmod 755 %s", binPath)); err != nil {
		return fmt.Errorf("failed to chmod feralos-dbus-audit binary: %w", err)
	}
	fmt.Printf("[INFO] feralos-dbus-audit binary: %s\n", binPath)

	if _, err := shell.ExecuteCmd(chroot, "feralos-dbus-audit"); err != nil {
		fmt.Printf("[WARNING] feralos-dbus-audit: %v — activation targets lack a *-user-dinit package (create the units in feralos-aports)\n", err)
		return nil
	}
	fmt.Println("[INFO] feralos-dbus-audit: all activation targets supervised")
	return nil
}
