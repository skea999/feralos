package internal

import (
	"feralos/src/utils/config"
	"feralos/src/utils/shell"
	"fmt"
	"os/exec"
	"strings"
)

// SetupUsers creates users, sets passwords, configures groups, SSH, and shell.
func SetupUsers(conf *config.Config) error {
	chroot := conf.Internal.ChrootDir
	u := conf.User
	// niri family (niri + niriWayle): both run the niri compositor with the
	// turnstile/per-user-dinit session; wayle-session DEPENDS on
	// niri-user-dinit, so the login-shell prerequisites arrive either way.
	isNiri := u.DesktopEnvironment == "niri" || u.DesktopEnvironment == "niriWayle"

	// 1. Root password — uses exec.Command with stdin (no shell, no logging)
	fmt.Println("[INFO] Setting root password...")
	passwdCmd := exec.Command("chroot", chroot, "passwd", "root")
	passwdCmd.Stdin = strings.NewReader(u.Password + "\n" + u.Password + "\n")
	if output, err := passwdCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to set root password: %w\n%s", err, string(output))
	}

	// 2. Create user — no shell, safe for variable username.
	// niri family: passwd shell = /usr/libexec/feralos/login-shell (shipped by
	// niri-user-dinit; installer re-points the symlink to fish, see below).
	// The path is ALSO listed in /etc/shells (see below) so pkexec accepts
	// the session shell. niriWayle inherits this via its wayle-session →
	// niri-user-dinit dependency.
	fmt.Printf("[INFO] Creating user %s...\n", u.UserName)
	adduserArgs := []string{"-D"}
	if isNiri {
		adduserArgs = append(adduserArgs, "-s", "/usr/libexec/feralos/login-shell")
	}
	adduserArgs = append(adduserArgs, u.UserName)
	if _, err := shell.ExecuteCmdArgs(chroot, "adduser", adduserArgs...); err != nil {
		return fmt.Errorf("failed to create user %s: %w", u.UserName, err)
	}
	// niri-user-dinit ships /usr/libexec/feralos/login-shell -> /bin/ash, but
	// /bin/ash is busybox: invoked as `login-shell`, busybox looks up an applet
	// with that name and fails ("login-shell: applet not found"). Any consumer
	// of the passwd shell then breaks (sshd, `su -`, the installer's own
	// `su - <user> -c 'dms setup ...'`). Re-point the symlink at a real shell
	// binary — fish, the pre-turnstile user shell.
	if isNiri {
		if _, err := shell.ExecuteCmd(chroot, "ln -sf /usr/bin/fish /usr/libexec/feralos/login-shell"); err != nil {
			fmt.Printf("[WARNING] Failed to re-point login-shell to fish: %v\n", err)
		}
		// pkexec validates the session user's passwd shell against
		// /etc/shells and refuses with RC=127 ("The value for the SHELL
		// variable was not found in the /etc/shells file") otherwise —
		// EVERY privileged DMS operation died in the session (user
		// management, all polkit actions; run 2026-10-11 debug_run).
		// Listing the login-shell path is safe NOW: the FeralOS niri-session
		// (niri-user-dinit, replaces="niri") has NO /etc/shells bash re-exec
		// trap — only upstream Alpine's script did, which is why this path
		// was originally kept out of /etc/shells. Backward risk: reverting
		// to upstream niri-session would re-arm that trap.
		if _, err := shell.ExecuteCmd(chroot, "grep -qxF '/usr/libexec/feralos/login-shell' /etc/shells || echo '/usr/libexec/feralos/login-shell' >> /etc/shells"); err != nil {
			fmt.Printf("[WARNING] Failed to list login-shell in /etc/shells: %v\n", err)
		}
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
		_, err := shell.ExecuteCmdArgsWithAllowFail(chroot, "getent", []string{"group", group}, true, false)
		if err != nil {
			if _, err := shell.ExecuteCmdArgs(chroot, "addgroup", group); err != nil {
				fmt.Printf("[WARNING] Failed to create group %s: %v\n", group, err)
				continue
			}
		}
		if _, err := shell.ExecuteCmdArgs(chroot, "adduser", u.UserName, group); err != nil {
			fmt.Printf("[WARNING] Failed to add %s to group %s: %v\n", u.UserName, group, err)
		}
	}

	// 5. Add to standard groups
	extraGroups := []string{"lp", "input", "cdrom", "dialout", "video", "netdev", "plugdev", "users"}
	for _, group := range extraGroups {
		if _, err := shell.ExecuteCmdArgs(chroot, "adduser", u.UserName, group); err != nil {
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

	// 8. Shell (fish) — root only for niri: chsh would clobber the
	// login-shell trick (step 2) and break niri-session's dinit branch.
	fmt.Println("[INFO] Setting default shell to fish...")
	if _, err := shell.ExecuteCmdArgs(chroot, "chsh", "root", "-s", "/usr/bin/fish"); err != nil {
		fmt.Printf("[WARNING] Failed to set root shell: %v\n", err)
	}
	if !isNiri {
		if _, err := shell.ExecuteCmdArgs(chroot, "chsh", u.UserName, "-s", "/usr/bin/fish"); err != nil {
			fmt.Printf("[WARNING] Failed to set user shell: %v\n", err)
		}
	}

	// 9. System limits — uses shell redirection, UserName validated in config
	fmt.Println("[INFO] Configuring system limits...")
	if _, err := shell.ExecuteCmdArgs(chroot, "mkdir", "-p", "/etc/security"); err != nil {
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
	if _, err := shell.ExecuteCmdArgs(chroot, "addgroup", "incus-admin"); err != nil {
		return fmt.Errorf("failed to create incus-admin group: %w", err)
	}

	fmt.Println("[INFO] incus-admin group created successfully")
	return nil
}

func setupSSHKey(chroot, userName, sshKey string) error {
	if _, err := shell.ExecuteCmdArgs(chroot, "mkdir", "-p", "/root/.ssh"); err != nil {
		return fmt.Errorf("failed to create root .ssh: %w", err)
	}
	if _, err := shell.ExecuteCmdArgs(chroot, "chmod", "700", "/root/.ssh"); err != nil {
		return fmt.Errorf("failed to chmod root .ssh: %w", err)
	}
	escapedSshKey := strings.ReplaceAll(sshKey, "'", "'\\''")
	if _, err := shell.ExecuteCmd(chroot, fmt.Sprintf("echo '%s' > /root/.ssh/authorized_keys", escapedSshKey)); err != nil {
		return fmt.Errorf("failed to write root authorized_keys: %w", err)
	}
	if _, err := shell.ExecuteCmdArgs(chroot, "chmod", "600", "/root/.ssh/authorized_keys"); err != nil {
		return fmt.Errorf("failed to chmod root authorized_keys: %w", err)
	}

	if _, err := shell.ExecuteCmdArgs(chroot, "mkdir", "-p", "/home/"+userName+"/.ssh"); err != nil {
		return fmt.Errorf("failed to create user .ssh: %w", err)
	}
	if _, err := shell.ExecuteCmdArgs(chroot, "chmod", "700", "/home/"+userName+"/.ssh"); err != nil {
		return fmt.Errorf("failed to chmod user .ssh: %w", err)
	}
	if _, err := shell.ExecuteCmd(chroot, fmt.Sprintf("echo '%s' > /home/%s/.ssh/authorized_keys", escapedSshKey, userName)); err != nil {
		return fmt.Errorf("failed to write user authorized_keys: %w", err)
	}
	if _, err := shell.ExecuteCmdArgs(chroot, "chmod", "600", "/home/"+userName+"/.ssh/authorized_keys"); err != nil {
		return fmt.Errorf("failed to chmod user authorized_keys: %w", err)
	}
	if _, err := shell.ExecuteCmdArgs(chroot, "chown", "-R", userName+":"+userName, "/home/"+userName+"/.ssh"); err != nil {
		return fmt.Errorf("failed to chmod user authorized_keys: %w", err)
	}

	return nil
}
