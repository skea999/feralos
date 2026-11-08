package firstboot

import "feralos/src/utils/config"

// SetupTpm compiles the firstBoot tpm binary and copies it to
// /mnt/opt/feralos/bin/feralos-tpm. Called after chrootPhase so /mnt is
// still mounted and firstBoot binaries can be placed into the target.
// mounted and firstBoot binaries can be placed into the target.
func SetupTpm(conf *config.Config) error {
	if !conf.User.EnableTPM {
		return nil
	}
	return buildBinary(conf, "tpm", "tpm")
}
