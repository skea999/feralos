package btrfsUnencrypted

import (
	"alpineInstaller/hostPhase/disks/common"
	"alpineInstaller/utils/config"
	"alpineInstaller/utils/misc"
	"alpineInstaller/utils/shell"
	"alpineInstaller/utils/swap"
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

	if len(u.Disks) != 1 {
		return fmt.Errorf("btrfsUnencrypted requires exactly 1 disk, got %d: %v", len(u.Disks), u.Disks)
	}
	targetDisk := u.Disks[0]

	fmt.Printf("[INFO] Starting single disk partitioning (Unencrypted): %s\n", targetDisk)

	// 1. GPT Partitioning — EFI (512M) + ROOT (remainder)
	// No swap partition — uses swapfile in @swap subvolume
	partitionSpacer := misc.GetPartitionSpacer(targetDisk)

	fdiskCommands := "g\nn\n1\n\n+512M\nt\n1\n1\nn\n2\n\n\nt\n2\n20\nw\n"

	fdiskCmd := fmt.Sprintf("fdisk -W always %s <<'FDISK_EOF'\n%s\nFDISK_EOF", targetDisk, fdiskCommands)
	if _, fdiskErr := shell.ExecuteCmd("", fdiskCmd); fdiskErr != nil {
		return fmt.Errorf("fdisk failed on %s: %w", targetDisk, fdiskErr)
	}

	// 2. Format EFI partition
	efiPart := targetDisk + partitionSpacer + "1"
	if _, err := shell.ExecuteCmdArgs("", "mkfs.vfat", "-F", "32", efiPart); err != nil {
		return fmt.Errorf("failed to format EFI partition: %w", err)
	}
	label := "EFI" + strings.ToUpper(misc.GetDiskSuffix(targetDisk))
	if _, err := shell.ExecuteCmdArgs("", "fatlabel", efiPart, label); err != nil {
		return fmt.Errorf("failed to label EFI partition: %w", err)
	}

	// 3. Format ROOT partition directly as Btrfs (NO LUKS)
	rootPart := targetDisk + partitionSpacer + "2"
	fmt.Println("[INFO] Creating Btrfs filesystem on ROOT partition (no encryption)...")
	if _, err := shell.ExecuteCmdArgs("", "mkfs.btrfs", "-f", "-L", "myroot", rootPart); err != nil {
		return fmt.Errorf("failed to create btrfs filesystem: %w", err)
	}

	// 4. Setup subvolumes and mounts
	if err := setupSubvolumesAndMounts(i.ChrootDir, rootPart, targetDisk); err != nil {
		return err
	}

	// 5. Create swapfile in @swap subvolume (only if enabled)
	if u.EnableSwap {
		swapSizeMiB, err := swap.ResolveSwapSizeMiB(u.SwapSize)
		if err != nil {
			return fmt.Errorf("failed to resolve swap size: %w", err)
		}
		fmt.Println("[INFO] Creating swapfile in /swap subvolume...")
		if err := common.CreateSwapfile(i.ChrootDir, swapSizeMiB); err != nil {
			return fmt.Errorf("swapfile creation failed: %w", err)
		}
		u.Runlevels["boot"] = append(u.Runlevels["boot"], "swap")
	} else {
		fmt.Println("[INFO] Swapfile disabled — @swap subvolume kept for future use")
	}

	// 6. Kernel cmdline — root UUID directly, no LUKS
	fmt.Println("[INFO] Injecting kernel cmdline for booster...")
	rootUUID, _ := getRootUUID(rootPart)

	kernelCmdline := fmt.Sprintf(
		"root=UUID=%s rootfstype=btrfs rootflags=subvol=@ rootdelay=30 rootwait",
		rootUUID,
	)
	u.KernelCmdline = append(u.KernelCmdline, kernelCmdline)

	// 7. Add modules (no dm_mod, dm_crypt — no LUKS)
	u.Modules = append(u.Modules, "crc32c_cryptoapi", "zstd", "btrfs", "efivarfs", "nvme")

	// 8. Generate Fstab
	return GenerateFstab(conf)
}

// getRootUUID returns the UUID of a btrfs filesystem on a partition (not LUKS).
func getRootUUID(device string) (string, error) {
	out, err := shell.ExecuteCmdArgs("", "blkid", "-o", "value", "-s", "UUID", device)
	if err != nil {
		return "", fmt.Errorf("failed to get UUID for %s: %w", device, err)
	}
	return strings.TrimSpace(out), nil
}

func setupSubvolumesAndMounts(chroot string, rootDevice string, disk string) error {
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
		{Name: "@nix-store", Path: "/nix/store", Options: "compress=zstd:3,autodefrag,ssd"},
		{Name: "@nix-var-nix", Path: "/nix/var/nix", Options: "compress=zstd:3,autodefrag,ssd"},
		{Name: "@nix-var-log", Path: "/nix/var/log", Options: "autodefrag,ssd,nodev,noexec,nosuid", NoCoW: true},
		{Name: "@btrbk", Path: "/btrbk", Options: "compress=zstd:3,autodefrag,ssd"},
	}

	// Step 1: Mount root WITHOUT compression to create subvolumes
	fmt.Println("[INFO] Mounting root without compression to create subvolumes...")
	if _, err := shell.ExecuteCmdArgs("", "mount", "-t", "btrfs", "-o", "autodefrag,ssd", rootDevice, chroot); err != nil {
		return fmt.Errorf("failed to mount btrfs root: %w", err)
	}

	// Step 2: Create all subvolumes
	for _, s := range subvols {
		if _, err := shell.ExecuteCmdArgs("", "btrfs", "subvolume", "create", chroot+"/"+s.Name); err != nil {
			return fmt.Errorf("failed to create subvolume %s: %w", s.Name, err)
		}
	}

	// Step 3: Set NoCoW attribute on NoCoW subvolumes
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
		opts := "subvol=" + s.Name + "," + s.Options
		if s.NoCoW {
			opts += ",nodatacow"
		}
		if _, err := shell.ExecuteCmdArgs("", "mount", "-t", "btrfs", "-o", opts, rootDevice, target); err != nil {
			return fmt.Errorf("failed to mount subvolume %s: %w", s.Name, err)
		}
	}

	// EFI Mount
	partitionSpacer := misc.GetPartitionSpacer(disk)
	if _, err := shell.ExecuteCmdArgs("", "mkdir", "-p", chroot+"/boot/efi"); err != nil {
		return fmt.Errorf("failed to create /boot/efi: %w", err)
	}
	if _, err := shell.ExecuteCmdArgs("", "mount", disk+partitionSpacer+"1", chroot+"/boot/efi"); err != nil {
		return fmt.Errorf("failed to mount EFI: %w", err)
	}

	return nil
}
