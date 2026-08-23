package features

import (
	"alpineInstaller/utils/config"
	"fmt"
)

// TpmPackages returns the APK packages needed for TPM2 support.
func TpmPackages(conf *config.Config) []string {
	if !conf.User.EnableTPM {
		return nil
	}
	return []string{"tpm2-tools", "tpm2-tss", "tpm2-tss-tcti-device"}
}

// SetupTPM is a placeholder for future TPM-specific configuration.
// Currently TPM package installation is handled via TpmPackages in packages.go.
func SetupTPM(conf *config.Config) error {
	if !conf.User.EnableTPM {
		return nil
	}
	fmt.Println("[INFO] TPM enabled.")
	return nil
}
