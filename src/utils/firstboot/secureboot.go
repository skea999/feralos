package firstboot

import "feralos/src/utils/config"

// SetupSecureboot compiles the firstBoot secureboot binary host-side and
// builds it into the target at /opt/feralos/bin/feralos-secureboot. Called
// after chrootPhase so the target is still mounted and firstBoot binaries
// can be placed into it.
func SetupSecureboot(conf *config.Config) error {
	if !conf.User.EnableSecureboot {
		return nil
	}
	return buildBinary(conf, "secureboot", "secureboot")
}
