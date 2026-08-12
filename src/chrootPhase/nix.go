package chrootPhase

import (
	"alpineInstaller/utils/config"
	"alpineInstaller/utils/shell"
	"fmt"
	"strings"
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
extra-experimental-features = nix-command flakes`
	// Escape single quotes in nixConf for shell execution
	escapedNixConf := strings.ReplaceAll(nixConf, "'", "'\\''")
	if _, err := shell.ExecuteCmd(chroot, fmt.Sprintf("echo '%s' > /etc/nix/nix.conf", escapedNixConf)); err != nil {
		return fmt.Errorf("failed to write nix.conf: %w", err)
	}

	// 3. Add user to nix group
	if _, err := shell.ExecuteCmd(chroot, fmt.Sprintf("adduser %s nix", u.UserName)); err != nil {
		fmt.Printf("[WARNING] Failed to add user to nix group: %v\n", err)
	}

	// 4. Set build directory permissions
	if _, err := shell.ExecuteCmd(chroot, "chown root:nixbld /nix/var/nix/build && chmod 775 /nix/var/nix/build"); err != nil {
		fmt.Printf("[WARNING] Failed to set build dir permissions: %v\n", err)
	}

	// 5. Enable nix-daemon service
	if _, err := shell.ExecuteCmd(chroot, "rc-update add nix-daemon default"); err != nil {
		fmt.Printf("[WARNING] Failed to add nix-daemon to default: %v\n", err)
	}

	// 6. Add stable channel
	if _, err := shell.ExecuteCmd(chroot, "nix-channel --add https://nixos.org/channels/nixos-26.05 nixpkgs"); err != nil {
		fmt.Printf("[WARNING] Failed to add nix channel: %v\n", err)
	}
	// Note: nix-channel --update is not run in chroot (pivot_root fails)
	// Channel will be updated on first boot when nix-daemon starts

	// 7. Environment variables for Nix
	fmt.Println("[INFO] Setting Nix environment variables...")
	nixEnv := `export NIXPKGS_ALLOW_INSECURE=1
export NIXPKGS_ALLOW_UNFREE=1
export XKB_CONFIG_ROOT=/usr/share/X11/xkb`
	// Escape single quotes in nixEnv for shell execution
	escapedNixEnv := strings.ReplaceAll(nixEnv, "'", "'\\''")
	if _, err := shell.ExecuteCmd(chroot, fmt.Sprintf("echo '%s' > /etc/profile.d/nix-daemon.sh", escapedNixEnv)); err != nil {
		return fmt.Errorf("failed to write nix env: %w", err)
	}

	// 8. Fish shell integration
	nixFishEnv := `set -gx NIXPKGS_ALLOW_INSECURE 1
set -gx NIXPKGS_ALLOW_UNFREE 1
set -gx XKB_CONFIG_ROOT /usr/share/X11/xkb`
	// Escape single quotes in nixFishEnv for shell execution
	escapedNixFishEnv := strings.ReplaceAll(nixFishEnv, "'", "'\\''")
	if _, err := shell.ExecuteCmd(chroot, fmt.Sprintf("echo '%s' > /etc/fish/conf.d/nix.fish", escapedNixFishEnv)); err != nil {
		return fmt.Errorf("failed to write nix fish env: %w", err)
	}

	// 9. Bash shell integration
	nixBashEnv := `export NIXPKGS_ALLOW_INSECURE=1
export NIXPKGS_ALLOW_UNFREE=1
export XKB_CONFIG_ROOT=/usr/share/X11/xkb`
	// Escape single quotes in nixBashEnv for shell execution
	escapedNixBashEnv := strings.ReplaceAll(nixBashEnv, "'", "'\\''")
	// Create /etc/bash/ directory if it doesn't exist
	if _, err := shell.ExecuteCmd(chroot, "mkdir -p /etc/bash"); err != nil {
		fmt.Printf("[WARNING] Failed to create /etc/bash: %v\n", err)
	}
	if _, err := shell.ExecuteCmd(chroot, fmt.Sprintf("echo '%s' > /etc/bash/nix.sh", escapedNixBashEnv)); err != nil {
		return fmt.Errorf("failed to write nix bash env: %w", err)
	}

	// 10. Symlink to bash profile.d
	if _, err := shell.ExecuteCmd(chroot, "ln -sf /etc/bash/nix.sh /etc/profile.d/nix-bash.sh"); err != nil {
		fmt.Printf("[WARNING] Failed to symlink nix bash env: %v\n", err)
	}

	fmt.Println("[INFO] Nix configured. Run 'source /etc/profile.d/nix-daemon.sh' after first boot.")
	return nil
}
