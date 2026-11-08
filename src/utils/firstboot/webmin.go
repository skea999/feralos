package firstboot

import "feralos/src/utils/config"

// SetupWebmin compiles the firstBoot webmin binary host-side and builds it
// into the target at /opt/feralos/bin/feralos-webmin. Called after
// chrootPhase so the target is still mounted and firstBoot binaries can be
// placed into it.
func SetupWebmin(conf *config.Config) error {
	if !conf.User.EnableWebmin {
		return nil
	}
	return buildBinary(conf, "webmin", "webmin")
}
