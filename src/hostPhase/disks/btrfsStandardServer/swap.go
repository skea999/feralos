package btrfsStandardServer

import (
	"alpineInstaller/utils/config"
	"alpineInstaller/utils/shell"
	"fmt"
	"strings"
)

// SetupDmcryptSwap configures LUKS swap with dmcrypt for auto-unlock at boot.
// Runs in hostPhase (called from RunBtrfsSetup): adds the swap keyfile to the
// swap partitions and writes the dmcrypt config into the chroot.
func SetupDmcryptSwap(conf *config.Config) error {
	u := conf.User
	chroot := conf.Internal.ChrootDir
	d1 := u.Disks[0]
	d2 := u.Disks[1]
	sp1 := getPartitionSpacer(d1)
	sp2 := getPartitionSpacer(d2)

	swapKey := "/etc/dmcrypt/swap.key"
	if _, err := shell.ExecuteCmd(chroot, fmt.Sprintf("mkdir -p /etc/dmcrypt && touch %s && chmod 400 %s && dd bs=512 count=4 if=/dev/urandom of=%s", swapKey, swapKey, swapKey)); err != nil {
		return fmt.Errorf("failed to create swap keyfile: %w", err)
	}

	swapPart1 := d1 + sp1 + "3"
	swapPart2 := d2 + sp2 + "3"

	if _, err := shell.ExecuteCmd(chroot, fmt.Sprintf("printf '%%s' '%s' | cryptsetup luksAddKey %s %s", u.Password, swapPart1, swapKey)); err != nil {
		return fmt.Errorf("failed to add key to swap1: %w", err)
	}
	if _, err := shell.ExecuteCmd(chroot, fmt.Sprintf("printf '%%s' '%s' | cryptsetup luksAddKey %s %s", u.Password, swapPart2, swapKey)); err != nil {
		return fmt.Errorf("failed to add key to swap2: %w", err)
	}

	swapUUID1, err := shell.ExecuteCmd(chroot, fmt.Sprintf("blkid -o value -s UUID %s", swapPart1))
	if err != nil {
		return fmt.Errorf("failed to get swap1 UUID: %w", err)
	}
	swapUUID2, err := shell.ExecuteCmd(chroot, fmt.Sprintf("blkid -o value -s UUID %s", swapPart2))
	if err != nil {
		return fmt.Errorf("failed to get swap2 UUID: %w", err)
	}

	swapUUID1 = strings.TrimSpace(swapUUID1)
	swapUUID2 = strings.TrimSpace(swapUUID2)

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

	escapedDmcryptConf := strings.ReplaceAll(dmcryptConf, "'", "'\\''")
	if _, err := shell.ExecuteCmd(chroot, fmt.Sprintf("echo '%s' > /etc/conf.d/dmcrypt", escapedDmcryptConf)); err != nil {
		return fmt.Errorf("failed to write dmcrypt config: %w", err)
	}

	return nil
}
