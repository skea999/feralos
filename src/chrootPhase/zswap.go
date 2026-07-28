package chrootPhase

import (
	"alpineInstaller/utils/config"
	"alpineInstaller/utils/files"
	"alpineInstaller/utils/shell"
	"fmt"
	"strings"
)

// SetupZswap configures zswap for compressed swap in memory.
// zswap compresses pages before they go to swap, reducing I/O and improving performance.
// Controlled by conf.User.EnableZswap flag.
// Checks if swap exists before configuring — no swap = no zswap.
func SetupZswap(conf *config.Config) error {
	if !conf.User.EnableZswap {
		fmt.Println("[INFO] zswap disabled, skipping...")
		return nil
	}

	chroot := conf.Internal.ChrootDir
	u := &conf.User

	// Check if swap is configured in fstab
	swapCheck, _ := shell.RunChrootCommand(chroot, "grep -c 'swap' /etc/fstab 2>/dev/null || echo 0")
	swapCheck = strings.TrimSpace(swapCheck)
	if swapCheck == "0" {
		fmt.Println("[INFO] No swap found in fstab, skipping zswap...")
		return nil
	}

	fmt.Println("[INFO] Configuring zswap...")

	// 1. Append zswap cmdline args to KernelCmdline slice
	// (configurator.go writes the final combined cmdline to /etc/kernel/cmdline)
	zswapCmdline := "zswap.enabled=1 zswap.compressor=zstd zswap.zpool=zsmalloc zswap.max_pool_percent=50 zswap.non_same_page_filled_pages_enabled=1"
	u.KernelCmdline = append(u.KernelCmdline, zswapCmdline)

	// 2. Write zswap sysctl
	zswapSysctl := `vm.swappiness=100
vm.page-cluster=0
vm.extfrag_threshold=0
vm.vfs_cache_pressure=50
vm.dirty_background_ratio = 5
vm.dirty_ratio = 10
vm.watermark_scale_factor = 200
vm.dirty_writeback_centisecs = 250
vm.dirty_expire_centisecs = 1500
`
	if err := files.WriteFileInChroot(chroot, "/etc/sysctl.d/zswap.conf", zswapSysctl); err != nil {
		return fmt.Errorf("failed to write zswap sysctl: %w", err)
	}

	// 3. Calculate vm.min_free_kbytes as 1% of total memory (must run inside chroot)
	if _, err := shell.RunChrootCommand(chroot, "awk '/MemTotal/ {printf \"vm.min_free_kbytes=%d\\n\", $2 * 0.01}' /proc/meminfo >> /etc/sysctl.d/zswap.conf"); err != nil {
		fmt.Printf("[WARNING] Failed to calculate min_free_kbytes: %v\n", err)
	}

	// 4. Add zsmalloc module
	if _, err := shell.RunChrootCommand(chroot, "grep -q '^zsmalloc$' /etc/modules || echo 'zsmalloc' >> /etc/modules"); err != nil {
		fmt.Printf("[WARNING] Failed to add zsmalloc module: %v\n", err)
	}

	// 5. Add zswap monitor aliases
	zswapAliases := `alias zswapMonitor1="rg --no-ignore --hidden --text . /sys/kernel/debug/zswap/"
alias zswapMonitor2="fd . /sys/kernel/debug/zswap/ -x rg . {}"
`
	if err := files.WriteFileInChroot(chroot, "/etc/profile.d/zswap-aliases.sh", zswapAliases); err != nil {
		return fmt.Errorf("failed to write zswap aliases: %w", err)
	}

	fmt.Println("[INFO] zswap configured.")
	return nil
}
