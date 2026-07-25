package chrootPhase

import (
	"alpineInstaller/utils/config"
	"alpineInstaller/utils/shell"
	"fmt"
	"strings"
)

// Run executes all final configuration steps inside Alpine chroot environment.
func Run(conf *config.Config) error {
	u := conf.User
	i := conf.Internal
	chroot := i.ChrootDir

	fmt.Println("[INFO] Starting ChrootPhase.Run...")

	// 1. Package Update and Installation
	// Uses u.AllPackages which already contains Standard + Extra + packages derived from rules
	fmt.Println("[DEBUG] Updating repositories and installing all selected packages")
	if _, err := shell.RunChrootCommand(chroot, "apk update"); err != nil {
		return fmt.Errorf("apk update failed: %w", err)
	}

	pkgList := strings.Join(u.AllPackages, " ")
	if _, err := shell.RunChrootCommand(chroot, "apk add "+pkgList); err != nil {
		return fmt.Errorf("failed to install packages: %w", err)
	}

	// 2. User and Password Setup
	fmt.Printf("[DEBUG] Configuring users: root and %s\n", u.UserName)
	// Root Password
	if _, err := shell.RunChrootCommand(chroot, fmt.Sprintf("echo -e '%s\n%s' | passwd root", u.Password, u.Password)); err != nil {
		return fmt.Errorf("failed to set root password: %w", err)
	}

	// Create user without predefined groups (only his primary group)
	if _, err := shell.RunChrootCommand(chroot, "adduser -D "+u.UserName); err != nil {
		return fmt.Errorf("failed to create user %s: %w", u.UserName, err)
	}
	if _, err := shell.RunChrootCommand(chroot, fmt.Sprintf("echo -e '%s\n%s' | passwd %s", u.Password, u.Password, u.UserName)); err != nil {
		return fmt.Errorf("failed to set password for %s: %w", u.UserName, err)
	}

	// Add to DYNAMIC groups (including wheel if present in doas/sudo rules)
	for _, group := range u.UserGroups {
		fmt.Printf("[DEBUG] Adding user %s to group %s\n", u.UserName, group)
		if _, err := shell.RunChrootCommand(chroot, fmt.Sprintf("addgroup %s %s", u.UserName, group)); err != nil {
			return fmt.Errorf("failed to add %s to group %s: %w", u.UserName, group, err)
		}
	}

	// 3. Timezone
	fmt.Printf("[DEBUG] Setting timezone to %s\n", u.Timezone)
	if _, err := shell.RunChrootCommand(chroot, fmt.Sprintf("ln -sf /usr/share/zoneinfo/%s /etc/localtime", u.Timezone)); err != nil {
		return fmt.Errorf("failed to set timezone: %w", err)
	}

	// 4. Runlevels (OpenRC)
	// This u.Runlevels map contains system defaults + those added by extra packages
	fmt.Println("[DEBUG] Configuring OpenRC runlevels")
	setupRunlevels(chroot, u.Runlevels)

	// 5. Boot Finalization (GRUB and Initramfs)
	fmt.Println("[DEBUG] Generating Initramfs and installing GRUB bootloader")
	if err := finalizeBoot(chroot); err != nil {
		return fmt.Errorf("boot finalization failed: %w", err)
	}

	fmt.Println("[INFO] ChrootPhase.Run completed successfully.")
	return nil
}

func setupRunlevels(chroot string, levels map[string][]string) {
	for level, services := range levels {
		for _, svc := range services {
			// Execute: rc-update add <service> <runlevel>
			// Best effort — don't fail the whole install if one service fails
			_, _ = shell.RunChrootCommand(chroot, fmt.Sprintf("rc-update add %s %s", svc, level))
		}
	}
}

func finalizeBoot(chroot string) error {
	// GRUB installation on both EFI disks for redundancy (RAID1)
	// Disk 1
	if _, err := shell.RunChrootCommand(chroot, "grub-install --target=x86_64-efi --efi-directory=/boot/efi --bootloader-id=Alpine1 --recheck --no-floppy --removable"); err != nil {
		return err
	}
	// Disk 2
	if _, err := shell.RunChrootCommand(chroot, "grub-install --target=x86_64-efi --efi-directory=/boot/efi2 --bootloader-id=Alpine2 --recheck --no-floppy --removable"); err != nil {
		return err
	}

	// Initramfs regeneration (necessary to include raid1/btrfs modules loaded previously)
	// We find the kernel version present for mkinitfs
	kernelCmd := "ls /lib/modules/ | head -n1"
	kVer, _ := shell.RunChrootCommand(chroot, kernelCmd)
	kVer = strings.TrimSpace(kVer)

	if _, err := shell.RunChrootCommand(chroot, fmt.Sprintf("mkinitfs -c /etc/mkinitfs/mkinitfs.conf -b / %s", kVer)); err != nil {
		return err
	}

	// Final GRUB configuration file generation
	if _, err := shell.RunChrootCommand(chroot, "grub-mkconfig -o /boot/grub/grub.cfg"); err != nil {
		return err
	}

	return nil
}
