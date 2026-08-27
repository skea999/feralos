package firstboot

import (
	"alpineInstaller/utils/config"
	"alpineInstaller/utils/shell"
	"fmt"
)

// SetupTpm compiles the firstBoot tpm binary and copies it to
// /mnt/opt/feralos_tpm. Called after chrootPhase so /mnt is still
// mounted and firstBoot binaries can be placed into the target.
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

	// Set executable permissions
	if _, err := shell.ExecuteCmd("", fmt.Sprintf("chmod 755 %s", binPath)); err != nil {
		return fmt.Errorf("failed to chmod tpm binary: %w", err)
	}

	fmt.Printf("[INFO] TPM binary: %s\n", binPath)
	return nil
}
