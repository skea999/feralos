package features

import (
	"alpineInstaller/utils/config"
	"alpineInstaller/utils/shell"
	"fmt"
	"strings"
)

// ZswapModules returns the kernel modules needed for zswap.
func ZswapModules(conf *config.Config) []string {
	if !conf.User.EnableZswap {
		return nil
	}
	return []string{"zsmalloc"}
}

// SetupZswap configures the zswap compressed swap cache for non-server
// layouts (btrfsStandard, btrfsRaid0). Toggleable via EnableZswap, with
// conservative tuning. Guards: skips for btrfsStandardServer (handled by
// SetupZswapServer) and when no swap is configured in the target fstab
// (zswap is a compressed cache for swap — pointless without swap).
// NOTE: swap setup (swapfile) lives in the disk layout (hostPhase) — this
// file only activates the zswap kernel feature.
func SetupZswap(conf *config.Config) error {
	if conf.User.DiskLayout == "btrfsStandardServer" {
		fmt.Println("[INFO] zswap: btrfsStandardServer handled by SetupZswapServer, skipping...")
		return nil
	}
	if !conf.User.EnableZswap {
		fmt.Println("[INFO] zswap disabled, skipping...")
		return nil
	}

	chroot := conf.Internal.ChrootDir
	u := &conf.User

	// zswap is a compressed cache for swap — without swap it does nothing.
	swapPresent, _ := shell.ExecuteCmd(chroot, "grep -q swap /etc/fstab && echo yes || echo no")
	if strings.TrimSpace(swapPresent) != "yes" {
		fmt.Println("[INFO] zswap: no swap configured in fstab, skipping...")
		return nil
	}

	fmt.Println("[INFO] Configuring zswap...")

	zswapCmdline := "zswap.enabled=1 zswap.compressor=zstd zswap.zpool=zsmalloc zswap.max_pool_percent=20"
	u.KernelCmdline = append(u.KernelCmdline, zswapCmdline)

	zswapSysctl := `vm.swappiness=60
vm.page-cluster=0
vm.vfs_cache_pressure=50
vm.watermark_scale_factor=200`
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

	fmt.Println("[INFO] zswap configured.")
	return nil
}
