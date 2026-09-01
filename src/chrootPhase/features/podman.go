package features

import (
	"alpineInstaller/utils/config"
	"alpineInstaller/utils/shell"
	"fmt"
)

// PodmanPackages returns the APK packages needed for Podman.
func PodmanPackages(conf *config.Config) []string {
	if !conf.User.EnablePodman {
		return nil
	}
	// docker is created as dummy package via internal_rules.yaml (podman: replace: docker)
	return []string{"podman", "podman-openrc", "iptables", "podman-compose", "podman-docker", "fuse-overlayfs"}
}

// SetupPodman configures Podman rootless containers.
func SetupPodman(conf *config.Config) error {
	if !conf.User.EnablePodman {
		fmt.Println("[INFO] Podman disabled, skipping...")
		return nil
	}

	chroot := conf.Internal.ChrootDir
	u := conf.User

	fmt.Println("[INFO] Configuring Podman rootless containers...")

	// Enable podman service
	if _, err := shell.ExecuteCmdArgs(chroot, "rc-update", "add", "podman", "default"); err != nil {
		fmt.Printf("[WARNING] Failed to add podman to default runlevel: %v\n", err)
	}

	// Enable cgroups (required by podman)
	if _, err := shell.ExecuteCmdArgs(chroot, "rc-update", "add", "cgroups", "default"); err != nil {
		fmt.Printf("[WARNING] Failed to add cgroups to default runlevel: %v\n", err)
	}

	// Configure subuid/subgid for rootless containers
	subuidContent := fmt.Sprintf("%s:100000:65536\n", u.UserName)
	subgidContent := fmt.Sprintf("%s:100000:65536\n", u.UserName)
	if _, err := shell.ExecuteCmd(chroot, fmt.Sprintf("echo '%s' > /etc/subuid", subuidContent)); err != nil {
		return fmt.Errorf("failed to write /etc/subuid: %w", err)
	}
	if _, err := shell.ExecuteCmd(chroot, fmt.Sprintf("echo '%s' > /etc/subgid", subgidContent)); err != nil {
		return fmt.Errorf("failed to write /etc/subgid: %w", err)
	}

	// Storage driver: btrfs (root filesystem is btrfs). Alpine ships a full
	// storage.conf — override only the driver line, keep the rest.
	fmt.Println("[INFO] Configuring Podman storage driver (btrfs)...")
	if _, err := shell.ExecuteCmd(chroot, "mkdir -p /etc/containers"); err != nil {
		fmt.Printf("[WARNING] Failed to create containers config dir: %v\n", err)
	}
	if _, err := shell.ExecuteCmd(chroot, `sed -i 's/^#*driver[[:space:]]*=.*/driver = "btrfs"/' /etc/containers/storage.conf`); err != nil {
		fmt.Printf("[WARNING] Failed to set storage driver: %v\n", err)
	}

	fmt.Println("[INFO] Podman configured for rootless containers.")
	return nil
}
