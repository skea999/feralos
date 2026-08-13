package misc

import "strings"

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
