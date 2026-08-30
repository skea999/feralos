package features

import (
	"alpineInstaller/utils/config"
	"alpineInstaller/utils/shell"
	"fmt"
)

// WebminPackages returns the APK packages needed for Webmin.
func WebminPackages(conf *config.Config) []string {
	if !conf.User.EnableWebmin {
		return nil
	}
	// Webmin is installed via setup.sh, but we need basic dependencies
	return []string{"perl", "perl-io-socket-ssl", "net-tools", "procps", "cronie"}
}

// SetupWebmin installs and configures Webmin web administration panel.
// Webmin requires Podman (for container management features).
func SetupWebmin(conf *config.Config) error {
	if !conf.User.EnableWebmin {
		fmt.Println("[INFO] Webmin disabled, skipping...")
		return nil
	}

	// Webmin requires Podman
	if !conf.User.EnablePodman {
		fmt.Println("[INFO] Webmin requires Podman — enabling automatically...")
		conf.User.EnablePodman = true
	}

	chroot := conf.Internal.ChrootDir
	u := conf.User

	fmt.Println("[INFO] Installing and configuring Webmin...")

	// Download and install Webmin setup.sh
	fmt.Println("[INFO] Downloading Webmin setup.sh...")
	if _, err := shell.ExecuteCmd(chroot, "curl -sSL https://raw.githubusercontent.com/webmin/webmin/master/setup.sh -o /tmp/webmin-setup.sh"); err != nil {
		return fmt.Errorf("failed to download webmin setup.sh: %w", err)
	}

	// Run setup.sh in unattended mode
	fmt.Println("[INFO] Running Webmin setup.sh...")
	if _, err := shell.ExecuteCmd(chroot, "sh /tmp/webmin-setup.sh --unattended"); err != nil {
		return fmt.Errorf("failed to run webmin setup.sh: %w", err)
	}

	// Configure listen address
	listenAddr := "127.0.0.1"
	if conf.User.WebminListenAll {
		listenAddr = "0.0.0.0"
	}
	fmt.Printf("[INFO] Configuring Webmin to listen on %s:10000\n", listenAddr)

	// Update miniserv.conf listen address
	listenLine := fmt.Sprintf("sed -i 's/^bind=.*/bind=%s/' /etc/webmin/miniserv.conf", listenAddr)
	if _, err := shell.ExecuteCmd(chroot, listenLine); err != nil {
		fmt.Printf("[WARNING] Failed to set Webmin listen address: %v\n", err)
	}

	// Enable webmin service
	if _, err := shell.ExecuteCmdArgs(chroot, "rc-update", "add", "webmin", "default"); err != nil {
		fmt.Printf("[WARNING] Failed to add webmin to default runlevel: %v\n", err)
	}

	// Configure Webmin modules
	if err := configureWebminModules(conf); err != nil {
		fmt.Printf("[WARNING] Failed to configure some Webmin modules: %v\n", err)
	}

	// Set root password for Webmin auth
	if _, err := shell.ExecuteCmd(chroot, fmt.Sprintf("echo 'root:%s' | chpasswd", u.Password)); err != nil {
		fmt.Printf("[WARNING] Failed to set root password for Webmin: %v\n", err)
	}

	// Clean up setup script
	_, _ = shell.ExecuteCmdArgs(chroot, "rm", "-f", "/tmp/webmin-setup.sh")

	listenInfo := "localhost:10000"
	if conf.User.WebminListenAll {
		listenInfo = "0.0.0.0:10000 (all interfaces)"
	}
	fmt.Printf("[INFO] Webmin configured. Access at https://%s\n", listenInfo)
	fmt.Println("[INFO] Enabled modules: FirewallD, Software Packages (apk), Disk/Filesystems, Custom Commands, SMART Status, Process Monitoring, Cron Jobs, Log Viewer")

	// Create .desktop file for Webmin PWA
	if err := createWebminDesktopFile(conf); err != nil {
		fmt.Printf("[WARNING] Failed to create Webmin .desktop file: %v\n", err)
	}

	return nil
}

// createWebminDesktopFile creates a .desktop file to launch Webmin as a Firefox PWA.
func createWebminDesktopFile(conf *config.Config) error {
	chroot := conf.Internal.ChrootDir
	u := conf.User

	webminUrl := "https://localhost:10000"
	if conf.User.WebminListenAll {
		webminUrl = "https://localhost:10000"
	}

	desktopContent := fmt.Sprintf(`[Desktop Entry]
Name=Webmin
Comment=Web-based system administration panel
Exec=firefox --ssb %s
Icon=preferences-system
Terminal=false
Type=Application
Categories=System;
StartupNotify=true
`, webminUrl)

	desktopDir := fmt.Sprintf("/home/%s/.local/share/applications", u.UserName)
	if _, err := shell.ExecuteCmdArgs(chroot, "mkdir", "-p", desktopDir); err != nil {
		return fmt.Errorf("failed to create applications dir: %w", err)
	}

	desktopFile := fmt.Sprintf("%s/webmin.desktop", desktopDir)
	if _, err := shell.ExecuteCmd(chroot, fmt.Sprintf("cat > %s << 'DESKTOP'\n%s\nDESKTOP", desktopFile, desktopContent)); err != nil {
		return fmt.Errorf("failed to write webmin.desktop: %w", err)
	}

	// Set ownership
	if _, err := shell.ExecuteCmd(chroot, fmt.Sprintf("chown %s:%s %s", u.UserName, u.UserName, desktopFile)); err != nil {
		fmt.Printf("[WARNING] Failed to set .desktop file ownership: %v\n", err)
	}

	fmt.Println("[INFO] Webmin .desktop file created for Firefox PWA")
	return nil
}

// configureWebminModules enables and configures the built-in Webmin modules.
func configureWebminModules(conf *config.Config) error {
	chroot := conf.Internal.ChrootDir

	// FirewallD module — configure to use firewall-cmd
	fmt.Println("[INFO] Configuring Webmin FirewallD module...")
	firewalldModuleDir := "/etc/webmin/firewalld"
	if _, err := shell.ExecuteCmdArgs(chroot, "mkdir", "-p", firewalldModuleDir); err != nil {
		return fmt.Errorf("failed to create firewalld module dir: %w", err)
	}

	firewallCmdConfig := "firewall_cmd=/usr/sbin/firewall-cmd\nfirewall_type=firewalld\n"
	if _, err := shell.ExecuteCmd(chroot, fmt.Sprintf("echo '%s' > %s/config", firewallCmdConfig, firewalldModuleDir)); err != nil {
		return fmt.Errorf("failed to write firewalld config: %w", err)
	}

	// Software Packages module — configure for apk
	fmt.Println("[INFO] Configuring Webmin Software Packages module for apk...")
	packagesModuleDir := "/etc/webmin/software"
	if _, err := shell.ExecuteCmdArgs(chroot, "mkdir", "-p", packagesModuleDir); err != nil {
		return fmt.Errorf("failed to create software module dir: %w", err)
	}

	packagesConfig := "package_type=apk\nupdate_command=apk update\ninstall_command=apk add\nremove_command=apk del\nsearch_command=apk search\n"
	if _, err := shell.ExecuteCmd(chroot, fmt.Sprintf("echo '%s' > %s/config", packagesConfig, packagesModuleDir)); err != nil {
		return fmt.Errorf("failed to write software config: %w", err)
	}

	// Disk and Network Filesystems module — configure for btrfs
	fmt.Println("[INFO] Configuring Webmin Disk/Filesystems module for btrfs...")
	mountModuleDir := "/etc/webmin/mount"
	if _, err := shell.ExecuteCmdArgs(chroot, "mkdir", "-p", mountModuleDir); err != nil {
		return fmt.Errorf("failed to create mount module dir: %w", err)
	}

	mountConfig := "fstab_file=/etc/fstab\nmount_command=/bin/mount\numount_command=/bin/umount\n"
	if _, err := shell.ExecuteCmd(chroot, fmt.Sprintf("echo '%s' > %s/config", mountConfig, mountModuleDir)); err != nil {
		return fmt.Errorf("failed to write mount config: %w", err)
	}

	// Custom Commands module — pre-configure for btrbk and nix
	fmt.Println("[INFO] Configuring Webmin Custom Commands module...")
	customModuleDir := "/etc/webmin/custom"
	if _, err := shell.ExecuteCmdArgs(chroot, "mkdir", "-p", customModuleDir); err != nil {
		return fmt.Errorf("failed to create custom module dir: %w", err)
	}

	// Create custom commands directory for user scripts
	if _, err := shell.ExecuteCmdArgs(chroot, "mkdir", "-p", "/etc/webmin/custom/commands"); err != nil {
		fmt.Printf("[WARNING] Failed to create custom commands dir: %v\n", err)
	}

	customConfig := "commands_dir=/etc/webmin/custom/commands\n"
	if _, err := shell.ExecuteCmd(chroot, fmt.Sprintf("echo '%s' > %s/config", customConfig, customModuleDir)); err != nil {
		return fmt.Errorf("failed to write custom config: %w", err)
	}

	// SMART Status module — configure for disk health monitoring
	fmt.Println("[INFO] Configuring Webmin SMART Status module...")
	smartModuleDir := "/etc/webmin/smart-status"
	if _, err := shell.ExecuteCmdArgs(chroot, "mkdir", "-p", smartModuleDir); err != nil {
		return fmt.Errorf("failed to create smart-status module dir: %w", err)
	}

	smartConfig := "smartctl_command=/usr/sbin/smartctl\nsmartd_conf=/etc/smartd.conf\n"
	if _, err := shell.ExecuteCmd(chroot, fmt.Sprintf("echo '%s' > %s/config", smartConfig, smartModuleDir)); err != nil {
		return fmt.Errorf("failed to write smart-status config: %w", err)
	}

	// Process Monitoring module — configure for real-time process view
	fmt.Println("[INFO] Configuring Webmin Process Monitoring module...")
	procModuleDir := "/etc/webmin/proc"
	if _, err := shell.ExecuteCmdArgs(chroot, "mkdir", "-p", procModuleDir); err != nil {
		return fmt.Errorf("failed to create proc module dir: %w", err)
	}

	procConfig := "ps_command=/bin/ps\nkill_command=/bin/kill\nrenice_command=/usr/bin/renice\n"
	if _, err := shell.ExecuteCmd(chroot, fmt.Sprintf("echo '%s' > %s/config", procConfig, procModuleDir)); err != nil {
		return fmt.Errorf("failed to write proc config: %w", err)
	}

	// Cron Jobs module — configure for scheduled tasks
	fmt.Println("[INFO] Configuring Webmin Cron Jobs module...")
	cronModuleDir := "/etc/webmin/cron"
	if _, err := shell.ExecuteCmdArgs(chroot, "mkdir", "-p", cronModuleDir); err != nil {
		return fmt.Errorf("failed to create cron module dir: %w", err)
	}

	cronConfig := "cron_dir=/etc/crontabs\ncron_command=/usr/bin/crontab\n"
	if _, err := shell.ExecuteCmd(chroot, fmt.Sprintf("echo '%s' > %s/config", cronConfig, cronModuleDir)); err != nil {
		return fmt.Errorf("failed to write cron config: %w", err)
	}

	// Log Viewer module — configure for system log viewing
	fmt.Println("[INFO] Configuring Webmin Log Viewer module...")
	logModuleDir := "/etc/webmin/logviewer"
	if _, err := shell.ExecuteCmdArgs(chroot, "mkdir", "-p", logModuleDir); err != nil {
		return fmt.Errorf("failed to create logviewer module dir: %w", err)
	}

	logConfig := "log_files=/var/log/messages\nlog_files=/var/log/syslog\nlog_files=/var/log/auth.log\nlog_command=/usr/bin/tail\n"
	if _, err := shell.ExecuteCmd(chroot, fmt.Sprintf("echo '%s' > %s/config", logConfig, logModuleDir)); err != nil {
		return fmt.Errorf("failed to write logviewer config: %w", err)
	}

	// Enable modules in Webmin config
	modulesConfig := "/etc/webmin/miniserv.conf"
	enableModule := fmt.Sprintf("sed -i 's|^#.*firewalld|firewalld|' %s", modulesConfig)
	if _, err := shell.ExecuteCmd(chroot, enableModule); err != nil {
		fmt.Printf("[WARNING] Failed to enable firewalld module: %v\n", err)
	}

	return nil
}
