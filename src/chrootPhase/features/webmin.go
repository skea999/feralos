package features

import (
	"alpineInstaller/utils/config"
	"alpineInstaller/utils/shell"
	"fmt"
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
// Webmin runs in a Podman container with minimal permissions, managed via compose.
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

	// Create /opt/webmin directory structure
	fmt.Println("[INFO] Creating /opt/webmin directory structure...")
	dirs := []string{"/opt/webmin", "/opt/webmin/config", "/opt/webmin/modules", "/opt/webmin/commands"}
	for _, dir := range dirs {
		if _, err := shell.ExecuteCmdArgs(chroot, "mkdir", "-p", dir); err != nil {
			return fmt.Errorf("failed to create %s: %w", dir, err)
		}
	}

	// Determine listen address
	listenAddr := "127.0.0.1"
	if conf.User.WebminListenAll {
		listenAddr = "0.0.0.0"
	}
	fmt.Printf("[INFO] Configuring Webmin to listen on %s:10000\n", listenAddr)

	// Generate docker-compose.yml
	if err := generateComposeFile(conf, listenAddr); err != nil {
		return fmt.Errorf("failed to generate compose file: %w", err)
	}

	// Generate module configurations
	generateModuleConfigs(conf)

	// Stop and remove existing webmin container if any
	_, _ = shell.ExecuteCmd(chroot, "podman rm -f webmin 2>/dev/null")

	// Start with podman compose
	fmt.Println("[INFO] Starting Webmin with podman compose...")
	if _, err := shell.ExecuteCmd(chroot, "cd /opt/webmin && podman compose up -d"); err != nil {
		// Fallback to podman-compose if podman compose not available
		fmt.Println("[INFO] Trying podman-compose...")
		if _, err2 := shell.ExecuteCmd(chroot, "cd /opt/webmin && podman-compose up -d"); err2 != nil {
			return fmt.Errorf("failed to start webmin: %w (also tried podman-compose: %w)", err, err2)
		}
	}

	// Create oneshot service to ensure Webmin is running
	if err := createWebminService(conf); err != nil {
		fmt.Printf("[WARNING] Failed to create webmin service: %v\n", err)
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
	fmt.Printf("[INFO] Config: /opt/webmin/docker-compose.yml\n")
	fmt.Printf("[INFO] Modules: /opt/webmin/modules/\n")
	fmt.Printf("[INFO] Commands: /opt/webmin/commands/\n")
	fmt.Println("[INFO] Enabled modules: FirewallD, Software Packages (apk), Disk/Filesystems, Custom Commands, SMART Status, Process Monitoring, Cron Jobs, Log Viewer")

	// Create .desktop file for Webmin PWA
	if err := createWebminDesktopFile(conf); err != nil {
		fmt.Printf("[WARNING] Failed to create Webmin .desktop file: %v\n", err)
	}

	return nil
}

// createWebminService creates an OpenRC oneshot service to ensure Webmin is running at boot.
func createWebminService(conf *config.Config) error {
	chroot := conf.Internal.ChrootDir

	serviceScript := `#!/sbin/openrc-run

description="Webmin Podman Container - oneshot ensure running"

depend() {
    need net
    after podman
}

start() {
    ebegin "Checking Webmin container"
    if podman ps --format '{{.Names}}' | grep -q '^webmin$'; then
        einfo "Webmin already running"
        eend 0
    else
        einfo "Starting Webmin container"
        cd /opt/webmin && podman compose up -d
        eend $?
    fi
}
`

	serviceFile := "/etc/init.d/webmin-ensure"

	// Write service script directly using shell.ExecuteCmd
	if _, err := shell.ExecuteCmd(chroot, fmt.Sprintf("cat > %s << 'SERVICE'\n%s\nSERVICE", serviceFile, serviceScript)); err != nil {
		return fmt.Errorf("failed to install webmin service: %w", err)
	}
	if _, err := shell.ExecuteCmd(chroot, fmt.Sprintf("chmod 755 %s", serviceFile)); err != nil {
		return fmt.Errorf("failed to chmod webmin service: %w", err)
	}

	// Enable the service
	if _, err := shell.ExecuteCmdArgs(chroot, "rc-update", "add", "webmin-ensure", "default"); err != nil {
		fmt.Printf("[WARNING] Failed to add webmin-ensure to default runlevel: %v\n", err)
	}

	fmt.Println("[INFO] Webmin oneshot service created: /etc/init.d/webmin-ensure")
	return nil
}

// generateComposeFile creates the docker-compose.yml for Webmin.
func generateComposeFile(conf *config.Config, listenAddr string) error {
	composeContent := fmt.Sprintf(`services:
  webmin:
    image: docker.io/webmin/webmin:latest
    container_name: webmin
    hostname: feralos-webmin
    restart: unless-stopped
    read_only: true
    cap_drop:
      - ALL
    cap_add:
      - SYS_CHROOT
      - NET_BIND_SERVICE
      - AUDIT_WRITE
      - SETUID
      - SETGID
      - FOWNER
      - DAC_OVERRIDE
      - SYS_ADMIN
    ports:
      - "%s:10000"
    volumes:
      - /opt/webmin/config:/etc/webmin
      - /opt/webmin/commands:/etc/webmin/custom/commands
      - /etc:/host/etc:ro
      - /var:/host/var:ro
      - /proc:/host/proc:ro
      - /sys:/host/sys:ro
      - /run:/host/run:ro
      - /boot:/host/boot:ro
      - /home:/host/home:rw
      - /root:/host/root:rw
    environment:
      - WEBMIN_CONFIG=/etc/webmin
      - MINISERV_CONFIG=/etc/webmin/miniserv.conf
`, listenAddr)

	chroot := conf.Internal.ChrootDir
	composeFile := "/opt/webmin/docker-compose.yml"

	// Write directly using cat heredoc
	escaped := strings.ReplaceAll(composeContent, "'", "'\\''")
	if _, err := shell.ExecuteCmd(chroot, fmt.Sprintf("cat > %s << 'COMPOSE'\n%s\nCOMPOSE", composeFile, escaped)); err != nil {
		return fmt.Errorf("failed to write compose file: %w", err)
	}

	// Set ownership
	if _, err := shell.ExecuteCmd(chroot, "chown root:root /opt/webmin/docker-compose.yml"); err != nil {
		fmt.Printf("[WARNING] Failed to set compose file ownership: %v\n", err)
	}

	fmt.Println("[INFO] Generated /opt/webmin/docker-compose.yml")
	return nil
}

// generateModuleConfigs creates module configuration files in /opt/webmin/config/.
func generateModuleConfigs(conf *config.Config) {
	chroot := conf.Internal.ChrootDir

	configs := map[string]string{
		"/opt/webmin/config/firewalld/config":    "firewall_cmd=/usr/sbin/firewall-cmd\nfirewall_type=firewalld\n",
		"/opt/webmin/config/software/config":     "package_type=apk\nupdate_command=apk update\ninstall_command=apk add\nremove_command=apk del\nsearch_command=apk search\n",
		"/opt/webmin/config/custom/config":       "commands_dir=/etc/webmin/custom/commands\n",
		"/opt/webmin/config/smart-status/config": "smartctl_command=/usr/sbin/smartctl\nsmartd_conf=/etc/smartd.conf\n",
		"/opt/webmin/config/proc/config":         "ps_command=/bin/ps\nkill_command=/bin/kill\nrenice_command=/usr/bin/renice\n",
		"/opt/webmin/config/cron/config":         "cron_dir=/etc/crontabs\ncron_command=/usr/bin/crontab\n",
		"/opt/webmin/config/logviewer/config":    "log_files=/var/log/messages\nlog_files=/var/log/syslog\nlog_files=/var/log/auth.log\nlog_command=/usr/bin/tail\n",
	}

	for path, content := range configs {
		if err := writeHostFile(chroot, path, content); err != nil {
			fmt.Printf("[WARNING] Failed to write %s: %v\n", path, err)
		}
	}

	fmt.Println("[INFO] Generated module configs in /opt/webmin/config/")
}

// writeHostFile writes content to a file on the host filesystem using shell.ExecuteCmd.
func writeHostFile(chroot, path, content string) error {
	dir := path[:strings.LastIndex(path, "/")]
	if _, err := shell.ExecuteCmdArgs(chroot, "mkdir", "-p", dir); err != nil {
		return fmt.Errorf("failed to create dir %s: %w", dir, err)
	}
	escaped := strings.ReplaceAll(content, "'", "'\\''")
	if _, err := shell.ExecuteCmd(chroot, fmt.Sprintf("echo '%s' > %s", escaped, path)); err != nil {
		return fmt.Errorf("failed to write to %s: %w", path, err)
	}
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

	if _, err := shell.ExecuteCmd(chroot, fmt.Sprintf("chown %s:%s %s", u.UserName, u.UserName, desktopFile)); err != nil {
		fmt.Printf("[WARNING] Failed to set .desktop file ownership: %v\n", err)
	}

	fmt.Println("[INFO] Webmin .desktop file created for Firefox PWA")
	return nil
}

// installWebminModules installs additional modules not bundled in Docker image.
func installWebminModules(conf *config.Config) error {
	chroot := conf.Internal.ChrootDir

	fmt.Println("[INFO] Installing Webmin modules...")

	// Custom Commands module
	fmt.Println("[INFO] Installing Custom Commands module...")
	downloadCmd := "curl -sSL https://download.webmin.com/download/modules/custom.wbm.gz -o /tmp/custom.wbm.gz"
	if _, err := shell.ExecuteCmd(chroot, downloadCmd); err != nil {
		return fmt.Errorf("failed to download custom module: %w", err)
	}

	extractCmd := "cd /tmp && gunzip custom.wbm.gz 2>/dev/null; podman cp /tmp/custom.wbm webmin:/tmp/custom.wbm"
	if _, err := shell.ExecuteCmd(chroot, extractCmd); err != nil {
		return fmt.Errorf("failed to extract custom module: %w", err)
	}

	installCmd := "podman exec webmin /usr/share/webmin/install-module.pl /tmp/custom.wbm"
	if _, err := shell.ExecuteCmd(chroot, installCmd); err != nil {
		fmt.Printf("[WARNING] Failed to install custom module: %v\n", err)
	}

	_, _ = shell.ExecuteCmdArgs(chroot, "rm", "-f", "/tmp/custom.wbm.gz", "/tmp/custom.wbm")

	// Docker module (works with Podman via symlink)
	fmt.Println("[INFO] Installing Docker module for Podman...")
	downloadDocker := "curl -sSL https://github.com/dave-lang/webmin-docker/releases/latest/download/docker.wbm.gz -o /tmp/docker.wbm.gz"
	if _, err := shell.ExecuteCmd(chroot, downloadDocker); err != nil {
		return fmt.Errorf("failed to download docker module: %w", err)
	}

	extractDocker := "cd /tmp && gunzip docker.wbm.gz 2>/dev/null; podman cp /tmp/docker.wbm webmin:/tmp/docker.wbm"
	if _, err := shell.ExecuteCmd(chroot, extractDocker); err != nil {
		return fmt.Errorf("failed to extract docker module: %w", err)
	}

	installDocker := "podman exec webmin /usr/share/webmin/install-module.pl /tmp/docker.wbm"
	if _, err := shell.ExecuteCmd(chroot, installDocker); err != nil {
		fmt.Printf("[WARNING] Failed to install docker module: %v\n", err)
	}

	_, _ = shell.ExecuteCmdArgs(chroot, "rm", "-f", "/tmp/docker.wbm.gz", "/tmp/docker.wbm")

	return nil
}

// configureWebminModules configures the built-in Webmin modules inside the container.
func configureWebminModules(conf *config.Config) error {
	chroot := conf.Internal.ChrootDir

	fmt.Println("[INFO] Configuring Webmin modules...")

	// FirewallD module
	fmt.Println("[INFO] Configuring FirewallD module...")
	firewallCmdConfig := "firewall_cmd=/usr/sbin/firewall-cmd\nfirewall_type=firewalld\n"
	if err := writeContainerConfig(chroot, "/etc/webmin/firewalld/config", firewallCmdConfig); err != nil {
		return fmt.Errorf("failed to write firewalld config: %w", err)
	}

	// Software Packages module
	fmt.Println("[INFO] Configuring Software Packages module for apk...")
	packagesConfig := "package_type=apk\nupdate_command=apk update\ninstall_command=apk add\nremove_command=apk del\nsearch_command=apk search\n"
	if err := writeContainerConfig(chroot, "/etc/webmin/software/config", packagesConfig); err != nil {
		return fmt.Errorf("failed to write software config: %w", err)
	}

	// Custom Commands module
	fmt.Println("[INFO] Configuring Custom Commands module...")
	customConfig := "commands_dir=/etc/webmin/custom/commands\n"
	if err := writeContainerConfig(chroot, "/etc/webmin/custom/config", customConfig); err != nil {
		return fmt.Errorf("failed to write custom config: %w", err)
	}

	// SMART Status module
	fmt.Println("[INFO] Configuring SMART Status module...")
	smartConfig := "smartctl_command=/usr/sbin/smartctl\nsmartd_conf=/etc/smartd.conf\n"
	if err := writeContainerConfig(chroot, "/etc/webmin/smart-status/config", smartConfig); err != nil {
		return fmt.Errorf("failed to write smart-status config: %w", err)
	}

	// Process Monitoring module
	fmt.Println("[INFO] Configuring Process Monitoring module...")
	procConfig := "ps_command=/bin/ps\nkill_command=/bin/kill\nrenice_command=/usr/bin/renice\n"
	if err := writeContainerConfig(chroot, "/etc/webmin/proc/config", procConfig); err != nil {
		return fmt.Errorf("failed to write proc config: %w", err)
	}

	// Cron Jobs module
	fmt.Println("[INFO] Configuring Cron Jobs module...")
	cronConfig := "cron_dir=/etc/crontabs\ncron_command=/usr/bin/crontab\n"
	if err := writeContainerConfig(chroot, "/etc/webmin/cron/config", cronConfig); err != nil {
		return fmt.Errorf("failed to write cron config: %w", err)
	}

	// Log Viewer module
	fmt.Println("[INFO] Configuring Log Viewer module...")
	logConfig := "log_files=/var/log/messages\nlog_files=/var/log/syslog\nlog_files=/var/log/auth.log\nlog_command=/usr/bin/tail\n"
	if err := writeContainerConfig(chroot, "/etc/webmin/logviewer/config", logConfig); err != nil {
		return fmt.Errorf("failed to write logviewer config: %w", err)
	}

	return nil
}

// writeContainerConfig writes content to a file inside the webmin Podman container.
// Uses podman exec + echo to write directly inside the container.
func writeContainerConfig(chroot, path, content string) error {
	const container = "webmin"

	// Create parent directory in container
	dir := path[:strings.LastIndex(path, "/")]
	if _, err := shell.ExecuteCmd(chroot, fmt.Sprintf("podman exec %s mkdir -p %s", container, dir)); err != nil {
		return fmt.Errorf("failed to create dir %s: %w", dir, err)
	}

	// Write content directly inside container using echo
	escaped := strings.ReplaceAll(content, "'", "'\\''")
	if _, err := shell.ExecuteCmd(chroot, fmt.Sprintf("podman exec %s sh -c 'echo \"%s\" > %s'", container, escaped, path)); err != nil {
		return fmt.Errorf("failed to write %s: %w", path, err)
	}
	return nil
}
