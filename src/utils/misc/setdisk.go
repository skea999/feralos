package misc

import (
	"alpineInstaller/utils/shell"
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
