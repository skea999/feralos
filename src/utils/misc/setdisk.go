package misc

import (
	"alpineInstaller/utils/shell"
	"fmt"
	"strings"
)

// GetPartitionSpacer returns "p" for NVMe devices, "" otherwise.
// NVMe devices use partition names like nvme0n1p1 (with "p" before partition number).
func GetPartitionSpacer(disk string) string {
	if strings.Contains(disk, "nvme") {
		return "p"
	}
	return ""
}

// GetDiskSuffix returns the disk name without path prefix (e.g., "sda", "nvme0n1").
func GetDiskSuffix(disk string) string {
	parts := strings.Split(disk, "/")
	return parts[len(parts)-1]
}

// GetBtrfsUUID returns the UUID of a btrfs filesystem.
func GetBtrfsUUID(device string) string {
	out, err := shell.ExecuteCmdArgs("", "blkid", "-o", "value", "-s", "UUID", device)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out)
}

// SetTempDirModes enforces the FHS modes on freshly-created btrfs subvolume
// root dirs: a new subvolume comes up 0755 root:root, which breaks O_TMPFILE
// and any unprivileged temp write (flatpak --user, podman — round 47).
// 1777 sticky world-writable is the FHS standard; the mode persists in the
// subvolume inode, so this is a one-time fix at install.
func SetTempDirModes(chroot string) error {
	for _, dir := range []string{"/tmp", "/var/tmp"} {
		if _, err := shell.ExecuteCmdArgs("", "mkdir", "-p", chroot+dir); err != nil {
			return err
		}
		if _, err := shell.ExecuteCmdArgs("", "chmod", "1777", chroot+dir); err != nil {
			return fmt.Errorf("failed to set mode 1777 on %s: %w", dir, err)
		}
	}
	return nil
}
