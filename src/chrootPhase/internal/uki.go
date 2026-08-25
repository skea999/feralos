package internal

import (
	"alpineInstaller/utils/config"
	"alpineInstaller/utils/shell"
	"fmt"
	"strings"
)

// BuildUKI builds an unsigned Unified Kernel Image and deploys it to both EFI partitions.
func BuildUKI(conf *config.Config) error {
	shell.SetCaller("uki")
	chroot := conf.Internal.ChrootDir
	u := conf.User

	fmt.Println("[INFO] Building and deploying UKI...")

	ucodeImg := ""
	cpuInfo, _ := shell.ExecuteCmd(chroot, "cat /proc/cpuinfo")
	if strings.Contains(cpuInfo, "AMD") {
		ucodeImg = "/boot/amd-ucode.img"
	} else if strings.Contains(cpuInfo, "GenuineIntel") {
		ucodeImg = "/boot/intel-ucode.img"
	}
	fmt.Printf("[DEBUG] Microcode: %s\n", ucodeImg)

	if _, err := shell.ExecuteCmd(chroot, "ALPINE_KERNEL=$(ls /lib/modules/ | head -1) && booster build --force --kernel-version \"${ALPINE_KERNEL}\" /boot/booster-lts.img"); err != nil {
		return fmt.Errorf("booster build failed: %w", err)
	}

	if _, err := shell.ExecuteCmd(chroot, "mkdir -p /boot/efi/Alpine"); err != nil {
		return fmt.Errorf("failed to create EFI output directory: %w", err)
	}

	ukiCmd := "CMDLINE=$(cat /etc/kernel/cmdline) && KERNEL_VER=$(ls /lib/modules/ | head -1) && "
	if ucodeImg != "" {
		ukiCmd += fmt.Sprintf("efi-mkuki -k \"${KERNEL_VER}\" -c \"${CMDLINE}\" -o /boot/efi/Alpine/linux-lts.efi /boot/vmlinuz-lts %s /boot/booster-lts.img", ucodeImg)
	} else {
		ukiCmd += "efi-mkuki -k \"${KERNEL_VER}\" -c \"${CMDLINE}\" -o /boot/efi/Alpine/linux-lts.efi /boot/vmlinuz-lts /boot/booster-lts.img"
	}
	if _, err := shell.ExecuteCmd(chroot, ukiCmd); err != nil {
		return fmt.Errorf("efi-mkuki failed: %w", err)
	}

	if _, err := shell.ExecuteCmd(chroot, "mkdir -p /boot/efi/EFI/BOOT && cp -af /boot/efi/Alpine/linux-lts.efi /boot/efi/EFI/BOOT/BOOTX64.EFI"); err != nil {
		return fmt.Errorf("failed to create fallback BOOTX64.EFI: %w", err)
	}

	// Mirror to secondary ESP (only if /boot/efi2 exists — dual-disk layouts)
	if _, err := shell.ExecuteCmd(chroot, "if [ -d /boot/efi2 ]; then cp -af /boot/efi/. /boot/efi2/ && mkdir -p /boot/efi2/EFI/BOOT && cp -af /boot/efi2/Alpine/linux-lts.efi /boot/efi2/EFI/BOOT/BOOTX64.EFI; fi"); err != nil {
		return fmt.Errorf("failed to mirror to efi2: %w", err)
	}

	if len(u.Disks) >= 2 {
		d1 := u.Disks[0]
		d2 := u.Disks[1]
		sp1 := getPartitionSpacer(d1)
		sp2 := getPartitionSpacer(d2)

		if _, err := shell.ExecuteCmd(chroot, fmt.Sprintf("efibootmgr --disk %s --part 1 --create --label 'Alpine Linux' --loader /Alpine/linux-lts.efi --verbose || true", d1+sp1)); err != nil {
			fmt.Printf("[WARNING] efibootmgr primary failed: %v\n", err)
		}
		if _, err := shell.ExecuteCmd(chroot, fmt.Sprintf("efibootmgr --disk %s --part 1 --create --label 'Alpine Linux Backup' --loader /Alpine/linux-lts.efi --verbose || true", d2+sp2)); err != nil {
			fmt.Printf("[WARNING] efibootmgr backup failed: %v\n", err)
		}
	}

	if _, err := shell.ExecuteCmd(chroot, "efibootmgr -v || true"); err != nil {
		fmt.Printf("[WARNING] efibootmgr listing failed: %v\n", err)
	}

	return nil
}

func getPartitionSpacer(disk string) string {
	if strings.Contains(disk, "nvme") {
		return "p"
	}
	return ""
}
