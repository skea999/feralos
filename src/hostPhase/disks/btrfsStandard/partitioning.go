package btrfsStandard

import (
	"alpineInstaller/utils/config"
	"alpineInstaller/utils/shell"
	"fmt"
	"os"
	"os/exec"
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

	fmt.Printf("[INFO] Starting single disk partitioning (Standard): %s\n", targetDisk)

	// 1. Cleanup environment
	fmt.Println("[INFO] Stopping existing swaps and mounts...")
	_, _ = shell.RunCommand("swapoff", "-a")
	// Unmount if something is mounted at ChrootDir (ignore errors)
	_, _ = shell.RunCommand("sh", "-c", "mountpoint -q "+i.ChrootDir+" && umount -fR "+i.ChrootDir+" || true")

	// 2. GPT Partitioning
	// n1: EFI (512M), n2: LUKS ROOT (remainder)
	// No swap partition — uses swapfile in @swap subvolume
	partitionSpacer := getPartitionSpacer(targetDisk)

	fdiskCommands := "g\nn\n1\n\n+512M\nt\n1\n1\nn\n2\n\n\nt\n2\n20\nw\n"

	cmd := exec.Command("fdisk", "-W", "always", targetDisk)
	cmd.Stdin = strings.NewReader(fdiskCommands)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("fdisk failed on %s: %w", targetDisk, err)
	}

	// 3. Format EFI partition
	efiPart := targetDisk + partitionSpacer + "1"
	if _, err := shell.RunCommand("mkfs.vfat", "-F", "32", efiPart); err != nil {
		return fmt.Errorf("failed to format EFI partition: %w", err)
	}
	label := "EFI" + strings.ToUpper(getDiskSuffix(targetDisk))
	if _, err := shell.RunCommand("fatlabel", efiPart, label); err != nil {
		return fmt.Errorf("failed to label EFI partition: %w", err)
	}

	// 4. LUKS Encryption on ROOT partition (partition 2)
	fmt.Println("[INFO] Configuring LUKS encryption on ROOT partition...")
	luksRoot, err := SetupLuksPartition(targetDisk+partitionSpacer+"2", u.Password)
	if err != nil {
		return fmt.Errorf("LUKS setup failed on ROOT: %w", err)
	}

	// 4.1 Format LUKS container as Btrfs
	fmt.Println("[INFO] Creating Btrfs filesystem on LUKS container...")
	if _, err := shell.RunCommand("mkfs.btrfs", "-f", "-L", "myroot", luksRoot); err != nil {
		return fmt.Errorf("failed to create btrfs filesystem: %w", err)
	}

	// 5. Setup subvolumes and mounts
	if err := setupSubvolumesAndMounts(i.ChrootDir, luksRoot, targetDisk); err != nil {
		return err
	}

	// 6. Create swapfile in @swap subvolume
	fmt.Println("[INFO] Creating swapfile in /swap subvolume...")
	if err := createSwapfile(i.ChrootDir); err != nil {
		return fmt.Errorf("swapfile creation failed: %w", err)
	}

	// 7. Inject kernel cmdline for booster
	fmt.Println("[INFO] Injecting kernel cmdline for booster...")
	rootUUID, _ := LuksUUID(targetDisk + partitionSpacer + "2")

	kernelCmdline := fmt.Sprintf(
		"rd.luks.name=%s=luks-%s root=UUID=%s rootfstype=btrfs rootflags=subvol=@ rootdelay=30 rootwait console=ttyS0,115200",
		rootUUID, rootUUID,
		getBtrfsUUID(luksRoot),
	)
	u.KernelCmdline = append(u.KernelCmdline, kernelCmdline)

	// 8. Add modules
	u.Modules = append(u.Modules, "dm_mod", "dm_crypt", "crc32c_cryptoapi", "zstd", "btrfs", "efivarfs", "nvme")

	// 9. Generate Fstab
	return GenerateFstab(conf)
}

// createSwapfile creates a swapfile in the @swap subvolume.
// Btrfs requires NoCoW for swapfiles.
// NoCoW is already set on /swap directory by setupSubvolumesAndMounts (chattr +C).
// Compression is disabled via compress=none mount option on @swap subvolume.
func createSwapfile(chroot string) error {
	swapDir := chroot + "/swap"
	swapFile := swapDir + "/swapfile"

	// Ensure swap directory exists
	if err := os.MkdirAll(swapDir, 0755); err != nil {
		return fmt.Errorf("failed to create swap directory: %w", err)
	}

	// Verify NoCoW is set on directory
	fmt.Println("[INFO] Verifying NoCoW on /swap directory...")
	if _, err := shell.RunCommand("lsattr", "-d", swapDir); err != nil {
		fmt.Printf("[WARNING] Could not verify NoCoW: %v\n", err)
	}

	// Create swapfile with dd (8G)
	if _, err := shell.RunCommand("dd", "if=/dev/zero", "of="+swapFile, "bs=1M", "count=8192", "status=progress"); err != nil {
		return fmt.Errorf("failed to create swapfile: %w", err)
	}

	// Set permissions
	if _, err := shell.RunCommand("chmod", "600", swapFile); err != nil {
		return fmt.Errorf("failed to chmod swapfile: %w", err)
	}

	// Format as swap
	if _, err := shell.RunCommand("mkswap", swapFile); err != nil {
		return fmt.Errorf("failed to mkswap: %w", err)
	}

	return nil
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

func setupSubvolumesAndMounts(chroot string, luksDevice string, disk string) error {
	subvols := []Subvolume{
		{Name: "@", Path: "/", Options: "compress=zstd:3,autodefrag,ssd"},
		{Name: "@swap", Path: "/swap", Options: "autodefrag,ssd,nodev,noexec,nosuid", NoCoW: true},
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
		// Nix package manager
		{Name: "@nix-store", Path: "/nix/store", Options: "compress=zstd:3,autodefrag,ssd"},
		{Name: "@nix-var-nix", Path: "/nix/var/nix", Options: "compress=zstd:3,autodefrag,ssd"},
		{Name: "@nix-var-log", Path: "/nix/var/log", Options: "autodefrag,ssd,nodev,noexec,nosuid", NoCoW: true},
		{Name: "@nix-var-nix-build", Path: "/nix/var/nix/build", Options: "compress=zstd:3,autodefrag,ssd"},
		// Btrbk backup
		{Name: "@btrbk", Path: "/btrbk", Options: "compress=zstd:3,autodefrag,ssd"},
	}

	// Step 1: Mount root WITHOUT compression to create subvolumes
	fmt.Println("[INFO] Mounting root without compression to create subvolumes...")
	if _, err := shell.RunCommand("mount", "-t", "btrfs", "-o", "autodefrag,ssd", luksDevice, chroot); err != nil {
		return fmt.Errorf("failed to mount btrfs root: %w", err)
	}

	// Step 2: Create all subvolumes
	for _, s := range subvols {
		if _, err := shell.RunCommand("btrfs", "subvolume", "create", chroot+"/"+s.Name); err != nil {
			return fmt.Errorf("failed to create subvolume %s: %w", s.Name, err)
		}
	}

	// Step 3: Disable compression on NoCoW subvolumes via btrfs property set
	for _, s := range subvols {
		if s.NoCoW {
			target := chroot + "/" + s.Name
			if _, err := shell.RunCommand("btrfs", "property", "set", target, "compression", "none"); err != nil {
				return fmt.Errorf("failed to disable compression on %s: %w", s.Name, err)
			}
		}
	}

	// Step 4: NoCoW already handled by btrfs property set compression none (Step 3)
	// chattr +C is not needed on btrfs subvolumes — btrfs property set handles it

	// Step 5: Unmount temporary root
	if _, err := shell.RunCommand("umount", chroot); err != nil {
		return fmt.Errorf("failed to unmount btrfs root: %w", err)
	}

	// Step 6: Mount each subvolume with its correct options
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
	}

	// EFI Mount
	partitionSpacer := getPartitionSpacer(disk)
	if err := os.MkdirAll(chroot+"/boot/efi", 0755); err != nil {
		return fmt.Errorf("failed to create /boot/efi: %w", err)
	}
	if _, err := shell.RunCommand("mount", disk+partitionSpacer+"1", chroot+"/boot/efi"); err != nil {
		return fmt.Errorf("failed to mount EFI: %w", err)
	}

	return nil
}
