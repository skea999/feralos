package btrfsStandard

import (
	"alpineInstaller/installer/utility"
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

func RunBtrfsSetup(conf *utility.Config) error {
	u := &conf.User
	i := &conf.Internal

	if len(u.Disks) < 1 {
		return fmt.Errorf("no disk specified in the configuration 'disks'")
	}
	targetDisk := u.Disks[0]

	fmt.Printf("[INFO] Starting single-disk partitioning (Standard): %s\n", targetDisk)

	// 1. Pulizia ambiente (Swap)
	utility.RunCommand("swapoff", "-a")

	// 2. GPT Partitioning
	// n1: EFI (512M), n2: SWAP (8G), n3: ROOT/DATA (Restante)
	fdiskCommands := "g\nn\n1\n\n+512M\nt\n1\n1\nn\n2\n\n+8G\nt\n2\n19\nn\n3\n\n\nt\n3\n20\nw\n"

	cmd := exec.Command("fdisk", "-W", "always", targetDisk)
	cmd.Stdin = strings.NewReader(fdiskCommands)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("fdisk failed on %s: %w", targetDisk, err)
	}

	// 3. Formatting and Labeling
	fmt.Println("[INFO] Formatting partitions...")
	utility.RunCommand("mkfs.vfat", "-F", "32", targetDisk+"1")
	utility.RunCommand("fatlabel", targetDisk+"1", "EFI")
	utility.RunCommand("mkswap", "-L", "swap", targetDisk+"2")
	utility.RunCommand("mkfs.btrfs", "-f", "-L", "root", targetDisk+"3")

	// 4. Subvolume Creation and Mount
	if err := setupSubvolumesAndMounts(i.ChrootDir, targetDisk); err != nil {
		return err
	}

	// 5. Iniezione Variabili Dinamiche
	// In a single-disk standard layout, mdadm or raid features are not needed
	u.Modules = append(u.Modules, "btrfs")
	u.MkinitfsFeatures = append(u.MkinitfsFeatures, "btrfs")
	u.GrubCmdline = append(u.GrubCmdline, "rootfstype=btrfs", "rootflags=subvol=@")
	u.Runlevels["boot"] = append(u.Runlevels["boot"], "btrfs-scan")

	// 6. Fstab Generation
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

	// Temporary mount of the btrfs root to create the subvolumes
	utility.RunCommand("mount", disk+"3", chroot)
	for _, s := range subvols {
		utility.RunCommand("btrfs", "subvolume", "create", chroot+"/"+s.Name)
	}
	utility.RunCommand("umount", chroot)

	// Mount sistematico dei subvolumi nei rispettivi path
	for _, s := range subvols {
		target := chroot + s.Path
		if s.Path != "" {
			os.MkdirAll(target, 0755)
		}

		opts := "subvol=" + s.Name + "," + s.Options
		if s.NoCoW {
			opts += ",nodatacow"
		}

		utility.RunCommand("mount", "-t", "btrfs", "-o", opts, disk+"3", target)

		// Applica attributo NoCoW se richiesto (importante per performance VM/DB)
		if s.NoCoW {
			utility.RunCommand("chattr", "+C", target)
			utility.RunCommand("btrfs", "property", "set", target, "compression", "none")
		}
	}

	// Boot EFI Mount
	os.MkdirAll(chroot+"/boot/efi", 0755)
	utility.RunCommand("mount", disk+"1", chroot+"/boot/efi")

	return nil
}
