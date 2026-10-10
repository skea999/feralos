package features

import (
	"feralos/src/utils/config"
)

// WebminPackages returns the APK packages needed for Webmin.
// Webmin runs in a Podman container — no APK packages needed on host.
// The actual setup is handled by the firstBoot/webmin standalone binary.
func WebminPackages(conf *config.Config) []string {
	if !conf.User.EnableWebmin {
		return nil
	}
	return nil
}
