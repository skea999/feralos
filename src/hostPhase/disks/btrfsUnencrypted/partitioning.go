package btrfsUnencrypted

import (
	"alpineInstaller/utils/config"
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

	if len(u.Disks) < 1 {
		return fmt.Errorf("no disk specified in 'disks' configuration")
	}
	targetDisk := u.Disks[0]

	fmt.Printf("[INFO] Starting single disk partitioning (Unencrypted): %s\n", targetDisk)

	// 1. GPT Partitioning — EFI (512M) + ROOT (remainder)
	// No swap partition — uses swapfile in @swap subvolume
	partitionSpacer := getPartitionSpacer(targetDisk)

	fdiskCommands := "g\nn\n1\n\n+512M\nt\n1\n1\nn\n2\n\n\nt\n2\n20\nw\n"

	fdiskCmd := fmt.Sprintf("fdisk -W always %s <<'FDISK_EOF'\n%s\nFDISK_EOF", targetDisk, fdiskCommands)
	if _, fdiskErr := shell.ExecuteCmd("", fdiskCmd); fdiskErr != nil {
		return fmt.Errorf("fdisk failed on %s: %w", targetDisk, fdiskErr)
	}

	// 2. Format EFI partition
	efiPart := targetDisk + partitionSpacer + "1"
	if _, err := shell.ExecuteCmd("", fmt.Sprintf("mkfs.vfat -F 32 %s", efiPart)); err != nil {
		return fmt.Errorf("failed to format EFI partition: %w", err)
	}
	label := "EFI" + strings.ToUpper(getDiskSuffix(targetDisk))
	if _, err := shell.ExecuteCmd("", fmt.Sprintf("fatlabel %s %s", efiPart, label)); err != nil {
		return fmt.Errorf("failed to label EFI partition: %w", err)
	}

	// 3. Format ROOT partition directly as Btrfs (NO LUKS)
	rootPart := targetDisk + partitionSpacer + "2"
	fmt.Println("[INFO] Creating Btrfs filesystem on ROOT partition (no encryption)...")
	if _, err := shell.ExecuteCmd("", fmt.Sprintf("mkfs.btrfs -f -L myroot %s", rootPart)); err != nil {
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
		if err := createSwapfile(i.ChrootDir, swapSizeMiB); err != nil {
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
		"root=UUID=%s rootfstype=btrfs rootflags=subvol=@ rootdelay=30 rootwait console=ttyS0,115200",
		rootUUID,
	)
	u.KernelCmdline = append(u.KernelCmdline, kernelCmdline)

	// 7. Add modules (no dm_mod, dm_crypt — no LUKS)
	u.Modules = append(u.Modules, "crc32c_cryptoapi", "zstd", "btrfs", "efivarfs", "nvme")

	// 8. Generate Fstab
	return GenerateFstab(conf)
}

// createSwapfile creates a swapfile using btrfs native command.
func createSwapfile(chroot string, sizeMiB int) error {
	swapDir := chroot + "/swap"
	swapFile := swapDir + "/swapfile"

	if _, err := shell.ExecuteCmd("", fmt.Sprintf("mkdir -p %s", swapDir)); err != nil {
		return fmt.Errorf("failed to create swap directory: %w", err)
	}

	_, _ = shell.ExecuteCmd("", fmt.Sprintf("rm -f %s", swapFile))

	fmt.Println("[INFO] Creating swapfile with btrfs filesystem mkswapfile...")
	if _, err := shell.ExecuteCmd("", fmt.Sprintf("btrfs filesystem mkswapfile --size %dM %s", sizeMiB, swapFile)); err != nil {
		return fmt.Errorf("failed to create swapfile: %w", err)
	}

	lsattrOut, _ := shell.ExecuteCmd("", fmt.Sprintf("lsattr %s", swapFile))
	fmt.Printf("[INFO] lsattr swapfile: %s\n", strings.TrimSpace(lsattrOut))

	return nil
}

// getRootUUID returns the UUID of a btrfs filesystem on a partition (not LUKS).
func getRootUUID(device string) (string, error) {
	out, err := shell.ExecuteCmd("", fmt.Sprintf("blkid -o value -s UUID %s", device))
	if err != nil {
		return "", fmt.Errorf("failed to get UUID for %s: %w", device, err)
	}
	return strings.TrimSpace(out), nil
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

func setupSubvolumesAndMounts(chroot string, rootDevice string, disk string) error {
	subvols := []Subvolume{
		{Name: "@", Path: "/", Options: "compress=zstd:3,autodefrag,ssd"},
		{Name: "@swap", Path: "/swap", Options: "ssd,compress=none,nodev,noexec,nosuid", NoCoW: true},
		{Name: "@home", Path: "/home", Options: "compress=zstd:3,autodefrag,ssd"},
		{Name: "@opt", Path: "/opt", Options: "compress=zstd:3,autodefrag,ssd"},
		{Name: "@root", Path: "/root", Options: "compress=zstd:3,autodefrag,ssd"},
		{Name: "@srv", Path: "/srv", Options: "autodefrag,ssd,nodev,noexec,nosuid", NoCoW: true},
		{Name: "@tmp", Path: "/tmp", Options: "compress=zstd:3,autodefrag,ssd,noexec,nosuid"},
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
		{Name: "@nix-var-nix-build", Path: "/nix/var/nix/build", Options: "compress=zstd:3,autodefrag,ssd"},
		{Name: "@btrbk", Path: "/btrbk", Options: "compress=zstd:3,autodefrag,ssd"},
	}

	// Step 1: Mount root WITHOUT compression to create subvolumes
	fmt.Println("[INFO] Mounting root without compression to create subvolumes...")
	if _, err := shell.ExecuteCmd("", fmt.Sprintf("mount -t btrfs -o autodefrag,ssd %s %s", rootDevice, chroot)); err != nil {
		return fmt.Errorf("failed to mount btrfs root: %w", err)
	}

	// Step 2: Create all subvolumes
	for _, s := range subvols {
		if _, err := shell.ExecuteCmd("", fmt.Sprintf("btrfs subvolume create %s/%s", chroot, s.Name)); err != nil {
			return fmt.Errorf("failed to create subvolume %s: %w", s.Name, err)
		}
	}

	// Step 3: Set NoCoW attribute on NoCoW subvolumes
	for _, s := range subvols {
		if s.NoCoW {
			target := chroot + "/" + s.Name
			if _, err := shell.ExecuteCmd("", fmt.Sprintf("chattr +C %s", target)); err != nil {
				return fmt.Errorf("failed to set NoCoW on %s: %w", s.Name, err)
			}
		}
	}

	// Step 4: Unmount temporary root
	if _, err := shell.ExecuteCmd("", fmt.Sprintf("umount %s", chroot)); err != nil {
		return fmt.Errorf("failed to unmount btrfs root: %w", err)
	}

	// Step 5: Mount each subvolume with its correct options
	for _, s := range subvols {
		target := chroot + s.Path
		if s.Path != "" {
			if _, err := shell.ExecuteCmd("", fmt.Sprintf("mkdir -p %s", target)); err != nil {
				return fmt.Errorf("failed to create directory %s: %w", target, err)
			}
		}
		opts := "subvol=" + s.Name + "," + s.Options
		if s.NoCoW {
			opts += ",nodatacow"
		}
		if _, err := shell.ExecuteCmd("", fmt.Sprintf("mount -t btrfs -o %s %s %s", opts, rootDevice, target)); err != nil {
			return fmt.Errorf("failed to mount subvolume %s: %w", s.Name, err)
		}
	}

	// EFI Mount
	partitionSpacer := getPartitionSpacer(disk)
	if _, err := shell.ExecuteCmd("", fmt.Sprintf("mkdir -p %s", chroot+"/boot/efi")); err != nil {
		return fmt.Errorf("failed to create /boot/efi: %w", err)
	}
	if _, err := shell.ExecuteCmd("", fmt.Sprintf("mount %s %s", disk+partitionSpacer+"1", chroot+"/boot/efi")); err != nil {
		return fmt.Errorf("failed to mount EFI: %w", err)
	}

	return nil
}
