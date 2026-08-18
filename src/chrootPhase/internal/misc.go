package internal

import (
	"alpineInstaller/utils/config"
	"alpineInstaller/utils/shell"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// SetupMisc configures editor, PS1, timezone, pager,
// subuid/subgid, limits, sudoers, LXC, mkinitfs removal, smartd, cron, keymap.
func SetupMisc(conf *config.Config) error {
	chroot := conf.Internal.ChrootDir
	u := conf.User

	// 1. Editor
	fmt.Println("[INFO] Setting editor to micro...")
	if _, err := shell.ExecuteCmd(chroot, "echo 'export EDITOR=micro' > /etc/profile.d/editor.sh"); err != nil {
		return fmt.Errorf("failed to write editor.sh: %w", err)
	}

	// 3. PS1
	ps1Content := `echo 'if [ "$(whoami)" = "root" ] ; then
    PS1="\[\033[31;1m\]\u\[\033[0m\]@\[\033[1;33m\]\h\[\033[0m\]:\w# "
else
    PS1="\[\033[32;1m\]\u\[\033[0m\]@\[\033[1;33m\]\h\[\033[0m\]:\w> "
fi' > /etc/profile.d/console.sh`
	if _, err := shell.ExecuteCmd(chroot, ps1Content); err != nil {
		return fmt.Errorf("failed to write console.sh: %w", err)
	}

	// 4. Timezone
	fmt.Printf("[INFO] Setting timezone to %s\n", u.Timezone)
	if _, err := shell.ExecuteCmd(chroot, fmt.Sprintf("ln -sf /usr/share/zoneinfo/%s /etc/localtime", u.Timezone)); err != nil {
		return fmt.Errorf("failed to set timezone: %w", err)
	}

	// 5. Locale
	fmt.Println("[INFO] Setting up locale...")
	localeContent := fmt.Sprintf("echo 'export CHARSET=%s\nexport LANG=%s\nLC_COLLATE=%s' > /etc/profile.d/20locale.sh", u.Locale, u.Locale, u.Locale)
	if _, err := shell.ExecuteCmd(chroot, localeContent); err != nil {
		return fmt.Errorf("failed to write locale config: %w", err)
	}

	// 6. Keymap
	fmt.Println("[INFO] Setting keyboard layout...")
	if _, err := shell.ExecuteCmd(chroot, "echo 'KEYMAP=\"/usr/share/bkeymaps/it/it.bmap.gz\"' > /etc/conf.d/loadkmap"); err != nil {
		return fmt.Errorf("failed to write keymap config: %w", err)
	}

	// 7. Pager (fish only)
	fmt.Println("[INFO] Configuring pager...")
	if _, err := shell.ExecuteCmd(chroot, "sed -i 's/PAGER=less/PAGER=bat/g' /etc/profile"); err != nil {
		fmt.Printf("[WARNING] Failed to set pager: %v\n", err)
	}
	escapedPagerFish := strings.ReplaceAll("set -gx PAGER bat", "'", "'\\''")
	if _, err := shell.ExecuteCmd(chroot, fmt.Sprintf("echo '%s' > /etc/fish/conf.d/pager.fish", escapedPagerFish)); err != nil {
		return fmt.Errorf("failed to write pager.fish: %w", err)
	}

	// 8. Subuid/subgid
	fmt.Println("[INFO] Configuring subuid/subgid...")
	if _, err := shell.ExecuteCmd(chroot, "echo 'root:100000:65536' > /etc/subuid"); err != nil {
		return fmt.Errorf("failed to write subuid: %w", err)
	}
	if _, err := shell.ExecuteCmd(chroot, "echo 'root:100000:65536' > /etc/subgid"); err != nil {
		return fmt.Errorf("failed to write subgid: %w", err)
	}

	// 10. System limits
	fmt.Println("[INFO] Configuring system limits...")
	if _, err := shell.ExecuteCmd(chroot, "mkdir -p /etc/security"); err != nil {
		fmt.Printf("[WARNING] Failed to create /etc/security: %v\n", err)
	}
	limits := fmt.Sprintf(`echo "* hard nofile 100000
* soft nofile 100000
root hard nofile 100000
root soft nofile 100000
%s hard nofile 524288
%s soft nofile 524288" > /etc/security/limits.conf`, u.UserName, u.UserName)
	if _, err := shell.ExecuteCmd(chroot, limits); err != nil {
		return fmt.Errorf("failed to write limits.conf: %w", err)
	}

	// 11. Sudoers
	fmt.Println("[INFO] Configuring sudo-rs permissions...")
	if err := os.MkdirAll(filepath.Join(chroot, "etc", "sudoers.d"), 0755); err != nil {
		return fmt.Errorf("failed to create /etc/sudoers.d directory: %w", err)
	}
	if _, err := shell.ExecuteCmd(chroot, "echo '%wheel ALL=(ALL) ALL' > /etc/sudoers.d/wheel"); err != nil {
		return fmt.Errorf("failed to write sudoers wheel config: %w", err)
	}

	// 12. LXC
	fmt.Println("[INFO] Writing LXC service configuration...")
	if _, err := shell.ExecuteCmd(chroot, "echo 'systemd_container=yes' > /etc/conf.d/lxc"); err != nil {
		return fmt.Errorf("failed to write LXC config: %w", err)
	}

	// 13. Remove mkinitfs
	fmt.Println("[INFO] Removing mkinitfs...")
	if _, err := shell.ExecuteCmd(chroot, "apk del mkinitfs 2>/dev/null || true"); err != nil {
		fmt.Printf("[WARNING] Failed to remove mkinitfs: %v\n", err)
	}
	if _, err := shell.ExecuteCmd(chroot, "rm -f /etc/kernel-hooks.d/50-secureboot.hook"); err != nil {
		fmt.Printf("[WARNING] Failed to remove old secureboot hook: %v\n", err)
	}

	// 14. Smartd config
	fmt.Println("[INFO] Configuring smartd...")
	escapedSmartd := strings.ReplaceAll("DEVICESCAN -a -o on -S on -s (S/../.././02|L/../../6/03) -W 4,40,45", "'", "'\\''")
	if _, err := shell.ExecuteCmd(chroot, fmt.Sprintf("echo '%s' > /etc/smartd.conf", escapedSmartd)); err != nil {
		return fmt.Errorf("failed to write smartd.conf: %w", err)
	}

	// 15. APK audit cron
	fmt.Println("[INFO] Configuring APK audit cron...")
	if _, err := shell.ExecuteCmd(chroot, "echo '0 4 * * 0 /usr/bin/apk audit --full --quiet || true' >> /etc/crontabs/root"); err != nil {
		fmt.Printf("[WARNING] Failed to write cron job: %v\n", err)
	}

	return nil
}
