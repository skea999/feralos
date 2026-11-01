package service

import (
	"testing"
)

func TestHasHealthyBtrfsSingleLayout(t *testing.T) {
	detail := `Warning: Permanently added '[127.0.0.1]:2223' (ED25519) to the list of known hosts.
===blkid===
/dev/vdb2: UUID="a04fe215-c945-41ba-8589-0efac9d9e890" TYPE="crypto_LUKS"
/dev/vdb1: LABEL_FATBOOT="EFIVDC" LABEL="EFIVDC" UUID="307D-5A85" BLOCK_SIZE="512" TYPE="vfat"
/dev/mapper/luks-a04fe215-c945-41ba-8589-0efac9d9e890: LABEL="myroot" UUID="5f2f4631-04a0-4c84-91c6-a6c1733166f6" UUID_SUB="7d491e25-7120-46df-98b7-886f113aa15e" BLOCK_SIZE="4096" TYPE="btrfs"

===mapper===
/dev/mapper/luks-a04fe215-c945-41ba-8589-0efac9d9e890

===mounts===
/dev/dm-0 / btrfs rw,relatime,compress=zstd:3,ssd,space_cache=v2,autodefrag,subvolid=256,subvol=/@ 0 0

===btrfs_show===
Label: 'myroot'  uuid: 5f2f4631-04a0-4c84-91c6-a6c1733166f6
	Total devices 1 FS bytes used 9.56GiB
	devid    1 size 15.48GiB used 11.52GiB path /dev/mapper/luks-a04fe215-c945-41ba-8589-0efac9d9e890

===btrfs_usage===
Overall:
    Device size:		   15.48GiB
    Device allocated:		   11.52GiB
    Device unallocated:		    3.96GiB
    Device missing:		     0.00B
    Device slack:		     0.00B
    Used:			       9.58GiB
    Free (estimated):		   5.43GiB	(min: 3.46GiB)
    Free (statfs, df):		   5.43GiB
    Data ratio:				1.00
    Metadata ratio:			2.00
    Global reserve:		   5.50MiB	(used: 0.00B)
    Multiple profiles:		        no

             Data     Metadata  System
Id Path      single   DUP       DUP      Unallocated Total    Slack
-- --------- -------- --------- -------- ----------- -------- -----
 1 /dev/dm-0 11.01GiB 512.00MiB 16.00MiB     3.96GiB 15.48GiB     -
-- --------- -------- --------- -------- ----------- -------- -----
   Total     11.01GiB 256.00MiB  8.00MiB     3.96GiB 15.48GiB 0.00B
   Used       9.53GiB  27.23MiB 16.00KiB`

	if !hasHealthyBtrfsSingleLayout(detail) {
		t.Error("expected hasHealthyBtrfsSingleLayout to return true for single-disk btrfs layout")
	}
}
