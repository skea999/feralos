package features

import (
	"alpineInstaller/utils/config"
	"alpineInstaller/utils/shell"
	"fmt"
)

// SetupSecureboot compiles the firstBoot secureboot binary and copies it to
// the target's home directory. Runs during hostPhase (go build on host,
// output written to mounted chroot). The binary is autonomous — it handles
// package installation, key generation, sbctl enrollment, UKI build/sign,
// and hook config. The user runs it after first boot.
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

	fmt.Printf("[INFO] Secureboot binary: %s/opt/feralos_secureboot\n", chroot)
	return nil
}
