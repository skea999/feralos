package features

import (
	"alpineInstaller/utils/config"
	"alpineInstaller/utils/shell"
	"fmt"
)

// SetupSecureboot handles secureboot-related cleanup in the chroot.
// Currently removes the legacy mkinitfs secureboot hook that is no longer
// needed when using booster + UKI. Only runs when EnableSecureboot is true.
func SetupSecureboot(conf *config.Config) error {
	if !conf.User.EnableSecureboot {
		return nil
	}

	chroot := conf.Internal.ChrootDir

	fmt.Println("[INFO] Secureboot: removing legacy mkinitfs hook...")
	if _, err := shell.ExecuteCmd(chroot, "rm -f /etc/kernel-hooks.d/50-secureboot.hook"); err != nil {
		fmt.Printf("[WARNING] Failed to remove old secureboot hook: %v\n", err)
	}

	return nil
}
