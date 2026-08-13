package features

import (
	"alpineInstaller/utils/config"
	"alpineInstaller/utils/shell"
	"fmt"
	"strings"
)

// SetupDmcryptSwap configures LUKS swap with dmcrypt for auto-unlock at boot.
func SetupDmcryptSwap(conf *config.Config) error {
	u := conf.User

	if u.DiskLayout == "btrfsStandard" {
		fmt.Println("[INFO] btrfsStandard layout: skipping dmcrypt (uses swapfile)")
		return nil
	}

	if len(u.Disks) < 2 {
		return fmt.Errorf("dmcrypt swap requires at least 2 disks")
	}

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

// SetupZswap configures dmcrypt LUKS swap and zswap compressed swap.
func SetupZswap(conf *config.Config) error {
	// 1. Dmcrypt LUKS swap (always for btrfsStandardServer)
	if err := SetupDmcryptSwap(conf); err != nil {
		return fmt.Errorf("dmcrypt swap setup failed: %w", err)
	}

	// 2. Zswap compressed swap (conditional on EnableZswap)
	if !conf.User.EnableZswap {
		fmt.Println("[INFO] zswap disabled, skipping...")
		return nil
	}

	chroot := conf.Internal.ChrootDir
	u := &conf.User

	fmt.Println("[INFO] Configuring zswap with swap file...")

	swapSize := "8G"
	swapFile := "/swap/swapfile"

	if _, err := shell.ExecuteCmd(chroot, "mkdir -p /swap"); err != nil {
		return fmt.Errorf("failed to create /swap directory: %w", err)
	}

	swapMounted, _ := shell.ExecuteCmd(chroot, "mountpoint -q /swap && echo yes || echo no")
	swapMounted = strings.TrimSpace(swapMounted)

	if swapMounted == "no" {
		fmt.Println("[WARNING] /swap not mounted, creating swap file on root filesystem")
		swapFile = "/swapfile"
	}

	fmt.Printf("[INFO] Creating %s swap file...\n", swapSize)
	if _, err := shell.ExecuteCmd(chroot, fmt.Sprintf("fallocate -l %s %s", swapSize, swapFile)); err != nil {
		if _, err := shell.ExecuteCmd(chroot, fmt.Sprintf("dd if=/dev/zero of=%s bs=1M count=8192 status=progress", swapFile)); err != nil {
			return fmt.Errorf("failed to create swap file: %w", err)
		}
	}

	if _, err := shell.ExecuteCmd(chroot, fmt.Sprintf("chmod 600 %s", swapFile)); err != nil {
		return fmt.Errorf("failed to set swap file permissions: %w", err)
	}

	if _, err := shell.ExecuteCmd(chroot, fmt.Sprintf("mkswap %s", swapFile)); err != nil {
		return fmt.Errorf("failed to format swap file: %w", err)
	}

	fstabEntry := fmt.Sprintf("%s none swap sw 0 0", swapFile)
	checkFstab, _ := shell.ExecuteCmd(chroot, fmt.Sprintf("grep -c '%s' /etc/fstab || echo 0", swapFile))
	checkFstab = strings.TrimSpace(checkFstab)
	if checkFstab == "0" {
		if _, err := shell.ExecuteCmd(chroot, fmt.Sprintf("echo '%s' >> /etc/fstab", fstabEntry)); err != nil {
			return fmt.Errorf("failed to add swap to fstab: %w", err)
		}
	}

	zswapCmdline := "zswap.enabled=1 zswap.compressor=zstd zswap.zpool=zsmalloc zswap.max_pool_percent=50 zswap.non_same_page_filled_pages_enabled=1"
	u.KernelCmdline = append(u.KernelCmdline, zswapCmdline)

	zswapSysctl := `vm.swappiness=100
vm.page-cluster=0
vm.extfrag_threshold=0
vm.vfs_cache_pressure=50
vm.dirty_background_ratio = 5
vm.dirty_ratio = 10
vm.watermark_scale_factor = 200
vm.dirty_writeback_centisecs = 250
vm.dirty_expire_centisecs = 1500`
	escapedZswapSysctl := strings.ReplaceAll(zswapSysctl, "'", "'\\''")
	if _, err := shell.ExecuteCmd(chroot, fmt.Sprintf("echo '%s' > /etc/sysctl.d/zswap.conf", escapedZswapSysctl)); err != nil {
		return fmt.Errorf("failed to write zswap sysctl: %w", err)
	}

	if _, err := shell.ExecuteCmd(chroot, "awk '/MemTotal/ {printf \"vm.min_free_kbytes=%d\\n\", $2 * 0.01}' /proc/meminfo >> /etc/sysctl.d/zswap.conf"); err != nil {
		fmt.Printf("[WARNING] Failed to calculate min_free_kbytes: %v\n", err)
	}

	zswapAliases := `alias zswapMonitor1="rg --no-ignore --hidden --text . /sys/kernel/debug/zswap/"
alias zswapMonitor2="fd . /sys/kernel/debug/zswap/ -x rg . {}"`
	escapedZswapAliases := strings.ReplaceAll(zswapAliases, "'", "'\\''")
	if _, err := shell.ExecuteCmd(chroot, fmt.Sprintf("echo '%s' > /etc/profile.d/zswap-aliases.sh", escapedZswapAliases)); err != nil {
		return fmt.Errorf("failed to write zswap aliases: %w", err)
	}

	fmt.Println("[INFO] zswap configured with swap file.")
	return nil
}

func getPartitionSpacer(disk string) string {
	if strings.Contains(disk, "nvme") {
		return "p"
	}
	return ""
}

func getDiskSuffix(disk string) string {
	parts := strings.Split(disk, "/")
	return parts[len(parts)-1]
}
