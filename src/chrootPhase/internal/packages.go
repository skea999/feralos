package internal

import (
	"alpineInstaller/utils/config"
	"alpineInstaller/utils/shell"
	"fmt"
	"strings"
)

// InstallDummyPackages creates dummy packages for replaced C packages.
func InstallDummyPackages(conf *config.Config) error {
	chroot := conf.Internal.ChrootDir
	dummies := conf.Internal.DummyPackages
	provides := conf.Internal.ProvidePackages
	if len(dummies) == 0 && len(provides) == 0 {
		return nil
	}

	fmt.Println("[INFO] Installing dummy packages (memory-safe replacements)...")

	for replacement, rule := range dummies {
		replaced := rule.Replace
		replacementBin := "/usr/bin/" + replacement
		replacedBin := "/usr/bin/" + replaced

		if err := createDummySymlinkPackage(chroot, replaced, replacementBin, replacedBin); err != nil {
			fmt.Printf("[WARNING] Failed to create dummy %s: %v\n", replaced, err)
			continue
		}
	}

	for _, pkgName := range provides {
		if err := createProvideDummy(chroot, pkgName); err != nil {
			fmt.Printf("[WARNING] Failed to create provide dummy %s: %v\n", pkgName, err)
			continue
		}
	}

	return nil
}

func createProvideDummy(chroot, pkgName string) error {
	fmt.Printf("[INFO] Creating provide dummy: %s v999 (empty)\n", pkgName)

	apkPath := fmt.Sprintf("/tmp/provide-%s.apk", pkgName)
	_, _ = shell.ExecuteCmd(chroot, "rm -rf "+apkPath)

	mkpkgCmd := fmt.Sprintf(
		"apk mkpkg --info name:%s --info version:999 --info description:Dummy %s for dependency satisfaction --info arch:noarch --output %s",
		pkgName, pkgName, apkPath,
	)
	if _, err := shell.ExecuteCmd(chroot, mkpkgCmd); err != nil {
		return fmt.Errorf("apk mkpkg failed for %s: %w", pkgName, err)
	}

	if _, err := shell.ExecuteCmd(chroot, "apk add --allow-untrusted "+apkPath); err != nil {
		return fmt.Errorf("failed to install provide dummy %s: %w", pkgName, err)
	}

	verify, _ := shell.ExecuteCmd(chroot, "apk info -e "+pkgName+" 2>/dev/null && echo INSTALLED || echo NOT_INSTALLED")
	if strings.Contains(verify, "INSTALLED") {
		fmt.Printf("[INFO] Provide dummy %s v999 installed\n", pkgName)
	} else {
		fmt.Printf("[WARNING] Provide dummy %s may not be properly installed\n", pkgName)
	}

	_, _ = shell.ExecuteCmd(chroot, "rm -rf "+apkPath)
	return nil
}

func createDummySymlinkPackage(chroot, replacedPkg, replacementBin, replacedBin string) error {
	fmt.Printf("[INFO] Creating dummy %s → symlink to %s\n", replacedPkg, replacementBin)

	tmpDir := fmt.Sprintf("/tmp/dummy-%s", replacedPkg)
	apkPath := fmt.Sprintf("/tmp/dummy-%s.apk", replacedPkg)

	_, _ = shell.ExecuteCmd(chroot, "rm -rf "+tmpDir+" "+apkPath)

	if _, err := shell.ExecuteCmd(chroot, "mkdir -p "+tmpDir+"/usr/bin"); err != nil {
		return fmt.Errorf("failed to create dir: %w", err)
	}

	symlinkCmd := fmt.Sprintf("ln -s %s %s%s", replacementBin, tmpDir, replacedBin)
	if _, err := shell.ExecuteCmd(chroot, symlinkCmd); err != nil {
		return fmt.Errorf("failed to create symlink %s → %s: %w", replacedBin, replacementBin, err)
	}

	infoName := fmt.Sprintf("name:%s", replacedPkg)
	mkpkgCmd := fmt.Sprintf(
		"apk mkpkg --files %s --info %s --info version:999 --info description:Dummy %s replaced by %s --info arch:noarch --output %s",
		tmpDir, infoName, replacedPkg, replacementBin, apkPath,
	)
	if _, err := shell.ExecuteCmd(chroot, mkpkgCmd); err != nil {
		return fmt.Errorf("apk mkpkg failed for %s: %w", replacedPkg, err)
	}

	if _, err := shell.ExecuteCmd(chroot, "apk add --allow-untrusted "+apkPath); err != nil {
		return fmt.Errorf("failed to install dummy %s: %w", replacedPkg, err)
	}

	verify, _ := shell.ExecuteCmd(chroot, "apk info -e "+replacedPkg+" 2>/dev/null && echo INSTALLED || echo NOT_INSTALLED")
	if strings.Contains(verify, "INSTALLED") {
		fmt.Printf("[INFO] Dummy %s v999 installed (symlink to %s)\n", replacedPkg, replacementBin)
	} else {
		fmt.Printf("[WARNING] Dummy %s may not be properly installed\n", replacedPkg)
	}

	_, _ = shell.ExecuteCmd(chroot, "rm -rf "+tmpDir+" "+apkPath)
	return nil
}

// SetupPackages installs all required packages inside the chroot.
func SetupPackages(conf *config.Config) error {
	shell.SetCaller("packages")
	chroot := conf.Internal.ChrootDir
	u := conf.User

	fmt.Println("[INFO] Installing packages...")

	if _, err := shell.ExecuteCmd(chroot, "apk update"); err != nil {
		return fmt.Errorf("apk update failed: %w", err)
	}

	if err := InstallDummyPackages(conf); err != nil {
		fmt.Printf("[WARNING] Dummy package installation failed: %v\n", err)
	}

	var allPkgs []string
	allPkgs = append(allPkgs, u.AllPackages...)

	if u.EnableTPM {
		allPkgs = append(allPkgs, "tpm2-tools", "tpm2-tss", "tpm2-tss-tcti-device")
	}

	cpuInfo, _ := shell.ExecuteCmd(chroot, "cat /proc/cpuinfo")
	if strings.Contains(cpuInfo, "AMD") {
		allPkgs = append(allPkgs, "amd-ucode")
	} else if strings.Contains(cpuInfo, "GenuineIntel") {
		allPkgs = append(allPkgs, "intel-ucode")
	}

	firmwarePkgs := DetectFirmwarePackages(chroot)
	allPkgs = append(allPkgs, firmwarePkgs...)

	if u.EnableNix {
		allPkgs = append(allPkgs, "nix", "nix-openrc", "nix-bash-completion", "nix-zsh-completion")
	}

	if u.EnableHardening {
		allPkgs = append(allPkgs, "bubblewrap", "apparmor", "apparmor-profiles", "libapparmor", "apparmor-utils")
	}

	pkgList := strings.Join(allPkgs, " ")
	if _, err := shell.ExecuteCmd(chroot, "apk add "+pkgList); err != nil {
		return fmt.Errorf("failed to install packages: %w", err)
	}

	// Final APK upgrade
	if err := FinalUpgrade(chroot); err != nil {
		fmt.Printf("[WARNING] Final upgrade failed: %v\n", err)
	}

	return nil
}

// FinalUpgrade runs apk update && apk upgrade as the last step.
func FinalUpgrade(chroot string) error {
	fmt.Println("[INFO] Final APK upgrade...")
	if _, err := shell.ExecuteCmd(chroot, "apk update && apk upgrade"); err != nil {
		fmt.Printf("[WARNING] Final apk upgrade failed: %v\n", err)
	}
	return nil
}
