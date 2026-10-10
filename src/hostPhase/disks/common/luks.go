package common

import (
	"feralos/src/utils/shell"
	"fmt"
	"os/exec"
	"strings"
)

// LuksFormat encrypts a partition with LUKS2.
// Uses exec.Command with stdin pipe to pass password securely (no shell, no logging).
func LuksFormat(device, password string) error {
	shell.SetCaller("luks")
	fmt.Printf("[INFO] LUKS2 format: %s\n", device)

	cmd := exec.Command("cryptsetup", "luksFormat", device,
		"--type", "luks2", "-v", "-c", "aes-xts-plain64", "-s", "512",
		"--hash", "sha512", "--iter-time", "5000", "--use-urandom", "-q", "--key-file", "-")
	cmd.Stdin = strings.NewReader(password)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("luksFormat failed on %s: %w\n%s", device, err, string(output))
	}
	return nil
}

// LuksOpen opens a LUKS-encrypted partition and maps it to /dev/mapper/<name>.
// Uses exec.Command with stdin pipe (no shell, no logging).
func LuksOpen(device, name, password string) error {
	fmt.Printf("[INFO] LUKS open: %s -> %s\n", device, name)

	cmd := exec.Command("cryptsetup", "luksOpen", device, name, "--key-file", "-")
	cmd.Stdin = strings.NewReader(password)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("luksOpen failed on %s: %w\n%s", device, err, string(output))
	}
	return nil
}

// LuksUUID returns the UUID of a LUKS-encrypted partition.
func LuksUUID(device string) (string, error) {
	out, err := shell.ExecuteCmdArgs("", "blkid", "-o", "value", "-s", "UUID", device)
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
// Uses exec.Command with stdin pipe (no shell, no logging).
func LuksAddKey(device, keyfile, password string) error {
	fmt.Printf("[INFO] LUKS add key: %s -> %s\n", device, keyfile)

	cmd := exec.Command("cryptsetup", "luksAddKey", device, keyfile)
	cmd.Stdin = strings.NewReader(password)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("luksAddKey failed on %s: %w\n%s", device, err, string(output))
	}
	return nil
}

// SetupLuksPartition formats, opens, and returns the mapper path for a partition.
// This is the main entry point for LUKS setup on a single partition.
func SetupLuksPartition(device, password string) (string, error) {
	if _, err := shell.ExecuteCmdArgs("", "wipefs", "-a", device); err != nil {
		return "", fmt.Errorf("wipefs failed on %s: %w", device, err)
	}

	if err := LuksFormat(device, password); err != nil {
		return "", err
	}

	uuid, err := LuksUUID(device)
	if err != nil {
		return "", err
	}

	mapperName := "luks-" + uuid
	if err := LuksOpen(device, mapperName, password); err != nil {
		return "", err
	}

	return LuksMapperPath(uuid), nil
}
