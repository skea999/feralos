package features

import (
	"alpineInstaller/utils/config"
	"alpineInstaller/utils/shell"
	"fmt"
	"os"
	"path/filepath"
)

// TpmPackages returns the APK packages needed for TPM2 support.
// clevis@edge is NOT included — the firstBoot binary adds @edge repos
// and installs clevis itself (it manages its own edge repo lifecycle).
func TpmPackages(conf *config.Config) []string {
	if !conf.User.EnableTPM {
		return nil
	}
	return []string{"tpm2-tools", "tpm2-tss", "tpm2-tss-tcti-device"}
}

// SetupTpm compiles the firstBoot tpm binary and copies it to the target's
// home directory. The binary is autonomous — it checks TPM availability,
// gets LUKS UUIDs, installs clevis, binds to TPM2, rebuilds initramfs + UKI.
// The user runs it after first boot to activate TPM auto-unlock.
func SetupTpm(conf *config.Config) error {
	if !conf.User.EnableTPM {
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

	// Build the TPM binary on the host, output into the chroot
	binPath := filepath.Join(chroot, homeDir, "feralos_tpm")
	buildCmd := fmt.Sprintf(
		"go build -o %s ./firstBoot/tpm/",
		binPath,
	)
	if _, err := shell.ExecuteCmd("", buildCmd); err != nil {
		return fmt.Errorf("failed to build tpm binary: %w", err)
	}

	// Set ownership
	owner := u.UserName + ":" + u.UserName
	if _, err := shell.ExecuteCmd("", fmt.Sprintf("chown %s %s && chmod 755 %s", owner, binPath, binPath)); err != nil {
		return fmt.Errorf("failed to chown tpm binary: %w", err)
	}

	fmt.Printf("[INFO] TPM binary installed at %s/home/%s/feralos_tpm\n", chroot, u.UserName)
	fmt.Println("[INFO] Run it after first boot to activate TPM auto-unlock.")
	return nil
}
