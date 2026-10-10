package btrfsRaid0

import (
	"feralos/src/hostPhase/disks/common"
	"feralos/src/utils/config"
	"feralos/src/utils/misc"
	"feralos/src/utils/shell"
	"feralos/src/utils/swap"
	"fmt"
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

	// Verify exactly 2 disks for RAID0
	if len(u.Disks) != 2 {
		return fmt.Errorf("btrfsRaid0 requires exactly 2 disks, got %d: %v", len(u.Disks), u.Disks)
	}

	d1 := u.Disks[0]
	d2 := u.Disks[1]

	if d1 == d2 {
		return fmt.Errorf("error: disk1 and disk2 are the same (%s)", d1)
	}

	fmt.Printf("[INFO] Starting dual disk partitioning (RAID0): %s + %s\n", d1, d2)

	// 1. Partitioning loop (both disks get same layout)
	// With swap: 1: EFI (512M), 2: LUKS swap (swapSize/2 each), 3: LUKS ROOT
	// (remainder — no size formulas). Without swap: 1: EFI, 2: LUKS ROOT.
	// swapSize is the TOTAL swap size, split across the 2 partitions.
	swapSizeK := 0
	if u.EnableSwap {
		swapSizeMiB, err := swap.ResolveSwapSizeMiB(u.SwapSize)
		if err != nil {
			return fmt.Errorf("failed to resolve swap size: %w", err)
		}
		swapSizeK = swapSizeMiB / 2 * 1024
	}
	rootPart := "2"
	if u.EnableSwap {
		rootPart = "3"
	}

	for _, disk := range []string{d1, d2} {
		fmt.Printf("[INFO] Partitioning disk %s...\n", disk)

		var fdiskCommands string
		if u.EnableSwap {
			fdiskCommands = fmt.Sprintf("g\nn\n1\n\n+512M\nt\n1\n1\nn\n2\n\n+%dK\nt\n2\n19\nn\n3\n\n\nt\n3\n20\nw\n", swapSizeK)
		} else {
			fdiskCommands = "g\nn\n1\n\n+512M\nt\n1\n1\nn\n2\n\n\nt\n2\n20\nw\n"
		}

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

	// 3. LUKS Encryption on ROOT partitions (partition 2, or 3 with swap)
	fmt.Println("[INFO] Configuring LUKS encryption on ROOT partitions...")
	luksRoot1, err := common.SetupLuksPartition(d1+misc.GetPartitionSpacer(d1)+rootPart, u.LuksPassword)
	if err != nil {
		return fmt.Errorf("LUKS setup failed on %s: %w", d1, err)
	}
	luksRoot2, err := common.SetupLuksPartition(d2+misc.GetPartitionSpacer(d2)+rootPart, u.LuksPassword)
	if err != nil {
		return fmt.Errorf("LUKS setup failed on %s: %w", d2, err)
	}

	// 4. LUKS Encryption on Swap partitions (partition 2) — only when enabled
	var swapPart1, swapPart2, swapUUID1, swapUUID2, swapSuffix1, swapSuffix2 string
	if u.EnableSwap {
		fmt.Println("[INFO] Configuring LUKS encryption on Swap partitions...")
		swapPart1 = d1 + misc.GetPartitionSpacer(d1) + "2"
		swapPart2 = d2 + misc.GetPartitionSpacer(d2) + "2"

		if luksErr := common.LuksFormat(swapPart1, u.LuksPassword); luksErr != nil {
			return fmt.Errorf("LUKS format swap failed on %s: %w", d1, luksErr)
		}
		if luksErr := common.LuksFormat(swapPart2, u.LuksPassword); luksErr != nil {
			return fmt.Errorf("LUKS format swap failed on %s: %w", d2, luksErr)
		}

		// Open swap LUKS
		swapUUID1, err = common.LuksUUID(swapPart1)
		if err != nil {
			return fmt.Errorf("failed to get swap1 UUID: %w", err)
		}
		swapUUID2, err = common.LuksUUID(swapPart2)
		if err != nil {
			return fmt.Errorf("failed to get swap2 UUID: %w", err)
		}
		if err := common.LuksOpen(swapPart1, "luks-"+swapUUID1, u.LuksPassword); err != nil {
			return fmt.Errorf("LUKS open swap failed on %s: %w", d1, err)
		}
		if err := common.LuksOpen(swapPart2, "luks-"+swapUUID2, u.LuksPassword); err != nil {
			return fmt.Errorf("LUKS open swap failed on %s: %w", d2, err)
		}

		// Format swap
		swapSuffix1 = misc.GetDiskSuffix(d1)
		swapSuffix2 = misc.GetDiskSuffix(d2)
		if _, err := shell.ExecuteCmdArgs("", "mkswap", "-L", fmt.Sprintf("swap-%s%s2", swapSuffix1, misc.GetPartitionSpacer(d1)), common.LuksMapperPath(swapUUID1)); err != nil {
			return fmt.Errorf("mkswap failed on %s: %w", d1, err)
		}
		if _, err := shell.ExecuteCmdArgs("", "mkswap", "-L", fmt.Sprintf("swap-%s%s2", swapSuffix2, misc.GetPartitionSpacer(d2)), common.LuksMapperPath(swapUUID2)); err != nil {
			return fmt.Errorf("mkswap failed on %s: %w", d2, err)
		}
	}

	// 5. Create Btrfs RAID0 on top of LUKS
	// Pure RAID0 (data + metadata): with striping, losing one disk loses
	// everything anyway — metadata raid1 would only survive without the data.
	fmt.Println("[INFO] Creating Btrfs RAID0 on LUKS devices...")
	if _, err := shell.ExecuteCmdArgs("", "mkfs.btrfs", "-L", "myroot", "-d", "raid0", "-m", "raid0", luksRoot1, luksRoot2); err != nil {
		return fmt.Errorf("mkfs.btrfs RAID0 failed: %w", err)
	}

	// 6. Setup subvolumes and mounts
	if err := setupSubvolumesAndMounts(i.ChrootDir, luksRoot1, d1, d2); err != nil {
		return err
	}

	// 6a. Configure dmcrypt LUKS swap (keyfile + luksAddKey + conf.d) — only
	// when enabled. Root fs is now mounted at ChrootDir — write the keyfile
	// directly into the chroot (no temp copy). The swap LUKS partitions are
	// still open (mappers active), so luksAddKey works here. apk never
	// overwrites these new files.
	if u.EnableSwap {
		swapKey := "/etc/dmcrypt/swap.key"
		hostSwapKey := i.ChrootDir + swapKey
		if _, err := shell.ExecuteCmdArgs("", "mkdir", "-p", i.ChrootDir+"/etc/dmcrypt", i.ChrootDir+"/etc/conf.d"); err != nil {
			return fmt.Errorf("failed to create swap keyfile: %w", err)
		}
		if _, err := shell.ExecuteCmdArgs("", "install", "-m", "400", "/dev/null", hostSwapKey); err != nil {
			return fmt.Errorf("failed to create swap keyfile: %w", err)
		}
		if _, err := shell.ExecuteCmdArgs("", "dd", "if=/dev/urandom", "of="+hostSwapKey, "bs=512", "count=4", "conv=notrunc"); err != nil {
			return fmt.Errorf("failed to create swap keyfile: %w", err)
		}
		if err := common.LuksAddKey(swapPart1, hostSwapKey, u.LuksPassword); err != nil {
			return fmt.Errorf("failed to add key to swap1: %w", err)
		}
		if err := common.LuksAddKey(swapPart2, hostSwapKey, u.LuksPassword); err != nil {
			return fmt.Errorf("failed to add key to swap2: %w", err)
		}
		// Encrypted swap unlock — BOTH init systems (dual-init distro):
		//   /etc/crypttab       → dinit (chimera early-cryptdisks runs our
		//                         dinit-cryptdisks hook, keyfile in field 3)
		//   /etc/conf.d/dmcrypt → OpenRC (dmcrypt initd legacy format)
		// fstab then swapon's /dev/mapper/swap-* (chimera early-swap / OpenRC swap).
		crypttab := fmt.Sprintf(`swap-%s%s UUID=%s %s luks,discard
swap-%s%s UUID=%s %s luks,discard
`, swapSuffix1, misc.GetPartitionSpacer(d1)+"2", swapUUID1, swapKey,
			swapSuffix2, misc.GetPartitionSpacer(d2)+"2", swapUUID2, swapKey)
		escapedCrypttab := strings.ReplaceAll(crypttab, "'", "'\\''")
		if _, err := shell.ExecuteCmd("", fmt.Sprintf("echo '%s' > %s/etc/crypttab", escapedCrypttab, i.ChrootDir)); err != nil {
			return fmt.Errorf("failed to write crypttab: %w", err)
		}
		dmcryptConf := fmt.Sprintf(`
target=swap-%s%s
source=UUID=%s
key=%s

target=swap-%s%s
source=UUID=%s
key=%s

`, swapSuffix1, misc.GetPartitionSpacer(d1)+"2", swapUUID1, swapKey,
			swapSuffix2, misc.GetPartitionSpacer(d2)+"2", swapUUID2, swapKey)
		escapedDmcryptConf := strings.ReplaceAll(dmcryptConf, "'", "'\\''")
		if _, err := shell.ExecuteCmd("", fmt.Sprintf("echo '%s' > %s/etc/conf.d/dmcrypt", escapedDmcryptConf, i.ChrootDir)); err != nil {
			return fmt.Errorf("failed to write dmcrypt config: %w", err)
		}
		// swap service is conditional — only in the runlevel when swap exists
		u.Runlevels["boot"] = append(u.Runlevels["boot"], "swap")
	}

	// 7. Inject kernel cmdline for booster
	fmt.Println("[INFO] Injecting kernel cmdline for booster...")
	raidUUID1, _ := common.LuksUUID(d1 + misc.GetPartitionSpacer(d1) + rootPart)
	raidUUID2, _ := common.LuksUUID(d2 + misc.GetPartitionSpacer(d2) + rootPart)

	kernelCmdline := fmt.Sprintf(
		"rd.luks.name=%s=luks-%s rd.luks.name=%s=luks-%s root=UUID=%s rootfstype=btrfs rootflags=subvol=@,device=/dev/mapper/luks-%s rootdelay=30 rootwait",
		raidUUID1, raidUUID1,
		raidUUID2, raidUUID2,
		misc.GetBtrfsUUID(luksRoot1),
		raidUUID2,
	)
	u.KernelCmdline = append(u.KernelCmdline, kernelCmdline)

	// 8. Add modules
	u.Modules = append(u.Modules, "dm_mod", "dm_crypt", "crc32c_cryptoapi", "zstd", "btrfs", "efivarfs", "nvme")

	// 9. Generate Fstab
	return GenerateFstab(conf)
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
		// Nix package manager
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

	// EFI Mounts (both disks)
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

	return nil
}
