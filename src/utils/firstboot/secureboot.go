package firstboot

import "feralos/src/utils/config"

// SetupSecureboot compiles the firstBoot secureboot binary and copies it to
// /mnt/opt/feralos_secureboot. Called after chrootPhase so /mnt is still
// mounted and firstBoot binaries can be placed into the target.
func SetupSecureboot(conf *config.Config) error {
	if !conf.User.EnableSecureboot {
		return nil
	}
	return buildBinary(conf, "secureboot", "secureboot")
}
