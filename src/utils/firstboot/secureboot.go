package firstboot

import (
	"alpineInstaller/utils/config"
	"alpineInstaller/utils/shell"
	"fmt"
)

// SetupSecureboot compiles the firstBoot secureboot binary and copies it to
// /mnt/opt/feralos_secureboot. Called after chrootPhase so /mnt is still
// mounted and firstBoot binaries can be placed into the target.
func SetupSecureboot(conf *config.Config) error {
	if !conf.User.EnableSecureboot {
		return nil
	}

	chroot := conf.Internal.ChrootDir

	// Build the binary on host (cd src for go.mod), output into mounted chroot
	binPath := chroot + "/opt/feralos_secureboot"
	buildCmd := fmt.Sprintf("cd src && go build -o %s ./firstBoot/secureboot/", binPath)
	if _, err := shell.ExecuteCmd("", buildCmd); err != nil {
		return fmt.Errorf("failed to build secureboot binary: %w", err)
	}

	// Set executable permissions
	if _, err := shell.ExecuteCmd("", fmt.Sprintf("chmod 755 %s", binPath)); err != nil {
		return fmt.Errorf("failed to chmod secureboot binary: %w", err)
	}

	fmt.Printf("[INFO] Secureboot binary: %s\n", binPath)
	return nil
}
