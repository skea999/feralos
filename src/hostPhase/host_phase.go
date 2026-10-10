package hostPhase

import (
	"feralos/src/hostPhase/disks/btrfsRaid0"
	"feralos/src/hostPhase/disks/btrfsStandard"
	"feralos/src/hostPhase/disks/btrfsStandardServer"
	"feralos/src/hostPhase/disks/btrfsUnencrypted"
	"feralos/src/hostPhase/internal"
	"feralos/src/utils/config"
	"fmt"
)

// Run executes all host-side operations: disk layout, mount, bootstrap,
// chroot preparation. Called by main after cleanup.Cleanup() and before
// chrootPhase.Run(). No cleanup (done in main), no firstBoot (done in main).
func Run(conf *config.Config) error {
	fmt.Println("[INFO] Starting HostPhase.Run...")

	// 1. Disk Partitioning & LUKS Setup
	fmt.Printf("[INFO] Step 1: Disk layout %s\n", conf.User.DiskLayout)
	if len(conf.User.Disks) == 0 {
		return fmt.Errorf("no target disks selected; cannot partition")
	}
	switch conf.User.DiskLayout {
	case "btrfsStandardServer":
		if err := btrfsStandardServer.RunBtrfsSetup(conf); err != nil {
			return fmt.Errorf("btrfsStandardServer setup failed: %w", err)
		}
	case "btrfsStandard":
		if err := btrfsStandard.RunBtrfsSetup(conf); err != nil {
			return fmt.Errorf("btrfsStandard setup failed: %w", err)
		}
	case "btrfsRaid0":
		if err := btrfsRaid0.RunBtrfsSetup(conf); err != nil {
			return fmt.Errorf("btrfsRaid0 setup failed: %w", err)
		}
	case "btrfsUnencrypted":
		if err := btrfsUnencrypted.RunBtrfsSetup(conf); err != nil {
			return fmt.Errorf("btrfsUnencrypted setup failed: %w", err)
		}
	default:
		return fmt.Errorf("unknown disk layout: %s", conf.User.DiskLayout)
	}

	// 2. Mount virtual filesystems (proc, sys, dev, etc.)
	fmt.Println("[INFO] Step 2: Mounting virtual filesystems...")
	if err := internal.MountVirtualFilesystems(conf.Internal.ChrootDir); err != nil {
		return fmt.Errorf("bind mounts failed: %w", err)
	}

	// 3. Bootstrap Alpine base system
	fmt.Println("[INFO] Step 3: Bootstrapping Alpine base system...")
	if err := internal.BootstrapAlpine(conf); err != nil {
		return fmt.Errorf("bootstrap failed: %w", err)
	}

	// 4. Prepare chroot environment (APK repos)
	fmt.Println("[INFO] Step 4: Preparing chroot environment...")
	if err := internal.PrepareChrootEnv(conf); err != nil {
		return fmt.Errorf("chroot prep failed: %w", err)
	}

	fmt.Println("[INFO] HostPhase.Run completed successfully.")
	return nil
}
