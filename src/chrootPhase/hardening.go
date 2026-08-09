package chrootPhase

import (
	"alpineInstaller/utils/config"
	"alpineInstaller/utils/files"
	"alpineInstaller/utils/shell"
	"fmt"
	"os"
	"path/filepath"
)

// WriteHardening applies kernel hardening configuration to the chroot.
// Controlled by conf.User.EnableHardening flag.
func WriteHardening(conf *config.Config) error {
	if !conf.User.EnableHardening {
		fmt.Println("[INFO] Hardening disabled, skipping...")
		return nil
	}

	chroot := conf.Internal.ChrootDir
	u := &conf.User

	fmt.Println("[INFO] Applying kernel hardening...")

	// AppArmor packages are installed in InstallPackages (packages.go)

	// 1. Add AppArmor to boot runlevel
	if _, err := shell.RunChrootCommand(chroot, "rc-update add apparmor boot"); err != nil {
		fmt.Printf("[WARNING] Failed to add apparmor to boot: %v\n", err)
	}

	// 3. Kernel hardening parameters via kernel cmdline
	loglevelArg := "loglevel=0"
	dmesgRestrictArg := "sysctl.kernel.dmesg_restrict=1"
	dmesgRestrictVal := "1"
	printkVal := "3 3 3 3"

	if u.EnableDebug {
		loglevelArg = "loglevel=7"
		dmesgRestrictArg = "sysctl.kernel.dmesg_restrict=0"
		dmesgRestrictVal = "0"
		printkVal = "7 4 1 7"
	}

	hardeningCmdline := fmt.Sprintf(
		"sysctl.kernel.kptr_restrict=2 %s sysctl.kernel.unprivileged_bpf_disabled=1 sysctl.net.core.bpf_jit_harden=2 sysctl.dev.tty.ldisc_autoload=0 sysctl.kernel.kexec_load_disabled=1 sysctl.kernel.perf_event_paranoid=3 sysctl.kernel.yama.ptrace_scope=2 sysctl.fs.protected_symlinks=1 sysctl.fs.protected_hardlinks=1 sysctl.fs.protected_fifos=2 sysctl.fs.protected_regular=2 slab_nomerge init_on_alloc=1 init_on_free=1 page_alloc.shuffle=1 pti=on randomize_kstack_offset=on vsyscall=none module.sig_enforce=1 lockdown=confidentiality %s apparmor=1 security=apparmor random.trust_cpu=off intel_iommu=on efi=disable_early_pci_dma sysctl.kernel.sysrq=4 random.trust_bootloader=off iommu.passthrough=0 iommu.strict=1 ipv6.disable=1",
		dmesgRestrictArg, loglevelArg,
	)
	u.KernelCmdline = append(u.KernelCmdline, hardeningCmdline)

	// 4. Sysctl hardening config
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
kernel.sysrq=4
`, dmesgRestrictVal)
	if err := files.WriteFileInChroot(chroot, "/etc/sysctl.d/99-cmdline-hardening.conf", sysctlHardening); err != nil {
		return fmt.Errorf("failed to write sysctl hardening: %w", err)
	}

	// 5. Memory protection sysctl
	memfdVal := "1"
	iouringVal := "2"
	if u.EnableDebug {
		memfdVal = "0"
		iouringVal = "0"
	}

	memProtect := fmt.Sprintf(`# Kernel hardening - Memory protection
kernel.unprivileged_userns_apparmor_policy=1
kernel.io_uring_disabled=%s
vm.memfd_noexec=%s
kernel.ftrace_enabled=0
`, iouringVal, memfdVal)
	if err := files.WriteFileInChroot(chroot, "/etc/sysctl.d/98-hardening.conf", memProtect); err != nil {
		return fmt.Errorf("failed to write mem protection sysctl: %w", err)
	}

	// 6. Module blacklisting
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
install joydev /bin/false
`
	if err := files.WriteFileInChroot(chroot, "/etc/modprobe.d/disabledModules.conf", modprobeBlacklist); err != nil {
		return fmt.Errorf("failed to write modprobe blacklist: %w", err)
	}

	// 7. Set-permissions service
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
}
`
	if err := files.WriteFileInChroot(chroot, "/etc/init.d/set-permissions", permissionsService); err != nil {
		return fmt.Errorf("failed to write set-permissions service: %w", err)
	}
	permPath := filepath.Join(chroot, "/etc/init.d/set-permissions")
	if err := os.Chmod(permPath, 0755); err != nil {
		return fmt.Errorf("failed to chmod set-permissions: %w", err)
	}

	// 8. Disable core dumps
	if err := files.WriteFileInChroot(chroot, "/etc/sysctl.d/97-disableCoreDumps.conf", "kernel.core_pattern=|/bin/false\n"); err != nil {
		return fmt.Errorf("failed to write core dump config: %w", err)
	}

	// 11. Disable SUID dumps
	if err := files.WriteFileInChroot(chroot, "/etc/sysctl.d/96-disableSuidDumps.conf", "fs.suid_dumpable=0\n"); err != nil {
		return fmt.Errorf("failed to write suid dump config: %w", err)
	}

	// 12. ASLR
	if err := files.WriteFileInChroot(chroot, "/etc/sysctl.d/95-randomizeAddresses.conf", "kernel.randomize_va_space=2\n"); err != nil {
		return fmt.Errorf("failed to write ASLR config: %w", err)
	}

	// 13. Limit kernel logs
	if err := files.WriteFileInChroot(chroot, "/etc/sysctl.d/94-limitLogs.conf", fmt.Sprintf("kernel.printk=%s\n", printkVal)); err != nil {
		return fmt.Errorf("failed to write log limit config: %w", err)
	}

	// 14. IPv4 hardening
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
net.ipv4.conf.default.log_martians=1
`
	if err := files.WriteFileInChroot(chroot, "/etc/sysctl.d/93-ipv4Configs.conf", ipv4Conf); err != nil {
		return fmt.Errorf("failed to write ipv4 config: %w", err)
	}

	// 15. Secure boot and home directories
	if _, err := shell.RunChrootCommand(chroot, "chmod 700 /boot /lib/modules"); err != nil {
		fmt.Printf("[WARNING] Failed to chmod /boot: %v\n", err)
	}

	// 16. Secure umask
	if _, err := shell.RunChrootCommand(chroot, "sed -i 's/umask 022/umask 0077/g' /etc/profile"); err != nil {
		fmt.Printf("[WARNING] Failed to set umask: %v\n", err)
	}

	fmt.Println("[INFO] Hardening complete.")
	return nil
}
