package features

import (
	"alpineInstaller/utils/config"
	"alpineInstaller/utils/shell"
	"fmt"
	"os"
	"path/filepath"
)

// SecurebootPackages returns the APK packages needed for Secure Boot setup.
// These are installed during the chroot phase so the firstBoot binary has
// everything it needs when the user runs it after first boot.
func SecurebootPackages(conf *config.Config) []string {
	if !conf.User.EnableSecureboot {
		return nil
	}
	return []string{
		"efi-mkkeys",
		"efitools",
		"secureboot-hook",
		"systemd-efistub",
		"sbctl",
	}
}

// SetupSecureboot compiles the firstBoot secureboot binary and copies it to
// the target's home directory. The binary is autonomous — it handles key
// generation, sbctl enrollment, UKI build/sign, and hook config. The user
// runs it after first boot to activate Secure Boot.
func SetupSecureboot(conf *config.Config) error {
	if !conf.User.EnableSecureboot {
		return nil
	}

	chroot := conf.Internal.ChrootDir
	u := &conf.User

	homeDir := "/home/" + u.UserName

	// Ensure home directory exists (SetupUsers runs before features)
	if _, err := os.Stat(filepath.Join(chroot, homeDir)); os.IsNotExist(err) {
		if err := os.MkdirAll(filepath.Join(chroot, homeDir), 0755); err != nil {
			return fmt.Errorf("failed to create home dir: %w", err)
		}
	}

	// Build the secureboot binary on the host, output into the chroot
	binPath := filepath.Join(chroot, homeDir, "feralos_secureboot")
	buildCmd := fmt.Sprintf(
		"go build -o %s ./firstBoot/secureboot/",
		binPath,
	)
	if _, err := shell.ExecuteCmd("", buildCmd); err != nil {
		return fmt.Errorf("failed to build secureboot binary: %w", err)
	}

	// Set ownership (chroot dir is mounted, chown works via host)
	owner := u.UserName + ":" + u.UserName
	if _, err := shell.ExecuteCmd("", fmt.Sprintf("chown %s %s && chmod 755 %s", owner, binPath, binPath)); err != nil {
		return fmt.Errorf("failed to chown secureboot binary: %w", err)
	}

	fmt.Printf("[INFO] Secureboot binary installed at %s/home/%s/feralos_secureboot\n", chroot, u.UserName)
	fmt.Println("[INFO] Run it after first boot to activate Secure Boot.")
	return nil
}
