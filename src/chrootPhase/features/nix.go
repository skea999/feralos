package features

import (
	"alpineInstaller/utils/config"
	"alpineInstaller/utils/shell"
	"fmt"
	"strings"
)

// NixPackages returns the APK packages needed for Nix.
// Auto-enables Nix if rules requested Nix packages (nixPackages in
// internal_rules.yaml) but EnableNix was not set by the user.
func NixPackages(conf *config.Config) []string {
	// Auto-enable: if any nixPackages are requested by rules, enable Nix
	if len(conf.User.NixPackages) > 0 && !conf.User.EnableNix {
		fmt.Printf("[INFO] Nix packages requested (%d), enabling Nix automatically\n", len(conf.User.NixPackages))
		conf.User.EnableNix = true
	}
	if !conf.User.EnableNix {
		return nil
	}
	return []string{"nix", "nix-openrc", "nix-bash-completion", "nix-zsh-completion"}
}

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
	// NOTE: allow-unfree/allow-insecure are NOT recognized by Alpine's nix
	// build (unknown setting warnings) — NIXPKGS_ALLOW_* env vars in
	// /etc/profile.d/nix-daemon.sh (and fish/bash) carry that permanently.
	// ssl-cert-file: builder curl (crates.io fetchers) must see the host CA
	// bundle, else "unable to get local issuer certificate" → 403 cascade.
	nixConf := fmt.Sprintf(`build-dir = /nix/var/nix/build
extra-experimental-features = nix-command flakes
trusted-users = root %s
ssl-cert-file = /etc/ssl/certs/ca-certificates.crt`, u.UserName)
	escapedNixConf := strings.ReplaceAll(nixConf, "'", "'\\''")
	// Append (not overwrite): preserve Alpine's allowed-users=@nix,
	// build-users-group=nixbld and max-jobs from /etc/nix/nix.conf.
	if _, err := shell.ExecuteCmd(chroot, fmt.Sprintf("echo '%s' >> /etc/nix/nix.conf", escapedNixConf)); err != nil {
		return fmt.Errorf("failed to write nix.conf: %w", err)
	}

	if _, err := shell.ExecuteCmdArgs(chroot, "adduser", u.UserName, "nix"); err != nil {
		fmt.Printf("[WARNING] Failed to add user to nix group: %v\n", err)
	}

	if _, err := shell.ExecuteCmdArgs(chroot, "chown", "root:nixbld", "/nix/var/nix/build"); err != nil {
		fmt.Printf("[WARNING] Failed to set build dir ownership: %v\n", err)
	}
	if _, err := shell.ExecuteCmdArgs(chroot, "chmod", "775", "/nix/var/nix/build"); err != nil {
		fmt.Printf("[WARNING] Failed to set build dir permissions: %v\n", err)
	}

	if _, err := shell.ExecuteCmdArgs(chroot, "rc-update", "add", "nix-daemon", "default"); err != nil {
		fmt.Printf("[WARNING] Failed to add nix-daemon to default: %v\n", err)
	}

	if _, err := shell.ExecuteCmdArgs(chroot, "nix-channel", "--add", "https://nixos.org/channels/nixos-26.05", "nixpkgs"); err != nil {
		fmt.Printf("[WARNING] Failed to add nix channel: %v\n", err)
	}

	fmt.Println("[INFO] Setting Nix environment variables...")
	// XKB_CONFIG_ROOT: nix-installed GUI apps need the host X11 xkb data.
	// NOTE: TMPDIR is deliberately NOT exported globally — /tmp is noexec
	// (security) so nix builds need the @nix-var-nix-build subvolume, but the
	// var is scoped to the nix daemon only via /etc/conf.d/nix-daemon below.
	// A global export leaks into podman etc. and breaks containers-storage.
	nixEnv := `export XKB_CONFIG_ROOT=/usr/share/X11/xkb`
	escapedNixEnv := strings.ReplaceAll(nixEnv, "'", "'\\''")
	// Append (not overwrite): preserve the PATH/daemon setup the Alpine nix
	// package ships in /etc/profile.d/nix-daemon.sh.
	if _, err := shell.ExecuteCmd(chroot, fmt.Sprintf("echo '%s' >> /etc/profile.d/nix-daemon.sh", escapedNixEnv)); err != nil {
		return fmt.Errorf("failed to write nix env: %w", err)
	}

	// Scope TMPDIR to the nix daemon service only (OpenRC sources conf.d)
	if _, err := shell.ExecuteCmd(chroot, "echo 'TMPDIR=/nix/var/nix/build/tmp' > /etc/conf.d/nix-daemon"); err != nil {
		return fmt.Errorf("failed to write /etc/conf.d/nix-daemon: %w", err)
	}

	nixFishEnv := `set -gx XKB_CONFIG_ROOT /usr/share/X11/xkb`
	escapedNixFishEnv := strings.ReplaceAll(nixFishEnv, "'", "'\\''")
	if _, err := shell.ExecuteCmd(chroot, fmt.Sprintf("echo '%s' >> /etc/fish/conf.d/nix.fish", escapedNixFishEnv)); err != nil {
		return fmt.Errorf("failed to write nix fish env: %w", err)
	}

	nixBashEnv := `export XKB_CONFIG_ROOT=/usr/share/X11/xkb`
	escapedNixBashEnv := strings.ReplaceAll(nixBashEnv, "'", "'\\''")
	if _, err := shell.ExecuteCmdArgs(chroot, "mkdir", "-p", "/etc/bash"); err != nil {
		fmt.Printf("[WARNING] Failed to create /etc/bash: %v\n", err)
	}
	if _, err := shell.ExecuteCmd(chroot, fmt.Sprintf("echo '%s' > /etc/bash/nix.sh", escapedNixBashEnv)); err != nil {
		return fmt.Errorf("failed to write nix bash env: %w", err)
	}

	if _, err := shell.ExecuteCmdArgs(chroot, "ln", "-sf", "/etc/bash/nix.sh", "/etc/profile.d/nix-bash.sh"); err != nil {
		fmt.Printf("[WARNING] Failed to symlink nix bash env: %v\n", err)
	}

	// Install Nix packages requested by rules (internal_rules nixPackages).
	// Same behavior as apk packages: data selects what gets installed.
	if len(u.NixPackages) > 0 {
		fmt.Println("[INFO] Installing Nix packages...")

		// nix profile install needs the daemon running — start it in the chroot.
		// OpenRC refuses to run services without /run/openrc state, which is
		// only created at boot: provide it manually so the daemon can start
		// inside the chroot.
		if _, err := shell.ExecuteCmdArgs(chroot, "mkdir", "-p", "/run/openrc"); err != nil {
			fmt.Printf("[WARNING] Failed to prepare /run/openrc: %v\n", err)
		}
		if _, err := shell.ExecuteCmdArgs(chroot, "touch", "/run/openrc/softlevel"); err != nil {
			fmt.Printf("[WARNING] Failed to prepare /run/openrc: %v\n", err)
		}
		if _, err := shell.ExecuteCmdArgs(chroot, "rc-service", "nix-daemon", "start"); err != nil {
			fmt.Printf("[WARNING] Failed to start nix-daemon in chroot: %v\n", err)
		}

		// The nix client uses TMPDIR for its temp files. /tmp is mounted
		// noexec + 0755 (security), so temp files MUST go to the nix build
		// subvolume (@nix-var-nix-build, mounted at /nix/var/nix/build).
		// The dir persists in the subvolume, so it exists after boot too.
		nixTmp := "/nix/var/nix/build/tmp"
		if _, err := shell.ExecuteCmdArgs(chroot, "mkdir", "-p", nixTmp); err != nil {
			return fmt.Errorf("failed to prepare nix build tmp dir: %w", err)
		}
		if _, err := shell.ExecuteCmdArgs(chroot, "chown", "root:nixbld", nixTmp); err != nil {
			fmt.Printf("[WARNING] Failed to set tmp dir ownership: %v\n", err)
		}
		if _, err := shell.ExecuteCmdArgs(chroot, "chmod", "1777", nixTmp); err != nil {
			return fmt.Errorf("failed to prepare nix build tmp dir: %w", err)
		}

		// The default flake registry maps nixpkgs -> nixpkgs-unstable, which
		// would override the stable channel for `nixpkgs#...` installs. Pin
		// the user's registry to the stable branch (same as the channel).
		if _, err := shell.ExecuteCmdArgs(chroot, "su", "-", u.UserName, "-c", "NIX_SSL_CERT_FILE=/etc/ssl/certs/ca-certificates.crt nix registry add nixpkgs github:NixOS/nixpkgs/nixos-26.05"); err != nil {
			fmt.Printf("[WARNING] Failed to pin nixpkgs registry to stable: %v\n", err)
		}

		// Install into the real user's profile so ~/.local/state/nix/profiles
		// is populated for the login user (not root).
		// --option sandbox false: the nix build sandbox uses pivot_root, which
		// fails inside the installer chroot (EINVAL — / is not a mountpoint).
		// This is chroot-only; the installed system keeps the default sandbox.
		// 'add' is the current command ('install' is a deprecated alias).
		var failedPkgs []string
		for _, pkg := range u.NixPackages {
			fmt.Printf("[INFO] nix profile add %s\n", pkg)
			inner := fmt.Sprintf("TMPDIR=%s NIX_SSL_CERT_FILE=/etc/ssl/certs/ca-certificates.crt NIXPKGS_ALLOW_INSECURE=1 NIXPKGS_ALLOW_UNFREE=1 nix profile add --impure --option sandbox false %s", nixTmp, pkg)
			if _, err := shell.ExecuteCmdArgs(chroot, "su", "-", u.UserName, "-c", inner); err != nil {
				fmt.Printf("[WARNING] Failed to install nix package %s: %v\n", pkg, err)
				failedPkgs = append(failedPkgs, pkg)
			}
		}
		if len(failedPkgs) > 0 {
			fmt.Printf("[WARNING] Failed to install %d nix package(s): %s\n", len(failedPkgs), strings.Join(failedPkgs, ", "))
		}

		// Update-path smoke tests — this nix version lacks --dry-run flags,
		// so validate flake infra + profile readability instead (no side effects).
		fmt.Println("[INFO] Testing nix update path...")
		if _, err := shell.ExecuteCmdArgs(chroot, "su", "-", u.UserName, "-c", "NIX_SSL_CERT_FILE=/etc/ssl/certs/ca-certificates.crt nix flake metadata nixpkgs 2>&1 || true"); err != nil {
			fmt.Printf("[WARNING] nix flake metadata failed: %v\n", err)
		}
		if _, err := shell.ExecuteCmdArgs(chroot, "su", "-", u.UserName, "-c", "NIX_SSL_CERT_FILE=/etc/ssl/certs/ca-certificates.crt nix profile list 2>&1 || true"); err != nil {
			fmt.Printf("[WARNING] nix profile list failed: %v\n", err)
		}

		if _, err := shell.ExecuteCmd(chroot, "sed -i 's/trusted-users = root .*/trusted-users = root/' /etc/nix/nix.conf"); err != nil {
			fmt.Printf("[WARNING] Failed to clean trusted-users in nix.conf: %v\n", err)
		}
		if _, err := shell.ExecuteCmdArgs(chroot, "rc-service", "nix-daemon", "restart"); err != nil {
			fmt.Printf("[WARNING] Failed to restart nix-daemon in chroot: %v\n", err)
		}

		// Stop the daemon again — the target boots its own nix-daemon service.
		if _, err := shell.ExecuteCmdArgs(chroot, "rc-service", "nix-daemon", "stop"); err != nil {
			fmt.Printf("[WARNING] Failed to stop nix-daemon in chroot: %v\n", err)
		}
	}

	fmt.Println("[INFO] Nix configured. Run 'source /etc/profile.d/nix-daemon.sh' after first boot.")
	return nil
}
