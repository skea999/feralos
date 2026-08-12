package chrootPhase

import (
	"alpineInstaller/utils/config"
	"alpineInstaller/utils/shell"
	"fmt"
	"strings"
)

// SetupServices configures OpenRC runlevels, aliases, editor, pager, and console prompt.
// These are commands that must run inside the chroot.
func SetupServices(conf *config.Config) error {
	shell.SetCaller("services")
	chroot := conf.Internal.ChrootDir
	u := conf.User

	// 1. OpenRC Runlevels
	fmt.Println("[INFO] Configuring OpenRC services...")
	setupRunlevels(chroot, u.Runlevels)

	// 2. Networking configuration — auto-detect if not specified
	iface := u.DefaultNetworkInterface
	if iface == "" {
		var err error
		iface, err = detectNetworkInterface(chroot)
		if err != nil {
			fmt.Printf("[WARNING] Could not detect network interface: %v — skipping network config\n", err)
		}
	}
	if iface != "" {
		fmt.Printf("[INFO] Configuring network interface %s...\n", iface)
		if err := setupNetworking(chroot, iface); err != nil {
			return fmt.Errorf("networking setup failed: %w", err)
		}
	}

	// 3. Symlink aliases to fish and bash
	fmt.Println("[INFO] Configuring shell symlinks...")
	if _, err := shell.ExecuteCmd(chroot, "ln -sfn /etc/profile.d/alias.sh /etc/fish/conf.d/alias.fish"); err != nil {
		fmt.Printf("[WARNING] Failed to symlink fish alias: %v\n", err)
	}
	// Create /etc/bash/ directory if it doesn't exist
	if _, err := shell.ExecuteCmd(chroot, "mkdir -p /etc/bash"); err != nil {
		fmt.Printf("[WARNING] Failed to create /etc/bash: %v\n", err)
	}
	if _, err := shell.ExecuteCmd(chroot, "ln -sfn /etc/profile.d/alias.sh /etc/bash/alias.sh"); err != nil {
		fmt.Printf("[WARNING] Failed to symlink bash alias: %v\n", err)
	}

	// 4. Editor symlink
	if _, err := shell.ExecuteCmd(chroot, "ln -sfn /etc/profile.d/editor.sh /etc/bash/editor.sh"); err != nil {
		fmt.Printf("[WARNING] Failed to symlink bash editor: %v\n", err)
	}

	// 5. Pager (sed must run inside chroot)
	fmt.Println("[INFO] Configuring pager...")
	if _, err := shell.ExecuteCmd(chroot, "sed -i 's/PAGER=less/PAGER=bat/g' /etc/profile"); err != nil {
		fmt.Printf("[WARNING] Failed to set pager: %v\n", err)
	}
	// Escape single quotes in pager.fish content for shell execution
	escapedPagerFish := strings.ReplaceAll("set -gx PAGER bat", "'", "'\\''")
	if _, err := shell.ExecuteCmd(chroot, fmt.Sprintf("echo '%s' > /etc/fish/conf.d/pager.fish", escapedPagerFish)); err != nil {
		return fmt.Errorf("failed to write pager.fish: %w", err)
	}
	// Escape single quotes in pager.sh content for shell execution
	escapedPagerSh := strings.ReplaceAll("export PAGER=bat", "'", "'\\''")
	if _, err := shell.ExecuteCmd(chroot, fmt.Sprintf("echo '%s' > /etc/bash/pager.sh", escapedPagerSh)); err != nil {
		return fmt.Errorf("failed to write pager.sh: %w", err)
	}

	// 6. Timezone symlink (must run inside chroot)
	fmt.Printf("[INFO] Setting timezone to %s\n", u.Timezone)
	if _, err := shell.ExecuteCmd(chroot, fmt.Sprintf("ln -sf /usr/share/zoneinfo/%s /etc/localtime", u.Timezone)); err != nil {
		return fmt.Errorf("failed to set timezone: %w", err)
	}

	// 7. Remove mkinitfs (we use booster)
	fmt.Println("[INFO] Removing mkinitfs...")
	if _, err := shell.ExecuteCmd(chroot, "apk del mkinitfs 2>/dev/null || true"); err != nil {
		fmt.Printf("[WARNING] Failed to remove mkinitfs: %v\n", err)
	}
	if _, err := shell.ExecuteCmd(chroot, "rm -f /etc/kernel-hooks.d/50-secureboot.hook"); err != nil {
		fmt.Printf("[WARNING] Failed to remove old secureboot hook: %v\n", err)
	}

	// 9. Smartd config
	fmt.Println("[INFO] Configuring smartd...")
	// Escape single quotes in smartd content for shell execution
	escapedSmartd := strings.ReplaceAll("DEVICESCAN -a -o on -S on -s (S/../.././02|L/../../6/03) -W 4,40,45", "'", "'\\''")
	if _, err := shell.ExecuteCmd(chroot, fmt.Sprintf("echo '%s' > /etc/smartd.conf", escapedSmartd)); err != nil {
		return fmt.Errorf("failed to write smartd.conf: %w", err)
	}

	// 11. APK audit cron job
	fmt.Println("[INFO] Configuring APK audit cron...")
	if _, err := shell.ExecuteCmd(chroot, "echo '0 4 * * 0 /usr/bin/apk audit --full --quiet || true' >> /etc/crontabs/root"); err != nil {
		fmt.Printf("[WARNING] Failed to write cron job: %v\n", err)
	}

	return nil
}

func setupRunlevels(chroot string, levels map[string][]string) {
	for level, services := range levels {
		for _, svc := range services {
			_, _ = shell.ExecuteCmd(chroot, fmt.Sprintf("rc-update add %s %s", svc, level))
		}
	}
}

// detectNetworkInterface finds the first non-loopback network interface
// by listing /sys/class/net/ inside the chroot.
func detectNetworkInterface(chroot string) (string, error) {
	out, err := shell.ExecuteCmd(chroot, "ls /sys/class/net/ | grep -v ^lo$")
	if err != nil {
		return "", fmt.Errorf("failed to list network interfaces: %w", err)
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	for _, line := range lines {
		iface := strings.TrimSpace(line)
		if iface != "" && iface != "lo" {
			return iface, nil
		}
	}
	return "", fmt.Errorf("no non-loopback network interface found")
}

func setupNetworking(chroot, iface string) error {
	// Create /etc/network/interfaces with loopback and DHCP on specified interface
	interfacesContent := fmt.Sprintf(`auto lo
iface lo inet loopback

auto %s
iface %s inet dhcp`, iface, iface)

	// Escape single quotes in interfacesContent for shell execution
	escapedInterfacesContent := strings.ReplaceAll(interfacesContent, "'", "'\\''")
	if _, err := shell.ExecuteCmd(chroot, fmt.Sprintf("echo '%s' > /etc/network/interfaces", escapedInterfacesContent)); err != nil {
		return fmt.Errorf("failed to write /etc/network/interfaces: %w", err)
	}

	return nil
}
