package firstboot

import "alpineInstaller/utils/config"

// SetupWebmin compiles the firstBoot webmin binary and copies it to
// /mnt/opt/feralos_webmin. Called after chrootPhase so /mnt is still
// mounted and firstBoot binaries can be placed into the target.
func SetupWebmin(conf *config.Config) error {
	if !conf.User.EnableWebmin {
		return nil
	}
	return buildBinary(conf, "webmin", "webmin")
}
