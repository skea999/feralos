package hostPhase

import (
	"alpineInstaller/installer/utility"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// PrepareChrootEnv writes all configuration files from Host to Guest
// before entering the chroot environment.
func PrepareChrootEnv(conf *utility.Config) error {
	// u = User variables (from variables.yaml + dynamic runtime)
	// i = Internal system rules (from internalRules.yaml)
	u := conf.User
	i := conf.Internal
	chroot := i.ChrootDir

	fmt.Println("[INFO] Preparing Chroot environment from Host...")

	// 1. Networking & DNS
	fmt.Println("[DEBUG] Injecting network and DNS configuration")
	utility.WriteFileInChroot(chroot, "/etc/resolv.conf", "nameserver 208.67.222.222\nnameserver 208.67.220.220\n")
	utility.WriteFileInChroot(chroot, "/etc/hostname", u.Hostname+"\n")
	hostsContent := fmt.Sprintf("127.0.0.1 %s %s.localdomain\n::1 %s %s.localdomain\n", u.Hostname, u.Hostname, u.Hostname, u.Hostname)
	utility.WriteFileInChroot(chroot, "/etc/hosts", hostsContent)

	// 2. Localization
	fmt.Println("[DEBUG] Setting up localization and keyboard layout")
	localeContent := fmt.Sprintf("export CHARSET=%s\nexport LANG=%s\nLC_COLLATE=%s\n", u.Locale, u.Locale, u.Locale)
	utility.WriteFileInChroot(chroot, "/etc/profile.d/20locale.sh", localeContent)
	utility.WriteFileInChroot(chroot, "/etc/conf.d/loadkmap", "KEYMAP=\"/usr/share/bkeymaps/it/it.bmap.gz\"\n")

	// 3. System Limits & Users Security
	fmt.Println("[DEBUG] Writing system limits and subuid/subgid")
	limits := fmt.Sprintf(`* hard nofile 100000
* soft nofile 100000
root hard nofile 100000
root soft nofile 100000
%s hard nofile 524288
%s soft nofile 524288
`, u.UserName, u.UserName)
	utility.WriteFileInChroot(chroot, "/etc/security/limits.conf", limits)
	utility.WriteFileInChroot(chroot, "/etc/subuid", "root:100000:65536\n")
	utility.WriteFileInChroot(chroot, "/etc/subgid", "root:100000:65536\n")

	// 4. APK Repositories
	fmt.Println("[DEBUG] Configuring APK repositories")
	repoContent := fmt.Sprintf("%s/%s/main\n%s/%s/community\n", u.Mirror, u.Branch, u.Mirror, u.Branch)
	os.MkdirAll(filepath.Join(chroot, "/etc/apk"), 0755)
	utility.WriteFileInChroot(chroot, "/etc/apk/repositories", repoContent)

	// 5. Shell Alias & Environment
	fmt.Println("[DEBUG] Injecting shell aliases and editor environment")
	aliasContent := `alias ll="ls -lah"
alias sudo=doas
alias nano=micro
alias rm="rm -i"
alias apk="apk --progress"
alias untar="tar xzvkf"
alias mv="mv -v"
`
	os.MkdirAll(filepath.Join(chroot, "/etc/profile.d"), 0755)
	utility.WriteFileInChroot(chroot, "/etc/profile.d/alias.sh", aliasContent)
	utility.WriteFileInChroot(chroot, "/etc/profile.d/editor.sh", "export EDITOR=micro\n")

	// PS1 Console Setup
	ps1Content := `if [ "$(whoami)" = "root" ] ; then
    PS1="\[\033[31;1m\]\u\[\033[0m\]@\[\033[1;33m\]\h\[\033[0m\]:\w# "
else
    PS1="\[\033[32;1m\]\u\[\033[0m\]@\[\033[1;33m\]\h\[\033[0m\]:\w> "
fi
`
	utility.WriteFileInChroot(chroot, "/etc/profile.d/console.sh", ps1Content)

	// 6. Doas Config
	fmt.Println("[DEBUG] Configuring doas permissions")
	os.MkdirAll(filepath.Join(chroot, "/etc/doas.d"), 0755)
	utility.WriteFileInChroot(chroot, "/etc/doas.d/doas.conf", "permit persist :wheel\n")

	// 7. Kernel Modules
	// Now using the unified u.Modules list populated by ApplyRules
	fmt.Println("[DEBUG] Writing kernel modules configuration")
	utility.WriteFileInChroot(chroot, "/etc/modules", strings.Join(u.Modules, "\n")+"\n")

	// 8. Modules Load (Specifically for RAID/LVM if modules are present)
	os.MkdirAll(filepath.Join(chroot, "/etc/modules-load.d"), 0755)
	for _, mod := range u.Modules {
		if mod == "raid1" {
			utility.WriteFileInChroot(chroot, "/etc/modules-load.d/raid1.conf", "raid1\n")
		}
	}

	// 9. Bootloader Config Files (GRUB & MKINITFS)
	fmt.Println("[DEBUG] Writing unified GRUB and MKINITFS configs")

	// Grub Cmdline from u.GrubCmdline (unified list)
	grubConfig := fmt.Sprintf("GRUB_CMDLINE_LINUX_DEFAULT=\"%s\"\nGRUB_DISABLE_OS_PROBER=false\nGRUB_PRELOAD_MODULES=\"fat btrfs mdraid1x part_gpt\"\n", strings.Join(u.GrubCmdline, " "))
	utility.WriteFileInChroot(chroot, "/etc/default/grub", grubConfig)

	// Mkinitfs Features from u.MkinitfsFeatures (unified list)
	utility.WriteFileInChroot(chroot, "/etc/mkinitfs/mkinitfs.conf", fmt.Sprintf("features=\"%s\"\n", strings.Join(u.MkinitfsFeatures, " ")))

	// 10. Service Configs (ZRAM & LXC)
	fmt.Println("[DEBUG] Writing ZRAM and LXC service configurations")
	utility.WriteFileInChroot(chroot, "/etc/conf.d/zram-init", "load_on_start=yes\nunload_on_stop=yes\nnum_devices=1\ntype0=swap\nsize0=8000\n")
	utility.WriteFileInChroot(chroot, "/etc/conf.d/lxc", "systemd_container=yes\n")

	return nil
}
