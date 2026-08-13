package features

import (
	"alpineInstaller/utils/config"
	"alpineInstaller/utils/shell"
	"fmt"
	"strings"
)

// SetupNix installs and configures the Nix package manager.
func SetupNix(conf *config.Config) error {
	if !conf.User.EnableNix {
		fmt.Println("[INFO] Nix disabled, skipping...")
		return nil
	}

	chroot := conf.Internal.ChrootDir
	u := conf.User

	fmt.Println("[INFO] Installing and configuring Nix...")

	fmt.Println("[INFO] Configuring Nix daemon...")
	nixConf := `build-dir = /nix/var/nix/build
extra-experimental-features = nix-command flakes`
	escapedNixConf := strings.ReplaceAll(nixConf, "'", "'\\''")
	if _, err := shell.ExecuteCmd(chroot, fmt.Sprintf("echo '%s' > /etc/nix/nix.conf", escapedNixConf)); err != nil {
		return fmt.Errorf("failed to write nix.conf: %w", err)
	}

	if _, err := shell.ExecuteCmd(chroot, fmt.Sprintf("adduser %s nix", u.UserName)); err != nil {
		fmt.Printf("[WARNING] Failed to add user to nix group: %v\n", err)
	}

	if _, err := shell.ExecuteCmd(chroot, "chown root:nixbld /nix/var/nix/build && chmod 775 /nix/var/nix/build"); err != nil {
		fmt.Printf("[WARNING] Failed to set build dir permissions: %v\n", err)
	}

	if _, err := shell.ExecuteCmd(chroot, "rc-update add nix-daemon default"); err != nil {
		fmt.Printf("[WARNING] Failed to add nix-daemon to default: %v\n", err)
	}

	if _, err := shell.ExecuteCmd(chroot, "nix-channel --add https://nixos.org/channels/nixos-26.05 nixpkgs"); err != nil {
		fmt.Printf("[WARNING] Failed to add nix channel: %v\n", err)
	}

	fmt.Println("[INFO] Setting Nix environment variables...")
	nixEnv := `export NIXPKGS_ALLOW_INSECURE=1
export NIXPKGS_ALLOW_UNFREE=1
export XKB_CONFIG_ROOT=/usr/share/X11/xkb`
	escapedNixEnv := strings.ReplaceAll(nixEnv, "'", "'\\''")
	if _, err := shell.ExecuteCmd(chroot, fmt.Sprintf("echo '%s' > /etc/profile.d/nix-daemon.sh", escapedNixEnv)); err != nil {
		return fmt.Errorf("failed to write nix env: %w", err)
	}

	nixFishEnv := `set -gx NIXPKGS_ALLOW_INSECURE 1
set -gx NIXPKGS_ALLOW_UNFREE 1
set -gx XKB_CONFIG_ROOT /usr/share/X11/xkb`
	escapedNixFishEnv := strings.ReplaceAll(nixFishEnv, "'", "'\\''")
	if _, err := shell.ExecuteCmd(chroot, fmt.Sprintf("echo '%s' > /etc/fish/conf.d/nix.fish", escapedNixFishEnv)); err != nil {
		return fmt.Errorf("failed to write nix fish env: %w", err)
	}

	nixBashEnv := `export NIXPKGS_ALLOW_INSECURE=1
export NIXPKGS_ALLOW_UNFREE=1
export XKB_CONFIG_ROOT=/usr/share/X11/xkb`
	escapedNixBashEnv := strings.ReplaceAll(nixBashEnv, "'", "'\\''")
	if _, err := shell.ExecuteCmd(chroot, "mkdir -p /etc/bash"); err != nil {
		fmt.Printf("[WARNING] Failed to create /etc/bash: %v\n", err)
	}
	if _, err := shell.ExecuteCmd(chroot, fmt.Sprintf("echo '%s' > /etc/bash/nix.sh", escapedNixBashEnv)); err != nil {
		return fmt.Errorf("failed to write nix bash env: %w", err)
	}

	if _, err := shell.ExecuteCmd(chroot, "ln -sf /etc/bash/nix.sh /etc/profile.d/nix-bash.sh"); err != nil {
		fmt.Printf("[WARNING] Failed to symlink nix bash env: %v\n", err)
	}

	fmt.Println("[INFO] Nix configured. Run 'source /etc/profile.d/nix-daemon.sh' after first boot.")
	return nil
}
