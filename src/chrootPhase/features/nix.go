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
	// trusted-users: lets the wheel user pass --option sandbox false during
	// chroot install (the nix build sandbox pivot_root fails inside the
	// installer chroot). The installed system keeps the default sandbox.
	nixConf := `build-dir = /nix/var/nix/build
extra-experimental-features = nix-command flakes
trusted-users = root @wheel`
	escapedNixConf := strings.ReplaceAll(nixConf, "'", "'\\''")
	// Append (not overwrite): preserve Alpine's allowed-users=@nix,
	// build-users-group=nixbld and max-jobs from /etc/nix/nix.conf.
	if _, err := shell.ExecuteCmd(chroot, fmt.Sprintf("echo '%s' >> /etc/nix/nix.conf", escapedNixConf)); err != nil {
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
	// TMPDIR points at the nix build subvolume: /tmp is mounted noexec + 0755
	// (security), so nix temp files must live on @nix-var-nix-build. This is
	// permanent — future builds by the user also use the subvolume.
	nixEnv := `export NIXPKGS_ALLOW_INSECURE=1
export NIXPKGS_ALLOW_UNFREE=1
export XKB_CONFIG_ROOT=/usr/share/X11/xkb
export TMPDIR=/nix/var/nix/build/tmp`
	escapedNixEnv := strings.ReplaceAll(nixEnv, "'", "'\\''")
	// Append (not overwrite): preserve the PATH/daemon setup the Alpine nix
	// package ships in /etc/profile.d/nix-daemon.sh.
	if _, err := shell.ExecuteCmd(chroot, fmt.Sprintf("echo '%s' >> /etc/profile.d/nix-daemon.sh", escapedNixEnv)); err != nil {
		return fmt.Errorf("failed to write nix env: %w", err)
	}

	nixFishEnv := `set -gx NIXPKGS_ALLOW_INSECURE 1
set -gx NIXPKGS_ALLOW_UNFREE 1
set -gx XKB_CONFIG_ROOT /usr/share/X11/xkb
set -gx TMPDIR /nix/var/nix/build/tmp`
	escapedNixFishEnv := strings.ReplaceAll(nixFishEnv, "'", "'\\''")
	if _, err := shell.ExecuteCmd(chroot, fmt.Sprintf("echo '%s' >> /etc/fish/conf.d/nix.fish", escapedNixFishEnv)); err != nil {
		return fmt.Errorf("failed to write nix fish env: %w", err)
	}

	nixBashEnv := `export NIXPKGS_ALLOW_INSECURE=1
export NIXPKGS_ALLOW_UNFREE=1
export XKB_CONFIG_ROOT=/usr/share/X11/xkb
export TMPDIR=/nix/var/nix/build/tmp`
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

	// Install Nix packages requested by rules (internalRules nixPackages).
	// Same behavior as apk packages: data selects what gets installed.
	if len(u.NixPackages) > 0 {
		fmt.Println("[INFO] Installing Nix packages...")

		// nix profile install needs the daemon running — start it in the chroot.
		// OpenRC refuses to run services without /run/openrc state, which is
		// only created at boot: provide it manually so the daemon can start
		// inside the chroot.
		if _, err := shell.ExecuteCmd(chroot, "mkdir -p /run/openrc && touch /run/openrc/softlevel"); err != nil {
			fmt.Printf("[WARNING] Failed to prepare /run/openrc: %v\n", err)
		}
		if _, err := shell.ExecuteCmd(chroot, "rc-service nix-daemon start"); err != nil {
			fmt.Printf("[WARNING] Failed to start nix-daemon in chroot: %v\n", err)
		}

		// The nix client uses TMPDIR for its temp files. /tmp is mounted
		// noexec + 0755 (security), so temp files MUST go to the nix build
		// subvolume (@nix-var-nix-build, mounted at /nix/var/nix/build).
		// The dir persists in the subvolume, so it exists after boot too.
		nixTmp := "/nix/var/nix/build/tmp"
		if _, err := shell.ExecuteCmd(chroot, fmt.Sprintf("mkdir -p %s && chmod 1777 %s", nixTmp, nixTmp)); err != nil {
			return fmt.Errorf("failed to prepare nix build tmp dir: %w", err)
		}

		// The default flake registry maps nixpkgs -> nixpkgs-unstable, which
		// would override the stable channel for `nixpkgs#...` installs. Pin
		// the user's registry to the stable branch (same as the channel).
		if _, err := shell.ExecuteCmd(chroot, fmt.Sprintf("su - %s -c 'nix registry add nixpkgs github:NixOS/nixpkgs/nixos-26.05'", u.UserName)); err != nil {
			fmt.Printf("[WARNING] Failed to pin nixpkgs registry to stable: %v\n", err)
		}

		// Install into the real user's profile so ~/.local/state/nix/profiles
		// is populated for the login user (not root).
		// --option sandbox false: the nix build sandbox uses pivot_root, which
		// fails inside the installer chroot (EINVAL — / is not a mountpoint).
		// This is chroot-only; the installed system keeps the default sandbox.
		// 'add' is the current command ('install' is a deprecated alias).
		for _, pkg := range u.NixPackages {
			fmt.Printf("[INFO] nix profile add %s\n", pkg)
			cmd := fmt.Sprintf("su - %s -c 'TMPDIR=%s NIXPKGS_ALLOW_INSECURE=1 NIXPKGS_ALLOW_UNFREE=1 nix profile add --option sandbox false %s'", u.UserName, nixTmp, pkg)
			if _, err := shell.ExecuteCmd(chroot, cmd); err != nil {
				return fmt.Errorf("failed to install nix package %s: %w", pkg, err)
			}
		}

		// Stop the daemon again — the target boots its own nix-daemon service.
		if _, err := shell.ExecuteCmd(chroot, "rc-service nix-daemon stop"); err != nil {
			fmt.Printf("[WARNING] Failed to stop nix-daemon in chroot: %v\n", err)
		}
	}

	fmt.Println("[INFO] Nix configured. Run 'source /etc/profile.d/nix-daemon.sh' after first boot.")
	return nil
}
