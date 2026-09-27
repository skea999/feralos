package btrfsStandardServer

import (
	"alpineInstaller/hostPhase/disks/common"
	"alpineInstaller/utils/config"
	"alpineInstaller/utils/misc"
	"alpineInstaller/utils/shell"
	"fmt"
	"math"
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

	// Verify exactly 2 disks for RAID1
	if len(u.Disks) != 2 {
		return fmt.Errorf("btrfsStandardServer requires exactly 2 disks, got %d: %v", len(u.Disks), u.Disks)
	}

	d1 := u.Disks[0]
	d2 := u.Disks[1]

	if d1 == d2 {
		return fmt.Errorf("error: disk1 and disk2 are the same (%s)", d1)
	}

	// 1. Dynamic size calculation
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

		fdiskCmd := fmt.Sprintf("fdisk -W always %s <<'FDISK_EOF'\n%s\nFDISK_EOF", disk, fdiskCommands)
		if _, fdiskErr := shell.ExecuteCmd("", fdiskCmd); fdiskErr != nil {
			return fmt.Errorf("fdisk failed on %s: %w", disk, fdiskErr)
		}

		// Format EFI partition
		partitionSpacer := misc.GetPartitionSpacer(disk)
		efiPart := disk + partitionSpacer + "1"
		if _, mkfsErr := shell.ExecuteCmdArgs("", "mkfs.vfat", "-F", "32", efiPart); mkfsErr != nil {
			return fmt.Errorf("failed to format EFI on %s: %w", disk, mkfsErr)
		}
		label := "EFI" + strings.ToUpper(misc.GetDiskSuffix(disk))
		if _, labelErr := shell.ExecuteCmdArgs("", "fatlabel", efiPart, label); labelErr != nil {
			return fmt.Errorf("failed to label EFI on %s: %w", disk, labelErr)
		}
	}

	// 4. LUKS Encryption on RAID partitions (partition 2)
	fmt.Println("[INFO] Configuring LUKS encryption on RAID partitions...")
	luksRAID1, err := common.SetupLuksPartition(d1+misc.GetPartitionSpacer(d1)+"2", u.LuksPassword)
	if err != nil {
		return fmt.Errorf("LUKS setup failed on %s: %w", d1, err)
	}
	luksRAID2, err := common.SetupLuksPartition(d2+misc.GetPartitionSpacer(d2)+"2", u.LuksPassword)
	if err != nil {
		return fmt.Errorf("LUKS setup failed on %s: %w", d2, err)
	}

	// 5. LUKS Encryption on Swap partitions (partition 3)
	fmt.Println("[INFO] Configuring LUKS encryption on Swap partitions...")
	swapPart1 := d1 + misc.GetPartitionSpacer(d1) + "3"
	swapPart2 := d2 + misc.GetPartitionSpacer(d2) + "3"

	if luksErr := common.LuksFormat(swapPart1, u.LuksPassword); luksErr != nil {
		return fmt.Errorf("LUKS format swap failed on %s: %w", d1, luksErr)
	}
	if luksErr := common.LuksFormat(swapPart2, u.LuksPassword); luksErr != nil {
		return fmt.Errorf("LUKS format swap failed on %s: %w", d2, luksErr)
	}

	// Open swap LUKS
	swapUUID1, err := common.LuksUUID(swapPart1)
	if err != nil {
		return err
	}
	swapUUID2, err := common.LuksUUID(swapPart2)
	if err != nil {
		return err
	}

	if err := common.LuksOpen(swapPart1, "luks-"+swapUUID1, u.LuksPassword); err != nil {
		return fmt.Errorf("LUKS open swap failed on %s: %w", d1, err)
	}
	if err := common.LuksOpen(swapPart2, "luks-"+swapUUID2, u.LuksPassword); err != nil {
		return fmt.Errorf("LUKS open swap failed on %s: %w", d2, err)
	}

	// Format swap
	swapSuffix1 := misc.GetDiskSuffix(d1)
	swapSuffix2 := misc.GetDiskSuffix(d2)
	if _, err := shell.ExecuteCmdArgs("", "mkswap", "-L", fmt.Sprintf("swap-%s%s3", swapSuffix1, misc.GetPartitionSpacer(d1)), common.LuksMapperPath(swapUUID1)); err != nil {
		return fmt.Errorf("mkswap failed on %s: %w", d1, err)
	}
	if _, err := shell.ExecuteCmdArgs("", "mkswap", "-L", fmt.Sprintf("swap-%s%s3", swapSuffix2, misc.GetPartitionSpacer(d2)), common.LuksMapperPath(swapUUID2)); err != nil {
		return fmt.Errorf("mkswap failed on %s: %w", d2, err)
	}

	// 6. Format data partitions (partition 4)
	fmt.Println("[INFO] Formatting data partitions...")
	dataPart1 := d1 + misc.GetPartitionSpacer(d1) + "4"
	dataPart2 := d2 + misc.GetPartitionSpacer(d2) + "4"
	if _, err := shell.ExecuteCmdArgs("", "mkfs.btrfs", "-L", fmt.Sprintf("data-%s%s4", swapSuffix1, misc.GetPartitionSpacer(d1)), dataPart1); err != nil {
		return fmt.Errorf("mkfs.btrfs data failed on %s: %w", d1, err)
	}
	if _, err := shell.ExecuteCmdArgs("", "mkfs.btrfs", "-L", fmt.Sprintf("data-%s%s4", swapSuffix2, misc.GetPartitionSpacer(d2)), dataPart2); err != nil {
		return fmt.Errorf("mkfs.btrfs data failed on %s: %w", d2, err)
	}

	// 7. Create Btrfs RAID1 on top of LUKS
	fmt.Println("[INFO] Creating Btrfs RAID1 on LUKS devices...")
	if _, err := shell.ExecuteCmdArgs("", "mkfs.btrfs", "-L", "myroot", "-d", "raid1", "-m", "raid1", luksRAID1, luksRAID2); err != nil {
		return fmt.Errorf("mkfs.btrfs RAID1 failed: %w", err)
	}

	// 8. Setup subvolumes and mounts
	if err := setupSubvolumesAndMounts(i.ChrootDir, luksRAID1, d1, d2); err != nil {
		return err
	}

	// 9. Configure dmcrypt LUKS swap (keyfile + luksAddKey + conf.d).
	// Root fs is now mounted at ChrootDir — write the keyfile directly into
	// the chroot (no temp copy). The swap LUKS partitions are still open
	// (mappers active), so luksAddKey works here. apk never overwrites these
	// new files.
	swapKey := "/etc/dmcrypt/swap.key"
	hostSwapKey := i.ChrootDir + swapKey
	if _, err := shell.ExecuteCmdArgs("", "mkdir", "-p", i.ChrootDir+"/etc/dmcrypt", i.ChrootDir+"/etc/conf.d"); err != nil {
		return fmt.Errorf("failed to create swap keyfile: %w", err)
	}
	if _, err := shell.ExecuteCmdArgs("", "dd", "if=/dev/urandom", "of="+hostSwapKey, "bs=512", "count=4"); err != nil {
		return fmt.Errorf("failed to create swap keyfile: %w", err)
	}
	if _, err := shell.ExecuteCmdArgs("", "chmod", "400", hostSwapKey); err != nil {
		return fmt.Errorf("failed to create swap keyfile: %w", err)
	}
	if err := common.LuksAddKey(swapPart1, hostSwapKey, u.LuksPassword); err != nil {
		return fmt.Errorf("failed to add key to swap1: %w", err)
	}
	if err := common.LuksAddKey(swapPart2, hostSwapKey, u.LuksPassword); err != nil {
		return fmt.Errorf("failed to add key to swap2: %w", err)
	}
	// Encrypted swap unlock: /etc/crypttab (FeralOS dinit contract —
	// dinit-chimera early-cryptdisks runs our dinit-cryptdisks hook;
	// OpenRC dmcrypt reads the same file natively).
	// fstab then swapon's /dev/mapper/swap-* (chimera early-swap / OpenRC swap).
	crypttab := fmt.Sprintf(`swap-%s%s UUID=%s %s luks,discard
swap-%s%s UUID=%s %s luks,discard
`, swapSuffix1, misc.GetPartitionSpacer(d1)+"3", swapUUID1, swapKey,
		swapSuffix2, misc.GetPartitionSpacer(d2)+"3", swapUUID2, swapKey)
	escapedCrypttab := strings.ReplaceAll(crypttab, "'", "'\\''")
	if _, err := shell.ExecuteCmd("", fmt.Sprintf("echo '%s' > %s/etc/crypttab", escapedCrypttab, i.ChrootDir)); err != nil {
		return fmt.Errorf("failed to write crypttab: %w", err)
	}

	// 9. Inject kernel cmdline for booster
	fmt.Println("[INFO] Injecting kernel cmdline for booster...")
	raidUUID1, _ := common.LuksUUID(d1 + misc.GetPartitionSpacer(d1) + "2")
	raidUUID2, _ := common.LuksUUID(d2 + misc.GetPartitionSpacer(d2) + "2")

	kernelCmdline := fmt.Sprintf(
		"rd.luks.name=%s=luks-%s rd.luks.name=%s=luks-%s root=UUID=%s rootfstype=btrfs rootflags=subvol=@,device=/dev/mapper/luks-%s rootdelay=30 rootwait",
		raidUUID1, raidUUID1,
		raidUUID2, raidUUID2,
		misc.GetBtrfsUUID(luksRAID1),
		raidUUID2,
	)
	u.KernelCmdline = append(u.KernelCmdline, kernelCmdline)

	// 10. Add modules
	u.Modules = append(u.Modules, "dm_mod", "dm_crypt", "crc32c_cryptoapi", "zstd", "btrfs", "efivarfs", "nvme")

	// 10a. swap service always in runlevel for btrfsStandardServer (LUKS swap)
	u.Runlevels["boot"] = append(u.Runlevels["boot"], "swap")

	// 11. Generate Fstab
	return GenerateFstab(conf)
}

func getMinDiskSize(d1, d2 string) (int64, error) {
	var minSize int64 = math.MaxInt64
	for _, d := range []string{d1, d2} {
		out, err := shell.ExecuteCmdArgs("", "blockdev", "--getsize64", d)
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

func setupSubvolumesAndMounts(chroot string, luksDevice string, d1 string, d2 string) error {
	subvols := []Subvolume{
		{Name: "@", Path: "/", Options: "compress=zstd:3,autodefrag,ssd"},
		{Name: "@swap", Path: "/swap", Options: "ssd,compress=none,nodev,noexec,nosuid", NoCoW: true},
		{Name: "@home", Path: "/home", Options: "compress=zstd:3,autodefrag,ssd"},
		{Name: "@opt", Path: "/opt", Options: "compress=zstd:3,autodefrag,ssd"},
		{Name: "@root", Path: "/root", Options: "compress=zstd:3,autodefrag,ssd"},
		{Name: "@srv", Path: "/srv", Options: "autodefrag,ssd,nodev,noexec,nosuid", NoCoW: true},
		{Name: "@tmp", Path: "/tmp", Options: "compress=zstd:3,autodefrag,ssd,nosuid"},
		{Name: "@var", Path: "/var", Options: "compress=zstd:3,autodefrag,ssd"},
		{Name: "@var-lib-ex", Path: "/var/lib/ex", Options: "autodefrag,ssd,nodev,noexec,nosuid", NoCoW: true},
		{Name: "@var-lib-libvirt", Path: "/var/lib/libvirt", Options: "compress=zstd:3,autodefrag,ssd"},
		{Name: "@var-lib-incus", Path: "/var/lib/incus", Options: "compress=zstd:3,autodefrag,ssd"},
		{Name: "@var-lib-containerd", Path: "/var/lib/containerd", Options: "compress=zstd:3,autodefrag,ssd"},
		{Name: "@var-lib-docker", Path: "/var/lib/docker", Options: "compress=zstd:3,autodefrag,ssd"},
		{Name: "@var-lib-containers", Path: "/var/lib/containers", Options: "compress=zstd:3,autodefrag,ssd"},
		{Name: "@var-lib-vector", Path: "/var/lib/vector", Options: "compress=zstd:3,autodefrag,ssd"},
		{Name: "@var-cache", Path: "/var/cache", Options: "compress=zstd:3,autodefrag,ssd"},
		{Name: "@var-log", Path: "/var/log", Options: "autodefrag,ssd,nodev,noexec,nosuid", NoCoW: true},
		{Name: "@var-opt", Path: "/var/opt", Options: "compress=zstd:3,autodefrag,ssd"},
		{Name: "@var-spool", Path: "/var/spool", Options: "autodefrag,ssd,nodev,noexec,nosuid", NoCoW: true},
		{Name: "@var-tmp", Path: "/var/tmp", Options: "autodefrag,ssd,nodev,noexec,nosuid", NoCoW: true},
		{Name: "@usr-local", Path: "/usr/local", Options: "compress=zstd:3,autodefrag,ssd"},
		{Name: "@boot", Path: "/boot", Options: "compress=zstd:3,autodefrag,ssd"},
		// Nix package manager (always present — Nix can be enabled later)
		{Name: "@nix-store", Path: "/nix/store", Options: "compress=zstd:3,autodefrag,ssd"},
		{Name: "@nix-var-nix", Path: "/nix/var/nix", Options: "compress=zstd:3,autodefrag,ssd"},
		{Name: "@nix-var-log", Path: "/nix/var/log", Options: "autodefrag,ssd,nodev,noexec,nosuid", NoCoW: true},
		// Btrbk backup
		{Name: "@btrbk", Path: "/btrbk", Options: "compress=zstd:3,autodefrag,ssd"},
	}

	// Step 1: Mount root WITHOUT compression to create subvolumes
	fmt.Println("[INFO] Mounting root without compression to create subvolumes...")
	if _, err := shell.ExecuteCmdArgs("", "mount", "-t", "btrfs", "-o", "autodefrag,ssd", luksDevice, chroot); err != nil {
		return fmt.Errorf("failed to mount btrfs root: %w", err)
	}

	// Step 2: Create all subvolumes
	for _, s := range subvols {
		if _, err := shell.ExecuteCmdArgs("", "btrfs", "subvolume", "create", chroot+"/"+s.Name); err != nil {
			return fmt.Errorf("failed to create subvolume %s: %w", s.Name, err)
		}
	}

	// Step 3: Set NoCoW attribute on NoCoW subvolumes (chattr +C)
	// NODATACOW implies no compression (btrfs docs), so no 'btrfs property
	// set compression none' is needed — it would set the 'm' attribute on
	// every file, which conflicts with chattr +C (NixOS/nixpkgs #384047).
	// chattr +C on directory makes all files created within inherit NoCoW.
	for _, s := range subvols {
		if s.NoCoW {
			target := chroot + "/" + s.Name
			if _, err := shell.ExecuteCmdArgs("", "chattr", "+C", target); err != nil {
				return fmt.Errorf("failed to set NoCoW on %s: %w", s.Name, err)
			}
		}
	}

	// Step 4: Unmount temporary root
	if _, err := shell.ExecuteCmdArgs("", "umount", chroot); err != nil {
		return fmt.Errorf("failed to unmount btrfs root: %w", err)
	}

	// Step 5: Mount each subvolume with its correct options
	for _, s := range subvols {
		target := chroot + s.Path
		if s.Path != "" {
			if _, err := shell.ExecuteCmdArgs("", "mkdir", "-p", target); err != nil {
				return fmt.Errorf("failed to create directory %s: %w", target, err)
			}
		}
		// Mount with compress=none in options
		opts := "subvol=" + s.Name + "," + s.Options
		if s.NoCoW {
			opts += ",nodatacow"
		}
		if _, err := shell.ExecuteCmdArgs("", "mount", "-t", "btrfs", "-o", opts, luksDevice, target); err != nil {
			return fmt.Errorf("failed to mount subvolume %s: %w", s.Name, err)
		}
	}

	// FHS modes: fresh subvolume roots come up 0755 — /tmp and /var/tmp
	// must be 1777 sticky (O_TMPFILE, unprivileged temp writes)
	if err := misc.SetTempDirModes(chroot); err != nil {
		return err
	}

	// EFI Mounts
	partitionSpacer1 := misc.GetPartitionSpacer(d1)
	partitionSpacer2 := misc.GetPartitionSpacer(d2)

	if _, err := shell.ExecuteCmdArgs("", "mkdir", "-p", chroot+"/boot/efi"); err != nil {
		return fmt.Errorf("failed to create /boot/efi: %w", err)
	}
	if _, err := shell.ExecuteCmdArgs("", "mount", d1+partitionSpacer1+"1", chroot+"/boot/efi"); err != nil {
		return fmt.Errorf("failed to mount EFI1: %w", err)
	}
	if _, err := shell.ExecuteCmdArgs("", "mkdir", "-p", chroot+"/boot/efi2"); err != nil {
		return fmt.Errorf("failed to create /boot/efi2: %w", err)
	}
	if _, err := shell.ExecuteCmdArgs("", "mount", d2+partitionSpacer2+"1", chroot+"/boot/efi2"); err != nil {
		return fmt.Errorf("failed to mount EFI2: %w", err)
	}

	// Data Mounts
	s1 := strings.ToLower(misc.GetDiskSuffix(d1))
	s2 := strings.ToLower(misc.GetDiskSuffix(d2))
	if _, err := shell.ExecuteCmdArgs("", "mkdir", "-p", chroot+"/data/data-"+s1+partitionSpacer1+"4"); err != nil {
		return fmt.Errorf("failed to create data dir: %w", err)
	}
	if _, err := shell.ExecuteCmdArgs("", "mount", "-t", "btrfs", "-o", "compress=zstd:3,autodefrag,ssd", d1+partitionSpacer1+"4", chroot+"/data/data-"+s1+partitionSpacer1+"4"); err != nil {
		return fmt.Errorf("failed to mount data1: %w", err)
	}
	if _, err := shell.ExecuteCmdArgs("", "mkdir", "-p", chroot+"/data/data-"+s2+partitionSpacer2+"4"); err != nil {
		return fmt.Errorf("failed to create data dir: %w", err)
	}
	if _, err := shell.ExecuteCmdArgs("", "mount", "-t", "btrfs", "-o", "compress=zstd:3,autodefrag,ssd", d2+partitionSpacer2+"4", chroot+"/data/data-"+s2+partitionSpacer2+"4"); err != nil {
		return fmt.Errorf("failed to mount data2: %w", err)
	}

	return nil
}
