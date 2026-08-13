package chrootPhase

import (
	"alpineInstaller/chrootPhase/internal"
	"alpineInstaller/chrootPhase/internal/features"
	"alpineInstaller/utils/config"
	"alpineInstaller/utils/shell"
	"fmt"
)

// Run executes all final configuration steps inside Alpine chroot environment.
func Run(conf *config.Config) error {
	i := &conf.Internal
	chroot := i.ChrootDir

	fmt.Println("[INFO] Starting ChrootPhase.Run...")

	// 1. Install packages (apk add)
	if err := internal.SetupPackages(conf); err != nil {
		return fmt.Errorf("package installation failed: %w", err)
	}

	// 1a. List installed packages
	fmt.Println("[INFO] Installed packages:")
	pkgList, err := shell.ExecuteCmd(chroot, "apk info")
	if err != nil {
		fmt.Printf("[WARNING] Failed to list packages: %v\n", err)
	}

	// 2. Setup logging (vector config — AFTER package install)
	if err := internal.SetupLogging(conf, pkgList); err != nil {
		fmt.Printf("[WARNING] Vector config failed: %v\n", err)
	}

	// 3. Setup network
	if err := internal.SetupNetwork(conf); err != nil {
		return fmt.Errorf("network setup failed: %w", err)
	}

	// 4. Setup misc
	if err := internal.SetupMisc(conf); err != nil {
		return fmt.Errorf("misc setup failed: %w", err)
	}

	// 5. Setup users
	if err := internal.SetupUsers(conf); err != nil {
		return fmt.Errorf("user setup failed: %w", err)
	}

	// 6. Setup services (OpenRC runlevels)
	if err := internal.SetupServices(conf); err != nil {
		return fmt.Errorf("service setup failed: %w", err)
	}

	// 7. Setup zswap (dmcrypt + zswap)
	if err := features.SetupZswap(conf); err != nil {
		return fmt.Errorf("zswap setup failed: %w", err)
	}

	// 8. Setup nix (optional)
	if err := features.SetupNix(conf); err != nil {
		return fmt.Errorf("nix setup failed: %w", err)
	}

	// 9. Setup storage
	if err := internal.SetupStorage(conf); err != nil {
		return fmt.Errorf("storage setup failed: %w", err)
	}

	// 10. Setup hardening
	if err := features.SetupHardening(conf); err != nil {
		return fmt.Errorf("hardening failed: %w", err)
	}

	// 11. Setup boot (booster config, cmdline, hooks)
	if err := internal.SetupBoot(conf); err != nil {
		return fmt.Errorf("boot setup failed: %w", err)
	}

	// 12. Build unsigned UKI
	fmt.Println("[INFO] Building unsigned UKI...")
	if err := internal.BuildUKI(conf); err != nil {
		return fmt.Errorf("UKI build failed: %w", err)
	}

	fmt.Println("[INFO] ChrootPhase.Run completed successfully.")
	return nil
}
