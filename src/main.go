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
	"alpineInstaller/utils/config"
	"alpineInstaller/utils/createConfig"
	"alpineInstaller/utils/logic"
	"alpineInstaller/utils/shell"
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

	// 5. Check Host Tools
	fmt.Println("[INFO] Step 3: Checking host dependencies...")
	if err := checkHostDependencies(); err != nil {
		log.Fatalf("[ERROR] Missing host tools: %v", err)
	}

	// 6. Disk Partitioning & LUKS Setup
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
	fmt.Println("[INFO] Step 4: Mounting virtual filesystems...")
	if err := hostPhase.MountVirtualFilesystems(conf.Internal.ChrootDir); err != nil {
		log.Fatalf("[ERROR] Bind mounts failed: %v", err)
	}

	// 8. Alpine Bootstrap
	fmt.Println("[INFO] Step 5: Bootstrapping Alpine base system...")
	if err := hostPhase.BootstrapAlpine(conf); err != nil {
		log.Fatalf("[ERROR] Bootstrap failed: %v", err)
	}

	// 9. Generate LUKS keyfile and embed in initramfs
	fmt.Println("[INFO] Step 6: Generating LUKS keyfile for auto-unlock...")
	keyfileArgs, err := hostPhase.GenerateAndEmbedKeyfile(conf)
	if err != nil {
		log.Fatalf("[ERROR] Keyfile generation failed: %v", err)
	}
	conf.User.KernelCmdline = append(conf.User.KernelCmdline, keyfileArgs...)

	// 10. Host Side Preparation
	fmt.Println("[INFO] Step 7: Preparing configuration files from host...")
	if err := hostPhase.PrepareChrootEnv(conf); err != nil {
		log.Fatalf("[ERROR] Chroot prep failed: %v", err)
	}

	// 11. Write Booster Config (booster.yaml + kernel cmdline)
	fmt.Println("[INFO] Step 8: Writing booster configuration...")
	if err := hostPhase.WriteBoosterConfig(conf); err != nil {
		log.Fatalf("[ERROR] Booster config failed: %v", err)
	}

	// 12. Write Booster Hook (kernel upgrade hook)
	fmt.Println("[INFO] Step 9: Writing booster kernel hook...")
	if err := hostPhase.WriteBoosterHook(conf.Internal.ChrootDir); err != nil {
		log.Fatalf("[ERROR] Booster hook failed: %v", err)
	}

	// 13. Write Secureboot Config
	fmt.Println("[INFO] Step 10: Writing secureboot hook config...")
	if err := hostPhase.WriteSecurebootConf(conf.Internal.ChrootDir); err != nil {
		log.Fatalf("[ERROR] Secureboot config failed: %v", err)
	}

	// 14. Inject APK Hooks
	fmt.Println("[INFO] Step 11: Injecting APK commit hooks...")
	if err := hostPhase.InjectApkHooks(conf.Internal.ChrootDir); err != nil {
		log.Fatalf("[ERROR] Hooks injection failed: %v", err)
	}

	// 15. Chroot Phase (Final Internal Configuration)
	fmt.Println("[INFO] Step 12: Entering Chroot for final configuration...")
	if err := chrootPhase.Run(conf); err != nil {
		log.Fatalf("[ERROR] Chroot phase failed: %v", err)
	}

	fmt.Println("--------------------------------------------------")
	fmt.Println("   INSTALLATION COMPLETED SUCCESSFULLY!          ")
	fmt.Println("--------------------------------------------------")
}

// checkHostDependencies ensures the Live environment has the necessary tools.
func checkHostDependencies() error {
	shell.SetCaller("deps")
	deps := []string{"btrfs-progs", "cryptsetup", "curl", "tar", "dosfstools", "rsync", "efitools", "sbsigntool"}

	fmt.Println("[DEBUG] Updating apt and installing required host tools")
	if _, err := shell.RunCommand("apt", "update"); err != nil {
		return fmt.Errorf("apt update failed: %w", err)
	}

	args := append([]string{"install", "-y"}, deps...)
	_, err := shell.RunCommand("apt", args...)
	return err
}
