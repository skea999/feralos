package internal

import (
	"feralos/src/utils/config"
	"feralos/src/utils/shell"
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

	// Disk maintenance schedule (2026-09-30): daily btrbk snapshots, weekly
	// btrfs scrub, weekly TRIM. Binary paths are resolved in the chroot so
	// the cron lines never depend on the cron PATH; a missing tool skips its
	// job with a warning (the packages are layout-driven).
	btrfsBin, _ := shell.ExecuteCmd(chroot, "command -v btrfs 2>/dev/null || true")
	btrbkBin, _ := shell.ExecuteCmd(chroot, "command -v btrbk 2>/dev/null || true")
	cryptBin, _ := shell.ExecuteCmd(chroot, "command -v cryptsetup 2>/dev/null || true")
	btrfsBin = strings.TrimSpace(btrfsBin)
	btrbkBin = strings.TrimSpace(btrbkBin)
	cryptBin = strings.TrimSpace(cryptBin)

	if btrfsBin != "" && cryptBin != "" {
		// TRIM with minimal discard exposure: enable discard passthrough on
		// the active dm-crypt volumes, trim, then clear the passthrough. On
		// btrfsUnencrypted there are no luks-* mappers (loop is a no-op).
		trimScript := fmt.Sprintf(`#!/bin/sh
# FeralOS weekly TRIM (see installer storage.go). Enable discard passthrough
# on active dm-crypt volumes, trim btrfs free space, clear the passthrough.
for d in /dev/mapper/luks-*; do
	[ -e "$d" ] || continue
	%s refresh --allow-discards "$d" || true
done
%s filesystem trim -a /
rc=$?
for d in /dev/mapper/luks-*; do
	[ -e "$d" ] || continue
	%s refresh "$d" || true
done
exit $rc
`, cryptBin, btrfsBin, cryptBin)
		escapedTrim := strings.ReplaceAll(trimScript, "'", "'\\''")
		if _, err := shell.ExecuteCmd(chroot, fmt.Sprintf("echo '%s' > /usr/libexec/feralos-weekly-trim && chmod 755 /usr/libexec/feralos-weekly-trim", escapedTrim)); err != nil {
			fmt.Printf("[WARNING] Failed to write trim script: %v\n", err)
		}
	}

	cronLines := ""
	if btrfsBin != "" {
		cronLines += "\n0 3 * * 0 /usr/libexec/feralos-weekly-trim >> /var/log/btrfs-trim.log 2>&1"
		cronLines += "\n30 3 * * 0 " + btrfsBin + " scrub start -B / >> /var/log/btrfs-scrub.log 2>&1"
	} else {
		fmt.Println("[WARNING] btrfs not found — scrub/trim cron skipped")
	}
	if btrbkBin != "" {
		cronLines += "\n0 2 * * * " + btrbkBin + " run >> /var/log/btrbk/cron.log 2>&1"
	} else {
		fmt.Println("[WARNING] btrbk not found — snapshot cron skipped")
	}
	if cronLines != "" {
		escapedCron := strings.ReplaceAll(cronLines, "'", "'\\''")
		if _, err := shell.ExecuteCmd(chroot, fmt.Sprintf("echo '%s' >> /etc/crontabs/root", escapedCron)); err != nil {
			fmt.Printf("[WARNING] Failed to write disk maintenance cron: %v\n", err)
		}
	}

	fmt.Println("[INFO] Writing bridging sysctl...")
	bridgingConf := "echo 'net.ipv4.conf.all.bc_forwarding=1\nnet.ipv4.conf.default.bc_forwarding=1' > /etc/sysctl.d/bridging.conf"
	if _, err := shell.ExecuteCmd(chroot, bridgingConf); err != nil {
		return fmt.Errorf("failed to write bridging sysctl: %w", err)
	}

	return nil
}
