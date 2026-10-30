package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"feralos/src/chrootPhase"
	"feralos/src/hostPhase"
	"feralos/src/utils/cleanup"
	"feralos/src/utils/config"
	"feralos/src/utils/createConfig"
	"feralos/src/utils/firstboot"
	"feralos/src/utils/logic"
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
		switch {
		case os.IsNotExist(err):
			fmt.Println("[WARN] No configuration file found. Starting with defaults...")
			conf = &config.Config{}
			// Rules needed by the wizard (enum options for layout/DE/video)
			if rulesErr := config.LoadInternalRules(conf); rulesErr != nil {
				log.Fatalf("[ERROR] Configuration loading failed: %v", rulesErr)
			}
		case conf != nil:
			// File loaded but values invalid — the wizard below starts
			// pre-filled so the user can fix them; re-validated after it.
			fmt.Printf("[WARN] Configuration validation failed: %v\n", err)
			fmt.Println("[WARN] Entering the wizard so the values can be corrected...")
		default:
			log.Fatalf("[ERROR] Configuration loading failed: %v", err)
		}
	}

	// 3.1. Run Wizard (skips automatically if unattended: true)
	createConfig.PromptForConfig(conf)

	// 3.2. Re-validate before any disk work — the wizard pre-fills from the
	// file and can fix invalid values; anything still invalid (e.g. an
	// unattended run with a broken config) stops here, before host phase.
	if err := config.ValidateConfig(conf); err != nil {
		log.Fatalf("[ERROR] Configuration invalid after wizard: %v", err)
	}

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

	// 9. Wait for filesystems to sync before final cleanup
	fmt.Println("[INFO] Waiting 20s for filesystem sync...")
	time.Sleep(20 * time.Second)

	// 10. Final cleanup (always runs) — after the 20s sync wait, unmounts
	// the chroot mounts, stops swaps/RAIDs and closes LUKS (see cleanup.Cleanup).
	// BTRFS Transaction aborted -5 during runner shutdown is a QEMU race,
	// allowlisted in log_errors; see CHANGELOG 2026-08-29.
	fmt.Println("[INFO] Step 7: Final cleanup...")
	if err := cleanup.Cleanup(conf); err != nil {
		fmt.Printf("[WARNING] Final cleanup: %v\n", err)
	}

	fmt.Println("--------------------------------------------------")
	fmt.Println("   INSTALLATION COMPLETED SUCCESSFULLY!          ")
	fmt.Println("--------------------------------------------------")
}
