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

	applyHardeningCmdline(u, loglevelArg, dmesgRestrictArg)
	// static hardened sysctls + module blacklist + the whole family ship via
	// hardening-config (apk dependency); only the DEBUG relaxation is
	// installer-written (a drop-in that sorts last)
	if err := writeDebugSysctlOverrides(chroot, conf.User.EnableDebug); err != nil {
		return err
	}

	fmt.Println("[INFO] Hardening complete.")
	return nil
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
