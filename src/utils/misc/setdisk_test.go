package misc

import "testing"

// GetPartitionSpacer (setdisk.go) decides the efibootmgr --disk partition
// separator: NVMe devices need the "p" (nvme0n1p1), SATA/SCSI/SD
// names concatenate directly (sda1). Consumers: hostPhase disk layouts +
// chrootPhase/internal/uki.go (the unexported uki copy was deduplicated
// into this exported version).
func TestGetPartitionSpacer(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		disk string
		want string
	}{
		{"nvme full path", "/dev/nvme0n1", "p"},
		{"nvme controller-only", "/dev/nvme0", "p"},
		{"sata disk", "/dev/sda", ""},
		{"virtio disk", "/dev/vda", ""},
		{"mmc device", "/dev/mmcblk0", ""},
		{"empty disk string", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetPartitionSpacer(tt.disk); got != tt.want {
				t.Errorf("GetPartitionSpacer(%q) = %q, want %q", tt.disk, got, tt.want)
			}
		})
	}
}
