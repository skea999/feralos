package features

import (
	"alpineInstaller/utils/config"
	"alpineInstaller/utils/shell"
	"fmt"
)

// SetupTpm compiles the firstBoot tpm binary and copies it to the target's
// home directory. Runs during hostPhase (go build on host, output written
// to mounted chroot). The binary is autonomous — it checks TPM, installs
// clevis, binds LUKS to TPM2, rebuilds initramfs + UKI. The user runs
// it after first boot to activate TPM auto-unlock.
func SetupTpm(conf *config.Config) error {
	if !conf.User.EnableTPM {
		return nil
	}

	chroot := conf.Internal.ChrootDir

	// Build the binary on host (cd src for go.mod), output into mounted chroot
	binPath := chroot + "/opt/feralos_tpm"
	buildCmd := fmt.Sprintf("cd src && go build -o %s ./firstBoot/tpm/", binPath)
	if _, err := shell.ExecuteCmd("", buildCmd); err != nil {
		return fmt.Errorf("failed to build tpm binary: %w", err)
	}

	fmt.Printf("[INFO] TPM binary: %s/opt/feralos_tpm\n", chroot)
	return nil
}
