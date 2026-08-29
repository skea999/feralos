package internal

import (
	"alpineInstaller/utils/config"
	"alpineInstaller/utils/shell"
	"fmt"
	"os/exec"
	"strings"
)

// SetupUsers creates users, sets passwords, configures groups, SSH, and shell.
func SetupUsers(conf *config.Config) error {
	chroot := conf.Internal.ChrootDir
	u := conf.User

	// 1. Root password — uses exec.Command with stdin (no shell, no logging)
	fmt.Println("[INFO] Setting root password...")
	passwdCmd := exec.Command("chroot", chroot, "passwd", "root")
	passwdCmd.Stdin = strings.NewReader(u.Password + "\n" + u.Password + "\n")
	if output, err := passwdCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to set root password: %w\n%s", err, string(output))
	}

	// 2. Create user
	fmt.Printf("[INFO] Creating user %s...\n", u.UserName)
	if _, err := shell.ExecuteCmd(chroot, "adduser -D "+u.UserName); err != nil {
		return fmt.Errorf("failed to create user %s: %w", u.UserName, err)
	}
	passwdCmd = exec.Command("chroot", chroot, "passwd", u.UserName)
	passwdCmd.Stdin = strings.NewReader(u.Password + "\n" + u.Password + "\n")
	if output, err := passwdCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to set password for %s: %w\n%s", u.UserName, err, string(output))
	}

	// 3. Create incus-admin group if incus or incus-feature is installed
	if err := ensureIncusAdminGroup(chroot); err != nil {
		fmt.Printf("[WARNING] Failed to ensure incus-admin group: %v\n", err)
	}

	// 4. Add to dynamic groups
	for _, group := range u.UserGroups {
		checkGroup, _ := shell.ExecuteCmd(chroot, fmt.Sprintf("getent group %s >/dev/null 2>&1 && echo EXISTS || echo NOT_EXISTS", group))
		if !strings.Contains(checkGroup, "EXISTS") {
			if _, err := shell.ExecuteCmd(chroot, fmt.Sprintf("addgroup %s", group)); err != nil {
				fmt.Printf("[WARNING] Failed to create group %s: %v\n", group, err)
				continue
			}
		}
		if _, err := shell.ExecuteCmd(chroot, fmt.Sprintf("adduser %s %s", u.UserName, group)); err != nil {
			fmt.Printf("[WARNING] Failed to add %s to group %s: %v\n", u.UserName, group, err)
		}
	}

	// 5. Add to standard groups
	extraGroups := []string{"lp", "input", "cdrom", "dialout", "video", "netdev", "plugdev", "users"}
	for _, group := range extraGroups {
		if _, err := shell.ExecuteCmd(chroot, fmt.Sprintf("adduser %s %s", u.UserName, group)); err != nil {
			fmt.Printf("[WARNING] Failed to add %s to group %s: %v\n", u.UserName, group, err)
		}
	}

	// 6. SSH authorized key
	if u.SshAuthorizedKey != "" {
		fmt.Println("[INFO] Configuring SSH authorized key...")
		if err := setupSSHKey(chroot, u.UserName, u.SshAuthorizedKey); err != nil {
			return err
		}
	}

	// 7. Subuid/subgid
	fmt.Println("[INFO] Configuring subuid/subgid...")
	if _, err := shell.ExecuteCmd(chroot, "echo 'root:100000:65536' >> /etc/subuid"); err != nil {
		return fmt.Errorf("failed to write subuid: %w", err)
	}
	if _, err := shell.ExecuteCmd(chroot, "echo 'root:100000:65536' >> /etc/subgid"); err != nil {
		return fmt.Errorf("failed to write subgid: %w", err)
	}

	// 8. Shell (fish)
	fmt.Println("[INFO] Setting default shell to fish...")
	if _, err := shell.ExecuteCmd(chroot, "chsh root -s /usr/bin/fish"); err != nil {
		fmt.Printf("[WARNING] Failed to set root shell: %v\n", err)
	}
	if _, err := shell.ExecuteCmd(chroot, fmt.Sprintf("chsh %s -s /usr/bin/fish", u.UserName)); err != nil {
		fmt.Printf("[WARNING] Failed to set user shell: %v\n", err)
	}

	// 9. System limits
	fmt.Println("[INFO] Configuring system limits...")
	if _, err := shell.ExecuteCmd(chroot, "mkdir -p /etc/security"); err != nil {
		fmt.Printf("[WARNING] Failed to create /etc/security: %v\n", err)
	}
	limitsContent := fmt.Sprintf(`* hard nofile 100000
* soft nofile 100000
root hard nofile 100000
root soft nofile 100000
%s hard nofile 524288
%s soft nofile 524288`, u.UserName, u.UserName)
	escapedLimitsContent := strings.ReplaceAll(limitsContent, "'", "'\\''")
	if _, err := shell.ExecuteCmd(chroot, fmt.Sprintf("echo '%s' > /etc/security/limits.conf", escapedLimitsContent)); err != nil {
		return fmt.Errorf("failed to write limits.conf: %w", err)
	}

	return nil
}

// ensureIncusAdminGroup creates the incus-admin group if incus or incus-feature is installed.
func ensureIncusAdminGroup(chroot string) error {
	// Check if incus or incus-feature is installed
	checkIncus, _ := shell.ExecuteCmd(chroot, "apk info -e incus >/dev/null 2>&1 && echo INSTALLED || echo NOT_INSTALLED")
	checkIncusFeature, _ := shell.ExecuteCmd(chroot, "apk info -e incus-feature >/dev/null 2>&1 && echo INSTALLED || echo NOT_INSTALLED")

	if !strings.Contains(checkIncus, "INSTALLED") && !strings.Contains(checkIncusFeature, "INSTALLED") {
		fmt.Println("[DEBUG] incus/incus-feature not installed, skipping incus-admin group")
		return nil
	}

	// Check if group already exists
	checkGroup, _ := shell.ExecuteCmd(chroot, "getent group incus-admin >/dev/null 2>&1 && echo EXISTS || echo NOT_EXISTS")
	if strings.TrimSpace(checkGroup) == "EXISTS" {
		fmt.Println("[DEBUG] incus-admin group already exists")
		return nil
	}

	// Create the group
	fmt.Println("[INFO] Creating incus-admin group...")
	if _, err := shell.ExecuteCmd(chroot, "addgroup incus-admin"); err != nil {
		return fmt.Errorf("failed to create incus-admin group: %w", err)
	}

	fmt.Println("[INFO] incus-admin group created successfully")
	return nil
}

func setupSSHKey(chroot, userName, sshKey string) error {
	if _, err := shell.ExecuteCmd(chroot, "mkdir -p /root/.ssh && chmod 700 /root/.ssh"); err != nil {
		return fmt.Errorf("failed to create root .ssh: %w", err)
	}
	escapedSshKey := strings.ReplaceAll(sshKey, "'", "'\\''")
	if _, err := shell.ExecuteCmd(chroot, fmt.Sprintf("echo '%s' > /root/.ssh/authorized_keys", escapedSshKey)); err != nil {
		return fmt.Errorf("failed to write root authorized_keys: %w", err)
	}
	if _, err := shell.ExecuteCmd(chroot, "chmod 600 /root/.ssh/authorized_keys"); err != nil {
		return fmt.Errorf("failed to chmod root authorized_keys: %w", err)
	}

	if _, err := shell.ExecuteCmd(chroot, fmt.Sprintf("mkdir -p /home/%s/.ssh && chmod 700 /home/%s/.ssh", userName, userName)); err != nil {
		return fmt.Errorf("failed to create user .ssh: %w", err)
	}
	if _, err := shell.ExecuteCmd(chroot, fmt.Sprintf("echo '%s' > /home/%s/.ssh/authorized_keys", escapedSshKey, userName)); err != nil {
		return fmt.Errorf("failed to write user authorized_keys: %w", err)
	}
	if _, err := shell.ExecuteCmd(chroot, fmt.Sprintf("chmod 600 /home/%s/.ssh/authorized_keys && chown -R %s:%s /home/%s/.ssh", userName, userName, userName, userName)); err != nil {
		return fmt.Errorf("failed to chmod user authorized_keys: %w", err)
	}

	return nil
}

// IsEmpty checks if a string is empty or whitespace-only.
func IsEmpty(s string) bool {
	return strings.TrimSpace(s) == ""
}
