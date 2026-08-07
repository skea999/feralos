package hostPhase

import (
	"alpineInstaller/utils/config"
	"alpineInstaller/utils/files"
	"alpineInstaller/utils/shell"
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

	// 9. Service Configs (LXC)
	fmt.Println("[DEBUG] Writing LXC service configuration")
	if err := files.WriteFileInChroot(chroot, "/etc/conf.d/lxc", "systemd_container=yes\n"); err != nil {
		return fmt.Errorf("failed to write LXC config: %w", err)
	}

	// 10. Bridging sysctl (for QEMU/Incus)
	fmt.Println("[DEBUG] Writing bridging sysctl")
	bridgingConf := "net.ipv4.conf.all.bc_forwarding=1\nnet.ipv4.conf.default.bc_forwarding=1\n"
	if err := files.WriteFileInChroot(chroot, "/etc/sysctl.d/bridging.conf", bridgingConf); err != nil {
		return fmt.Errorf("failed to write bridging sysctl: %w", err)
	}

	// 11. Vector Logging Configuration
	fmt.Println("[DEBUG] Writing Vector logging configuration")
	vectorConfig := `# Vector configuration for FeralOS
# Replaces rsyslog + logrotate with memory-safe Rust alternative

data_dir: "/var/lib/vector"

sources:
  syslog:
    type: socket
    mode: unix_datagram
    path: /dev/log

transforms:
  syslog_parser:
    type: remap
    inputs:
      - syslog
    source: |
      structured = parse_syslog(.message) ?? {}
      . = structured

  auth_logs:
    type: filter
    inputs:
      - syslog_parser
    condition: '.facility == "auth" || .facility == "authpriv"'

  daemon_logs:
    type: filter
    inputs:
      - syslog_parser
    condition: '.facility == "daemon"'

  kernel_logs:
    type: filter
    inputs:
      - syslog_parser
    condition: '.facility == "kern"'

  syslog_logs:
    type: filter
    inputs:
      - syslog_parser
    condition: '.facility == "syslog"'

sinks:
  messages:
    type: file
    inputs:
      - syslog_parser
    path: "/var/log/messages"
    encoding:
      codec: text

  auth_log:
    type: file
    inputs:
      - auth_logs
    path: "/var/log/auth.log"
    encoding:
      codec: text

  daemon_log:
    type: file
    inputs:
      - daemon_logs
    path: "/var/log/daemon.log"
    encoding:
      codec: text

  kernel_log:
    type: file
    inputs:
      - kernel_logs
    path: "/var/log/kern.log"
    encoding:
      codec: text

  syslog_log:
    type: file
    inputs:
      - syslog_logs
    path: "/var/log/syslog"
    encoding:
      codec: text
`
	if err := files.WriteFileInChroot(chroot, "/etc/vector/vector.yaml", vectorConfig); err != nil {
		return fmt.Errorf("failed to write vector config: %w", err)
	}

	// 12. Btrbk Configuration (btrfs backup)
	fmt.Println("[DEBUG] Writing btrbk configuration")
	btrbkConfig := `# btrbk configuration for FeralOS
# Snapshot management for btrfs subvolumes

snapshot_dir  /mnt/btrbk/snapshots
subvolume     /
# Include all subvolumes for snapshots
incremental   yes
compact       yes
# Keep 7 daily, 4 weekly, 3 monthly snapshots
keep_daily    7
keep_weekly   4
keep_monthly  3
# Run as root
run_as        root
# Timestamp format
timestamp_format long
# Use ctime for comparison
compare       path
# Don't send to remote (local only)
stream_buffer none
# Log file
logdir        /var/log/btrbk
`
	if err := files.WriteFileInChroot(chroot, "/etc/btrbk/btrbk.conf", btrbkConfig); err != nil {
		return fmt.Errorf("failed to write btrbk config: %w", err)
	}

	// Create btrbk snapshot directory
	if _, err := shell.RunChrootCommand(chroot, "mkdir -p /mnt/btrbk/snapshots"); err != nil {
		fmt.Printf("[WARNING] Failed to create btrbk snapshot directory: %v\n", err)
	}

	// Create btrbk log directory
	if _, err := shell.RunChrootCommand(chroot, "mkdir -p /var/log/btrbk"); err != nil {
		fmt.Printf("[WARNING] Failed to create btrbk log directory: %v\n", err)
	}

	return nil
}
