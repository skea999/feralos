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
	chroot := conf.Internal.ChrootDir
	u := conf.User

	fmt.Println("[INFO] Installing packages...")

	if _, err := shell.RunChrootCommand(chroot, "apk update"); err != nil {
		return fmt.Errorf("apk update failed: %w", err)
	}

	// Add TPM packages if enabled
	tpmPackages := ""
	if u.EnableTPM {
		tpmPackages = " tpm2-tools tpm2-tss tpm2-tss-tcti-device"
	}

	pkgList := strings.Join(u.AllPackages, " ") + tpmPackages
	if _, err := shell.RunChrootCommand(chroot, "apk add "+pkgList); err != nil {
		return fmt.Errorf("failed to install packages: %w", err)
	}

	// Install earlyoom (separate for clarity)
	if _, err := shell.RunChrootCommand(chroot, "apk add earlyoom"); err != nil {
		return fmt.Errorf("failed to install earlyoom: %w", err)
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
