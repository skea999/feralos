package btrfsUnencrypted

import (
	"alpineInstaller/utils/config"
	"alpineInstaller/utils/misc"
	"alpineInstaller/utils/shell"
	"fmt"
	"strings"
)

// GenerateFstab writes /etc/fstab for single disk unencrypted layout.
// Same as btrfsStandard — uses LABEL=myroot for btrfs subvolumes, LABEL=EFI* for EFI.
func GenerateFstab(conf *config.Config) error {
	u := conf.User
	i := conf.Internal

	if len(u.Disks) < 1 {
		return fmt.Errorf("btrfsUnencrypted layout requires at least 1 disk, found: %d", len(u.Disks))
	}

	disk := u.Disks[0]

	fmt.Println("[INFO] Writing /etc/fstab for Btrfs Unencrypted Layout...")

	fstabPath := i.ChrootDir + "/etc/fstab"
	if _, err := shell.ExecuteCmd("", fmt.Sprintf("mkdir -p %s", i.ChrootDir+"/etc")); err != nil {
		return fmt.Errorf("failed to create /etc directory: %w", err)
	}
	var fstabContent strings.Builder

	// Header comment
	fstabContent.WriteString("# btrfs compression: per-subvolume via 'btrfs property set compression none'\n")
	fstabContent.WriteString("# NoCoW subvolumes: set via 'chattr +C <path>' during installation\n")
	fstabContent.WriteString("# These properties are set during installation and persist across reboots\n\n")

	// Btrfs subvolumes (LABEL=myroot)
	subvols := []struct {
		Name    string
		Path    string
		Options string
		NoCoW   bool
	}{
		{Name: "@", Path: "/", Options: "compress=zstd:3,autodefrag,ssd,subvol=@"},
		{Name: "@swap", Path: "/swap", Options: "ssd,compress=none,subvol=@swap", NoCoW: true},
		{Name: "@home", Path: "/home", Options: "compress=zstd:3,autodefrag,ssd,subvol=@home"},
		{Name: "@opt", Path: "/opt", Options: "compress=zstd:3,autodefrag,ssd,subvol=@opt"},
		{Name: "@root", Path: "/root", Options: "compress=zstd:3,autodefrag,ssd,subvol=@root"},
		{Name: "@srv", Path: "/srv", Options: "autodefrag,ssd,nodev,noexec,nosuid,subvol=@srv", NoCoW: true},
		{Name: "@tmp", Path: "/tmp", Options: "compress=zstd:3,autodefrag,ssd,noexec,nosuid,subvol=@tmp"},
		{Name: "@var", Path: "/var", Options: "compress=zstd:3,autodefrag,ssd,subvol=@var"},
		{Name: "@var-lib-ex", Path: "/var/lib/ex", Options: "autodefrag,ssd,nodev,noexec,nosuid,subvol=@var-lib-ex", NoCoW: true},
		{Name: "@var-lib-libvirt", Path: "/var/lib/libvirt", Options: "compress=zstd:3,autodefrag,ssd,subvol=@var-lib-libvirt"},
		{Name: "@var-lib-incus", Path: "/var/lib/incus", Options: "compress=zstd:3,autodefrag,ssd,subvol=@var-lib-incus"},
		{Name: "@var-lib-containerd", Path: "/var/lib/containerd", Options: "compress=zstd:3,autodefrag,ssd,subvol=@var-lib-containerd"},
		{Name: "@var-lib-docker", Path: "/var/lib/docker", Options: "compress=zstd:3,autodefrag,ssd,subvol=@var-lib-docker"},
		{Name: "@var-lib-containers", Path: "/var/lib/containers", Options: "compress=zstd:3,autodefrag,ssd,subvol=@var-lib-containers"},
		{Name: "@var-lib-vector", Path: "/var/lib/vector", Options: "compress=zstd:3,autodefrag,ssd,subvol=@var-lib-vector"},
		{Name: "@var-cache", Path: "/var/cache", Options: "compress=zstd:3,autodefrag,ssd,subvol=@var-cache"},
		{Name: "@var-log", Path: "/var/log", Options: "autodefrag,ssd,nodev,noexec,nosuid,subvol=@var-log", NoCoW: true},
		{Name: "@var-opt", Path: "/var/opt", Options: "compress=zstd:3,autodefrag,ssd,subvol=@var-opt"},
		{Name: "@var-spool", Path: "/var/spool", Options: "autodefrag,ssd,nodev,noexec,nosuid,subvol=@var-spool", NoCoW: true},
		{Name: "@var-tmp", Path: "/var/tmp", Options: "autodefrag,ssd,nodev,noexec,nosuid,subvol=@var-tmp", NoCoW: true},
		{Name: "@usr-local", Path: "/usr/local", Options: "compress=zstd:3,autodefrag,ssd,subvol=@usr-local"},
		{Name: "@boot", Path: "/boot", Options: "compress=zstd:3,autodefrag,ssd,subvol=@boot"},
		{Name: "@nix-store", Path: "/nix/store", Options: "compress=zstd:3,autodefrag,ssd,subvol=@nix-store"},
		{Name: "@nix-var-nix", Path: "/nix/var/nix", Options: "compress=zstd:3,autodefrag,ssd,subvol=@nix-var-nix"},
		{Name: "@nix-var-log", Path: "/nix/var/log", Options: "autodefrag,ssd,nodev,noexec,nosuid,subvol=@nix-var-log", NoCoW: true},
		{Name: "@nix-var-nix-build", Path: "/nix/var/nix/build", Options: "compress=zstd:3,autodefrag,ssd,subvol=@nix-var-nix-build"},
		{Name: "@btrbk", Path: "/btrbk", Options: "compress=zstd:3,autodefrag,ssd,subvol=@btrbk"},
	}

	for _, s := range subvols {
		opts := s.Options
		if s.NoCoW {
			opts += ",nodatacow"
			fstabContent.WriteString(fmt.Sprintf("# %s: NoCoW via nodatacow + btrfs property set compression none\n", s.Name))
		}
		fstabContent.WriteString(fmt.Sprintf("LABEL=myroot  %-25s  btrfs  %-70s  0  0\n", s.Path, opts))
	}

	// EFI Partition
	s1 := strings.ToUpper(misc.GetDiskSuffix(disk))
	fmt.Fprintf(&fstabContent, "\nLABEL=EFI%-10s  /boot/efi               vfat   rw,noatime,fmask=0022,dmask=0022,codepage=437,iocharset=ascii,shortname=mixed,utf8,errors=remount-ro  0  2\n", s1)

	// Swapfile
	if u.EnableSwap {
		fstabContent.WriteString("# Swapfile: NoCoW set via 'chattr +C' + compress=none mount option\n")
		fstabContent.WriteString("/swap/swapfile             none                    swap   defaults                                          0  0\n")
	}

	fstabContent.WriteString("# Generated by FeralOS Installer\n")
	writeCmd := fmt.Sprintf("cat > %s <<'FSTAB_EOF'\n%s\nFSTAB_EOF", fstabPath, fstabContent.String())
	if _, err := shell.ExecuteCmd("", writeCmd); err != nil {
		return fmt.Errorf("failed to write fstab: %w", err)
	}

	return nil
}
