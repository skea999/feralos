package chrootPhase

import (
	"alpineInstaller/utils/config"
	"alpineInstaller/utils/shell"
	"fmt"
	"strings"
)

// BuildAndDeployUKI builds a signed Unified Kernel Image and deploys it to both EFI partitions.
// This replaces GRUB entirely — the system boots directly from the UKI.
func BuildAndDeployUKI(conf *config.Config) error {
	shell.SetCaller("uki")
	chroot := conf.Internal.ChrootDir
	u := conf.User

	fmt.Println("[INFO] Building and deploying signed UKI...")

	// 1. Detect CPU vendor (microcode already installed in packages.go)
	ucodeImg := ""
	cpuInfo, _ := shell.RunCommand("sh", "-c", "cat /proc/cpuinfo")
	if strings.Contains(cpuInfo, "AMD") {
		ucodeImg = "/boot/amd-ucode.img"
	} else if strings.Contains(cpuInfo, "GenuineIntel") {
		ucodeImg = "/boot/intel-ucode.img"
	}
	fmt.Printf("[DEBUG] Microcode: %s\n", ucodeImg)

	// 2. EFI keys are generated in InstallPackages (before secureboot-hook runs)

	// 3. Build booster initramfs
	if _, err := shell.RunChrootCommand(chroot, "ALPINE_KERNEL=$(ls /lib/modules/ | head -1) && booster build --force --kernel-version \"${ALPINE_KERNEL}\" /boot/booster-lts.img"); err != nil {
		return fmt.Errorf("booster build failed: %w", err)
	}

	// 4. Build UKI with efi-mkuki
	// efi-mkuki supports: efi-mkuki <vmlinuz> [<microcode>...] [<initrd>]
	ukiCmd := "CMDLINE=$(cat /etc/kernel/cmdline) && "
	if ucodeImg != "" {
		ukiCmd += fmt.Sprintf("efi-mkuki -c \"${CMDLINE}\" -o /boot/efi/Alpine/linux-lts.efi /boot/vmlinuz-lts %s /boot/booster-lts.img", ucodeImg)
	} else {
		ukiCmd += "efi-mkuki -c \"${CMDLINE}\" -o /boot/efi/Alpine/linux-lts.efi /boot/vmlinuz-lts /boot/booster-lts.img"
	}
	if _, err := shell.RunChrootCommand(chroot, ukiCmd); err != nil {
		return fmt.Errorf("efi-mkuki failed: %w", err)
	}

	// 5. Sign UKI for Secure Boot
	if _, err := shell.RunChrootCommand(chroot, "sbsign --key /etc/uefi-keys/db.key --cert /etc/uefi-keys/db.crt --output /boot/efi/Alpine/linux-lts.efi /boot/efi/Alpine/linux-lts.efi"); err != nil {
		return fmt.Errorf("sbsign failed: %w", err)
	}

	// 6. Create fallback BOOTX64.EFI
	if _, err := shell.RunChrootCommand(chroot, "mkdir -p /boot/efi/EFI/BOOT && cp -af /boot/efi/Alpine/linux-lts.efi /boot/efi/EFI/BOOT/BOOTX64.EFI"); err != nil {
		return fmt.Errorf("failed to create fallback BOOTX64.EFI: %w", err)
	}

	// 7. Mirror to secondary EFI
	if _, err := shell.RunChrootCommand(chroot, "cp -af /boot/efi/. /boot/efi2/ && mkdir -p /boot/efi2/EFI/BOOT && cp -af /boot/efi2/Alpine/linux-lts.efi /boot/efi2/EFI/BOOT/BOOTX64.EFI"); err != nil {
		return fmt.Errorf("failed to mirror to efi2: %w", err)
	}

	// 8. Create EFI boot entries (only when running on an EFI-capable system;
	//    on a BIOS runner the chroot has no /sys/firmware/efi so efibootmgr
	//    cannot write NVRAM entries.  The BOOTX64.EFI fallback handles boot).
	efiAvail := false
	if _, err := shell.RunChrootCommand(chroot, "test -d /sys/firmware/efi"); err == nil {
		efiAvail = true
	}
	if efiAvail && len(u.Disks) >= 2 {
		d1 := u.Disks[0]
		d2 := u.Disks[1]
		sp1 := getPartitionSpacer(d1)
		sp2 := getPartitionSpacer(d2)

		// Primary boot entry
		if _, err := shell.RunChrootCommand(chroot, fmt.Sprintf("efibootmgr --disk %s --part 1 --create --label 'Alpine Linux' --loader /Alpine/linux-lts.efi --verbose || true", d1+sp1)); err != nil {
			fmt.Printf("[WARNING] efibootmgr primary failed: %v\n", err)
		}
		// Backup boot entry
		if _, err := shell.RunChrootCommand(chroot, fmt.Sprintf("efibootmgr --disk %s --part 1 --create --label 'Alpine Linux Backup' --loader /Alpine/linux-lts.efi --verbose || true", d2+sp2)); err != nil {
			fmt.Printf("[WARNING] efibootmgr backup failed: %v\n", err)
		}
	} else if !efiAvail {
		fmt.Println("[INFO] Skipping efibootmgr: no EFI firmware detected (BIOS runner)")
	}

	// 9. Show boot entries (only when EFI is available)
	if efiAvail {
		if _, err := shell.RunChrootCommand(chroot, "efibootmgr -v || true"); err != nil {
			fmt.Printf("[WARNING] efibootmgr listing failed: %v\n", err)
		}
	} else {
		fmt.Println("[INFO] Skipping efibootmgr: no EFI firmware detected (BIOS runner)")
	}

	return nil
}
