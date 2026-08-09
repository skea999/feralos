package chrootPhase

import (
	"alpineInstaller/utils/config"
	"alpineInstaller/utils/shell"
	"fmt"
	"strings"
)

// InstallDummyPackages creates dummy packages for replaced C packages.
// Reads rules from internalRules.yaml: for each rule, creates dummy for
// replaced package with symlink to /usr/bin/<replacement>.
// Also creates providePackages (empty dummies to satisfy dependencies).
// Must be called BEFORE the main apk add so dependency resolvers find dummies.
func InstallDummyPackages(conf *config.Config) error {
	chroot := conf.Internal.ChrootDir
	dummies := conf.Internal.DummyPackages
	provides := conf.Internal.ProvidePackages
	if len(dummies) == 0 && len(provides) == 0 {
		return nil
	}

	fmt.Println("[INFO] Installing dummy packages (memory-safe replacements)...")

	// Create replacement dummies (e.g., less → bat)
	for replacement, rule := range dummies {
		replaced := rule.Replace

		// Binary path: /usr/bin/<replacement>
		replacementBin := "/usr/bin/" + replacement
		// Binary path for replaced: /usr/bin/<replaced>
		replacedBin := "/usr/bin/" + replaced

		// Create dummy APK package with symlink
		if err := createDummySymlinkPackage(chroot, replaced, replacementBin, replacedBin); err != nil {
			fmt.Printf("[WARNING] Failed to create dummy %s: %v\n", replaced, err)
			continue
		}
	}

	// Create provide dummies (empty packages to satisfy dependencies)
	for _, pkgName := range provides {
		if err := createProvideDummy(chroot, pkgName); err != nil {
			fmt.Printf("[WARNING] Failed to create provide dummy %s: %v\n", pkgName, err)
			continue
		}
	}

	return nil
}

// createProvideDummy creates an empty dummy APK package (no files) to satisfy
// a dependency without installing the real package. Used for packages like
// util-linux-misc where we want the dependency satisfied but don't want
// the real package's files (e.g., /usr/bin/more) to conflict with alternatives.
func createProvideDummy(chroot, pkgName string) error {
	fmt.Printf("[INFO] Creating provide dummy: %s v999 (empty)\n", pkgName)

	apkPath := fmt.Sprintf("/tmp/provide-%s.apk", pkgName)

	// Clean up any previous attempt
	shell.RunChrootCommand(chroot, "rm -rf "+apkPath)

	// Build empty APK v3 package using apk mkpkg (no --files)
	mkpkgCmd := fmt.Sprintf(
		"apk mkpkg --info name:%s --info version:999 --info description:Dummy %s for dependency satisfaction --info arch:noarch --output %s",
		pkgName, pkgName, apkPath,
	)
	if _, err := shell.RunChrootCommand(chroot, mkpkgCmd); err != nil {
		return fmt.Errorf("apk mkpkg failed for %s: %w", pkgName, err)
	}

	// Install the dummy package
	if _, err := shell.RunChrootCommand(chroot, "apk add --allow-untrusted "+apkPath); err != nil {
		return fmt.Errorf("failed to install provide dummy %s: %w", pkgName, err)
	}

	// Verify
	verify, _ := shell.RunChrootCommand(chroot, "apk info -e "+pkgName+" 2>/dev/null && echo INSTALLED || echo NOT_INSTALLED")
	if strings.Contains(verify, "INSTALLED") {
		fmt.Printf("[INFO] Provide dummy %s v999 installed\n", pkgName)
	} else {
		fmt.Printf("[WARNING] Provide dummy %s may not be properly installed\n", pkgName)
	}

	// Clean up
	shell.RunChrootCommand(chroot, "rm -rf "+apkPath)

	return nil
}

// createDummySymlinkPackage creates a dummy APK v3 package where the binary
// is a symlink to the replacement package's binary.
func createDummySymlinkPackage(chroot, replacedPkg, replacementBin, replacedBin string) error {
	fmt.Printf("[INFO] Creating dummy %s → symlink to %s\n", replacedPkg, replacementBin)

	tmpDir := fmt.Sprintf("/tmp/dummy-%s", replacedPkg)
	apkPath := fmt.Sprintf("/tmp/dummy-%s.apk", replacedPkg)

	// Clean up any previous attempt
	shell.RunChrootCommand(chroot, "rm -rf "+tmpDir+" "+apkPath)

	// Create directory: /usr/bin
	if _, err := shell.RunChrootCommand(chroot, "mkdir -p "+tmpDir+"/usr/bin"); err != nil {
		return fmt.Errorf("failed to create dir: %w", err)
	}

	// Create symlink: /usr/bin/less → /usr/bin/bat
	symlinkCmd := fmt.Sprintf("ln -s %s %s%s", replacementBin, tmpDir, replacedBin)
	if _, err := shell.RunChrootCommand(chroot, symlinkCmd); err != nil {
		return fmt.Errorf("failed to create symlink %s → %s: %w", replacedBin, replacementBin, err)
	}

	// Build APK v3 package using apk mkpkg
	infoName := fmt.Sprintf("name:%s", replacedPkg)
	mkpkgCmd := fmt.Sprintf(
		"apk mkpkg --files %s --info %s --info version:999 --info description:Dummy %s replaced by %s --info arch:noarch --output %s",
		tmpDir, infoName, replacedPkg, replacementBin, apkPath,
	)
	if _, err := shell.RunChrootCommand(chroot, mkpkgCmd); err != nil {
		return fmt.Errorf("apk mkpkg failed for %s: %w", replacedPkg, err)
	}

	// Install the dummy package (untrusted because self-signed)
	if _, err := shell.RunChrootCommand(chroot, "apk add --allow-untrusted "+apkPath); err != nil {
		return fmt.Errorf("failed to install dummy %s: %w", replacedPkg, err)
	}

	// Verify installation
	verify, _ := shell.RunChrootCommand(chroot, "apk info -e "+replacedPkg+" 2>/dev/null && echo INSTALLED || echo NOT_INSTALLED")
	if strings.Contains(verify, "INSTALLED") {
		fmt.Printf("[INFO] Dummy %s v999 installed (symlink to %s)\n", replacedPkg, replacementBin)
	} else {
		fmt.Printf("[WARNING] Dummy %s may not be properly installed\n", replacedPkg)
	}

	// Clean up temp files
	shell.RunChrootCommand(chroot, "rm -rf "+tmpDir+" "+apkPath)

	return nil
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

	// Install dummy packages FIRST so dependency resolvers find them
	if err := InstallDummyPackages(conf); err != nil {
		fmt.Printf("[WARNING] Dummy package installation failed: %v\n", err)
		// Continue — dummies are optional, real packages will be installed
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

	// Install all packages together (dummies already installed, won't pull real C packages)
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
