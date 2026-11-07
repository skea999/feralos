package internal

import (
	"feralos/src/utils/config"
	"feralos/src/utils/shell"
	"fmt"
	"os/exec"
	"strings"
)

// SetupUsers creates users, sets passwords, configures groups, SSH, and shell.
// Desktop-specific shell behavior (the niri login-shell trick) is owned by
// the desktop setup (desktops/niri.go / niri_wayle.go) — this file is
// identical for every desktop.
func SetupUsers(conf *config.Config) error {
	chroot := conf.Internal.ChrootDir
	u := conf.User

	if err := setupRootPassword(chroot, u.Password); err != nil {
		return err
	}
	if err := createUserAccount(chroot, &u); err != nil {
		return err
	}

	// Create incus-admin group if incus or incus-feature is installed.
	// packageLogic.incus.groups fires at config-load time (ApplyRules) but
	// incus enters the package list later (features/extraPackages) —
	// late-arriving packages need this install-time group fallback
	// (regression run 2026-11-07T15-56-12Z).
	if err := ensureIncusAdminGroup(chroot); err != nil {
		fmt.Printf("[WARNING] Failed to ensure incus-admin group: %v\n", err)
	}

	setupUserGroups(chroot, &u)
	setupStandardGroups(chroot, &u, conf.Internal.StandardUserGroups)

	// 6. SSH authorized key
	if u.SshAuthorizedKey != "" {
		fmt.Println("[INFO] Configuring SSH authorized key...")
		if err := setupSSHKey(chroot, u.UserName, u.SshAuthorizedKey); err != nil {
			return err
		}
	}

	if err := appendSubuidSubgid(chroot); err != nil {
		return err
	}
	setupUserShell(chroot, &u)

	return setupLimits(chroot, &u, conf.Internal.Limits)
}

// setupRootPassword sets the root password. Uses exec.Command with stdin
// (no shell, no logging).
func setupRootPassword(chroot, password string) error {
	// 1. Root password
	fmt.Println("[INFO] Setting root password...")
	// Direct exec, NOT shell.ExecuteCmd: the password must never appear in logs (documented logging-rule exception).
	passwdCmd := exec.Command("chroot", chroot, "passwd", "root")
	passwdCmd.Stdin = strings.NewReader(password + "\n" + password + "\n")
	if output, err := passwdCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to set root password: %w\n%s", err, string(output))
	}
	return nil
}

// createUserAccount creates the primary user and sets its password. Generic
// for every desktop: the DE setup (niri/niriWayle) re-points the passwd
// shell afterwards (login-shell trick — idempotent usermod).
func createUserAccount(chroot string, u *config.UserVariables) error {
	// 2. Create user — no shell, safe for variable username.
	fmt.Printf("[INFO] Creating user %s...\n", u.UserName)
	if _, err := shell.ExecuteCmdArgs(chroot, "adduser", "-D", u.UserName); err != nil {
		return fmt.Errorf("failed to create user %s: %w", u.UserName, err)
	}
	// Direct exec, NOT shell.ExecuteCmd: the password must never appear in logs (documented logging-rule exception).
	passwdCmd := exec.Command("chroot", chroot, "passwd", u.UserName) //nolint:gosec // G204: installer-validated username; stdin-carried password never logged
	passwdCmd.Stdin = strings.NewReader(u.Password + "\n" + u.Password + "\n")
	if output, err := passwdCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to set password for %s: %w\n%s", u.UserName, err, string(output))
	}
	return nil
}

// setupUserGroups creates the dynamic groups from the rules (UserGroups) as
// needed and adds the user to each (best-effort per group).
func setupUserGroups(chroot string, u *config.UserVariables) {
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
}

// setupStandardGroups adds the user to the fixed group set from the rules
// (internal_rules standardUserGroups — best-effort per group).
func setupStandardGroups(chroot string, u *config.UserVariables, groups []string) {
	// 5. Add to standard groups
	for _, group := range groups {
		if _, err := shell.ExecuteCmdArgs(chroot, "adduser", u.UserName, group); err != nil {
			fmt.Printf("[WARNING] Failed to add %s to group %s: %v\n", u.UserName, group, err)
		}
	}
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

// appendSubuidSubgid appends the user subordinate id ranges (the root ranges
// were written unconditionally by SetupMisc earlier in the chroot run).
func appendSubuidSubgid(chroot string) error {
	// 7. Subuid/subgid
	fmt.Println("[INFO] Configuring subuid/subgid...")
	if err := shell.WriteFileAppend(chroot, "/etc/subuid", []byte("root:100000:65536\n"), 0644); err != nil {
		return fmt.Errorf("failed to write subuid: %w", err)
	}
	if err := shell.WriteFileAppend(chroot, "/etc/subgid", []byte("root:100000:65536\n"), 0644); err != nil {
		return fmt.Errorf("failed to write subgid: %w", err)
	}
	return nil
}

// setupUserShell sets fish as the default shell for root and the primary
// user. Desktops may override the USER shell afterwards (niri/niriWayle
// re-point it to /usr/libexec/feralos/login-shell — the DE setup runs later
// in chrootPhase, so the final passwd shell is the DE's).
func setupUserShell(chroot string, u *config.UserVariables) {
	// 8. Shell (fish)
	fmt.Println("[INFO] Setting default shell to fish...")
	if _, err := shell.ExecuteCmdArgs(chroot, "chsh", "root", "-s", "/usr/bin/fish"); err != nil {
		fmt.Printf("[WARNING] Failed to set root shell: %v\n", err)
	}
	if _, err := shell.ExecuteCmdArgs(chroot, "chsh", u.UserName, "-s", "/usr/bin/fish"); err != nil {
		fmt.Printf("[WARNING] Failed to set user shell: %v\n", err)
	}
}

// setupLimits writes /etc/security/limits.conf — single owner of the file
// (the former identical write in misc.go SetupMisc was
// removed). Values from the rules (internal_rules limits).
func setupLimits(chroot string, u *config.UserVariables, limits config.LimitsRules) error {
	// 9. System limits
	fmt.Println("[INFO] Configuring system limits...")
	if _, err := shell.ExecuteCmdArgs(chroot, "mkdir", "-p", "/etc/security"); err != nil {
		fmt.Printf("[WARNING] Failed to create /etc/security: %v\n", err)
	}
	limitsContent := fmt.Sprintf(`* hard nofile %d
* soft nofile %d
root hard nofile %d
root soft nofile %d
%s hard nofile %d
%s soft nofile %d`,
		limits.NoFileDefault, limits.NoFileDefault,
		limits.NoFileDefault, limits.NoFileDefault,
		u.UserName, limits.NoFileUser,
		u.UserName, limits.NoFileUser)
	if err := shell.WriteFile(chroot, "/etc/security/limits.conf", []byte(limitsContent+"\n"), 0644); err != nil {
		return fmt.Errorf("failed to write limits.conf: %w", err)
	}
	return nil
}

func setupSSHKey(chroot, userName, sshKey string) error {
	if _, err := shell.ExecuteCmdArgs(chroot, "mkdir", "-p", "/root/.ssh"); err != nil {
		return fmt.Errorf("failed to create root .ssh: %w", err)
	}
	if _, err := shell.ExecuteCmdArgs(chroot, "chmod", "700", "/root/.ssh"); err != nil {
		return fmt.Errorf("failed to chmod root .ssh: %w", err)
	}
	// Direct write (P2-5): the key is a variable — the old escaped-echo was
	// the fragile path this helper replaces. Mode 0644 like `echo >`; the
	// following chmod 600 lines are unchanged.
	if err := shell.WriteFile(chroot, "/root/.ssh/authorized_keys", []byte(sshKey+"\n"), 0644); err != nil {
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
	if err := shell.WriteFile(chroot, "/home/"+userName+"/.ssh/authorized_keys", []byte(sshKey+"\n"), 0644); err != nil {
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
