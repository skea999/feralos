package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"alpineInstaller/chrootPhase"
	"alpineInstaller/hostPhase"
	"alpineInstaller/utils/cleanup"
	"alpineInstaller/utils/config"
	"alpineInstaller/utils/createConfig"
	"alpineInstaller/utils/firstboot"
	"alpineInstaller/utils/logic"
)

func main() {
	// 1. Initial Permission Check
	if os.Geteuid() != 0 {
		log.Fatal("[ERROR] This installer must be run with root privileges.")
	}

	// 2. Parse Flags
	userConfigPath := flag.String("config", "variables.yaml", "Path to the user variables.yaml file")
	flag.Parse()

	fmt.Println("--------------------------------------------------")
	fmt.Println("       FERALOS INSTALLER (Go Version)             ")
	fmt.Println("       LUKS + Btrfs RAID1 + booster + UKI         ")
	fmt.Println("--------------------------------------------------")

	// 3. Load Configurations
	fmt.Println("[INFO] Step 1: Loading configuration files...")
	conf, err := config.LoadConfig(*userConfigPath)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("[WARN] No configuration file found. Starting with defaults...")
			conf = &config.Config{}
		} else {
			log.Fatalf("[ERROR] Configuration loading failed: %v", err)
		}
	}

	// 3.1. Run Wizard (skips automatically if unattended: true)
	createConfig.PromptForConfig(conf)

	// 4. Apply Dynamic Rules
	fmt.Println("[INFO] Step 2: Applying dynamic package logic...")
	logic.ApplyRules(conf)

	// 5. Cleanup before host phase (stop swaps, RAIDs, LUKS, unmount)
	fmt.Println("[INFO] Step 3: Cleanup before host phase...")
	if err := cleanup.Cleanup(conf); err != nil {
		fmt.Printf("[WARNING] Pre-host cleanup: %v\n", err)
	}

	// 6. Host Phase (disk layout, mount, bootstrap, chroot prep)
	fmt.Println("[INFO] Step 4: Running host phase...")
	if err := hostPhase.Run(conf); err != nil {
		log.Fatalf("[ERROR] Host phase failed: %v", err)
	}

	// 7. Chroot Phase (packages, services, boot, UKI)
	fmt.Println("[INFO] Step 5: Running chroot phase...")
	if err := chrootPhase.Run(conf); err != nil {
		log.Fatalf("[ERROR] Chroot phase failed: %v", err)
	}

	// 8. FirstBoot setup (install config, compile secureboot/tpm, chmod)
	// Runs AFTER chrootPhase so /etc/kernel/cmdline exists and /mnt is still mounted.
	fmt.Println("[INFO] Step 6: FirstBoot setup...")
	if err := firstboot.WriteInstallConfig(conf); err != nil {
		log.Fatalf("[ERROR] Install config failed: %v", err)
	}
	if err := firstboot.SetupSecureboot(conf); err != nil {
		fmt.Printf("[WARNING] Secureboot setup: %v\n", err)
	}
	if err := firstboot.SetupTpm(conf); err != nil {
		fmt.Printf("[WARNING] TPM setup: %v\n", err)
	}
	if err := firstboot.SetupWebmin(conf); err != nil {
		fmt.Printf("[WARNING] Webmin setup: %v\n", err)
	}

	// 9. Wait for filesystems to sync before shutdown
	fmt.Println("[INFO] Waiting 20s for filesystem sync...")
	time.Sleep(20 * time.Second)

	// 10. Final cleanup — DISABLED: causes BTRFS Transaction aborted -5, forced readonly
	// during runner shutdown (see runner-serial.log BTRFS error). Disks stay mounted
	// until poweroff; kernel will sync on shutdown. Was: cleanup.Cleanup(conf)
	fmt.Println("[INFO] Step 7: Final cleanup...")
	if err := cleanup.Cleanup(conf); err != nil {
		fmt.Printf("[WARNING] Final cleanup: %v\n", err)
	}

	fmt.Println("--------------------------------------------------")
	fmt.Println("   INSTALLATION COMPLETED SUCCESSFULLY!          ")
	fmt.Println("--------------------------------------------------")
}
