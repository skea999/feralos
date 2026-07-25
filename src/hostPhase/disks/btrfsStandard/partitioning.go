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
	u := &conf.User
	i := &conf.Internal

	if len(u.Disks) < 1 {
		return fmt.Errorf("no disk specified in 'disks' configuration")
	}
	targetDisk := u.Disks[0]

	fmt.Printf("[INFO] Starting single disk partitioning (Standard): %s\n", targetDisk)

	// 1. Cleanup environment (Swap)
	if _, err := shell.RunCommand("swapoff", "-a"); err != nil {
		return fmt.Errorf("failed to disable swap: %w", err)
	}

	// 2. GPT Partitioning
	// n1: EFI (512M), n2: SWAP (8G), n3: ROOT/DATA (Remaining)
	fdiskCommands := "g\nn\n1\n\n+512M\nt\n1\n1\nn\n2\n\n+8G\nt\n2\n19\nn\n3\n\n\nt\n3\n20\nw\n"

	cmd := exec.Command("fdisk", "-W", "always", targetDisk)
	cmd.Stdin = strings.NewReader(fdiskCommands)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("fdisk failed on %s: %w", targetDisk, err)
	}

	// 3. Formatting and Labeling
	fmt.Println("[INFO] Formatting partitions...")
	if _, err := shell.RunCommand("mkfs.vfat", "-F", "32", targetDisk+"1"); err != nil {
		return fmt.Errorf("failed to format EFI partition: %w", err)
	}
	if _, err := shell.RunCommand("fatlabel", targetDisk+"1", "EFI"); err != nil {
		return fmt.Errorf("failed to label EFI partition: %w", err)
	}
	if _, err := shell.RunCommand("mkswap", "-L", "swap", targetDisk+"2"); err != nil {
		return fmt.Errorf("failed to create swap: %w", err)
	}
	if _, err := shell.RunCommand("mkfs.btrfs", "-f", "-L", "root", targetDisk+"3"); err != nil {
		return fmt.Errorf("failed to format root partition: %w", err)
	}

	// 4. Create Subvolumes and Mount
	if err := setupSubvolumesAndMounts(i.ChrootDir, targetDisk); err != nil {
		return err
	}

	// 5. Dynamic Variables Injection
	// In single disk standard layout, mdadm or raid features are not needed
	u.Modules = append(u.Modules, "btrfs")
	u.MkinitfsFeatures = append(u.MkinitfsFeatures, "btrfs")
	u.GrubCmdline = append(u.GrubCmdline, "rootfstype=btrfs", "rootflags=subvol=@")
	u.Runlevels["boot"] = append(u.Runlevels["boot"], "btrfs-scan")

	// 6. Generate Fstab
	return GenerateFstab(conf)
}

func setupSubvolumesAndMounts(chroot string, disk string) error {
	subvols := []Subvolume{
		{Name: "@", Path: "", Options: "compress=zstd:3,autodefrag,ssd"},
		{Name: "@home", Path: "/home", Options: "compress=zstd:3,autodefrag,ssd"},
		{Name: "@opt", Path: "/opt", Options: "compress=zstd:3,autodefrag,ssd"},
		{Name: "@root", Path: "/root", Options: "compress=zstd:3,autodefrag,ssd"},
		{Name: "@srv", Path: "/srv", Options: "nodev,noexec,nosuid", NoCoW: true},
		{Name: "@tmp", Path: "/tmp", Options: "noexec,nosuid,compress=zstd:3,autodefrag,ssd"},
		{Name: "@var", Path: "/var", Options: "compress=zstd:3,autodefrag,ssd"},
		{Name: "@var-log", Path: "/var/log", Options: "nodev,noexec,nosuid", NoCoW: true},
		{Name: "@usr-local", Path: "/usr/local", Options: "compress=zstd:3,autodefrag,ssd"},
	}

	// Temporary mount of btrfs root to create subvolumes
	if _, err := shell.RunCommand("mount", disk+"3", chroot); err != nil {
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

	// Systematic mount of subvolumes to their respective paths
	for _, s := range subvols {
		target := chroot + s.Path
		if s.Path != "" {
			if err := os.MkdirAll(target, 0755); err != nil {
				return fmt.Errorf("failed to create directory %s: %w", target, err)
			}
		}

		opts := "subvol=" + s.Name + "," + s.Options
		if s.NoCoW {
			opts += ",nodatacow"
		}

		if _, err := shell.RunCommand("mount", "-t", "btrfs", "-o", opts, disk+"3", target); err != nil {
			return fmt.Errorf("failed to mount subvolume %s: %w", s.Name, err)
		}

		// Apply NoCoW attribute if requested (important for VM/DB performance)
		if s.NoCoW {
			if _, err := shell.RunCommand("chattr", "+C", target); err != nil {
				return fmt.Errorf("failed to set NoCoW attribute on %s: %w", target, err)
			}
			if _, err := shell.RunCommand("btrfs", "property", "set", target, "compression", "none"); err != nil {
				return fmt.Errorf("failed to disable compression on %s: %w", target, err)
			}
		}
	}

	// Boot EFI Mount
	if err := os.MkdirAll(chroot+"/boot/efi", 0755); err != nil {
		return fmt.Errorf("failed to create /boot/efi directory: %w", err)
	}
	if _, err := shell.RunCommand("mount", disk+"1", chroot+"/boot/efi"); err != nil {
		return fmt.Errorf("failed to mount EFI partition: %w", err)
	}

	return nil
}
