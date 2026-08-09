package chrootPhase

import (
	"alpineInstaller/utils/config"
	"alpineInstaller/utils/files"
	"alpineInstaller/utils/shell"
	"fmt"
)

// SetupNix installs and configures the Nix package manager.
// Controlled by conf.User.EnableNix flag.
// Subvolumes (@nix-store, @nix-var-nix, @nix-var-log, @nix-var-nix-build)
// are created separately in the disk layout.
func SetupNix(conf *config.Config) error {
	if !conf.User.EnableNix {
		fmt.Println("[INFO] Nix disabled, skipping...")
		return nil
	}

	chroot := conf.Internal.ChrootDir
	u := conf.User

	fmt.Println("[INFO] Installing and configuring Nix...")

	// Nix packages are installed in InstallPackages (packages.go)

	// 1. Configure nix daemon
	fmt.Println("[INFO] Configuring Nix daemon...")
	nixConf := `build-dir = /nix/var/nix/build
extra-experimental-features = nix-command flakes
`
	if err := files.WriteFileInChroot(chroot, "/etc/nix/nix.conf", nixConf); err != nil {
		return fmt.Errorf("failed to write nix.conf: %w", err)
	}

	// 3. Add user to nix group
	if _, err := shell.RunChrootCommand(chroot, fmt.Sprintf("adduser %s nix", u.UserName)); err != nil {
		fmt.Printf("[WARNING] Failed to add user to nix group: %v\n", err)
	}

	// 4. Set build directory permissions
	if _, err := shell.RunChrootCommand(chroot, "chown root:nixbld /nix/var/nix/build && chmod 775 /nix/var/nix/build"); err != nil {
		fmt.Printf("[WARNING] Failed to set build dir permissions: %v\n", err)
	}

	// 5. Enable nix-daemon service
	if _, err := shell.RunChrootCommand(chroot, "rc-update add nix-daemon default"); err != nil {
		fmt.Printf("[WARNING] Failed to add nix-daemon to default: %v\n", err)
	}

	// 6. Add stable channel
	if _, err := shell.RunChrootCommand(chroot, "nix-channel --add https://nixos.org/channels/nixos-26.05 nixpkgs"); err != nil {
		fmt.Printf("[WARNING] Failed to add nix channel: %v\n", err)
	}
	if _, err := shell.RunChrootCommand(chroot, "nix-channel --update"); err != nil {
		fmt.Printf("[WARNING] Failed to update nix channel: %v\n", err)
	}

	// 7. Environment variables for Nix
	fmt.Println("[INFO] Setting Nix environment variables...")
	nixEnv := `export NIXPKGS_ALLOW_INSECURE=1
export NIXPKGS_ALLOW_UNFREE=1
export XKB_CONFIG_ROOT=/usr/share/X11/xkb
`
	if err := files.WriteFileInChroot(chroot, "/etc/profile.d/nix-daemon.sh", nixEnv); err != nil {
		return fmt.Errorf("failed to write nix env: %w", err)
	}

	// 8. Fish shell integration
	nixFishEnv := `set -gx NIXPKGS_ALLOW_INSECURE 1
set -gx NIXPKGS_ALLOW_UNFREE 1
set -gx XKB_CONFIG_ROOT /usr/share/X11/xkb
`
	if err := files.WriteFileInChroot(chroot, "/etc/fish/conf.d/nix.fish", nixFishEnv); err != nil {
		return fmt.Errorf("failed to write nix fish env: %w", err)
	}

	// 9. Bash shell integration
	nixBashEnv := `export NIXPKGS_ALLOW_INSECURE=1
export NIXPKGS_ALLOW_UNFREE=1
export XKB_CONFIG_ROOT=/usr/share/X11/xkb
`
	if err := files.WriteFileInChroot(chroot, "/etc/bash/nix.sh", nixBashEnv); err != nil {
		return fmt.Errorf("failed to write nix bash env: %w", err)
	}

	// 10. Symlink to bash profile.d
	if _, err := shell.RunChrootCommand(chroot, "ln -sf /etc/bash/nix.sh /etc/profile.d/nix-bash.sh"); err != nil {
		fmt.Printf("[WARNING] Failed to symlink nix bash env: %v\n", err)
	}

	fmt.Println("[INFO] Nix configured. Run 'source /etc/profile.d/nix-daemon.sh' after first boot.")
	return nil
}
