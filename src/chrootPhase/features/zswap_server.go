package features

import (
	"feralos/src/utils/config"
	"feralos/src/utils/shell"
	"fmt"
	"strings"
)

// SetupZswapServer configures the zswap compressed swap cache for the
// btrfsStandardServer layout — always enabled (server requirement), with
// aggressive tuning. The EnableZswap toggle is ignored here.
// NOTE: swap setup (LUKS swap partitions) lives in the disk layout
// (hostPhase) — this file only activates the zswap kernel feature.
func SetupZswapServer(conf *config.Config) error {
	if conf.User.DiskLayout != "btrfsStandardServer" {
		fmt.Println("[INFO] zswap server: not btrfsStandardServer, skipping...")
		return nil
	}

	chroot := conf.Internal.ChrootDir
	u := &conf.User

	fmt.Println("[INFO] Configuring zswap (server)...")

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

	fmt.Println("[INFO] zswap configured.")
	return nil
}
