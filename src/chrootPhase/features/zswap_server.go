package features

import (
	"alpineInstaller/utils/config"
	"alpineInstaller/utils/shell"
	"fmt"
	"strings"
)

// SetupZswap configures the zswap compressed swap cache.
// NOTE: LUKS swap partition setup (dmcrypt) lives in the disk layout
// (hostPhase/disks/btrfsStandardServer/swap.go) — this file only activates
// the zswap kernel feature.
func SetupZswap(conf *config.Config) error {
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
