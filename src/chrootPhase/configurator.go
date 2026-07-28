package chrootPhase

import (
	"alpineInstaller/utils/config"
	"alpineInstaller/utils/files"
	"fmt"
	"strings"
)

// Run executes all final configuration steps inside Alpine chroot environment.
// This is the main orchestrator that calls modular functions.
// hostPhase has already written config files; chrootPhase runs commands.
func Run(conf *config.Config) error {
	u := &conf.User
	i := &conf.Internal
	chroot := i.ChrootDir

	fmt.Println("[INFO] Starting ChrootPhase.Run...")

	// 1. Install packages (apk add)
	if err := InstallPackages(conf); err != nil {
		return fmt.Errorf("package installation failed: %w", err)
	}

	// 2. Setup services (OpenRC, aliases, symlinks, timezone, pager)
	if err := SetupServices(conf); err != nil {
		return fmt.Errorf("service setup failed: %w", err)
	}

	// 3. Setup users (passwords, groups, SSH, shell)
	if err := SetupUsers(conf); err != nil {
		return fmt.Errorf("user setup failed: %w", err)
	}

	// 4. Setup dmcrypt swap (keyfile, luksAddKey)
	fmt.Println("[INFO] Configuring LUKS swap with dmcrypt...")
	if err := SetupDmcryptSwap(conf); err != nil {
		return fmt.Errorf("dmcrypt swap setup failed: %w", err)
	}

	// 5. Zswap configuration (optional, no disk dependencies)
	if err := SetupZswap(conf); err != nil {
		return fmt.Errorf("zswap setup failed: %w", err)
	}

	// 6. Nix package manager (optional)
	if err := SetupNix(conf); err != nil {
		return fmt.Errorf("nix setup failed: %w", err)
	}

	// 7. Hardening (optional)
	if u.EnableHardening {
		fmt.Println("[INFO] Applying hardening...")
		if err := WriteHardening(conf); err != nil {
			return fmt.Errorf("hardening failed: %w", err)
		}
	}

	// 7. Final kernel cmdline update (appends zswap + hardening args)
	fmt.Println("[INFO] Writing final kernel cmdline...")
	finalCmdline := strings.Join(u.KernelCmdline, " ")
	if err := files.WriteFileInChroot(chroot, "/etc/kernel/cmdline", finalCmdline+"\n"); err != nil {
		return fmt.Errorf("failed to write final kernel cmdline: %w", err)
	}

	// 8. Build signed UKI
	fmt.Println("[INFO] Building signed UKI...")
	if err := BuildAndDeployUKI(conf); err != nil {
		return fmt.Errorf("UKI build failed: %w", err)
	}

	// 9. Final APK upgrade
	if err := FinalUpgrade(chroot); err != nil {
		fmt.Printf("[WARNING] Final upgrade failed: %v\n", err)
	}

	fmt.Println("[INFO] ChrootPhase.Run completed successfully.")
	return nil
}
