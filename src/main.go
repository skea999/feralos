package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"alpineInstaller/chrootPhase"
	"alpineInstaller/hostPhase"
	"alpineInstaller/hostPhase/disks/btrfsStandard"
	"alpineInstaller/hostPhase/disks/btrfsStandardServer"
	"alpineInstaller/utility"
)

func main() {
	// 1. Initial Permission Check
	// The installer must write to block devices and perform mounts
	if os.Geteuid() != 0 {
		log.Fatal("[ERROR] This installer must be run with root privileges.")
	}

	// 2. Parse Flags
	// Allow user to specify a custom path for user variables
	userConfigPath := flag.String("config", "variables.yaml", "Path to the user variables.yaml file")
	flag.Parse()

	fmt.Println("--------------------------------------------------")
	fmt.Println("       ALPINE LINUX INSTALLER (Go Version)        ")
	fmt.Println("--------------------------------------------------")

	// 3. Load Configurations
	fmt.Println("[INFO] Step 1: Loading configuration files...")
	conf, err := utility.LoadConfig(*userConfigPath)
	if err != nil {
		// If error is that file doesn't exist, proceed with empty struct for wizard
		if os.IsNotExist(err) {
			fmt.Println("[WARN] No configuration file found. Starting with defaults...")
			conf = &utility.Config{}
			// Initialize minimum required fields if needed
		} else {
			log.Fatalf("[ERROR] Configuration loading failed: %v", err)
		}
	}

	// 3.1. Run Wizard (skips automatically if unattended: true)
	utility.PromptForConfig(conf)

	// 4. Apply Dynamic Rules
	// Iterate over packages and inject necessary groups, modules, and services
	fmt.Println("[INFO] Step 2: Applying dynamic package logic...")
	utility.ApplyRules(conf)

	// 5. Check Host Tools (Dependencies needed on Mint Live)
	fmt.Println("[INFO] Step 3: Checking host dependencies...")
	if err := checkHostDependencies(); err != nil {
		log.Fatalf("[ERROR] Missing host tools: %v", err)
	}

	// 6. Disk Partitioning & RAID Setup
	fmt.Printf("[INFO] Selected Disk Layout: %s\n", conf.User.DiskLayout)

	if len(conf.User.Disks) == 0 {
		log.Fatal("[ERROR] No target disks selected. Cannot proceed with partitioning.")
	}

	switch conf.User.DiskLayout {
	case "btrfsStandardServer":
		if err := btrfsStandardServer.RunBtrfsSetup(conf); err != nil {
			log.Fatalf("[ERROR] btrfsStandardServer setup failed: %v", err)
		}
	case "btrfsStandard":
		if err := btrfsStandard.RunBtrfsSetup(conf); err != nil {
			log.Fatalf("[ERROR] btrfsStandard setup failed: %v", err)
		}
	default:
		log.Fatalf("[ERROR] Unknown disk layout: %s", conf.User.DiskLayout)
	}

	// 7. Bind Mounts (Virtual Filesystems)
	// Preparing /dev, /proc, /sys for chroot
	fmt.Println("[INFO] Step 5: Mounting virtual filesystems...")
	if err := hostPhase.MountVirtualFilesystems(conf.Internal.ChrootDir); err != nil {
		log.Fatalf("[ERROR] Bind mounts failed: %v", err)
	}

	// 8. Alpine Bootstrap
	// Downloading and extracting the base Alpine rootfs
	fmt.Println("[INFO] Step 6: Bootstrapping Alpine base system...")
	if err := hostPhase.BootstrapAlpine(conf); err != nil {
		log.Fatalf("[ERROR] Bootstrap failed: %v", err)
	}

	// 9. Host Side Preparation
	// Writing config files (network, modules, grub, limits) from Host to Guest
	fmt.Println("[INFO] Step 7: Preparing configuration files from host...")
	if err := hostPhase.PrepareChrootEnv(conf); err != nil {
		log.Fatalf("[ERROR] Chroot prep failed: %v", err)
	}

	// 10. Inject APK Hooks
	// Setup kernel fallback and EFI sync scripts
	fmt.Println("[INFO] Step 8: Injecting APK commit hooks...")
	if err := hostPhase.InjectApkHooks(conf.Internal.ChrootDir); err != nil {
		log.Fatalf("[ERROR] Hooks injection failed: %v", err)
	}

	// 11. Chroot Phase (Final Internal Configuration)
	// Running apk add, adding users, setting up runlevels, grub-install
	fmt.Println("[INFO] Step 9: Entering Chroot for final configuration...")
	if err := chrootPhase.Run(conf); err != nil {
		log.Fatalf("[ERROR] Chroot phase failed: %v", err)
	}

	fmt.Println("--------------------------------------------------")
	fmt.Println("   INSTALLATION COMPLETED SUCCESSFULLY!          ")
	fmt.Println("--------------------------------------------------")
}

// checkHostDependencies ensures the Live environment has the necessary tools
func checkHostDependencies() error {
	deps := []string{"mdadm", "btrfs-progs", "curl", "tar", "dosfstools", "rsync"}

	fmt.Println("[DEBUG] Updating apt and installing required host tools")
	if _, err := utility.RunCommand("apt", "update"); err != nil {
		return fmt.Errorf("apt update failed: %w", err)
	}

	args := append([]string{"install", "-y"}, deps...)
	_, err := utility.RunCommand("apt", args...)
	return err
}
