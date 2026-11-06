package internal

import (
	"fmt"

	"feralos/src/utils/config"
	"feralos/src/utils/shell"
)

// SetupDbusAudit runs the install-time ADR-010 supervision audit INSIDE the
// chroot using the PACKAGED binary — feralos-dbus-audit (standardPackages)
// ships /usr/libexec/feralos/feralos-dbus-audit; the aports package is the
// single source of truth, so bump its _commit pin whenever
// src/firstBoot/dbus-audit changes (the installer does NOT compile the
// binary anymore). Called as chroot_phase step 8a — AFTER all package
// installations (nix/flatpak included), so every activation file is on disk
// and the auto-installer can pull supervision packages right here.
// --no-fail: the audit NEVER blocks the install (report + MISS lines only);
// the post-install/boot signal stays strict (exit 1, the oneshot marks
// itself FAILED so the user notices aports gaps).
func SetupDbusAudit(conf *config.Config) error {
	shell.SetCaller("dbus_audit")
	chroot := conf.Internal.ChrootDir
	const bin = "/usr/libexec/feralos/feralos-dbus-audit"

	if _, err := shell.ExecuteCmdArgs(chroot, "test", "-x", bin); err != nil {
		fmt.Println("[WARNING] feralos-dbus-audit binary missing (feralos-dbus-audit package not installed?) — skipping the audit")
		return nil //nolint:nilerr // intentional: missing binary = WARNING + skip, never block the install
	}

	// --no-fail → exit always 0: an extra package without a ready
	// *-user-dinit must never stop an interactive install (the dbus
	// raw-exec fallback keeps the system working until the supervision
	// package lands in aports; the next boot's audit installs it).
	// The report is complete either way — MISS lines name every gap.
	if _, err := shell.ExecuteCmd(chroot, bin+" --no-fail"); err != nil {
		// Unreachable with --no-fail (exit forced to 0); defensive — the
		// harness runs with ignoreErrors anyway, never block the install.
		fmt.Printf("[WARNING] feralos-dbus-audit run failed: %v\n", err)
	}
	return nil
}
