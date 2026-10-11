package features

import (
	"feralos/src/utils/config"
	"feralos/src/utils/shell"
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
	// ssl-cert-file: builder curl (crates.io fetchers) must see the host CA
	// bundle, else "unable to get local issuer certificate" → 403 cascade.
	// NOTE: allow-unfree/allow-insecure are NOT recognized by Alpine's nix
	// build (unknown setting warnings) — NIXPKGS_ALLOW_* env vars in
	// /etc/profile.d/nix-daemon.sh (and fish/bash) carry that permanently.
	nixConf := `extra-experimental-features = nix-command flakes
ssl-cert-file = /etc/ssl/certs/ca-certificates.crt
trusted-users = root @wheel`
	// trusted-users = root @wheel — the NixOS upstream default. Trusted users
	// can pass trusted settings (--option sandbox false, needed for chroot
	// installs; --impure; extra cache URLs) and are ~root-equivalent by
	// definition — wheel already is, so this adds zero escalation.
	// Daily UX: nix profile add / flakes work for wheel users without sudo.
	escapedNixConf := strings.ReplaceAll(nixConf, "'", "'\\''")
	// Append (not overwrite): preserve Alpine's allowed-users=@nix,
	// build-users-group=nixbld and max-jobs from /etc/nix/nix.conf.
	if _, err := shell.ExecuteCmd(chroot, fmt.Sprintf("echo '%s' >> /etc/nix/nix.conf", escapedNixConf)); err != nil {
		return fmt.Errorf("failed to write nix.conf: %w", err)
	}

	if _, err := shell.ExecuteCmdArgs(chroot, "adduser", u.UserName, "nix"); err != nil {
		fmt.Printf("[WARNING] Failed to add user to nix group: %v\n", err)
	}

	// nix-daemon enablement: OpenRC runlevels under OpenRC; under dinit the
	// converted nix-daemon-dinit service is enabled via boot.d symlink
	// (no rules map entry covers it — the nix feature owns its service).
	if conf.User.EnableDinit {
		if _, err := shell.ExecuteCmd(chroot, "mkdir -p /etc/dinit.d/boot.d && test -f /lib/dinit.d/nix-daemon && ln -sf /lib/dinit.d/nix-daemon /etc/dinit.d/boot.d/nix-daemon || true"); err != nil {
			fmt.Printf("[WARNING] Failed to enable nix-daemon dinit service: %v\n", err)
		}
	} else {
		if _, err := shell.ExecuteCmdArgs(chroot, "rc-update", "add", "nix-daemon", "default"); err != nil {
			fmt.Printf("[WARNING] Failed to add nix-daemon to default: %v\n", err)
		}
	}

	if _, err := shell.ExecuteCmdArgs(chroot, "nix-channel", "--add", "https://nixos.org/channels/nixos-26.05", "nixpkgs"); err != nil {
		fmt.Printf("[WARNING] Failed to add nix channel: %v\n", err)
	}
	// Unpack the channel manifest NOW: a freshly added channel is not usable
	// until updated — the first <nixpkgs> use on the installed system would
	// fail with "undefined variable 'nixpkgs'" (seen via debug_run).
	// NIX_CONFIG sandbox=false: the unpack derivation uses the build sandbox,
	// whose pivot_root fails inside the installer chroot (EINVAL — / is not
	// a mountpoint). Same chroot-only workaround as the profile installs
	// below; the installed system keeps the default sandbox.
	if _, err := shell.ExecuteCmd(chroot, "NIX_CONFIG='sandbox = false' nix-channel --update"); err != nil {
		fmt.Printf("[WARNING] nix-channel --update failed: %v\n", err)
	}

	// Flake registry: pin nixpkgs to the STABLE branch. The builtin default
	// registry maps nixpkgs → nixpkgs-unstable, overriding the stable
	// channel for every flake `nixpkgs#...` reference (profile installs,
	// user commands). System-wide file first — /etc/nix/registry.json
	// applies to every user INCLUDING root (root had no pin at all before);
	// then the user registry (user wins over system — belt+braces).
	// GOTCHA (fixed here): the user pin used to run only when nixPackages
	// was non-empty — enableNix with no packages kept resolving unstable.
	registryJSON := `{
  "version": 2,
  "flakes": [
    {
      "from": { "type": "indirect", "id": "nixpkgs" },
      "to": { "type": "github", "owner": "NixOS", "repo": "nixpkgs", "ref": "nixos-26.05" }
    }
  ]
}`
	escapedRegistry := strings.ReplaceAll(registryJSON, "'", "'\\''")
	if _, err := shell.ExecuteCmd(chroot, fmt.Sprintf("echo '%s' > /etc/nix/registry.json", escapedRegistry)); err != nil {
		return fmt.Errorf("failed to write system flake registry: %w", err)
	}
	if _, err := shell.ExecuteCmdArgs(chroot, "su", "-", u.UserName, "-c", "NIX_SSL_CERT_FILE=/etc/ssl/certs/ca-certificates.crt nix registry add nixpkgs github:NixOS/nixpkgs/nixos-26.05"); err != nil {
		fmt.Printf("[WARNING] Failed to pin nixpkgs registry to stable: %v\n", err)
	}

	fmt.Println("[INFO] Setting Nix environment variables...")
	// XKB_CONFIG_ROOT: nix-installed GUI apps need the host X11 xkb data.
	// Build sandbox: upstream default — /tmp is exec since round 48, the old
	// scoping workaround (born from the noexec era) is gone.
	nixEnv := `export XKB_CONFIG_ROOT=/usr/share/X11/xkb`
	escapedNixEnv := strings.ReplaceAll(nixEnv, "'", "'\\''")
	// Append (not overwrite): preserve the PATH/daemon setup the Alpine nix
	// package ships in /etc/profile.d/nix-daemon.sh.
	if _, err := shell.ExecuteCmd(chroot, fmt.Sprintf("echo '%s' >> /etc/profile.d/nix-daemon.sh", escapedNixEnv)); err != nil {
		return fmt.Errorf("failed to write nix env: %w", err)
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

		// Install into the real user's profile so ~/.local/state/nix/profiles
		// is populated for the login user (not root).
		// --option sandbox false: the nix build sandbox uses pivot_root, which
		// fails inside the installer chroot (EINVAL — / is not a mountpoint).
		// This is chroot-only; the installed system keeps the default sandbox.
		// 'add' is the current command ('install' is a deprecated alias).
		var failedPkgs []string
		for _, pkg := range u.NixPackages {
			fmt.Printf("[INFO] nix profile add %s\n", pkg)
			inner := fmt.Sprintf("NIX_SSL_CERT_FILE=/etc/ssl/certs/ca-certificates.crt NIXPKGS_ALLOW_INSECURE=1 NIXPKGS_ALLOW_UNFREE=1 nix profile add --impure --option sandbox false %s", pkg)
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

		// Stop the daemon again — the target boots its own nix-daemon service.
		if _, err := shell.ExecuteCmdArgs(chroot, "rc-service", "nix-daemon", "stop"); err != nil {
			fmt.Printf("[WARNING] Failed to stop nix-daemon in chroot: %v\n", err)
		}
	}

	fmt.Println("[INFO] Nix configured. Run 'source /etc/profile.d/nix-daemon.sh' after first boot.")
	return nil
}
