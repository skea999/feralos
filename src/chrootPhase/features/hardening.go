package features

import (
	"feralos/src/utils/config"
	"feralos/src/utils/shell"
	"fmt"
	"strings"
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

// DebugOverrides returns the kernel-cmdline overrides for EnableDebug:
// verbose log level and relaxed dmesg_restrict. The remaining relaxed sysctls
// (printk / io_uring_disabled / memfd_noexec) ship as the
// /etc/sysctl.d/99-feralos-debug.conf drop-in (writeDebugSysctlOverrides) —
// it sorts after the hardened drop-ins so the relaxed values win.
func DebugOverrides(conf *config.Config) (loglevel string, dmesgRestrict string) {
	if !conf.User.EnableDebug {
		return "loglevel=0", "sysctl.kernel.dmesg_restrict=1"
	}
	return "loglevel=7", "sysctl.kernel.dmesg_restrict=0"
}

// SetupHardening applies kernel hardening configuration to the chroot.
func SetupHardening(conf *config.Config) error {
	if !conf.User.EnableHardening {
		fmt.Println("[INFO] Hardening disabled, skipping...")
		return nil
	}

	chroot := conf.Internal.ChrootDir
	u := &conf.User

	fmt.Println("[INFO] Applying kernel hardening...")

	loglevelArg, dmesgRestrictArg := DebugOverrides(conf)

	enableApparmor(chroot, conf.User.EnableDinit)
	applyHardeningCmdline(u, loglevelArg, dmesgRestrictArg)
	// static hardened sysctls + module blacklist + the whole family ship via
	// hardening-config (apk dependency); only the DEBUG relaxation is
	// installer-written (a drop-in that sorts last)
	if err := writeDebugSysctlOverrides(chroot, conf.User.EnableDebug); err != nil {
		return err
	}
	restrictFilePermissions(chroot)
	runClamavSnapshot(chroot)

	fmt.Println("[INFO] Hardening complete.")
	return nil
}

// runClamavSnapshot pulls the signature DB ONCE at install time (the
// installer machine has network; the installed target may never). Alpine
// clamav-db 1.4.6 ships NO signatures — without this, clamd cannot start on
// the target (freshclam keeps retrying there, but only ever succeeds with a
// network). Best-effort: outcome verified by db-file existence, not exit
// (best-effort runs swallow command failures); freshclam chowns the files to
// the DatabaseOwner (clamav) per its config, so the clamd user can read them.
func runClamavSnapshot(chroot string) {
	// Minimal config: DatabaseMirror is HARD-REQUIRED by freshclam 1.x (no
	// default — without it: "No DatabaseMirror or PrivateMirror servers set",
	// exit 9 — run10); DatabaseDirectory/DatabaseOwner are the relevant
	// overrides. NOTE: the old DaemonNotify option was REMOVED from freshclam
	// 1.x — any occurrence is a parse error (run9: exit 2, DB never
	// populated). The chroot run therefore emits the benign "Clamd was NOT
	// notified" post-update warning (no clamd here) — allowlisted in the
	// harness log_errors sweep. On the TARGET the packaged freshclam.conf
	// applies and a running clamd accepts the notify.
	cfg := "# installer-time freshclam (no clamd in chroot)\nDatabaseMirror database.clamav.net\nDatabaseDirectory /var/lib/clamav\nDatabaseOwner clamav\n"
	if err := shell.WriteFile(chroot, "/tmp/freshclam-install.conf", []byte(cfg), 0644); err != nil {
		fmt.Printf("[WARNING] failed to write freshclam install config: %v\n", err)
	}
	if _, err := shell.ExecuteCmd(chroot, "freshclam --config-file=/tmp/freshclam-install.conf"); err != nil {
		fmt.Printf("[WARNING] freshclam failed: %v\n", err)
	}
	if _, err := shell.ExecuteCmd(chroot, "rm -f /tmp/freshclam-install.conf"); err != nil {
		fmt.Printf("[WARNING] failed to remove freshclam install config: %v\n", err)
	}
	// Outcome via ls OUTPUT, not exit code — best-effort runs swallow
	// command failures, an exit-based check would log a false success (run9).
	// List the DIRECTORY (always exists, exit 0 — a 4-file ls exits 2 on
	// missing operands and its [ERROR][IGNORED] marker trips log_errors,
	// run11): output carries .cvd/.cld only when signatures are present.
	out, _ := shell.ExecuteCmd(chroot, "ls /var/lib/clamav/ 2>/dev/null")
	if !strings.Contains(out, ".cvd") && !strings.Contains(out, ".cld") {
		fmt.Printf("[WARNING] clamav signature DB MISSING (clamd stays down until freshclam succeeds on a network)\n")
		return
	}
	fmt.Println("[INFO] clamav signature DB snapshot present.")
}

// enableApparmor wires the apparmor boot enablement: dinit gets the
// apparmor-dinit scripted service symlink (loads profiles into the kernel at
// boot — openrc apparmor start() equivalent; installed by the dinit
// conversion of apparmor-openrc), OpenRC is ALWAYS enabled (never
// conditional — fallback readiness: a dinit install must stay bootable
// under OpenRC).
func enableApparmor(chroot string, enableDinit bool) {
	if enableDinit {
		if _, err := shell.ExecuteCmd(chroot, "mkdir -p /etc/dinit.d/boot.d && test -f /lib/dinit.d/apparmor && ln -sf /lib/dinit.d/apparmor /etc/dinit.d/boot.d/apparmor || true"); err != nil {
			fmt.Printf("[WARNING] Failed to enable apparmor dinit service: %v\n", err)
		}
	}
	if _, err := shell.ExecuteCmd(chroot, "rc-update add apparmor boot"); err != nil {
		fmt.Printf("[WARNING] Failed to add apparmor to boot: %v\n", err)
	}
}

// applyHardeningCmdline appends the hardening kernel cmdline fragment to the
// runtime KernelCmdline list (rendered later into /etc/kernel/cmdline by
// internal.SetupBoot). lockdown=integrity (NOT confidentiality — 2026-11-15
// security-stack decision): LOCKDOWN_BPF_READ_KERNEL is confidentiality-only
// but makes the BPF verifier reject every kernel-memory-read program
// (bpf_trace.c) — Tracee (base security stack) cannot load its eBPF under
// confidentiality. integrity still blocks unsigned module loading, kexec and
// /dev/mem, and cannot be lowered at runtime.
func applyHardeningCmdline(u *config.UserVariables, loglevelArg, dmesgRestrictArg string) {
	hardeningCmdline := fmt.Sprintf(
		"sysctl.kernel.kptr_restrict=2 %s sysctl.kernel.unprivileged_bpf_disabled=1 sysctl.net.core.bpf_jit_harden=2 sysctl.dev.tty.ldisc_autoload=0 sysctl.kernel.kexec_load_disabled=1 sysctl.kernel.perf_event_paranoid=3 sysctl.kernel.yama.ptrace_scope=2 sysctl.fs.protected_symlinks=1 sysctl.fs.protected_hardlinks=1 sysctl.fs.protected_fifos=2 sysctl.fs.protected_regular=2 slab_nomerge init_on_alloc=1 init_on_free=1 page_alloc.shuffle=1 pti=on randomize_kstack_offset=on vsyscall=none module.sig_enforce=1 lockdown=integrity %s apparmor=1 security=apparmor random.trust_cpu=off intel_iommu=on efi=disable_early_pci_dma sysctl.kernel.sysrq=4 random.trust_bootloader=off iommu.passthrough=0 iommu.strict=1 ipv6.disable=1",
		dmesgRestrictArg, loglevelArg,
	)
	u.KernelCmdline = append(u.KernelCmdline, hardeningCmdline)
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
	return nil
}

// restrictFilePermissions applies the final permission posture: boot and
// module dirs root-only, stricter profile umask. Best-effort (warnings only).
func restrictFilePermissions(chroot string) {
	if _, err := shell.ExecuteCmd(chroot, "chmod 700 /boot /lib/modules"); err != nil {
		fmt.Printf("[WARNING] Failed to chmod /boot: %v\n", err)
	}

	if _, err := shell.ExecuteCmd(chroot, "sed -i 's/umask 022/umask 0077/g' /etc/profile"); err != nil {
		fmt.Printf("[WARNING] Failed to set umask: %v\n", err)
	}
}
