package chrootPhase

import (
	"alpineInstaller/utils/config"
	"alpineInstaller/utils/shell"
	"fmt"
	"strings"
)

// InstallPackages installs all required packages inside the chroot.
// Includes base packages, optional packages, and TPM packages if enabled.
func InstallPackages(conf *config.Config) error {
	shell.SetCaller("packages")
	chroot := conf.Internal.ChrootDir
	u := conf.User

	fmt.Println("[INFO] Installing packages...")

	if _, err := shell.RunChrootCommand(chroot, "apk update"); err != nil {
		return fmt.Errorf("apk update failed: %w", err)
	}

	// Build complete package list
	var allPkgs []string
	allPkgs = append(allPkgs, u.AllPackages...)

	// Add TPM packages if enabled
	if u.EnableTPM {
		allPkgs = append(allPkgs, "tpm2-tools", "tpm2-tss", "tpm2-tss-tcti-device")
	}

	// Add UKI microcode packages based on CPU (detect inside chroot)
	cpuInfo, _ := shell.RunChrootCommand(chroot, "cat /proc/cpuinfo")
	if strings.Contains(cpuInfo, "AMD") {
		allPkgs = append(allPkgs, "amd-ucode")
	} else if strings.Contains(cpuInfo, "GenuineIntel") {
		allPkgs = append(allPkgs, "intel-ucode")
	}

	// Add Nix packages if enabled
	if u.EnableNix {
		allPkgs = append(allPkgs, "nix", "nix-openrc", "nix-bash-completion", "nix-zsh-completion")
	}

	// Add hardening packages if enabled
	if u.EnableHardening {
		allPkgs = append(allPkgs, "bubblewrap", "apparmor", "apparmor-profiles", "libapparmor", "apparmor-utils")
	}

	// Install all packages together
	pkgList := strings.Join(allPkgs, " ")
	if _, err := shell.RunChrootCommand(chroot, "apk add "+pkgList); err != nil {
		return fmt.Errorf("failed to install packages: %w", err)
	}

	// Generate EFI keys AFTER package installation (secureboot-hook needs them)
	// Keys are always generated - the hook checks for their existence before signing.
	if _, err := shell.RunChrootCommand(chroot, "efi-mkkeys -s 'FeralOS' -o /etc/uefi-keys"); err != nil {
		return fmt.Errorf("efi-mkkeys failed: %w", err)
	}

	return nil
}

// FinalUpgrade runs apk update && apk upgrade as the last step.
func FinalUpgrade(chroot string) error {
	fmt.Println("[INFO] Final APK upgrade...")
	if _, err := shell.RunChrootCommand(chroot, "apk update && apk upgrade"); err != nil {
		fmt.Printf("[WARNING] Final apk upgrade failed: %v\n", err)
	}
	return nil
}
