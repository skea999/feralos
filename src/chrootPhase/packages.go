package chrootPhase

import (
	"alpineInstaller/utils/config"
	"alpineInstaller/utils/shell"
	"fmt"
	"strings"
)

// PackageDummy defines a package whose binaries should be replaced with no-op stubs.
type PackageDummy struct {
	Package string            // APK package name
	Binaries map[string]string // binary path → dummy shell script content
}

// dummyPackages lists all packages to replace with dummy stubs.
// Add new entries here to dummy-ify additional C packages.
var dummyPackages = []PackageDummy{
	{
		Package: "less",
		Binaries: map[string]string{
			"/usr/bin/less":    "#!/bin/sh\necho '[dummy] less disabled (memory-safety)' >&2\nexit 1\n",
			"/usr/bin/lesskey": "#!/bin/sh\necho '[dummy] lesskey disabled (memory-safety)' >&2\nexit 1\n",
			"/usr/bin/lessecho": "#!/bin/sh\necho '[dummy] lessecho disabled (memory-safety)' >&2\nexit 1\n",
		},
	},
}

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

	// Replace C package binaries with memory-safe dummy stubs
	for _, pkg := range dummyPackages {
		if err := ReplacePackageBinaries(chroot, pkg); err != nil {
			fmt.Printf("[WARNING] Failed to dummy %s: %v\n", pkg.Package, err)
		}
	}

	// Generate EFI keys AFTER package installation (secureboot-hook needs them)
	// Keys are always generated - the hook checks for their existence before signing.
	if _, err := shell.RunChrootCommand(chroot, "efi-mkkeys -s 'FeralOS' -o /etc/uefi-keys"); err != nil {
		return fmt.Errorf("efi-mkkeys failed: %w", err)
	}

	return nil
}

// ReplacePackageBinaries replaces all binaries of a package with no-op dummy scripts.
// The package remains "installed" in apk (metadata stays), but its executables
// are replaced with shell stubs that print a warning and exit 1.
func ReplacePackageBinaries(chroot string, pkg PackageDummy) error {
	fmt.Printf("[INFO] Replacing %s binaries with dummy stubs...\n", pkg.Package)

	// Verify package is installed
	installed, _ := shell.RunChrootCommand(chroot, "apk info -e "+pkg.Package+" 2>/dev/null && echo INSTALLED || echo NOT_INSTALLED")
	if !strings.Contains(installed, "INSTALLED") {
		fmt.Printf("[WARNING] Package %s not installed, skipping dummy replacement\n", pkg.Package)
		return nil
	}

	// List files from the package
	fileList, _ := shell.RunChrootCommand(chroot, "apk info -L "+pkg.Package+" 2>/dev/null")
	fmt.Printf("[DEBUG] %s files:\n%s\n", pkg.Package, fileList)

	// Replace each binary
	for binPath, script := range pkg.Binaries {
		// Check if binary exists
		check, _ := shell.RunChrootCommand(chroot, "ls -la "+binPath+" 2>/dev/null && echo EXISTS || echo MISSING")
		if strings.Contains(check, "MISSING") {
			fmt.Printf("[DEBUG] %s not found, skipping\n", binPath)
			continue
		}

		// Backup original (for debugging)
		shell.RunChrootCommand(chroot, "cp "+binPath+" "+binPath+".orig 2>/dev/null || true")

		// Write dummy script
		writeCmd := fmt.Sprintf("printf '%%s' '%s' > %s", strings.ReplaceAll(script, "'", "'\\''"), binPath)
		if _, err := shell.RunChrootCommand(chroot, writeCmd); err != nil {
			fmt.Printf("[WARNING] Failed to write %s: %v\n", binPath, err)
			continue
		}

		// Make executable
		shell.RunChrootCommand(chroot, "chmod +x "+binPath)

		// Verify
		verify, _ := shell.RunChrootCommand(chroot, binPath+" 2>&1 || true")
		fmt.Printf("[INFO] %s replaced: %s\n", binPath, strings.TrimSpace(verify))
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
