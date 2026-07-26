package chrootPhase

import (
	"alpineInstaller/utils/config"
	"alpineInstaller/utils/files"
	"alpineInstaller/utils/shell"
	"fmt"
	"strings"
)

// SetupDmcryptSwap configures LUKS swap with dmcrypt for auto-unlock at boot.
// Creates a keyfile, adds it to LUKS swap partitions, and writes /etc/conf.d/dmcrypt.
func SetupDmcryptSwap(conf *config.Config) error {
	chroot := conf.Internal.ChrootDir
	u := conf.User

	if len(u.Disks) < 2 {
		return fmt.Errorf("dmcrypt swap requires at least 2 disks")
	}

	d1 := u.Disks[0]
	d2 := u.Disks[1]
	sp1 := getPartitionSpacer(d1)
	sp2 := getPartitionSpacer(d2)

	// 1. Create swap keyfile
	swapKey := "/etc/dmcrypt/swap.key"
	if _, err := shell.RunChrootCommand(chroot, fmt.Sprintf("mkdir -p /etc/dmcrypt && touch %s && chmod 400 %s && dd bs=512 count=4 if=/dev/urandom of=%s", swapKey, swapKey, swapKey)); err != nil {
		return fmt.Errorf("failed to create swap keyfile: %w", err)
	}

	// 2. Add keyfile to LUKS swap partitions
	swapPart1 := d1 + sp1 + "3"
	swapPart2 := d2 + sp2 + "3"

	if _, err := shell.RunChrootCommand(chroot, fmt.Sprintf("printf '%%s' '%s' | cryptsetup luksAddKey %s %s", u.Password, swapPart1, swapKey)); err != nil {
		return fmt.Errorf("failed to add key to swap1: %w", err)
	}
	if _, err := shell.RunChrootCommand(chroot, fmt.Sprintf("printf '%%s' '%s' | cryptsetup luksAddKey %s %s", u.Password, swapPart2, swapKey)); err != nil {
		return fmt.Errorf("failed to add key to swap2: %w", err)
	}

	// 3. Get swap UUIDs
	swapUUID1, err := shell.RunChrootCommand(chroot, fmt.Sprintf("blkid -o value -s UUID %s", swapPart1))
	if err != nil {
		return fmt.Errorf("failed to get swap1 UUID: %w", err)
	}
	swapUUID2, err := shell.RunChrootCommand(chroot, fmt.Sprintf("blkid -o value -s UUID %s", swapPart2))
	if err != nil {
		return fmt.Errorf("failed to get swap2 UUID: %w", err)
	}

	swapUUID1 = strings.TrimSpace(swapUUID1)
	swapUUID2 = strings.TrimSpace(swapUUID2)

	// 4. Write dmcrypt config
	s1 := getDiskSuffix(d1)
	s2 := getDiskSuffix(d2)

	dmcryptConf := fmt.Sprintf(`
target=swap-%s%s
source=UUID=%s
key=%s

target=swap-%s%s
source=UUID=%s
key=%s

`, s1, sp1+"3", swapUUID1, swapKey,
		s2, sp2+"3", swapUUID2, swapKey)

	if err := files.WriteFileInChroot(chroot, "/etc/conf.d/dmcrypt", dmcryptConf); err != nil {
		return fmt.Errorf("failed to write dmcrypt config: %w", err)
	}

	return nil
}
