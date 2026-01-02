package chrootPhase

import (
	"alpineInstaller/installer/utility"
	"fmt"
	"strings"
)

// Run performs all final configuration steps inside the Alpine chroot environment.
func Run(conf *utility.Config) error {
	u := conf.User
	i := conf.Internal
	chroot := i.ChrootDir

	fmt.Println("[INFO] Starting ChrootPhase.Run...")

	// 1. Update and Package Installation
	// We use u.AllPackages which already contains Standard + Extra + packages derived from rules
	fmt.Println("[DEBUG] Updating repositories and installing all selected packages")
	if _, err := utility.RunChrootCommand(chroot, "apk update"); err != nil {
		return fmt.Errorf("apk update failed: %w", err)
	}

	pkgList := strings.Join(u.AllPackages, " ")
	if _, err := utility.RunChrootCommand(chroot, "apk add "+pkgList); err != nil {
		return fmt.Errorf("failed to install packages: %w", err)
	}

	// 2. Setup Utente e Password
	fmt.Printf("[DEBUG] Configuring users: root and %s\n", u.UserName)
	// Password di Root
	utility.RunChrootCommand(chroot, fmt.Sprintf("echo -e '%s\n%s' | passwd root", u.Password, u.Password))

	// User creation without default groups (only its primary group)
	utility.RunChrootCommand(chroot, "adduser -D "+u.UserName)
	utility.RunChrootCommand(chroot, fmt.Sprintf("echo -e '%s\n%s' | passwd %s", u.Password, u.Password, u.UserName))

	// Added to DYNAMIC groups (including wheel if present in doas/sudo rules)
	for _, group := range u.UserGroups {
		fmt.Printf("[DEBUG] Adding user %s to group %s\n", u.UserName, group)
		utility.RunChrootCommand(chroot, fmt.Sprintf("addgroup %s %s", u.UserName, group))
	}

	// 3. Timezone
	fmt.Printf("[DEBUG] Setting timezone to %s\n", u.Timezone)
	utility.RunChrootCommand(chroot, fmt.Sprintf("ln -sf /usr/share/zoneinfo/%s /etc/localtime", u.Timezone))

	// 4. Runlevels (OpenRC)
	// This u.Runlevels map holds system defaults + those added by extra packages
	fmt.Println("[DEBUG] Configuring OpenRC runlevels")
	setupRunlevels(chroot, u.Runlevels)

	// 5. Finalizzazione Boot (GRUB e Initramfs)
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
			// Executes: rc-update add <service> <runlevel>
			utility.RunChrootCommand(chroot, fmt.Sprintf("rc-update add %s %s", svc, level))
		}
	}
}

func finalizeBoot(chroot string) error {
	// GRUB installation on both EFI disks for redundancy (RAID1)
	// Disco 1
	if _, err := utility.RunChrootCommand(chroot, "grub-install --target=x86_64-efi --efi-directory=/boot/efi --bootloader-id=Alpine1 --recheck --no-floppy --removable"); err != nil {
		return err
	}
	// Disco 2
	if _, err := utility.RunChrootCommand(chroot, "grub-install --target=x86_64-efi --efi-directory=/boot/efi2 --bootloader-id=Alpine2 --recheck --no-floppy --removable"); err != nil {
		return err
	}

	// Initramfs regeneration (needed to include the raid1/btrfs modules loaded earlier)
	// Cerchiamo la versione del kernel presente per mkinitfs
	kernelCmd := "ls /lib/modules/ | head -n1"
	kVer, _ := utility.RunChrootCommand(chroot, kernelCmd)
	kVer = strings.TrimSpace(kVer)

	if _, err := utility.RunChrootCommand(chroot, fmt.Sprintf("mkinitfs -c /etc/mkinitfs/mkinitfs.conf -b / %s", kVer)); err != nil {
		return err
	}

	// Final GRUB configuration file generation
	if _, err := utility.RunChrootCommand(chroot, "grub-mkconfig -o /boot/grub/grub.cfg"); err != nil {
		return err
	}

	return nil
}
