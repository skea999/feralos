package features

import (
	"alpineInstaller/utils/config"
	"alpineInstaller/utils/shell"
	"fmt"
	"strings"
)

// FlatpakPackages returns the APK packages needed for Flatpak.
// Returns ["flatpak"] when any flatpak apps are requested, enabling the
// flatpak runtime inside the chroot.
func FlatpakPackages(conf *config.Config) []string {
	if len(conf.User.FlatpakPackages) == 0 {
		return nil
	}
	return []string{"flatpak"}
}

// SetupFlatpak configures Flathub remote and installs requested flatpak apps.
func SetupFlatpak(conf *config.Config) error {
	if len(conf.User.FlatpakPackages) == 0 {
		fmt.Println("[INFO] Flatpak disabled (no flatpakPackages), skipping...")
		return nil
	}

	chroot := conf.Internal.ChrootDir
	fmt.Println("[INFO] Configuring Flatpak...")

	// Add Flathub remote (idempotent)
	if _, err := shell.ExecuteCmd(chroot, "flatpak remote-add --if-not-exists flathub https://flathub.org/repo/flathub.flatpakrepo"); err != nil {
		fmt.Printf("[WARNING] Failed to add flathub remote: %v\n", err)
		// continue — remote may already exist or network transient
	}

	// Install each flatpak app: entries are "flathub:com.foo.Bar" or "com.foo.Bar"
	for _, entry := range conf.User.FlatpakPackages {
		if strings.TrimSpace(entry) == "" {
			continue
		}
		remote, appID, ok := strings.Cut(entry, ":")
		if !ok {
			remote = "flathub"
			appID = entry
		}
		if remote == "" {
			remote = "flathub"
		}
		if appID == "" {
			fmt.Printf("[WARNING] Skipping empty flatpak entry %q\n", entry)
			continue
		}
		fmt.Printf("[INFO] Installing flatpak %s from %s\n", appID, remote)
		// Use non-interactive install; allow failure per-package (network, etc.)
		if _, err := shell.ExecuteCmdArgs(chroot, "flatpak", "install", "-y", "--noninteractive", remote, appID); err != nil {
			fmt.Printf("[WARNING] Failed to install flatpak %s: %v\n", appID, err)
			continue
		}
	}

	fmt.Println("[INFO] Flatpak configured.")
	return nil
}
