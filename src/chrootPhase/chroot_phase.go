package chrootPhase

import (
	"feralos/src/chrootPhase/desktops"
	"feralos/src/chrootPhase/features"
	"feralos/src/chrootPhase/internal"
	"feralos/src/utils/config"
	"feralos/src/utils/shell"
	"fmt"
)

// Run executes all final configuration steps inside Alpine chroot environment.
func Run(conf *config.Config) error {
	i := &conf.Internal
	chroot := i.ChrootDir

	fmt.Println("[INFO] Starting ChrootPhase.Run...")

	// 0. Write resolv.conf BEFORE first apk update (DNS needed for apk)
	resolvConf := "nameserver 208.67.222.222\nnameserver 208.67.220.220\n"
	if err := shell.WriteFile(chroot, "/etc/resolv.conf", []byte(resolvConf), 0644); err != nil {
		return fmt.Errorf("failed to write resolv.conf: %w", err)
	}

	// 1. Install packages (apk add)
	if err := internal.SetupPackages(conf); err != nil {
		return fmt.Errorf("package installation failed: %w", err)
	}

	// 1b. Inject EFI sync hook (dual-disk only, runs after packages installed)
	if err := internal.InjectEfiSyncHook(chroot); err != nil {
		fmt.Printf("[WARNING] EFI sync hook: %v\n", err)
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

	// 7. Setup nix (optional) — must run before desktop setup so nix-installed
	// binaries are available for configuration steps (DMS is a native apk
	// since feralos-aports dms-shell; nix is an explicit enableNix feature).
	if err := features.SetupNix(conf); err != nil {
		return fmt.Errorf("nix setup failed: %w", err)
	}

	// 8. Setup flatpak (optional) — installs flatpak apps from flathub
	if err := features.SetupFlatpak(conf); err != nil {
		return fmt.Errorf("flatpak setup failed: %w", err)
	}

	// 9. Setup desktop environment (configurations — kde/gnome need none)
	if err := desktops.Setup(conf); err != nil {
		return fmt.Errorf("desktop setup failed: %w", err)
	}

	// 10. Setup zswap (server always enabled; other layouts toggleable)
	if err := features.SetupZswapServer(conf); err != nil {
		return fmt.Errorf("zswap server setup failed: %w", err)
	}
	if err := features.SetupZswap(conf); err != nil {
		return fmt.Errorf("zswap setup failed: %w", err)
	}

	// 11. Setup storage
	if err := internal.SetupStorage(conf); err != nil {
		return fmt.Errorf("storage setup failed: %w", err)
	}

	// 12. Setup hardening
	if err := features.SetupHardening(conf); err != nil {
		return fmt.Errorf("hardening failed: %w", err)
	}

	// 13. Setup podman (rootless containers)
	if err := features.SetupPodman(conf); err != nil {
		return fmt.Errorf("podman setup failed: %w", err)
	}

	// 14. Setup boot (booster config, cmdline, hooks)
	if err := internal.SetupBoot(conf); err != nil {
		return fmt.Errorf("boot setup failed: %w", err)
	}

	// 15. Build unsigned UKI
	fmt.Println("[INFO] Building unsigned UKI...")
	if err := internal.BuildUKI(conf); err != nil {
		return fmt.Errorf("UKI build failed: %w", err)
	}

	fmt.Println("[INFO] ChrootPhase.Run completed successfully.")
	return nil
}
