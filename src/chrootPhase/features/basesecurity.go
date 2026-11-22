package features

import (
	"feralos/src/utils/config"
	"fmt"
)

// BaseSecurity — the baseSecurity variable (ADR-014): the MAC baseline.
// ONE package: base-security-config pulls the whole family as apk
// dependencies (the AppArmor stack + the LSM cmdline fragment with the UKI
// rebuild trigger). ADR-013 R2 — complete functionality in one package.
//
// There is no Setup work in Go: every artifact is package-shipped (the
// cmdline fragment + trigger in the aports package) and the service
// enablement (apparmor: boot) rides the base-security-config packageLogic
// key in internal_rules.yaml — dep-installed packages have no reachable
// rules key of their own (the clamav/hardening pattern). This file exists
// as the feature's single owner (feature confinement): the packages list
// and the INFO trail.
func BaseSecurityPackages(conf *config.Config) []string {
	if !conf.User.EnableBaseSecurity {
		return nil
	}
	return []string{"base-security-config"}
}

// SetupBaseSecurity applies the base-security feature inside the chroot.
// Everything is packaged; the chroot work is only the INFO trail.
func SetupBaseSecurity(conf *config.Config) error {
	if !conf.User.EnableBaseSecurity {
		fmt.Println("[INFO] Base security disabled, skipping...")
		return nil
	}
	fmt.Println("[INFO] Base security: AppArmor baseline is package-shipped (base-security-config).")
	return nil
}
