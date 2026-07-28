package hostPhase

import (
	"alpineInstaller/utils/config"
	"alpineInstaller/utils/shell"
	"fmt"
	"strings"
)

// GenerateAndEmbedKeyfile generates a random keyfile, adds it to all LUKS root
// partitions, copies it into the chroot for initramfs inclusion, and returns the
// kernel cmdline arguments for rd.luks.key.
//
// With multiple LUKS devices, booster requires per-device keyfile spec:
//   rd.luks.key=/keyfile:luks-<uuid1>
//   rd.luks.key=/keyfile:luks-<uuid2>
//
// This allows booster to auto-unlock LUKS without interactive passphrase —
// critical for headless/QEMU environments.
func GenerateAndEmbedKeyfile(conf *config.Config) ([]string, error) {
	chroot := conf.Internal.ChrootDir
	password := conf.User.Password
	disks := conf.User.Disks
	diskLayout := conf.User.DiskLayout

	shell.SetCaller("keyfile")

	// Determine which disk/partitions hold LUKS root
	var luksParts []string
	switch diskLayout {
	case "btrfsStandard":
		targetDisk := disks[0]
		sp := getPartitionSpacer(targetDisk)
		luksParts = []string{targetDisk + sp + "2"}
	case "btrfsStandardServer":
		for _, disk := range disks {
			sp := getPartitionSpacer(disk)
			luksParts = append(luksParts, disk+sp+"2")
		}
	default:
		return nil, fmt.Errorf("unsupported disk layout for keyfile: %s", diskLayout)
	}

	// 1. Generate random keyfile on host (in /tmp, outside chroot)
	keyfileHost := "/tmp/luks-keyfile"
	fmt.Printf("[INFO] Generating LUKS keyfile: %s\n", keyfileHost)

	if _, err := shell.RunCommand("dd", "if=/dev/urandom", "of="+keyfileHost, "bs=512", "count=4"); err != nil {
		return nil, fmt.Errorf("failed to generate keyfile: %w", err)
	}
	if _, err := shell.RunCommand("chmod", "0400", keyfileHost); err != nil {
		return nil, fmt.Errorf("failed to chmod keyfile: %w", err)
	}

	// 2. Add keyfile to all LUKS root partitions
	var keyfileArgs []string
	for _, part := range luksParts {
		fmt.Printf("[INFO] Adding keyfile to LUKS partition: %s\n", part)
		addKeyCmd := fmt.Sprintf("printf '%%s' '%s' | cryptsetup luksAddKey %s %s", password, part, keyfileHost)
		if _, err := shell.RunCommand("sh", "-c", addKeyCmd); err != nil {
			return nil, fmt.Errorf("failed to add keyfile to LUKS %s: %w", part, err)
		}

		// Get UUID for per-device keyfile arg
		uuid, err := shell.RunCommand("blkid", "-o", "value", "-s", "UUID", part)
		if err != nil {
			return nil, fmt.Errorf("failed to get UUID for %s: %w", part, err)
		}
		uuid = strings.TrimSpace(uuid)
		keyfileArgs = append(keyfileArgs, fmt.Sprintf("rd.luks.key=/keyfile:luks-%s", uuid))
	}

	// 3. Copy keyfile into chroot root (booster will embed in initramfs)
	chrootKeyfile := chroot + "/keyfile"
	if _, err := shell.RunCommand("cp", keyfileHost, chrootKeyfile); err != nil {
		return nil, fmt.Errorf("failed to copy keyfile to chroot: %w", err)
	}
	if _, err := shell.RunCommand("chmod", "0400", chrootKeyfile); err != nil {
		return nil, fmt.Errorf("failed to chmod chroot keyfile: %w", err)
	}

	// 4. Also copy into /etc within chroot for persistence across rebuilds
	chrootKeyEtc := chroot + "/etc/luks-keyfile"
	if _, err := shell.RunCommand("cp", keyfileHost, chrootKeyEtc); err != nil {
		return nil, fmt.Errorf("failed to copy keyfile to chroot /etc: %w", err)
	}
	if _, err := shell.RunCommand("chmod", "0400", chrootKeyEtc); err != nil {
		return nil, fmt.Errorf("failed to chmod chroot /etc keyfile: %w", err)
	}

	// 5. Remove host copy (security)
	_, _ = shell.RunCommand("rm", "-f", keyfileHost)

	// 6. Return kernel cmdline arguments
	fmt.Println("[INFO] LUKS keyfile embedded — auto-unlock enabled")
	return keyfileArgs, nil
}

// getPartitionSpacer returns "p" for NVMe devices, "" otherwise.
// Mirrors the function in the disk layout packages to avoid import cycles.
func getPartitionSpacer(disk string) string {
	if strings.Contains(disk, "nvme") {
		return "p"
	}
	return ""
}
