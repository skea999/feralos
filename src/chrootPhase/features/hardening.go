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
	return []string{"bubblewrap", "apparmor", "apparmor-profiles", "libapparmor", "apparmor-utils"}
}

// DebugOverrides returns sysctl/kernel overrides when EnableDebug is true.
func DebugOverrides(conf *config.Config) (loglevel string, dmesgRestrict string, dmesgRestrictVal string, memfdVal string, iouringVal string) {
	if !conf.User.EnableDebug {
		return "loglevel=0", "sysctl.kernel.dmesg_restrict=1", "1", "1", "1"
	}
	return "loglevel=7", "sysctl.kernel.dmesg_restrict=0", "0", "0", "0"
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

	if conf.User.EnableDinit {
		// dinit: enable the apparmor-dinit scripted service (loads profiles
		// into the kernel at boot — openrc apparmor start() equivalent).
		// Installed by the dinit conversion (apparmor-openrc -> apparmor-dinit).
		if _, err := shell.ExecuteCmd(chroot, "mkdir -p /etc/dinit.d/boot.d && test -f /lib/dinit.d/apparmor && ln -sf /lib/dinit.d/apparmor /etc/dinit.d/boot.d/apparmor || true"); err != nil {
			fmt.Printf("[WARNING] Failed to enable apparmor dinit service: %v\n", err)
		}
	}
	// OpenRC enablement — ALWAYS, never conditional (fallback readiness:
	// a dinit install must stay bootable under OpenRC).
	if _, err := shell.ExecuteCmd(chroot, "rc-update add apparmor boot"); err != nil {
		fmt.Printf("[WARNING] Failed to add apparmor to boot: %v\n", err)
	}

	loglevelArg, dmesgRestrictArg, dmesgRestrictVal, memfdVal, iouringVal := DebugOverrides(conf)
	printkVal := "3 3 3 3"
	if conf.User.EnableDebug {
		printkVal = "7 4 1 7"
	}

	hardeningCmdline := fmt.Sprintf(
		"sysctl.kernel.kptr_restrict=2 %s sysctl.kernel.unprivileged_bpf_disabled=1 sysctl.net.core.bpf_jit_harden=2 sysctl.dev.tty.ldisc_autoload=0 sysctl.kernel.kexec_load_disabled=1 sysctl.kernel.perf_event_paranoid=3 sysctl.kernel.yama.ptrace_scope=2 sysctl.fs.protected_symlinks=1 sysctl.fs.protected_hardlinks=1 sysctl.fs.protected_fifos=2 sysctl.fs.protected_regular=2 slab_nomerge init_on_alloc=1 init_on_free=1 page_alloc.shuffle=1 pti=on randomize_kstack_offset=on vsyscall=none module.sig_enforce=1 lockdown=confidentiality %s apparmor=1 security=apparmor random.trust_cpu=off intel_iommu=on efi=disable_early_pci_dma sysctl.kernel.sysrq=4 random.trust_bootloader=off iommu.passthrough=0 iommu.strict=1 ipv6.disable=1",
		dmesgRestrictArg, loglevelArg,
	)
	u.KernelCmdline = append(u.KernelCmdline, hardeningCmdline)

	sysctlHardening := fmt.Sprintf(`kernel.kptr_restrict=2
kernel.dmesg_restrict=%s
kernel.unprivileged_bpf_disabled=1
net.core.bpf_jit_harden=2
dev.tty.ldisc_autoload=0
kernel.kexec_load_disabled=1
kernel.perf_event_paranoid=3
kernel.yama.ptrace_scope=2
fs.protected_symlinks=1
fs.protected_hardlinks=1
fs.protected_fifos=2
fs.protected_regular=2
kernel.sysrq=4`, dmesgRestrictVal)
	escapedSysctlHardening := strings.ReplaceAll(sysctlHardening, "'", "'\\''")
	if _, err := shell.ExecuteCmd(chroot, fmt.Sprintf("echo '%s' > /etc/sysctl.d/99-cmdline-hardening.conf", escapedSysctlHardening)); err != nil {
		return fmt.Errorf("failed to write sysctl hardening: %w", err)
	}

	memProtect := fmt.Sprintf(`# Kernel hardening - Memory protection
kernel.unprivileged_userns_apparmor_policy=1
kernel.io_uring_disabled=%s
vm.memfd_noexec=%s
kernel.ftrace_enabled=0`, iouringVal, memfdVal)
	escapedMemProtect := strings.ReplaceAll(memProtect, "'", "'\\''")
	if _, err := shell.ExecuteCmd(chroot, fmt.Sprintf("echo '%s' > /etc/sysctl.d/98-hardening.conf", escapedMemProtect)); err != nil {
		return fmt.Errorf("failed to write mem protection sysctl: %w", err)
	}

	modprobeBlacklist := `# Disable unnecessary network protocols
install dccp /bin/false
install sctp /bin/false
install rds /bin/false
install tipc /bin/false
install n-hdlc /bin/false
install ax25 /bin/false
install netrom /bin/false
install x25 /bin/false
install rose /bin/false
install decnet /bin/false
install econet /bin/false
install af_802154 /bin/false
install ipx /bin/false
install appletalk /bin/false
install psnap /bin/false
install p8023 /bin/false
install p8022 /bin/false
install can /bin/false
install atm /bin/false

# Disable unnecessary filesystems
install cramfs /bin/false
install freevxfs /bin/false
install jffs2 /bin/false
install hfs /bin/false
install hfsplus /bin/false
install squashfs /bin/false
install udf /bin/false
install cifs /bin/false
install ksmbd /bin/false
install gfs2 /bin/false

# Disable unnecessary hardware drivers
install vivid /bin/false
install bluetooth /bin/false
install uvcvideo /bin/false
install thunderbolt /bin/false
install firewire-core /bin/false
install firewire-ohci /bin/false
install firewire_sbp2 /bin/false
install ohci1394 /bin/false
install sbp2 /bin/false
install dv1394 /bin/false
install raw1394 /bin/false
install video1394 /bin/false
install msr /bin/false
install ath_pci /bin/false
install evbug /bin/false
install usbkbd /bin/false
install eepro100 /bin/false
install de4x5 /bin/false
install eth1394 /bin/false
install snd_intel8x0m /bin/false
install snd_aw2 /bin/false
install prism54 /bin/false
install bcm43xx /bin/false
install garmin_gps /bin/false
install asus_acpi /bin/false
install snd_pcsp /bin/false
install amd76x_edac /bin/false
install aty128fb /bin/false
install atyfb /bin/false
install radeonfb /bin/false
install cirrusfb /bin/false
install cyber2000fb /bin/false
install cyblafb /bin/false
install gx1fb /bin/false
install hgafb /bin/false
install i810fb /bin/false
install intelfb /bin/false
install kyrofb /bin/false
install lxfb /bin/false
install matroxfb_bases /bin/false
install neofb /bin/false
install nvidiafb /bin/false
install pm2fb /bin/false
install rivafb /bin/false
install s1d13xxxfb /bin/false
install savagefb /bin/false
install sisfb /bin/false
install sstfb /bin/false
install tdfxfb /bin/false
install tridentfb /bin/false
install vesafb /bin/false
install vfb /bin/false
install viafb /bin/false
install vt8623fb /bin/false
install udlfb /bin/false
blacklist cdrom
blacklist sr_mod

install ppdev /bin/false
install parport_pc /bin/false
install parport /bin/false
install intel_agp /bin/false
install intel_gtt /bin/false
install intel_pmc_bxt /bin/false
install psmouse /bin/false

install input_leds /bin/false
install mousedev /bin/false
install joydev /bin/false`
	escapedModprobeBlacklist := strings.ReplaceAll(modprobeBlacklist, "'", "'\\''")
	if _, err := shell.ExecuteCmd(chroot, fmt.Sprintf("echo '%s' > /etc/modprobe.d/disabledModules.conf", escapedModprobeBlacklist)); err != nil {
		return fmt.Errorf("failed to write modprobe blacklist: %w", err)
	}

	permissionsService := `#!/sbin/openrc-run

name="set-permissions"

start() {
  ebegin "Setting secure permissions"
  /bin/chmod 0755 /usr/bin/passwd
  /bin/chown root:root /usr/bin/passwd
  /bin/chmod 745 /bin/mount
  /bin/chown root:root /bin/mount
  /bin/chmod 0755 /home/
  /bin/chown root:root /home/
  /bin/chmod 0700 /root
  /bin/chown root:root /root
  /bin/chmod a-st -R /bin/
  /bin/chmod a-st -R /usr/local/bin/
  /bin/chmod a-st -R /sbin/
  /bin/chmod a-st -R /usr/sbin/
  /bin/chmod a-st -R /lib/
  /bin/chmod a-st -R /usr/local/lib/
  /bin/chmod a-st -R /usr/lib/
  /bin/chmod a-st -R /opt/
  eend $?
}`
	escapedPermissionsService := strings.ReplaceAll(permissionsService, "'", "'\\''")
	if _, err := shell.ExecuteCmd(chroot, fmt.Sprintf("echo '%s' > /etc/init.d/set-permissions", escapedPermissionsService)); err != nil {
		return fmt.Errorf("failed to write set-permissions service: %w", err)
	}
	if _, err := shell.ExecuteCmd(chroot, fmt.Sprintf("chmod 755 %s", "/etc/init.d/set-permissions")); err != nil {
		return fmt.Errorf("failed to chmod set-permissions: %w", err)
	}

	if _, err := shell.ExecuteCmd(chroot, "echo 'kernel.core_pattern=|/bin/false' > /etc/sysctl.d/97-disableCoreDumps.conf"); err != nil {
		return fmt.Errorf("failed to write core dump config: %w", err)
	}

	if _, err := shell.ExecuteCmd(chroot, "echo 'fs.suid_dumpable=0' > /etc/sysctl.d/96-disableSuidDumps.conf"); err != nil {
		return fmt.Errorf("failed to write suid dump config: %w", err)
	}

	if _, err := shell.ExecuteCmd(chroot, "echo 'kernel.randomize_va_space=2' > /etc/sysctl.d/95-randomizeAddresses.conf"); err != nil {
		return fmt.Errorf("failed to write ASLR config: %w", err)
	}

	if _, err := shell.ExecuteCmd(chroot, fmt.Sprintf("echo 'kernel.printk=%s' > /etc/sysctl.d/94-limitLogs.conf", printkVal)); err != nil {
		return fmt.Errorf("failed to write log limit config: %w", err)
	}

	ipv4Conf := `net.ipv4.tcp_syncookies=1
net.ipv4.tcp_rfc1337=1
net.ipv4.conf.all.rp_filter=1
net.ipv4.conf.default.rp_filter=1
net.ipv4.conf.all.accept_redirects=0
net.ipv4.conf.default.accept_redirects=0
net.ipv4.conf.all.secure_redirects=0
net.ipv4.conf.default.secure_redirects=0
net.ipv4.conf.all.send_redirects=0
net.ipv4.conf.default.send_redirects=0
net.ipv4.conf.all.accept_source_route=0
net.ipv4.conf.default.accept_source_route=0
net.ipv4.tcp_sack=0
net.ipv4.tcp_dsack=0
net.ipv4.tcp_fack=0
net.ipv4.tcp_timestamps=0
net.ipv4.icmp_echo_ignore_broadcasts=1
net.ipv4.icmp_ignore_bogus_error_responses=1
net.ipv4.conf.all.log_martians=1
net.ipv4.conf.default.log_martians=1`
	escapedIpv4Conf := strings.ReplaceAll(ipv4Conf, "'", "'\\''")
	if _, err := shell.ExecuteCmd(chroot, fmt.Sprintf("echo '%s' > /etc/sysctl.d/93-ipv4Configs.conf", escapedIpv4Conf)); err != nil {
		return fmt.Errorf("failed to write ipv4 config: %w", err)
	}

	if _, err := shell.ExecuteCmd(chroot, "chmod 700 /boot /lib/modules"); err != nil {
		fmt.Printf("[WARNING] Failed to chmod /boot: %v\n", err)
	}

	if _, err := shell.ExecuteCmd(chroot, "sed -i 's/umask 022/umask 0077/g' /etc/profile"); err != nil {
		fmt.Printf("[WARNING] Failed to set umask: %v\n", err)
	}

	fmt.Println("[INFO] Hardening complete.")
	return nil
}
