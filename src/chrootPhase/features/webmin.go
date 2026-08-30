package features

import (
	"alpineInstaller/utils/config"
	"alpineInstaller/utils/shell"
	"fmt"
	"os"
	"strings"
)

// WebminPackages returns the APK packages needed for Webmin.
func WebminPackages(conf *config.Config) []string {
	if !conf.User.EnableWebmin {
		return nil
	}
	// Webmin runs in a Podman container — no APK packages needed on host
	return nil
}

// SetupWebmin installs and configures Webmin web administration panel.
// Webmin runs in a Podman container with minimal permissions.
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

	fmt.Println("[INFO] Installing Webmin via Podman container...")

	// Pull Webmin image
	fmt.Println("[INFO] Pulling Webmin Docker image...")
	if _, err := shell.ExecuteCmd(chroot, "podman pull docker.io/webmin/webmin:latest"); err != nil {
		return fmt.Errorf("failed to pull webmin image: %w", err)
	}

	// Create persistent volume for Webmin config and modules
	fmt.Println("[INFO] Creating Webmin persistent volume...")
	if _, err := shell.ExecuteCmd(chroot, "podman volume create webmin-data 2>/dev/null || true"); err != nil {
		fmt.Printf("[WARNING] Failed to create webmin-data volume: %v\n", err)
	}

	// Determine listen address
	listenAddr := "127.0.0.1"
	if conf.User.WebminListenAll {
		listenAddr = "0.0.0.0"
	}
	fmt.Printf("[INFO] Configuring Webmin to listen on %s:10000\n", listenAddr)

	// Stop and remove existing webmin container if any
	_, _ = shell.ExecuteCmd(chroot, "podman rm -f webmin 2>/dev/null")

	// Run Webmin container with minimal permissions and persistent volume
	fmt.Println("[INFO] Starting Webmin container...")
	runCmd := fmt.Sprintf(`podman run -d \
		--name webmin \
		--hostname feralos-webmin \
		--read-only \
		--cap-drop ALL \
		--cap-add SYS_CHROOT \
		--cap-add NET_BIND_SERVICE \
		--cap-add AUDIT_WRITE \
		--cap-add SETUID \
		--cap-add SETGID \
		--cap-add FOWNER \
		--cap-add DAC_OVERRIDE \
		--cap-add SYS_ADMIN \
		-p %s:10000 \
		-v webmin-data:/etc/webmin \
		-v /etc:/host/etc:ro \
		-v /var:/host/var:ro \
		-v /proc:/host/proc:ro \
		-v /sys:/host/sys:ro \
		-v /run:/host/run:ro \
		-v /boot:/host/boot:ro \
		-v /home:/host/home:rw \
		-v /root:/host/root:rw \
		-e WEBMIN_CONFIG=/etc/webmin \
		-e MINISERV_CONFIG=/etc/webmin/miniserv.conf \
		--restart unless-stopped \
		docker.io/webmin/webmin:latest`, listenAddr)
	if _, err := shell.ExecuteCmd(chroot, runCmd); err != nil {
		return fmt.Errorf("failed to start webmin container: %w", err)
	}

	// Install Webmin modules inside container
	if err := installWebminModules(conf); err != nil {
		fmt.Printf("[WARNING] Failed to install some Webmin modules: %v\n", err)
	}

	// Configure Webmin modules (inside container)
	if err := configureWebminModules(conf); err != nil {
		fmt.Printf("[WARNING] Failed to configure some Webmin modules: %v\n", err)
	}

	// Set root password for Webmin auth
	if _, err := shell.ExecuteCmd(chroot, fmt.Sprintf("podman exec webmin sh -c 'echo \"root:%s\" | chpasswd'", u.Password)); err != nil {
		fmt.Printf("[WARNING] Failed to set root password for Webmin: %v\n", err)
	}

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

// installWebminModules installs additional modules not bundled in Docker image.
func installWebminModules(conf *config.Config) error {
	chroot := conf.Internal.ChrootDir

	fmt.Println("[INFO] Installing Webmin modules...")

	// Custom Commands module — needed for btrbk/nix scripts
	fmt.Println("[INFO] Installing Custom Commands module...")
	downloadCmd := "curl -sSL https://download.webmin.com/download/modules/custom.wbm.gz -o /tmp/custom.wbm.gz"
	if _, err := shell.ExecuteCmd(chroot, downloadCmd); err != nil {
		return fmt.Errorf("failed to download custom module: %w", err)
	}

	// Extract and install module
	extractCmd := "cd /tmp && gunzip custom.wbm.gz 2>/dev/null; podman cp /tmp/custom.wbm webmin:/tmp/custom.wbm"
	if _, err := shell.ExecuteCmd(chroot, extractCmd); err != nil {
		return fmt.Errorf("failed to extract custom module: %w", err)
	}

	installCmd := "podman exec webmin /usr/share/webmin/install-module.pl /tmp/custom.wbm"
	if _, err := shell.ExecuteCmd(chroot, installCmd); err != nil {
		fmt.Printf("[WARNING] Failed to install custom module: %v\n", err)
	}

	// Clean up
	_, _ = shell.ExecuteCmdArgs(chroot, "rm", "-f", "/tmp/custom.wbm.gz", "/tmp/custom.wbm")

	return nil
}

// createWebminDesktopFile creates a .desktop file to launch Webmin as a Firefox PWA.
func createWebminDesktopFile(conf *config.Config) error {
	chroot := conf.Internal.ChrootDir
	u := conf.User

	webminUrl := "https://localhost:10000"

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

// writeContainerConfig writes content to a file inside the webmin Podman container.
// Uses temp file + podman cp to avoid shell quoting issues.
func writeContainerConfig(chroot, path, content string) error {
	const container = "webmin"

	tmp, err := os.CreateTemp("", "webmin-config-")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tmp.Name())

	if _, err := tmp.WriteString(content); err != nil {
		tmp.Close()
		return fmt.Errorf("failed to write temp file: %w", err)
	}
	tmp.Close()

	// podman cp needs container:path — mkdir parent first
	dir := path[:strings.LastIndex(path, "/")]
	if _, err := shell.ExecuteCmd(chroot, fmt.Sprintf("podman exec %s mkdir -p %s", container, dir)); err != nil {
		return fmt.Errorf("failed to create dir %s: %w", dir, err)
	}
	if _, err := shell.ExecuteCmd(chroot, fmt.Sprintf("podman cp %s %s:%s", tmp.Name(), container, path)); err != nil {
		return fmt.Errorf("failed to copy config to %s:%s: %w", container, path, err)
	}
	return nil
}

func configureWebminModules(conf *config.Config) error {
	chroot := conf.Internal.ChrootDir

	// FirewallD module — configure to use firewall-cmd
	fmt.Println("[INFO] Configuring Webmin FirewallD module...")
	firewallCmdConfig := "firewall_cmd=/usr/sbin/firewall-cmd\nfirewall_type=firewalld\n"
	if err := writeContainerConfig(chroot, "/etc/webmin/firewalld/config", firewallCmdConfig); err != nil {
		return fmt.Errorf("failed to write firewalld config: %w", err)
	}

	// Software Packages module — configure for apk
	fmt.Println("[INFO] Configuring Webmin Software Packages module for apk...")
	packagesConfig := "package_type=apk\nupdate_command=apk update\ninstall_command=apk add\nremove_command=apk del\nsearch_command=apk search\n"
	if err := writeContainerConfig(chroot, "/etc/webmin/software/config", packagesConfig); err != nil {
		return fmt.Errorf("failed to write software config: %w", err)
	}

	// Custom Commands module — pre-configure for btrbk and nix
	fmt.Println("[INFO] Configuring Webmin Custom Commands module...")
	customConfig := "commands_dir=/etc/webmin/custom/commands\n"
	if err := writeContainerConfig(chroot, "/etc/webmin/custom/config", customConfig); err != nil {
		return fmt.Errorf("failed to write custom config: %w", err)
	}

	// SMART Status module
	fmt.Println("[INFO] Configuring Webmin SMART Status module...")
	smartConfig := "smartctl_command=/usr/sbin/smartctl\nsmartd_conf=/etc/smartd.conf\n"
	if err := writeContainerConfig(chroot, "/etc/webmin/smart-status/config", smartConfig); err != nil {
		return fmt.Errorf("failed to write smart-status config: %w", err)
	}

	// Process Monitoring module
	fmt.Println("[INFO] Configuring Webmin Process Monitoring module...")
	procConfig := "ps_command=/bin/ps\nkill_command=/bin/kill\nrenice_command=/usr/bin/renice\n"
	if err := writeContainerConfig(chroot, "/etc/webmin/proc/config", procConfig); err != nil {
		return fmt.Errorf("failed to write proc config: %w", err)
	}

	// Cron Jobs module
	fmt.Println("[INFO] Configuring Webmin Cron Jobs module...")
	cronConfig := "cron_dir=/etc/crontabs\ncron_command=/usr/bin/crontab\n"
	if err := writeContainerConfig(chroot, "/etc/webmin/cron/config", cronConfig); err != nil {
		return fmt.Errorf("failed to write cron config: %w", err)
	}

	// Log Viewer module
	fmt.Println("[INFO] Configuring Webmin Log Viewer module...")
	logConfig := "log_files=/var/log/messages\nlog_files=/var/log/syslog\nlog_files=/var/log/auth.log\nlog_command=/usr/bin/tail\n"
	if err := writeContainerConfig(chroot, "/etc/webmin/logviewer/config", logConfig); err != nil {
		return fmt.Errorf("failed to write logviewer config: %w", err)
	}

	return nil
}
