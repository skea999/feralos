package btrfsRaid0

import (
	"alpineInstaller/utils/shell"
	"fmt"
	"strings"
)

// LuksFormat encrypts a partition with LUKS2.
// Uses stdin pipe to pass password securely (matches reference shell behavior).
func LuksFormat(device, password string) error {
	shell.SetCaller("luks")
	fmt.Printf("[INFO] LUKS2 format: %s\n", device)

	// cryptsetup luksFormat --type luks2 -v -c aes-xts-plain64 -s 512 --hash sha512 --iter-time 5000 --use-urandom -q --key-file -
	cmd := fmt.Sprintf("printf '%%s' '%s' | cryptsetup luksFormat %s --type luks2 -v -c aes-xts-plain64 -s 512 --hash sha512 --iter-time 5000 --use-urandom -q --key-file -", password, device)
	if _, err := shell.ExecuteCmd("", cmd); err != nil {
		return fmt.Errorf("luksFormat failed on %s: %w", device, err)
	}
	return nil
}

// LuksOpen opens a LUKS-encrypted partition and maps it to /dev/mapper/<name>.
func LuksOpen(device, name, password string) error {
	fmt.Printf("[INFO] LUKS open: %s -> %s\n", device, name)

	cmd := fmt.Sprintf("printf '%%s' '%s' | cryptsetup luksOpen %s %s --key-file -", password, device, name)
	if _, err := shell.ExecuteCmd("", cmd); err != nil {
		return fmt.Errorf("luksOpen failed on %s: %w", device, err)
	}
	return nil
}

// LuksUUID returns the UUID of a LUKS-encrypted partition.
func LuksUUID(device string) (string, error) {
	out, err := shell.ExecuteCmd("", fmt.Sprintf("blkid -o value -s UUID %s", device))
	if err != nil {
		return "", fmt.Errorf("failed to get LUKS UUID for %s: %w", device, err)
	}
	return strings.TrimSpace(out), nil
}

// LuksMapperPath returns the /dev/mapper path for a LUKS UUID.
func LuksMapperPath(uuid string) string {
	return "/dev/mapper/luks-" + uuid
}

// LuksAddKey adds a keyfile to a LUKS partition (for swap auto-unlock).
func LuksAddKey(device, keyfile, password string) error {
	fmt.Printf("[INFO] LUKS add key: %s -> %s\n", device, keyfile)

	cmd := fmt.Sprintf("printf '%%s' '%s' | cryptsetup luksAddKey %s %s", password, device, keyfile)
	if _, err := shell.ExecuteCmd("", cmd); err != nil {
		return fmt.Errorf("luksAddKey failed on %s: %w", device, err)
	}
	return nil
}

// SetupLuksPartition formats, opens, and returns the mapper path for a partition.
// This is the main entry point for LUKS setup on a single partition.
func SetupLuksPartition(device, password string) (string, error) {
	// 1. Wipe existing signatures
	if _, err := shell.ExecuteCmd("", fmt.Sprintf("wipefs -a %s", device)); err != nil {
		return "", fmt.Errorf("wipefs failed on %s: %w", device, err)
	}

	// 2. Format with LUKS2
	if err := LuksFormat(device, password); err != nil {
		return "", err
	}

	// 3. Get UUID
	uuid, err := LuksUUID(device)
	if err != nil {
		return "", err
	}

	// 4. Open
	mapperName := "luks-" + uuid
	if err := LuksOpen(device, mapperName, password); err != nil {
		return "", err
	}

	return LuksMapperPath(uuid), nil
}
