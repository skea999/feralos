package chrootPhase

import (
	"alpineInstaller/utils/config"
	"alpineInstaller/utils/files"
	"alpineInstaller/utils/shell"
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

	// 1b. Write Vector config AFTER package install (apk add vector overwrites /etc/vector/vector.yaml)
	// This is the correct Vector config (syslog socket), not the demo_logs placeholder
	fmt.Println("[INFO] Writing Vector logging configuration (after package install)...")
	if err := WriteVectorConfigAfterInstall(chroot); err != nil {
		fmt.Printf("[WARNING] Vector config failed: %v\n", err)
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

	// 10. Print installed packages (for documentation/debugging)
	fmt.Println("[INFO] Installed packages:")
	if out, err := shell.RunChrootCommand(chroot, "apk info"); err != nil {
		fmt.Printf("[WARNING] Failed to list packages: %v\n", err)
	} else {
		fmt.Println(out)
	}

	fmt.Println("[INFO] ChrootPhase.Run completed successfully.")
	return nil
}

// WriteVectorConfigAfterInstall writes the correct Vector config AFTER package installation.
// This must run after InstallPackages because `apk add vector` overwrites /etc/vector/vector.yaml
// Only runs if vector is installed (check with apk info)
func WriteVectorConfigAfterInstall(chroot string) error {
	// Check if vector is installed via apk
	checkCmd := "apk info | grep -q '^vector$' && echo yes || echo no"
	out, _ := shell.RunChrootCommand(chroot, checkCmd)
	if strings.TrimSpace(out) != "yes" {
		fmt.Println("[INFO] Vector not installed, skipping config")
		return nil
	}

	vectorConfig := `# Vector configuration for FeralOS
# Replaces rsyslog + logrotate with memory-safe Rust alternative
# Uses /run/vector/syslog.sock (vector user has write access to /run/)

data_dir: "/var/lib/vector"

sources:
  syslog:
    type: socket
    mode: unix_datagram
    path: /run/vector/syslog.sock
    decoding:
      codec: syslog

transforms:
  auth_logs:
    type: filter
    inputs:
      - syslog
    condition: '.facility == "auth" || .facility == "authpriv"'

  daemon_logs:
    type: filter
    inputs:
      - syslog
    condition: '.facility == "daemon"'

  kernel_logs:
    type: filter
    inputs:
      - syslog
    condition: '.facility == "kern"'

sinks:
  messages:
    type: file
    inputs:
      - syslog
    path: "/var/log/messages"
    encoding:
      codec: text

  auth_log:
    type: file
    inputs:
      - auth_logs
    path: "/var/log/auth.log"
    encoding:
      codec: text

  daemon_log:
    type: file
    inputs:
      - daemon_logs
    path: "/var/log/daemon.log"
    encoding:
      codec: text

  kernel_log:
    type: file
    inputs:
      - kernel_logs
    path: "/var/log/kern.log"
    encoding:
      codec: text
`
	if err := files.WriteFileInChroot(chroot, "/etc/vector/vector.yaml", vectorConfig); err != nil {
		return fmt.Errorf("failed to write vector config: %w", err)
	}

	// Ensure vector data directory exists (persistent subvolume)
	if _, err := shell.RunChrootCommand(chroot, "mkdir -p /var/lib/vector && chown vector:vector /var/lib/vector"); err != nil {
		fmt.Printf("[WARNING] Failed to create vector data directory: %v\n", err)
	}

	// NOTE: /run/vector, /var/log/vector.log, and log files are handled by
	// vector-setup init script at boot time (non-persistent, tmpfs)

	return nil
}
