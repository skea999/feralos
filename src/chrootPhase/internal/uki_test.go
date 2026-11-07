package internal

import "testing"

// getPartitionSpacer (uki.go, unexported) decides the efibootmgr --disk
// partition separator: NVMe devices need the "p" (nvme0n1p1), SATA/SCSI/SD
// names concatenate directly (sda1).
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
			if got := getPartitionSpacer(tt.disk); got != tt.want {
				t.Errorf("getPartitionSpacer(%q) = %q, want %q", tt.disk, got, tt.want)
			}
		})
	}
}
