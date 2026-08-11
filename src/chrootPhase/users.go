package chrootPhase

import (
	"alpineInstaller/utils/config"
	"alpineInstaller/utils/files"
	"alpineInstaller/utils/shell"
	"fmt"
	"strings"
)

// SetupUsers creates users, sets passwords, configures groups, SSH, and shell.
// These commands must run inside the chroot.
func SetupUsers(conf *config.Config) error {
	chroot := conf.Internal.ChrootDir
	u := conf.User

	// 1. Root password
	fmt.Println("[INFO] Setting root password...")
	if _, err := shell.RunChrootCommand(chroot, fmt.Sprintf("echo -e '%s\n%s' | passwd root", u.Password, u.Password)); err != nil {
		return fmt.Errorf("failed to set root password: %w", err)
	}

	// 2. Create user
	fmt.Printf("[INFO] Creating user %s...\n", u.UserName)
	if _, err := shell.RunChrootCommand(chroot, "adduser -D "+u.UserName); err != nil {
		return fmt.Errorf("failed to create user %s: %w", u.UserName, err)
	}
	if _, err := shell.RunChrootCommand(chroot, fmt.Sprintf("echo -e '%s\n%s' | passwd %s", u.Password, u.Password, u.UserName)); err != nil {
		return fmt.Errorf("failed to set password for %s: %w", u.UserName, err)
	}

	// 3. Add to dynamic groups (from rules)
	for _, group := range u.UserGroups {
		// Create group first if it doesn't exist (some packages don't create runtime groups)
		if _, err := shell.RunChrootCommand(chroot, fmt.Sprintf("addgroup %s 2>/dev/null || true", group)); err != nil {
			fmt.Printf("[WARNING] Failed to create group %s: %v\n", group, err)
		}
		if _, err := shell.RunChrootCommand(chroot, fmt.Sprintf("adduser %s %s", u.UserName, group)); err != nil {
			fmt.Printf("[WARNING] Failed to add %s to group %s: %v\n", u.UserName, group, err)
		}
	}

	// 4. Add to standard groups (from reference)
	extraGroups := []string{"lp", "input", "cdrom", "dialout", "video", "netdev", "plugdev", "users", "qemu", "kvm"}
	for _, group := range extraGroups {
		if _, err := shell.RunChrootCommand(chroot, fmt.Sprintf("addgroup %s 2>/dev/null || true", group)); err != nil {
			fmt.Printf("[WARNING] Failed to create group %s: %v\n", group, err)
		}
		if _, err := shell.RunChrootCommand(chroot, fmt.Sprintf("adduser %s %s", u.UserName, group)); err != nil {
			fmt.Printf("[WARNING] Failed to add %s to group %s: %v\n", u.UserName, group, err)
		}
	}

	// 5. SSH authorized key
	if u.SshAuthorizedKey != "" {
		fmt.Println("[INFO] Configuring SSH authorized key...")
		if err := setupSSHKey(chroot, u.UserName, u.SshAuthorizedKey); err != nil {
			return err
		}
	}

	// 6. Subuid/subgid (must append inside chroot)
	fmt.Println("[INFO] Configuring subuid/subgid...")
	if _, err := shell.RunChrootCommand(chroot, "echo 'root:100000:65536' >> /etc/subuid"); err != nil {
		return fmt.Errorf("failed to write subuid: %w", err)
	}
	if _, err := shell.RunChrootCommand(chroot, "echo 'root:100000:65536' >> /etc/subgid"); err != nil {
		return fmt.Errorf("failed to write subgid: %w", err)
	}

	// 7. Shell (fish)
	fmt.Println("[INFO] Setting default shell to fish...")
	if _, err := shell.RunChrootCommand(chroot, "chsh root -s /usr/bin/fish"); err != nil {
		fmt.Printf("[WARNING] Failed to set root shell: %v\n", err)
	}
	if _, err := shell.RunChrootCommand(chroot, fmt.Sprintf("chsh %s -s /usr/bin/fish", u.UserName)); err != nil {
		fmt.Printf("[WARNING] Failed to set user shell: %v\n", err)
	}

	// 8. System limits
	fmt.Println("[INFO] Configuring system limits...")
	limitsContent := fmt.Sprintf(`* hard nofile 100000
* soft nofile 100000
root hard nofile 100000
root soft nofile 100000
%s hard nofile 524288
%s soft nofile 524288
`, u.UserName, u.UserName)
	if err := files.WriteFileInChroot(chroot, "/etc/security/limits.conf", limitsContent); err != nil {
		return fmt.Errorf("failed to write limits.conf: %w", err)
	}

	return nil
}

func setupSSHKey(chroot, userName, sshKey string) error {
	// Root SSH
	if _, err := shell.RunChrootCommand(chroot, "mkdir -p /root/.ssh && chmod 700 /root/.ssh"); err != nil {
		return fmt.Errorf("failed to create root .ssh: %w", err)
	}
	if err := files.WriteFileInChroot(chroot, "/root/.ssh/authorized_keys", sshKey+"\n"); err != nil {
		return fmt.Errorf("failed to write root authorized_keys: %w", err)
	}
	if _, err := shell.RunChrootCommand(chroot, "chmod 600 /root/.ssh/authorized_keys"); err != nil {
		return fmt.Errorf("failed to chmod root authorized_keys: %w", err)
	}

	// User SSH
	if _, err := shell.RunChrootCommand(chroot, fmt.Sprintf("mkdir -p /home/%s/.ssh && chmod 700 /home/%s/.ssh", userName, userName)); err != nil {
		return fmt.Errorf("failed to create user .ssh: %w", err)
	}
	if err := files.WriteFileInChroot(chroot, fmt.Sprintf("/home/%s/.ssh/authorized_keys", userName), sshKey+"\n"); err != nil {
		return fmt.Errorf("failed to write user authorized_keys: %w", err)
	}
	if _, err := shell.RunChrootCommand(chroot, fmt.Sprintf("chmod 600 /home/%s/.ssh/authorized_keys && chown -R %s:%s /home/%s/.ssh", userName, userName, userName, userName)); err != nil {
		return fmt.Errorf("failed to chmod user authorized_keys: %w", err)
	}

	return nil
}

// IsEmpty checks if a string is empty or whitespace-only.
func IsEmpty(s string) bool {
	return strings.TrimSpace(s) == ""
}
