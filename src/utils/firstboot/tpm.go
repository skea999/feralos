package firstboot

import "feralos/src/utils/config"

// SetupTpm compiles the firstBoot tpm binary host-side and builds it into
// the target at /opt/feralos/bin/feralos-tpm. Called after chrootPhase so
// the target is still mounted and firstBoot binaries can be placed into it.
func SetupTpm(conf *config.Config) error {
	if !conf.User.EnableTPM {
		return nil
	}
	return buildBinary(conf, "tpm", "tpm")
}
