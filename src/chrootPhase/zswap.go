package chrootPhase

import (
	"alpineInstaller/utils/config"
	"alpineInstaller/utils/files"
	"alpineInstaller/utils/shell"
	"fmt"
	"strings"
)

// SetupZswap configures zswap with a swap file in /swap subvolume.
// zswap compresses pages before they go to swap, reducing I/O and improving performance.
// Controlled by conf.User.EnableZswap flag.
func SetupZswap(conf *config.Config) error {
	if !conf.User.EnableZswap {
		fmt.Println("[INFO] zswap disabled, skipping...")
		return nil
	}

	chroot := conf.Internal.ChrootDir
	u := &conf.User

	fmt.Println("[INFO] Configuring zswap with swap file...")

	// 1. Create swap file in /swap subvolume (8GB)
	swapSize := "8G"
	swapFile := "/swap/swapfile"

	// Ensure /swap subvolume exists and is mounted
	if _, err := shell.RunChrootCommand(chroot, "mkdir -p /swap"); err != nil {
		return fmt.Errorf("failed to create /swap directory: %w", err)
	}

	// Check if @swap subvolume is mounted, if not create swap file on root
	// The @swap subvolume should be mounted at /swap from fstab
	swapMounted, _ := shell.RunChrootCommand(chroot, "mountpoint -q /swap && echo yes || echo no")
	swapMounted = strings.TrimSpace(swapMounted)

	if swapMounted == "no" {
		fmt.Println("[WARNING] /swap not mounted, creating swap file on root filesystem")
		swapFile = "/swapfile"
	}

	// Create swap file
	fmt.Printf("[INFO] Creating %s swap file...\n", swapSize)
	if _, err := shell.RunChrootCommand(chroot, fmt.Sprintf("fallocate -l %s %s", swapSize, swapFile)); err != nil {
		// fallocate may not work on btrfs, fall back to dd
		if _, err := shell.RunChrootCommand(chroot, fmt.Sprintf("dd if=/dev/zero of=%s bs=1M count=8192 status=progress", swapFile)); err != nil {
			return fmt.Errorf("failed to create swap file: %w", err)
		}
	}

	// Set correct permissions
	if _, err := shell.RunChrootCommand(chroot, fmt.Sprintf("chmod 600 %s", swapFile)); err != nil {
		return fmt.Errorf("failed to set swap file permissions: %w", err)
	}

	// Format as swap
	if _, err := shell.RunChrootCommand(chroot, fmt.Sprintf("mkswap %s", swapFile)); err != nil {
		return fmt.Errorf("failed to format swap file: %w", err)
	}

	// Enable swap
	if _, err := shell.RunChrootCommand(chroot, fmt.Sprintf("swapon %s", swapFile)); err != nil {
		return fmt.Errorf("failed to enable swap file: %w", err)
	}

	// Add to fstab if not already present
	fstabEntry := fmt.Sprintf("%s none swap sw 0 0", swapFile)
	checkFstab, _ := shell.RunChrootCommand(chroot, fmt.Sprintf("grep -c '%s' /etc/fstab || echo 0", swapFile))
	checkFstab = strings.TrimSpace(checkFstab)
	if checkFstab == "0" {
		if err := files.WriteFileInChroot(chroot, "/etc/fstab", fstabEntry+"\n"); err != nil {
			// Append instead of overwrite
			if _, err := shell.RunChrootCommand(chroot, fmt.Sprintf("echo '%s' >> /etc/fstab", fstabEntry)); err != nil {
				return fmt.Errorf("failed to add swap to fstab: %w", err)
			}
		}
	}

	// 2. Append zswap cmdline args to KernelCmdline slice
	zswapCmdline := "zswap.enabled=1 zswap.compressor=zstd zswap.zpool=zsmalloc zswap.max_pool_percent=50 zswap.non_same_page_filled_pages_enabled=1"
	u.KernelCmdline = append(u.KernelCmdline, zswapCmdline)

	// 3. Write zswap sysctl
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

	// 4. Calculate vm.min_free_kbytes as 1% of total memory
	if _, err := shell.RunChrootCommand(chroot, "awk '/MemTotal/ {printf \"vm.min_free_kbytes=%d\\n\", $2 * 0.01}' /proc/meminfo >> /etc/sysctl.d/zswap.conf"); err != nil {
		fmt.Printf("[WARNING] Failed to calculate min_free_kbytes: %v\n", err)
	}

	// 5. Add zsmalloc module
	if _, err := shell.RunChrootCommand(chroot, "grep -q '^zsmalloc$' /etc/modules || echo 'zsmalloc' >> /etc/modules"); err != nil {
		fmt.Printf("[WARNING] Failed to add zsmalloc module: %v\n", err)
	}

	// 6. Add zswap monitor aliases
	zswapAliases := `alias zswapMonitor1="rg --no-ignore --hidden --text . /sys/kernel/debug/zswap/"
alias zswapMonitor2="fd . /sys/kernel/debug/zswap/ -x rg . {}"
`
	if err := files.WriteFileInChroot(chroot, "/etc/profile.d/zswap-aliases.sh", zswapAliases); err != nil {
		return fmt.Errorf("failed to write zswap aliases: %w", err)
	}

	fmt.Println("[INFO] zswap configured with swap file.")
	return nil
}
