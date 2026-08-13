package hostPhase

import (
	"alpineInstaller/hostPhase/internal"
	"alpineInstaller/utils/config"
	"fmt"
)

// Run executes all host-side operations: bind mounts, bootstrap, APK repos, and hooks.
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

	// 4. Inject APK hooks (EFI sync for dual-disk)
	fmt.Println("[INFO] Step 4: Injecting APK commit hooks...")
	if err := internal.InjectApkHooks(conf.Internal.ChrootDir); err != nil {
		fmt.Printf("[WARNING] APK hooks injection failed: %v\n", err)
	}

	fmt.Println("[INFO] HostPhase.Run completed successfully.")
	return nil
}
