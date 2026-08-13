package internal

import (
	"alpineInstaller/utils/config"
	"alpineInstaller/utils/shell"
	"fmt"
	"strings"
)

// SetupStorage configures btrbk and bridging sysctl.
func SetupStorage(conf *config.Config) error {
	chroot := conf.Internal.ChrootDir

	fmt.Println("[INFO] Writing btrbk configuration...")
	if _, err := shell.ExecuteCmd(chroot, "mkdir -p /etc/btrbk"); err != nil {
		fmt.Printf("[WARNING] Failed to create /etc/btrbk: %v\n", err)
	}
	btrbkConfig := `# btrbk configuration for FeralOS
snapshot_dir  /btrbk/snapshots
subvolume     /
incremental   yes
compact       yes
keep_daily    7
keep_weekly   4
keep_monthly  3
run_as        root
timestamp_format long
compare       path
stream_buffer none
logdir        /var/log/btrbk`
	escapedBtrbkConfig := strings.ReplaceAll(btrbkConfig, "'", "'\\''")
	if _, err := shell.ExecuteCmd(chroot, fmt.Sprintf("echo '%s' > /etc/btrbk/btrbk.conf", escapedBtrbkConfig)); err != nil {
		return fmt.Errorf("failed to write btrbk config: %w", err)
	}

	if _, err := shell.ExecuteCmd(chroot, "mkdir -p /btrbk/snapshots /var/log/btrbk /mnt/btrbk/snapshots"); err != nil {
		fmt.Printf("[WARNING] Failed to create btrbk directories: %v\n", err)
	}

	fmt.Println("[INFO] Writing bridging sysctl...")
	bridgingConf := "echo 'net.ipv4.conf.all.bc_forwarding=1\nnet.ipv4.conf.default.bc_forwarding=1' > /etc/sysctl.d/bridging.conf"
	if _, err := shell.ExecuteCmd(chroot, bridgingConf); err != nil {
		return fmt.Errorf("failed to write bridging sysctl: %w", err)
	}

	return nil
}
