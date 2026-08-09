package btrfsStandardServer

import (
	"alpineInstaller/utils/config"
	"alpineInstaller/utils/shell"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

type Subvolume struct {
	Name    string
	Path    string
	Options string
	NoCoW   bool
}

func RunBtrfsSetup(conf *config.Config) error {
	shell.SetCaller("disks")
	u := &conf.User
	i := &conf.Internal

	// Verify at least 2 disks for RAID1
	if len(u.Disks) < 2 {
		return fmt.Errorf("error: RAID1 layout requires at least 2 disks, configured: %d", len(u.Disks))
	}

	d1 := u.Disks[0]
	d2 := u.Disks[1]

	if d1 == d2 {
		return fmt.Errorf("error: disk1 and disk2 are the same (%s)", d1)
	}

	// 1. Cleanup environment
	fmt.Println("[INFO] Stopping existing RAIDs and swaps...")
	_, _ = shell.RunCommand("swapoff", "-a")
	_, _ = shell.RunCommand("umount", "-fR", i.ChrootDir)
	_, _ = shell.RunCommand("mdadm", "--stop", "/dev/md?")
	_, _ = shell.RunCommand("dmsetup", "ls", "--target", "crypt", "--exec", "cryptsetup close")

	// 2. Dynamic size calculation
	minSizeBytes, err := getMinDiskSize(d1, d2)
	if err != nil {
		return err
	}

	// Logic: partitions for RAID, Swap, etc.
	firstPartK := (minSizeBytes / 1024 / 4 * 2) - (512 * 1024)
	secondPartK := (minSizeBytes / 1024 / 4)

	fmt.Printf("[DEBUG] Calculated RAID Partition: %d K\n", firstPartK)
	fmt.Printf("[DEBUG] Calculated Swap Partition: %d K\n", secondPartK)

	// 3. Partitioning loop (both disks get same layout)
	for _, disk := range []string{d1, d2} {
		fmt.Printf("[INFO] Partitioning disk %s...\n", disk)

		// Partition layout:
		// 1: EFI (512M)
		// 2: LUKS RAID (firstPartK)
		// 3: LUKS Swap (secondPartK)
		// 4: Data (remainder)
		fdiskCommands := fmt.Sprintf("g\nn\n1\n\n+512M\nt\n1\n1\nn\n2\n\n+%dK\nt\n2\n20\nn\n3\n\n+%dK\nt\n3\n19\nn\n4\n\n\nt\n4\n20\nw\n", firstPartK, secondPartK)

		cmd := exec.Command("fdisk", "-W", "always", disk)
		cmd.Stdin = strings.NewReader(fdiskCommands)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("fdisk failed on %s: %w", disk, err)
		}

		// Format EFI partition
		partitionSpacer := getPartitionSpacer(disk)
		efiPart := disk + partitionSpacer + "1"
		if _, err := shell.RunCommand("mkfs.vfat", "-F", "32", efiPart); err != nil {
			return fmt.Errorf("failed to format EFI on %s: %w", disk, err)
		}
		label := "EFI" + strings.ToUpper(getDiskSuffix(disk))
		if _, err := shell.RunCommand("fatlabel", efiPart, label); err != nil {
			return fmt.Errorf("failed to label EFI on %s: %w", disk, err)
		}
	}

	// 4. LUKS Encryption on RAID partitions (partition 2)
	fmt.Println("[INFO] Configuring LUKS encryption on RAID partitions...")
	luksRAID1, err := SetupLuksPartition(d1+getPartitionSpacer(d1)+"2", u.Password)
	if err != nil {
		return fmt.Errorf("LUKS setup failed on %s: %w", d1, err)
	}
	luksRAID2, err := SetupLuksPartition(d2+getPartitionSpacer(d2)+"2", u.Password)
	if err != nil {
		return fmt.Errorf("LUKS setup failed on %s: %w", d2, err)
	}

	// 5. LUKS Encryption on Swap partitions (partition 3)
	fmt.Println("[INFO] Configuring LUKS encryption on Swap partitions...")
	swapPart1 := d1 + getPartitionSpacer(d1) + "3"
	swapPart2 := d2 + getPartitionSpacer(d2) + "3"

	if err := LuksFormat(swapPart1, u.Password); err != nil {
		return fmt.Errorf("LUKS format swap failed on %s: %w", d1, err)
	}
	if err := LuksFormat(swapPart2, u.Password); err != nil {
		return fmt.Errorf("LUKS format swap failed on %s: %w", d2, err)
	}

	// Open swap LUKS
	swapUUID1, err := LuksUUID(swapPart1)
	if err != nil {
		return err
	}
	swapUUID2, err := LuksUUID(swapPart2)
	if err != nil {
		return err
	}

	if err := LuksOpen(swapPart1, "luks-"+swapUUID1, u.Password); err != nil {
		return fmt.Errorf("LUKS open swap failed on %s: %w", d1, err)
	}
	if err := LuksOpen(swapPart2, "luks-"+swapUUID2, u.Password); err != nil {
		return fmt.Errorf("LUKS open swap failed on %s: %w", d2, err)
	}

	// Format swap
	swapSuffix1 := getDiskSuffix(d1)
	swapSuffix2 := getDiskSuffix(d2)
	if _, err := shell.RunCommand("mkswap", "-L", "swap-"+swapSuffix1+getPartitionSpacer(d1)+"3", LuksMapperPath(swapUUID1)); err != nil {
		return fmt.Errorf("mkswap failed on %s: %w", d1, err)
	}
	if _, err := shell.RunCommand("mkswap", "-L", "swap-"+swapSuffix2+getPartitionSpacer(d2)+"3", LuksMapperPath(swapUUID2)); err != nil {
		return fmt.Errorf("mkswap failed on %s: %w", d2, err)
	}

	// 6. Format data partitions (partition 4)
	fmt.Println("[INFO] Formatting data partitions...")
	dataPart1 := d1 + getPartitionSpacer(d1) + "4"
	dataPart2 := d2 + getPartitionSpacer(d2) + "4"
	if _, err := shell.RunCommand("mkfs.btrfs", "-L", "data-"+swapSuffix1+getPartitionSpacer(d1)+"4", dataPart1); err != nil {
		return fmt.Errorf("mkfs.btrfs data failed on %s: %w", d1, err)
	}
	if _, err := shell.RunCommand("mkfs.btrfs", "-L", "data-"+swapSuffix2+getPartitionSpacer(d2)+"4", dataPart2); err != nil {
		return fmt.Errorf("mkfs.btrfs data failed on %s: %w", d2, err)
	}

	// 7. Create Btrfs RAID1 on top of LUKS
	fmt.Println("[INFO] Creating Btrfs RAID1 on LUKS devices...")
	if _, err := shell.RunCommand("mkfs.btrfs", "-L", "myroot", "-d", "raid1", "-m", "raid1", luksRAID1, luksRAID2); err != nil {
		return fmt.Errorf("mkfs.btrfs RAID1 failed: %w", err)
	}

	// 8. Setup subvolumes and mounts
	if err := setupSubvolumesAndMounts(i.ChrootDir, luksRAID1, d1, d2, swapUUID1, swapUUID2); err != nil {
		return err
	}

	// 9. Inject kernel cmdline for booster
	fmt.Println("[INFO] Injecting kernel cmdline for booster...")
	raidUUID1, _ := LuksUUID(d1 + getPartitionSpacer(d1) + "2")
	raidUUID2, _ := LuksUUID(d2 + getPartitionSpacer(d2) + "2")

	kernelCmdline := fmt.Sprintf(
		"rd.luks.name=%s=luks-%s rd.luks.name=%s=luks-%s root=UUID=%s rootfstype=btrfs rootflags=subvol=@,device=/dev/mapper/luks-%s rootdelay=30 rootwait console=ttyS0,115200",
		raidUUID1, raidUUID1,
		raidUUID2, raidUUID2,
		getBtrfsUUID(luksRAID1),
		raidUUID2,
	)
	u.KernelCmdline = append(u.KernelCmdline, kernelCmdline)

	// 10. Add modules
	u.Modules = append(u.Modules, "dm_mod", "dm_crypt", "crc32c_cryptoapi", "zstd", "btrfs", "efivarfs", "nvme")

	// 11. Generate Fstab
	return GenerateFstab(conf)
}

// getPartitionSpacer returns "p" for NVMe devices, "" otherwise.
func getPartitionSpacer(disk string) string {
	if strings.Contains(disk, "nvme") {
		return "p"
	}
	return ""
}

// getDiskSuffix returns the disk name without path prefix.
func getDiskSuffix(disk string) string {
	parts := strings.Split(disk, "/")
	return parts[len(parts)-1]
}

// getBtrfsUUID returns the UUID of a btrfs filesystem.
func getBtrfsUUID(device string) string {
	out, err := shell.RunCommand("blkid", "-o", "value", "-s", "UUID", device)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out)
}

func getMinDiskSize(d1, d2 string) (int64, error) {
	var minSize int64 = 9223372036854775807
	for _, d := range []string{d1, d2} {
		out, err := shell.RunCommand("blockdev", "--getsize64", d)
		if err != nil {
			return 0, err
		}
		size, _ := strconv.ParseInt(strings.TrimSpace(out), 10, 64)
		if size < (8 * 1024 * 1024 * 1024) {
			return 0, fmt.Errorf("disk %s is too small (<8GiB)", d)
		}
		if size < minSize {
			minSize = size
		}
	}
	return minSize, nil
}

func setupSubvolumesAndMounts(chroot string, luksDevice string, d1 string, d2 string, swapUUID1 string, swapUUID2 string) error {
	subvols := []Subvolume{
		{Name: "@", Path: "", Options: "compress=zstd:3,autodefrag,ssd"},
		{Name: "@swap", Path: "/swap", Options: "compress=no,autodefrag,ssd", NoCoW: true},
		{Name: "@home", Path: "/home", Options: "compress=zstd:3,autodefrag,ssd"},
		{Name: "@opt", Path: "/opt", Options: "compress=zstd:3,autodefrag,ssd"},
		{Name: "@root", Path: "/root", Options: "compress=zstd:3,autodefrag,ssd"},
		{Name: "@srv", Path: "/srv", Options: "compress=no,nodev,noexec,nosuid,autodefrag,ssd", NoCoW: true},
		{Name: "@tmp", Path: "/tmp", Options: "compress=zstd:3,autodefrag,ssd,noexec,nosuid"},
		{Name: "@var", Path: "/var", Options: "compress=zstd:3,autodefrag,ssd"},
		{Name: "@var-lib-ex", Path: "/var/lib/ex", Options: "compress=no,nodev,noexec,nosuid,autodefrag,ssd", NoCoW: true},
		{Name: "@var-lib-libvirt", Path: "/var/lib/libvirt", Options: "compress=zstd:3,autodefrag,ssd"},
		{Name: "@var-lib-incus", Path: "/var/lib/incus", Options: "compress=zstd:3,autodefrag,ssd"},
		{Name: "@var-lib-containerd", Path: "/var/lib/containerd", Options: "compress=zstd:3,autodefrag,ssd"},
		{Name: "@var-lib-docker", Path: "/var/lib/docker", Options: "compress=zstd:3,autodefrag,ssd"},
		{Name: "@var-lib-containers", Path: "/var/lib/containers", Options: "compress=zstd:3,autodefrag,ssd"},
		{Name: "@var-lib-vector", Path: "/var/lib/vector", Options: "compress=zstd:3,autodefrag,ssd"},
		{Name: "@var-cache", Path: "/var/cache", Options: "compress=zstd:3,autodefrag,ssd"},
		{Name: "@var-log", Path: "/var/log", Options: "compress=no,nodev,noexec,nosuid,autodefrag,ssd", NoCoW: true},
		{Name: "@var-opt", Path: "/var/opt", Options: "compress=zstd:3,autodefrag,ssd"},
		{Name: "@var-spool", Path: "/var/spool", Options: "compress=no,nodev,noexec,nosuid,autodefrag,ssd", NoCoW: true},
		{Name: "@var-tmp", Path: "/var/tmp", Options: "compress=no,nodev,noexec,nosuid,autodefrag,ssd", NoCoW: true},
		{Name: "@usr-local", Path: "/usr/local", Options: "compress=zstd:3,autodefrag,ssd"},
		{Name: "@boot", Path: "/boot", Options: "compress=zstd:3,autodefrag,ssd"},
		// Btrbk backup
		{Name: "@btrbk", Path: "/btrbk", Options: "compress=zstd:3,autodefrag,ssd"},
	}

	// Temporary mount to create subvolumes
	if _, err := shell.RunCommand("mount", "-t", "btrfs", "-o", "compress=zstd:3,autodefrag,ssd", luksDevice, chroot); err != nil {
		return fmt.Errorf("failed to mount btrfs root: %w", err)
	}
	for _, s := range subvols {
		if _, err := shell.RunCommand("btrfs", "subvolume", "create", chroot+"/"+s.Name); err != nil {
			return fmt.Errorf("failed to create subvolume %s: %w", s.Name, err)
		}
	}
	if _, err := shell.RunCommand("umount", chroot); err != nil {
		return fmt.Errorf("failed to unmount btrfs root: %w", err)
	}

	// Systematic mount of subvolumes
	for _, s := range subvols {
		target := chroot + s.Path
		if s.Path != "" {
			if err := os.MkdirAll(target, 0755); err != nil {
				return fmt.Errorf("failed to create directory %s: %w", target, err)
			}
		}
		opts := "subvol=" + s.Name + "," + s.Options
		if _, err := shell.RunCommand("mount", "-t", "btrfs", "-o", opts, luksDevice, target); err != nil {
			return fmt.Errorf("failed to mount subvolume %s: %w", s.Name, err)
		}
		if s.NoCoW {
			// Set NoCoW attribute on directory
			if _, err := shell.RunCommand("chattr", "+C", target); err != nil {
				return fmt.Errorf("failed to set NoCoW on %s: %w", target, err)
			}
			// Disable compression on the subvolume
			if _, err := shell.RunCommand("btrfs", "property", "set", target, "compression", "none"); err != nil {
				return fmt.Errorf("failed to disable compression on %s: %w", target, err)
			}
		}
	}

	// EFI Mounts
	partitionSpacer1 := getPartitionSpacer(d1)
	partitionSpacer2 := getPartitionSpacer(d2)

	if err := os.MkdirAll(chroot+"/boot/efi", 0755); err != nil {
		return fmt.Errorf("failed to create /boot/efi: %w", err)
	}
	if _, err := shell.RunCommand("mount", d1+partitionSpacer1+"1", chroot+"/boot/efi"); err != nil {
		return fmt.Errorf("failed to mount EFI1: %w", err)
	}
	if err := os.MkdirAll(chroot+"/boot/efi2", 0755); err != nil {
		return fmt.Errorf("failed to create /boot/efi2: %w", err)
	}
	if _, err := shell.RunCommand("mount", d2+partitionSpacer2+"1", chroot+"/boot/efi2"); err != nil {
		return fmt.Errorf("failed to mount EFI2: %w", err)
	}

	// Data Mounts
	s1 := strings.ToLower(getDiskSuffix(d1))
	s2 := strings.ToLower(getDiskSuffix(d2))
	if err := os.MkdirAll(chroot+"/data/data-"+s1+partitionSpacer1+"4", 0755); err != nil {
		return fmt.Errorf("failed to create data dir: %w", err)
	}
	if _, err := shell.RunCommand("mount", "-t", "btrfs", "-o", "compress=zstd:3,autodefrag,ssd", d1+partitionSpacer1+"4", chroot+"/data/data-"+s1+partitionSpacer1+"4"); err != nil {
		return fmt.Errorf("failed to mount data1: %w", err)
	}
	if err := os.MkdirAll(chroot+"/data/data-"+s2+partitionSpacer2+"4", 0755); err != nil {
		return fmt.Errorf("failed to create data dir: %w", err)
	}
	if _, err := shell.RunCommand("mount", "-t", "btrfs", "-o", "compress=zstd:3,autodefrag,ssd", d2+partitionSpacer2+"4", chroot+"/data/data-"+s2+partitionSpacer2+"4"); err != nil {
		return fmt.Errorf("failed to mount data2: %w", err)
	}

	return nil
}
