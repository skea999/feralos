package hostPhase

import (
	"alpineInstaller/utils/config"
	"alpineInstaller/utils/files"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// PrepareChrootEnv writes all configuration files from Host to Guest
// before entering the chroot environment.
func PrepareChrootEnv(conf *config.Config) error {
	// u = User variables (from variables.yaml + dynamic runtime)
	// i = Internal system rules (from internalRules.yaml)
	u := conf.User
	i := conf.Internal
	chroot := i.ChrootDir

	fmt.Println("[INFO] Preparing Chroot environment from Host...")

	// 1. Networking & DNS
	fmt.Println("[DEBUG] Injecting network and DNS configuration")
	if err := files.WriteFileInChroot(chroot, "/etc/resolv.conf", "nameserver 208.67.222.222\nnameserver 208.67.220.220\n"); err != nil {
		return fmt.Errorf("failed to write resolv.conf: %w", err)
	}
	if err := files.WriteFileInChroot(chroot, "/etc/hostname", u.Hostname+"\n"); err != nil {
		return fmt.Errorf("failed to write hostname: %w", err)
	}
	hostsContent := fmt.Sprintf("127.0.0.1 %s %s.localdomain\n::1 %s %s.localdomain\n", u.Hostname, u.Hostname, u.Hostname, u.Hostname)
	if err := files.WriteFileInChroot(chroot, "/etc/hosts", hostsContent); err != nil {
		return fmt.Errorf("failed to write hosts: %w", err)
	}

	// 2. Localization
	fmt.Println("[DEBUG] Setting up localization and keyboard layout")
	localeContent := fmt.Sprintf("export CHARSET=%s\nexport LANG=%s\nLC_COLLATE=%s\n", u.Locale, u.Locale, u.Locale)
	if err := files.WriteFileInChroot(chroot, "/etc/profile.d/20locale.sh", localeContent); err != nil {
		return fmt.Errorf("failed to write locale config: %w", err)
	}
	if err := files.WriteFileInChroot(chroot, "/etc/conf.d/loadkmap", "KEYMAP=\"/usr/share/bkeymaps/it/it.bmap.gz\"\n"); err != nil {
		return fmt.Errorf("failed to write keymap config: %w", err)
	}

	// 3. System Limits & Users Security
	fmt.Println("[DEBUG] Writing system limits and subuid/subgid")
	limits := fmt.Sprintf(`* hard nofile 100000
* soft nofile 100000
root hard nofile 100000
root soft nofile 100000
%s hard nofile 524288
%s soft nofile 524288
`, u.UserName, u.UserName)
	if err := files.WriteFileInChroot(chroot, "/etc/security/limits.conf", limits); err != nil {
		return fmt.Errorf("failed to write limits.conf: %w", err)
	}
	if err := files.WriteFileInChroot(chroot, "/etc/subuid", "root:100000:65536\n"); err != nil {
		return fmt.Errorf("failed to write subuid: %w", err)
	}
	if err := files.WriteFileInChroot(chroot, "/etc/subgid", "root:100000:65536\n"); err != nil {
		return fmt.Errorf("failed to write subgid: %w", err)
	}

	// 4. APK Repositories
	fmt.Println("[DEBUG] Configuring APK repositories")
	if err := os.MkdirAll(filepath.Join(chroot, "/etc/apk"), 0755); err != nil {
		return fmt.Errorf("failed to create /etc/apk directory: %w", err)
	}
	repoContent := fmt.Sprintf("%s/%s/main\n%s/%s/community\n", u.Mirror, u.Branch, u.Mirror, u.Branch)
	if err := files.WriteFileInChroot(chroot, "/etc/apk/repositories", repoContent); err != nil {
		return fmt.Errorf("failed to write repositories: %w", err)
	}

	// 5. Shell Alias & Environment
	fmt.Println("[DEBUG] Injecting shell aliases and editor environment")
	if err := os.MkdirAll(filepath.Join(chroot, "/etc/profile.d"), 0755); err != nil {
		return fmt.Errorf("failed to create /etc/profile.d directory: %w", err)
	}
	aliasContent := `alias ll="ls -lah"
alias nano=micro
alias rm="rm -i"
alias apk="apk --progress"
alias untar="tar xzvkf"
alias mv="mv -v"
`
	if err := files.WriteFileInChroot(chroot, "/etc/profile.d/alias.sh", aliasContent); err != nil {
		return fmt.Errorf("failed to write alias.sh: %w", err)
	}
	if err := files.WriteFileInChroot(chroot, "/etc/profile.d/editor.sh", "export EDITOR=micro\n"); err != nil {
		return fmt.Errorf("failed to write editor.sh: %w", err)
	}

	// PS1 Console Setup
	ps1Content := `if [ "$(whoami)" = "root" ] ; then
    PS1="\[\033[31;1m\]\u\[\033[0m\]@\[\033[1;33m\]\h\[\033[0m\]:\w# "
else
    PS1="\[\033[32;1m\]\u\[\033[0m\]@\[\033[1;33m\]\h\[\033[0m\]:\w> "
fi
`
	if err := files.WriteFileInChroot(chroot, "/etc/profile.d/console.sh", ps1Content); err != nil {
		return fmt.Errorf("failed to write console.sh: %w", err)
	}

	// 6. Sudo-rs Config
	fmt.Println("[DEBUG] Configuring sudo-rs permissions")
	if err := os.MkdirAll(filepath.Join(chroot, "/etc/sudoers.d"), 0755); err != nil {
		return fmt.Errorf("failed to create /etc/sudoers.d directory: %w", err)
	}
	if err := files.WriteFileInChroot(chroot, "/etc/sudoers.d/wheel", "%wheel ALL=(ALL) ALL\n"); err != nil {
		return fmt.Errorf("failed to write sudoers wheel config: %w", err)
	}

	// 7. Kernel Modules
	// Now using the unified u.Modules list populated by ApplyRules
	fmt.Println("[DEBUG] Writing kernel modules configuration")
	if err := files.WriteFileInChroot(chroot, "/etc/modules", strings.Join(u.Modules, "\n")+"\n"); err != nil {
		return fmt.Errorf("failed to write /etc/modules: %w", err)
	}

	// 8. Modules Load (Specifically for RAID/LVM if modules are present)
	if err := os.MkdirAll(filepath.Join(chroot, "/etc/modules-load.d"), 0755); err != nil {
		return fmt.Errorf("failed to create /etc/modules-load.d directory: %w", err)
	}
	for _, mod := range u.Modules {
		if mod == "raid1" {
			if err := files.WriteFileInChroot(chroot, "/etc/modules-load.d/raid1.conf", "raid1\n"); err != nil {
				return fmt.Errorf("failed to write raid1.conf: %w", err)
			}
		}
	}

	// 9. Service Configs (ZRAM & LXC)
	fmt.Println("[DEBUG] Writing ZRAM and LXC service configurations")
	if err := files.WriteFileInChroot(chroot, "/etc/conf.d/zram-init", "load_on_start=yes\nunload_on_stop=yes\nnum_devices=1\ntype0=swap\nsize0=8000\n"); err != nil {
		return fmt.Errorf("failed to write zram-init config: %w", err)
	}
	if err := files.WriteFileInChroot(chroot, "/etc/conf.d/lxc", "systemd_container=yes\n"); err != nil {
		return fmt.Errorf("failed to write LXC config: %w", err)
	}

	// 10. Bridging sysctl (for QEMU/Incus)
	fmt.Println("[DEBUG] Writing bridging sysctl")
	bridgingConf := "net.ipv4.conf.all.bc_forwarding=1\nnet.ipv4.conf.default.bc_forwarding=1\n"
	if err := files.WriteFileInChroot(chroot, "/etc/sysctl.d/bridging.conf", bridgingConf); err != nil {
		return fmt.Errorf("failed to write bridging sysctl: %w", err)
	}

	return nil
}
