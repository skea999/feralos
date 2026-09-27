package btrfsStandard

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
		return fmt.Errorf("btrfsStandard requires exactly 1 disk, got %d: %v", len(u.Disks), u.Disks)
	}
	targetDisk := u.Disks[0]

	fmt.Printf("[INFO] Starting single disk partitioning (Standard): %s\n", targetDisk)

	// 1. GPT Partitioning
	// n1: EFI (512M), n2: LUKS ROOT (remainder)
	// No swap partition — uses swapfile in @swap subvolume
	partitionSpacer := misc.GetPartitionSpacer(targetDisk)

	fdiskCommands := "g\nn\n1\n\n+512M\nt\n1\n1\nn\n2\n\n\nt\n2\n20\nw\n"

	fdiskCmd := fmt.Sprintf("fdisk -W always %s <<'FDISK_EOF'\n%s\nFDISK_EOF", targetDisk, fdiskCommands)
	if _, fdiskErr := shell.ExecuteCmd("", fdiskCmd); fdiskErr != nil {
		return fmt.Errorf("fdisk failed on %s: %w", targetDisk, fdiskErr)
	}

	// 3. Format EFI partition
	efiPart := targetDisk + partitionSpacer + "1"
	if _, err := shell.ExecuteCmdArgs("", "mkfs.vfat", "-F", "32", efiPart); err != nil {
		return fmt.Errorf("failed to format EFI partition: %w", err)
	}
	label := "EFI" + strings.ToUpper(misc.GetDiskSuffix(targetDisk))
	if _, err := shell.ExecuteCmdArgs("", "fatlabel", efiPart, label); err != nil {
		return fmt.Errorf("failed to label EFI partition: %w", err)
	}

	// 4. LUKS Encryption on ROOT partition (partition 2)
	fmt.Println("[INFO] Configuring LUKS encryption on ROOT partition...")
	luksRoot, err := common.SetupLuksPartition(targetDisk+partitionSpacer+"2", u.LuksPassword)
	if err != nil {
		return fmt.Errorf("LUKS setup failed on ROOT: %w", err)
	}

	// 4.1 Format LUKS container as Btrfs
	fmt.Println("[INFO] Creating Btrfs filesystem on LUKS container...")
	if _, err := shell.ExecuteCmdArgs("", "mkfs.btrfs", "-f", "-L", "myroot", luksRoot); err != nil {
		return fmt.Errorf("failed to create btrfs filesystem: %w", err)
	}

	// 5. Setup subvolumes and mounts
	if err := setupSubvolumesAndMounts(i.ChrootDir, luksRoot, targetDisk); err != nil {
		return err
	}

	// 6. Create swapfile in @swap subvolume (only if enabled — the @swap
	// subvolume itself is always created so swap can be enabled later)
	if u.EnableSwap {
		swapSizeMiB, err := swap.ResolveSwapSizeMiB(u.SwapSize)
		if err != nil {
			return fmt.Errorf("failed to resolve swap size: %w", err)
		}
		fmt.Println("[INFO] Creating swapfile in /swap subvolume...")
		if err := common.CreateSwapfile(i.ChrootDir, swapSizeMiB); err != nil {
			return fmt.Errorf("swapfile creation failed: %w", err)
		}
		// swap service is conditional — only in the runlevel when swap exists
		u.Runlevels["boot"] = append(u.Runlevels["boot"], "swap")
	} else {
		fmt.Println("[INFO] Swapfile disabled — @swap subvolume kept for future use")
	}

	// 7. Inject kernel cmdline for booster
	fmt.Println("[INFO] Injecting kernel cmdline for booster...")
	rootUUID, _ := common.LuksUUID(targetDisk + partitionSpacer + "2")

	kernelCmdline := fmt.Sprintf(
		"rd.luks.name=%s=luks-%s root=UUID=%s rootfstype=btrfs rootflags=subvol=@ rootdelay=30 rootwait",
		rootUUID, rootUUID,
		misc.GetBtrfsUUID(luksRoot),
	)
	u.KernelCmdline = append(u.KernelCmdline, kernelCmdline)

	// 8. Add modules
	u.Modules = append(u.Modules, "dm_mod", "dm_crypt", "crc32c_cryptoapi", "zstd", "btrfs", "efivarfs", "nvme")

	// 9. Generate Fstab
	return GenerateFstab(conf)
}

func setupSubvolumesAndMounts(chroot string, luksDevice string, disk string) error {
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
