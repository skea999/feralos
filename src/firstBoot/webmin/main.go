package main

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"

	"alpineInstaller/utils/shell"
)

type InstallConfig struct {
	DiskLayout     string   `yaml:"disk_layout"`
	Disks          []string `yaml:"disks"`
	LuksUUIDs      []string `yaml:"luks_uuids"`
	UserName       string   `yaml:"user_name"`
	Password       string   `yaml:"password"`
	Hostname       string   `yaml:"hostname"`
	Mirror         string   `yaml:"mirror"`
	Branch         string   `yaml:"branch"`
	EnablePodman   bool     `yaml:"enable_podman"`
	EnableWebmin   bool     `yaml:"enable_webmin"`
	WebminListenAll bool    `yaml:"webmin_listen_all"`
}

func loadConfig() *InstallConfig {
	cfg := &InstallConfig{}
	data, err := os.ReadFile("/opt/feralos/config.yaml")
	if err != nil {
		return cfg
	}
	_ = yaml.Unmarshal(data, cfg)
	return cfg
}

func main() {
	shell.SetCaller("webmin")
	fmt.Println("=== FeralOS — Webmin Setup ===")

	cfg := loadConfig()

	if !cfg.EnableWebmin {
		fmt.Println("[INFO] Webmin disabled, skipping...")
		return
	}

	// Podman is already installed and configured in chrootPhase
	// Just verify it's working
	fmt.Println("[INFO] Verifying Podman...")
	if _, err := shell.ExecuteCmd("", "podman --version"); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Podman not available: %v\n", err)
		os.Exit(1)
	}

	// --- Install Webmin ---
	setupWebmin(cfg)

	fmt.Println("\n=== Webmin Setup Complete ===")
	fmt.Printf("Access: https://localhost:10000\n")
	fmt.Printf("User: root\n")
	fmt.Printf("Config: /opt/webmin/docker-compose.yml\n")
}

func setupWebmin(cfg *InstallConfig) {
	fmt.Println("\n[2/2] Installing Webmin...")

	// Create /opt/webmin directory structure
	dirs := []string{"/opt/webmin", "/opt/webmin/config", "/opt/webmin/modules", "/opt/webmin/commands"}
	for _, dir := range dirs {
		if _, err := shell.ExecuteCmd("", fmt.Sprintf("mkdir -p %s", dir)); err != nil {
			fmt.Printf("[WARNING] Failed to create %s: %v\n", dir, err)
		}
	}

	// Determine listen address
	listenAddr := "127.0.0.1"
	if cfg.WebminListenAll {
		listenAddr = "0.0.0.0"
	}
	fmt.Printf("[INFO] Configuring Webmin to listen on %s:10000\n", listenAddr)

	// Generate docker-compose.yml
	generateComposeFile(listenAddr)

	// Generate module configurations
	generateModuleConfigs()

	// Stop and remove existing webmin container if any
	_, _ = shell.ExecuteCmd("", "podman rm -f webmin 2>/dev/null")

	// Start with podman compose
	fmt.Println("[INFO] Starting Webmin with podman compose...")
	if _, err := shell.ExecuteCmd("", "cd /opt/webmin && podman compose up -d"); err != nil {
		fmt.Println("[INFO] Trying podman-compose...")
		if _, err2 := shell.ExecuteCmd("", "cd /opt/webmin && podman-compose up -d"); err2 != nil {
			fmt.Printf("[WARNING] Failed to start webmin: %v (also tried podman-compose: %v)\n", err, err2)
		}
	}

	// Create oneshot service
	createWebminService()

	// Set root password
	if _, err := shell.ExecuteCmd("", fmt.Sprintf("echo 'root:%s' | chpasswd", cfg.Password)); err != nil {
		fmt.Printf("[WARNING] Failed to set root password: %v\n", err)
	}

	// Create .desktop file
	createDesktopFile()

	fmt.Println("[INFO] Webmin installed.")
	fmt.Printf("[INFO] Enabled modules: FirewallD, Software Packages, Disk/Filesystems, Custom Commands, SMART, Process, Cron, Logs\n")
}

func generateComposeFile(listenAddr string) {
	composeContent := fmt.Sprintf(`services:
  webmin:
    image: docker.io/pschatzmann/webmin:latest
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
      - "%s:10000:10000"
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

	escaped := strings.ReplaceAll(composeContent, "'", "'\\''")
	if _, err := shell.ExecuteCmd("", fmt.Sprintf("cat > /opt/webmin/docker-compose.yml << 'COMPOSE'\n%s\nCOMPOSE", escaped)); err != nil {
		fmt.Printf("[WARNING] Failed to write compose file: %v\n", err)
	}
	_, _ = shell.ExecuteCmd("", "chown root:root /opt/webmin/docker-compose.yml")
}

func generateModuleConfigs() {
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
		dir := path[:strings.LastIndex(path, "/")]
		if _, err := shell.ExecuteCmd("", fmt.Sprintf("mkdir -p %s", dir)); err != nil {
			fmt.Printf("[WARNING] Failed to create dir %s: %v\n", dir, err)
		}
		escaped := strings.ReplaceAll(content, "'", "'\\''")
		if _, err := shell.ExecuteCmd("", fmt.Sprintf("echo '%s' > %s", escaped, path)); err != nil {
			fmt.Printf("[WARNING] Failed to write %s: %v\n", path, err)
		}
	}
}

func createWebminService() {
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
	escaped := strings.ReplaceAll(serviceScript, "'", "'\\''")
	if _, err := shell.ExecuteCmd("", fmt.Sprintf("cat > %s << 'SERVICE'\n%s\nSERVICE", serviceFile, escaped)); err != nil {
		fmt.Printf("[WARNING] Failed to install webmin service: %v\n", err)
	}
	_, _ = shell.ExecuteCmd("", fmt.Sprintf("chmod 755 %s", serviceFile))
	_, _ = shell.ExecuteCmd("", "rc-update add webmin-ensure default")
}

func createDesktopFile() {
	desktopContent := `[Desktop Entry]
Name=Webmin
Comment=Web-based system administration panel
Exec=firefox --ssb https://localhost:10000
Icon=preferences-system
Terminal=false
Type=Application
Categories=System;
StartupNotify=true
`
	desktopDir := "~/projects/feralos/home/.local/share/applications"
	_, _ = shell.ExecuteCmd("", fmt.Sprintf("mkdir -p %s", desktopDir))
	desktopFile := desktopDir + "/webmin.desktop"
	escaped := strings.ReplaceAll(desktopContent, "'", "'\\''")
	_, _ = shell.ExecuteCmd("", fmt.Sprintf("cat > %s << 'DESKTOP'\n%s\nDESKTOP", desktopFile, escaped))
	_, _ = shell.ExecuteCmd("", fmt.Sprintf("chown <user>:<user> %s", desktopFile))
}
