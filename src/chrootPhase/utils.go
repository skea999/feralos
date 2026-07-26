package chrootPhase

import "strings"

// getPartitionSpacer returns "p" for NVMe devices, "" otherwise.
func getPartitionSpacer(disk string) string {
	if strings.Contains(disk, "nvme") {
		return "p"
	}
	return ""
}

// getDiskSuffix returns the disk name without path prefix (e.g., "sda", "nvme0n1").
func getDiskSuffix(disk string) string {
	parts := strings.Split(disk, "/")
	return parts[len(parts)-1]
}
