package features

import (
	"feralos/src/utils/config"
	"feralos/src/utils/shell"
	"fmt"
)

// HardeningPackages returns the APK packages needed for security hardening.
func HardeningPackages(conf *config.Config) []string {
	if !conf.User.EnableHardening {
		return nil
	}
	// ONE package: hardening-config pulls the whole family as apk
	// dependencies (apparmor stack, clamav stack incl. clamav-config, the
	// set-permissions posture). ADR-013 R2.
	return []string{"hardening-config"}
}

// SetupHardening applies kernel hardening configuration to the chroot.
func SetupHardening(conf *config.Config) error {
	if !conf.User.EnableHardening {
		fmt.Println("[INFO] Hardening disabled, skipping...")
		return nil
	}

	chroot := conf.Internal.ChrootDir

	fmt.Println("[INFO] Applying kernel hardening...")

	// the kernel cmdline fragment ships in hardening-config
	// (/etc/kernel/cmdline.d/50-hardening.conf — assembled by SetupBoot)
	// static hardened sysctls + module blacklist + the whole family ship via
	// hardening-config (apk dependency); only the DEBUG relaxation is
	// installer-written (a drop-in that sorts last)
	if err := writeDebugSysctlOverrides(chroot, conf.User.EnableDebug); err != nil {
		return err
	}

	fmt.Println("[INFO] Hardening complete.")
	return nil
}

// writeDebugSysctlOverrides relaxes the four debug-sensitive sysctls when
// enableDebug is set. The file sorts AFTER the hardened drop-ins shipped by
// hardening-config (99-feralos-debug > 99-cmdline-hardening / 98 / 94) —
// sysctl applies the last value per key. Not written without enableDebug:
// the hardened files then stand alone.
func writeDebugSysctlOverrides(chroot string, enableDebug bool) error {
	if !enableDebug {
		return nil
	}
	fmt.Println("[INFO] Writing debug sysctl overrides (relaxed posture)...")
	debug := `kernel.printk=7 4 1 7
kernel.dmesg_restrict=0
kernel.io_uring_disabled=0
vm.memfd_noexec=1`
	if err := shell.WriteFile(chroot, "/etc/sysctl.d/99-feralos-debug.conf", []byte(debug+"\n"), 0644); err != nil {
		return fmt.Errorf("failed to write debug sysctl overrides: %w", err)
	}
	// cmdline.d debug fragment: sorts after 50-hardening.conf — per-key last
	// wins (loglevel=7, dmesg relaxed)
	cmdlineDebug := `loglevel=7
sysctl.kernel.dmesg_restrict=0`
	if err := shell.WriteFile(chroot, "/etc/kernel/cmdline.d/99-debug.conf", []byte(cmdlineDebug+"\n"), 0644); err != nil {
		return fmt.Errorf("failed to write debug cmdline fragment: %w", err)
	}
	return nil
}
