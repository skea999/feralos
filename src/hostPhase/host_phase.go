package hostPhase

import (
	"alpineInstaller/hostPhase/features"
	"alpineInstaller/hostPhase/internal"
	"alpineInstaller/utils/config"
	"fmt"
)

// Run executes all host-side operations: bind mounts, bootstrap, APK repos,
// hooks, and firstBoot binary compilation.
func Run(conf *config.Config) error {
	fmt.Println("[INFO] Starting HostPhase.Run...")

	// 1. Mount virtual filesystems (proc, sys, dev, etc.)
	fmt.Println("[INFO] Step 1: Mounting virtual filesystems...")
	if err := internal.MountVirtualFilesystems(conf.Internal.ChrootDir); err != nil {
		return fmt.Errorf("bind mounts failed: %w", err)
	}

	// 2. Bootstrap Alpine base system
	fmt.Println("[INFO] Step 2: Bootstrapping Alpine base system...")
	if err := internal.BootstrapAlpine(conf); err != nil {
		return fmt.Errorf("bootstrap failed: %w", err)
	}

	// 3. Prepare chroot environment (APK repos)
	fmt.Println("[INFO] Step 3: Preparing chroot environment...")
	if err := internal.PrepareChrootEnv(conf); err != nil {
		return fmt.Errorf("chroot prep failed: %w", err)
	}

	// 4. Write install config (disk layout, LUKS UUIDs, user info)
	if err := features.WriteInstallConfig(conf); err != nil {
		return fmt.Errorf("install config failed: %w", err)
	}

	// 6. Compile and install firstBoot binaries (autonomous post-install tools)
	if err := features.SetupSecureboot(conf); err != nil {
		fmt.Printf("[WARNING] secureboot setup: %v\n", err)
	}
	if err := features.SetupTpm(conf); err != nil {
		fmt.Printf("[WARNING] tpm setup: %v\n", err)
	}

	fmt.Println("[INFO] HostPhase.Run completed successfully.")
	return nil
}
