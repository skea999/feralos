package internal

import (
	"alpineInstaller/utils/config"
	"alpineInstaller/utils/shell"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// SetupBoot writes booster config, kernel cmdline, and kernel hooks.
func SetupBoot(conf *config.Config) error {
	chroot := conf.Internal.ChrootDir
	u := conf.User

	// 1. Write final kernel cmdline
	fmt.Println("[INFO] Writing final kernel cmdline...")
	finalCmdline := strings.Join(u.KernelCmdline, " ")
	if _, err := shell.ExecuteCmd(chroot, "mkdir -p /etc/kernel"); err != nil {
		return fmt.Errorf("failed to create /etc/kernel directory: %w", err)
	}
	if _, err := shell.ExecuteCmd(chroot, fmt.Sprintf("echo '%s' > /etc/kernel/cmdline", finalCmdline)); err != nil {
		return fmt.Errorf("failed to write final kernel cmdline: %w", err)
	}

	// 2. Write booster.yaml
	fmt.Println("[INFO] Writing booster configuration...")
	forceLoadModules := strings.Join(u.Modules, ",")
	boosterYaml := fmt.Sprintf(`compression: zstd
enable_lvm: false
enable_mdraid: false
network:
  dhcp: off
modules_force_load: %s
mount_timeout: 3m
extra_files: busybox,mount,btrfs
append_all_modaliases: true`, forceLoadModules)
	escapedBoosterYaml := strings.ReplaceAll(boosterYaml, "'", "'\\''")
	if _, err := shell.ExecuteCmd(chroot, fmt.Sprintf("echo '%s' > /etc/booster.yaml", escapedBoosterYaml)); err != nil {
		return fmt.Errorf("failed to write booster.yaml: %w", err)
	}

	// 3. Write kernel hook
	if err := writeBoosterHook(chroot); err != nil {
		return fmt.Errorf("booster hook failed: %w", err)
	}

	return nil
}

func writeBoosterHook(chroot string) error {
	fmt.Println("[INFO] Writing booster kernel hook...")

	hookDir := filepath.Join(chroot, "etc", "kernel-hooks.d")
	if err := os.MkdirAll(hookDir, 0755); err != nil {
		return fmt.Errorf("failed to create kernel-hooks.d: %w", err)
	}

	hookContent := `#!/bin/sh
FLAVOR="$1"
NEW_VERSION="$2"
tmpdir=$(mktemp -d)
trap 'rm -rf "${tmpdir}"' EXIT

booster build --force --kernel-version "${NEW_VERSION}-${FLAVOR}" "${tmpdir}/initramfs"

cmdline="$(cat /etc/kernel/cmdline)"
output_dir="/boot/efi/Alpine"
output_name="linux-${FLAVOR}.efi"

mkdir -p "${output_dir}"

if [ -f /boot/amd-ucode.img ]; then
  efi-mkuki \
      -c "${cmdline}" \
      -o "${output_dir}/${output_name}" \
      "/boot/vmlinuz-${FLAVOR}" \
      /boot/amd-ucode.img \
      "${tmpdir}/initramfs"
elif [ -f /boot/intel-ucode.img ]; then
  efi-mkuki \
      -c "${cmdline}" \
      -o "${output_dir}/${output_name}" \
      "/boot/vmlinuz-${FLAVOR}" \
      /boot/intel-ucode.img \
      "${tmpdir}/initramfs"
else
  efi-mkuki \
      -c "${cmdline}" \
      -o "${output_dir}/${output_name}" \
      "/boot/vmlinuz-${FLAVOR}" \
      "${tmpdir}/initramfs"
fi

if [ -f "${output_dir}/${output_name}" ]; then
    cp -af "${output_dir}/${output_name}" "${output_dir}/linux-${FLAVOR}-backup.efi"
fi

mkdir -p "${output_dir}/../EFI/BOOT"
cp -af "${output_dir}/${output_name}" "${output_dir}/../EFI/BOOT/BOOTX64.EFI"

if [ -d /boot/efi2 ]; then
    cp -af /boot/efi/. /boot/efi2/
    mkdir -p /boot/efi2/EFI/BOOT
    cp -af "${output_dir}/${output_name}" /boot/efi2/EFI/BOOT/BOOTX64.EFI
fi
`
	hookPath := filepath.Join(hookDir, "50-booster.hook")
	escapedHookContent := strings.ReplaceAll(hookContent, "'", "'\\''")
	if _, err := shell.ExecuteCmd(chroot, fmt.Sprintf("echo '%s' > /etc/kernel-hooks.d/50-booster.hook", escapedHookContent)); err != nil {
		return fmt.Errorf("failed to write booster hook: %w", err)
	}
	if err := os.Chmod(hookPath, 0755); err != nil {
		return fmt.Errorf("failed to chmod booster hook: %w", err)
	}

	return nil
}
