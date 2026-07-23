package service

import "testing"

func TestHasHealthyBtrfsRaid1Layout(t *testing.T) {
	tests := []struct {
		name   string
		detail string
		want   bool
	}{
		{
			name: "accepts usage table header raid1 format",
			detail: `===mapper===
/dev/mapper/luks-first
/dev/mapper/luks-second

===mounts===
/dev/mapper/luks-first / btrfs rw,relatime,compress=zstd:3,ssd,space_cache=v2,autodefrag,subvolid=256,subvol=/@ 0 0

===btrfs_show===
Label: 'myroot'  uuid: abc
	Total devices 2 FS bytes used 1.56GiB
	devid    1 size 7.48GiB used 3.26GiB path /dev/mapper/luks-first
	devid    2 size 7.48GiB used 3.26GiB path /dev/mapper/luks-second

===btrfs_usage===
Overall:
    Device missing:		     0.00B
    Data ratio:			      2.00
    Metadata ratio:		      2.00

                                                         Data    Metadata  System
Id Path                                                  RAID1   RAID1     RAID1    Unallocated Total    Slack
-- ----------------------------------------------------- ------- --------- -------- ----------- -------- -----
 1 /dev/mapper/luks-first                                3.00GiB 256.00MiB  8.00MiB     4.23GiB  7.48GiB     -
 2 /dev/mapper/luks-second                               3.00GiB 256.00MiB  8.00MiB     4.23GiB  7.48GiB     -
`,
			want: true,
		},
		{
			name: "rejects degraded mount with missing device",
			detail: `===mapper===
/dev/mapper/luks-first

===mounts===
/dev/mapper/luks-first / btrfs rw,relatime,degraded,subvolid=256,subvol=/@ 0 0

===btrfs_show===
Label: 'myroot'  uuid: abc
	Total devices 1 FS bytes used 1.56GiB
	devid    1 size 7.48GiB used 3.26GiB path /dev/mapper/luks-first

===btrfs_usage===
Overall:
    Device missing:		     7.48GiB
    Data ratio:			      1.00
    Metadata ratio:		      1.00

BTRFS warning (device dm-0): devid 2 uuid deadbeef is missing
`,
			want: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := hasHealthyBtrfsRaid1Layout(test.detail)
			if got != test.want {
				t.Fatalf("hasHealthyBtrfsRaid1Layout() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestHasClevisTPM2Bindings(t *testing.T) {
	tests := []struct {
		name            string
		detail          string
		expectedDevices int
		want            bool
	}{
		{
			name: "accepts TPM2 bindings on both root volumes",
			detail: `===/dev/vda2===
1: tpm2 '{}'
===/dev/vdb2===
0: tpm2 '{"hash":"sha256"}'
`,
			expectedDevices: 2,
			want:            true,
		},
		{
			name: "rejects missing second TPM2 binding",
			detail: `===/dev/vda2===
1: tpm2 '{}'
===/dev/vdb2===
`,
			expectedDevices: 2,
			want:            false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := hasClevisTPM2Bindings(test.detail, test.expectedDevices)
			if got != test.want {
				t.Fatalf("hasClevisTPM2Bindings() = %v, want %v", got, test.want)
			}
		})
	}
}