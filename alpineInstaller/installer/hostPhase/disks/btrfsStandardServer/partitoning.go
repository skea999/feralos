package btrfsStandardServer

import (
	"alpineInstaller/installer/utility"
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

func RunBtrfsSetup(conf *utility.Config) error {
	u := &conf.User
	i := &conf.Internal

	// Check that there are at least 2 disks for the RAID1
	if len(u.Disks) < 2 {
		return fmt.Errorf("error: the RAID1 layout requires at least 2 disks, configured: %d", len(u.Disks))
	}

	d1 := u.Disks[0]
	d2 := u.Disks[1]

	if d1 == d2 {
		return fmt.Errorf("error: disk1 and disk2 are the same (%s)", d1)
	}

	// 2. Cleanup environment
	fmt.Println("[INFO] Stopping existing RAIDs and swaps...")
	utility.RunCommand("mdadm", "--stop", "/dev/md?")
	utility.RunCommand("swapoff", "-a")

	// 3. Dynamic size calculation
	minSizeBytes, err := getMinDiskSize(d1, d2)
	if err != nil {
		return err
	}

	// Logic: partitions for RAID, Swap, etc.
	firstPartK := (minSizeBytes / 1024 / 4 * 2) - (512 * 1024)
	secondPartK := (minSizeBytes / 1024 / 4)

	fmt.Printf("[DEBUG] Calculated Raid Partition: %d K\n", firstPartK)
	fmt.Printf("[DEBUG] Calculated Swap Partition: %d K\n", secondPartK)

	// 4. Partitioning loop (only on the first two disks in the list)
	for _, disk := range []string{d1, d2} {
		fmt.Printf("[INFO] Partitioning disk %s...\n", disk)

		fdiskCommands := fmt.Sprintf("g\nn\n1\n\n+512M\nt\n1\nn\n2\n\n+%dK\nt\n2\n43\nn\n3\n\n+%dK\nt\n3\n19\nn\n4\n\n\nt\n4\n20\nw\n", firstPartK, secondPartK)

		cmd := exec.Command("fdisk", "-W", "always", disk)
		cmd.Stdin = strings.NewReader(fdiskCommands)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("fdisk failed on %s: %w", disk, err)
		}

		suffix := strings.ToLower(disk[strings.LastIndex(disk, "/")+1:])
		utility.RunCommand("mkfs.vfat", "-F", "32", disk+"1")
		utility.RunCommand("fatlabel", disk+"1", "EFI"+strings.ToUpper(suffix))
		utility.RunCommand("mkswap", "-L", "swap-"+suffix, disk+"3")
		utility.RunCommand("mkfs.btrfs", "-f", "-L", "data-"+suffix, disk+"4")
	}

	// 5. RAID Creation
	utility.RunCommand("modprobe", "raid1")
	fmt.Println("[INFO] Creating RAID1 /dev/md0...")
	// NOTE: we use disk1+"2" and disk2+"2" (the second partition of each disk)
	cmdRaid := exec.Command("mdadm", "--create", "--level=1", "--raid-devices=2", "/dev/md0", d1+"2", d2+"2", "--quiet", "--force")
	cmdRaid.Stdin = strings.NewReader("y\n")
	cmdRaid.Run()

	// 6. Partitioning /dev/md0
	fmt.Println("[INFO] Partitioning RAID /dev/md0...")
	md0Commands := "g\nn\n1\n\n+512M\nt\n1\n20\nn\n2\n\n\nt\n2\n20\nw\n"
	cmdMd0 := exec.Command("fdisk", "-W", "always", "/dev/md0")
	cmdMd0.Stdin = strings.NewReader(md0Commands)
	cmdMd0.Run()

	utility.RunCommand("mkfs.btrfs", "-f", "-L", "boot", "/dev/md0p1")
	utility.RunCommand("mkfs.btrfs", "-f", "-L", "root", "/dev/md0p2")

	// 7. Subvolumes and Mounts
	if err := setupSubvolumesAndMounts(i.ChrootDir, d1, d2); err != nil {
		return err
	}

	// 8. Generate RAID configuration and Dynamic Variables Injection
	fmt.Println("[INFO] Saving RAID configuration and updating dynamic variables...")
	os.MkdirAll(i.ChrootDir+"/etc", 0755)

	u.Modules = append(u.Modules, "btrfs", "raid1")
	u.MkinitfsFeatures = append(u.MkinitfsFeatures, "btrfs", "raid", "mdadm")
	u.GrubCmdline = append(u.GrubCmdline, "rootfstype=btrfs", "rootflags=subvol=@")
	u.Runlevels["boot"] = append(u.Runlevels["boot"], "mdadm", "mdadm-raid", "btrfs-scan")

	mdadmCmd := fmt.Sprintf("mdadm --detail --scan > %s/etc/mdadm.conf", i.ChrootDir)
	if _, err := utility.RunCommand("sh", "-c", mdadmCmd); err != nil {
		fmt.Printf("[WARNING] Could not generate mdadm.conf: %v\n", err)
	}

	// 9. Generate Fstab
	return GenerateFstabWithLabels(conf)
}

func getMinDiskSize(d1, d2 string) (int64, error) {
	var minSize int64 = 9223372036854775807
	for _, d := range []string{d1, d2} {
		out, err := utility.RunCommand("blockdev", "--getsize64", d)
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

func setupSubvolumesAndMounts(chroot string, d1 string, d2 string) error {
	subvols := []Subvolume{
		{Name: "@", Path: "", Options: "compress=zstd:3,autodefrag,ssd"},
		{Name: "@swap", Path: "/swap", Options: "autodefrag,ssd", NoCoW: true},
		{Name: "@home", Path: "/home", Options: "compress=zstd:3,autodefrag,ssd"},
		{Name: "@opt", Path: "/opt", Options: "compress=zstd:3,autodefrag,ssd"},
		{Name: "@root", Path: "/root", Options: "compress=zstd:3,autodefrag,ssd"},
		{Name: "@srv", Path: "/srv", Options: "nodev,noexec,nosuid", NoCoW: true},
		{Name: "@tmp", Path: "/tmp", Options: "noexec,nosuid,compress=zstd:3,autodefrag,ssd"},
		{Name: "@var", Path: "/var", Options: "compress=zstd:3,autodefrag,ssd"},
		{Name: "@var-log", Path: "/var/log", Options: "nodev,noexec,nosuid", NoCoW: true},
		{Name: "@usr-local", Path: "/usr/local", Options: "compress=zstd:3,autodefrag,ssd"},
	}

	// Create Subvolumes
	utility.RunCommand("mount", "-o", "compress=zstd:3,autodefrag,ssd", "/dev/md0p2", chroot)
	for _, s := range subvols {
		utility.RunCommand("btrfs", "subvolume", "create", chroot+"/"+s.Name)
	}
	utility.RunCommand("umount", chroot)

	// Systematic Mount
	for _, s := range subvols {
		target := chroot + s.Path
		if s.Path != "" {
			os.MkdirAll(target, 0755)
		}
		opts := "subvol=" + s.Name + "," + s.Options
		if s.NoCoW {
			opts += ",nodatacow"
		}

		utility.RunCommand("mount", "-t", "btrfs", "-o", opts, "/dev/md0p2", target)
		if s.NoCoW {
			utility.RunCommand("chattr", "+C", target)
			utility.RunCommand("btrfs", "property", "set", target, "compression", "none")
		}
	}

	// Boot & EFI Mounts
	os.MkdirAll(chroot+"/boot", 0755)
	utility.RunCommand("mount", "/dev/md0p1", chroot+"/boot")
	os.MkdirAll(chroot+"/boot/efi", 0755)
	utility.RunCommand("mount", d1+"1", chroot+"/boot/efi")
	os.MkdirAll(chroot+"/boot/efi2", 0755)
	utility.RunCommand("mount", d2+"1", chroot+"/boot/efi2")

	// External Data Mounts
	s1 := strings.ToLower(d1[strings.LastIndex(d1, "/")+1:])
	s2 := strings.ToLower(d2[strings.LastIndex(d2, "/")+1:])
	os.MkdirAll(chroot+"/data/data-"+s1, 0755)
	utility.RunCommand("mount", "-t", "btrfs", "-o", "compress=zstd:3,autodefrag,ssd", d1+"4", chroot+"/data/data-"+s1)
	os.MkdirAll(chroot+"/data/data-"+s2, 0755)
	utility.RunCommand("mount", "-t", "btrfs", "-o", "compress=zstd:3,autodefrag,ssd", d2+"4", chroot+"/data/data-"+s2)

	return nil
}
