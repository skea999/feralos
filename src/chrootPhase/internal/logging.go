package internal

import (
	"alpineInstaller/utils/config"
	"alpineInstaller/utils/shell"
	"fmt"
	"strings"
)

// SetupLogging writes Vector config and vector-setup boot service.
func SetupLogging(conf *config.Config, pkgList string) error {
	chroot := conf.Internal.ChrootDir

	vectorInstalled := strings.Contains(pkgList, "\nvector\n") || strings.HasPrefix(pkgList, "vector\n") || strings.HasSuffix(pkgList, "\nvector")
	if !vectorInstalled {
		fmt.Println("[INFO] Vector not installed, skipping config")
		return nil
	}

	fmt.Println("[INFO] Writing Vector logging configuration...")
	vectorConfig := `# Vector configuration for FeralOS
data_dir: "/var/lib/vector"

sources:
  syslog:
    type: socket
    mode: unix_datagram
    path: /run/vector/syslog.sock
    decoding:
      codec: syslog

  dinit_services:
    type: file
    include:
      - "/var/log/dinit/*.log"
      - "/var/log/dinit/*.log.1"
    ignore_not_found: true

transforms:
  auth_logs:
    type: filter
    inputs:
      - syslog
    condition: '.facility == "auth" || .facility == "authpriv"'

  daemon_logs:
    type: filter
    inputs:
      - syslog
    condition: '.facility == "daemon"'

  kernel_logs:
    type: filter
    inputs:
      - syslog
    condition: '.facility == "kern"'

sinks:
  messages:
    type: file
    inputs:
      - syslog
      - dinit_services
    path: "/var/log/messages"
    encoding:
      codec: text

  auth_log:
    type: file
    inputs:
      - auth_logs
    path: "/var/log/auth.log"
    encoding:
      codec: text

  daemon_log:
    type: file
    inputs:
      - daemon_logs
    path: "/var/log/daemon.log"
    encoding:
      codec: text

  kernel_log:
    type: file
    inputs:
      - kernel_logs
    path: "/var/log/kern.log"
    encoding:
      codec: text
`
	escapedVectorConfig := strings.ReplaceAll(vectorConfig, "'", "'\\''")
	if _, err := shell.ExecuteCmd(chroot, fmt.Sprintf("echo '%s' > /etc/vector/vector.yaml", escapedVectorConfig)); err != nil {
		return fmt.Errorf("failed to write vector config: %w", err)
	}

	if _, err := shell.ExecuteCmd(chroot, "mkdir -p /var/lib/vector && chown vector:vector /var/lib/vector"); err != nil {
		fmt.Printf("[WARNING] Failed to create vector data directory: %v\n", err)
	}

	fmt.Println("[INFO] Creating vector-setup boot service...")
	vectorSetupInit := `#!/sbin/openrc-run

depend() {
    after localmount
    before vector
    keyword -stop
}

start() {
    ebegin "Setting up Vector directories and socket"
    mkdir -p /run/vector
    chown vector:vector /run/vector
    chmod 755 /run/vector
    ln -sf /run/vector/syslog.sock /dev/log
    chown vector:vector /var/log
    chown btrbk:btrbk /btrbk 2>/dev/null || true
    eend $?
}
`
	escapedVectorSetupInit := strings.ReplaceAll(vectorSetupInit, "'", "'\\''")
	if _, err := shell.ExecuteCmd(chroot, fmt.Sprintf("echo '%s' > /etc/init.d/vector-setup", escapedVectorSetupInit)); err != nil {
		fmt.Printf("[WARNING] Failed to write vector-setup init script: %v\n", err)
	}
	if _, err := shell.ExecuteCmd(chroot, "chmod +x /etc/init.d/vector-setup"); err != nil {
		fmt.Printf("[WARNING] Failed to make vector-setup executable: %v\n", err)
	}

	// Dinit: no vector-setup service needed — vector-dinit ships a tmpfiles
	// file (/run/vector, /dev/log, /var/log, /btrbk) processed by the
	// dinit-chimera early chain; vector-setup stays OpenRC-only.
	if conf.User.EnableDinit {
		// log dir for dinit services (log-type = file → /var/log/dinit/<name>.log);
		// vector ingests it via the dinit_services source, logrotate rotates it
		if _, err := shell.ExecuteCmd(chroot, "echo 'd /var/log/dinit 0755 root root -' > /usr/lib/tmpfiles.d/feralos-dinit.conf"); err != nil {
			fmt.Printf("[WARNING] Failed to write dinit tmpfiles config: %v\n", err)
		}

		// dinit-boot-log: mirrors the PID1 status trail (/run/dinit-boot.log)
		// into the persistent /var/log/dinit/boot.log — vector ingests it via
		// the dinit_services glob, logrotate rotates it (copytruncate + tail -F)
		fmt.Println("[INFO] Creating dinit-boot-log service...")
		bootLogScript := `#!/bin/sh
# FeralOS: persist the dinit status trail. dinit starts before /var/log is
# mounted and before /var/log/dinit exists (tmpfiles) — PID1 writes to /run.
cp /run/dinit-boot.log /var/log/dinit/boot.log 2>/dev/null || true
exec tail -F /run/dinit-boot.log >> /var/log/dinit/boot.log 2>/dev/null
`
		escapedBootLogScript := strings.ReplaceAll(bootLogScript, "'", "'\\''")
		if _, err := shell.ExecuteCmd(chroot, fmt.Sprintf("echo '%s' > /usr/libexec/dinit-boot-log && chmod 755 /usr/libexec/dinit-boot-log", escapedBootLogScript)); err != nil {
			fmt.Printf("[WARNING] Failed to write dinit-boot-log script: %v\n", err)
		}
		bootLogService := `# dinit service: dinit-boot-log (FeralOS)
# Mirrors /run/dinit-boot.log into /var/log/dinit/boot.log (persistent;
# vector source dinit_services + logrotate). log-type=none: its own output
# must never loop back into the file it tails.
type = process
command = /usr/libexec/dinit-boot-log
restart = true
depends-on = local.target
log-type = none
`
		escapedBootLogService := strings.ReplaceAll(bootLogService, "'", "'\\''")
		if _, err := shell.ExecuteCmd(chroot, fmt.Sprintf("echo '%s' > /lib/dinit.d/dinit-boot-log", escapedBootLogService)); err != nil {
			fmt.Printf("[WARNING] Failed to write dinit-boot-log service: %v\n", err)
		}
	}

	// Logrotate: vector does NOT rotate files — it only tails rotated ones
	// (copytruncate + delaycompress recommended by vector docs). Covers the
	// vector sinks AND the dinit service logs; works for OpenRC and dinit.
	fmt.Println("[INFO] Writing logrotate configuration...")
	logrotateConf := `/var/log/messages /var/log/auth.log /var/log/daemon.log /var/log/kern.log /var/log/vector.log {
    daily
    rotate 7
    compress
    delaycompress
    missingok
    notifempty
    copytruncate
}

/var/log/dinit/*.log {
    daily
    rotate 7
    compress
    delaycompress
    missingok
    notifempty
    copytruncate
}
`
	escapedLogrotate := strings.ReplaceAll(logrotateConf, "'", "'\\''")
	if _, err := shell.ExecuteCmd(chroot, fmt.Sprintf("mkdir -p /etc/logrotate.d && echo '%s' > /etc/logrotate.d/feralos", escapedLogrotate)); err != nil {
		fmt.Printf("[WARNING] Failed to write logrotate config: %v\n", err)
	}

	return nil
}
