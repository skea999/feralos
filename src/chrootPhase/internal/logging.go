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

	// Dinit-only vector plumbing (service logs + PID1 boot trail): present
	// only when enableDinit, so the config stays valid on OpenRC installs.
	dinitSource := ""
	dinitMsgInput := ""
	dinitJSONInput := ""
	dinitLogrotate := ""
	if conf.User.EnableDinit {
		// NOTE: live files only — no "*.log.1". logrotate uses copytruncate,
		// so the rotated copy is a NEW file for vector's file source and would
		// be re-read from the beginning → duplicate entries (fixed 2026-09-30).
		dinitSource = `
  dinit_services:
    type: file
    include:
      - "/var/log/dinit/*.log"
    ignore_not_found: true
`
		dinitMsgInput = "\n      - dinit_services"
		dinitJSONInput = "\n      - dinit_services"
		dinitLogrotate = `
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
	}

	// Extra file sources: configured ONLY when the owning package/feature is
	// present (never configure sources for software that is not installed).
	// Incus writes its own structured daemon log in addition to the
	// supervised incusd -> /var/log/dinit/incusd.log stream.
	incusInstalled := strings.Contains(pkgList, "\nincus\n") || strings.Contains(pkgList, "\nincus-feature\n")
	incusSource := ""
	incusJSONInput := ""
	if incusInstalled {
		incusSource = `
  incus_logs:
    type: file
    include:
      - "/var/log/incus/*.log"
    ignore_not_found: true
`
		incusJSONInput = "\n      - incus_logs"
	}

	// Debug-only local Vector API (vector top / tap). enableDebug is the
	// test-only flag — production configs never expose the API.
	apiBlock := ""
	if conf.User.EnableDebug {
		apiBlock = `api:
  enabled: true
  address: "127.0.0.1:8686"

`
	}

	// Sinks use disk buffers (data_dir=/var/lib/vector) so a vector crash or
	// restart cannot lose the last unflushed log lines (2026-09-30).
	fmt.Println("[INFO] Writing Vector logging configuration...")
	vectorConfig := `# Vector configuration for FeralOS
data_dir: "/var/lib/vector"

` + apiBlock + `sources:
  syslog:
    type: socket
    mode: unix_datagram
    path: /run/vector/syslog.sock
    decoding:
      codec: syslog

  internal_logs:
    type: internal_logs
` + dinitSource + incusSource + `
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

  sudo_logs:
    type: filter
    inputs:
      - syslog
    condition: '.facility == "authpriv" && contains(to_string(.message) ?? "", "sudo")'

  firewalld_logs:
    type: filter
    inputs:
      - syslog
    condition: 'contains(to_string(.message) ?? "", "firewalld")'

  # JSON view of everything (syslog + vector internal logs + dinit service
  # logs [+ incus when installed]) — the dinit service name is extracted from
  # the file path; query with jq. Sinks added 2026-09-30 (Logging Round 2).
  feralos_json:
    type: remap
    inputs:
      - syslog
      - internal_logs` + dinitJSONInput + incusJSONInput + `
    source: |
      if exists(.file) {
        .service = to_string(.file) ?? ""
      }
      .collector = "feralos-vector"

sinks:
  messages:
    type: file
    inputs:
      - syslog` + dinitMsgInput + `
      - internal_logs
    path: "/var/log/messages"
    buffer:
      type: disk
      max_size: 268435488
    encoding:
      codec: text

  auth_log:
    type: file
    inputs:
      - auth_logs
    path: "/var/log/auth.log"
    buffer:
      type: disk
      max_size: 268435488
    encoding:
      codec: text

  daemon_log:
    type: file
    inputs:
      - daemon_logs
    path: "/var/log/daemon.log"
    buffer:
      type: disk
      max_size: 268435488
    encoding:
      codec: text

  kernel_log:
    type: file
    inputs:
      - kernel_logs
    path: "/var/log/kern.log"
    buffer:
      type: disk
      max_size: 268435488
    encoding:
      codec: text

  sudo_log:
    type: file
    inputs:
      - sudo_logs
    path: "/var/log/sudo.log"
    buffer:
      type: disk
      max_size: 268435488
    encoding:
      codec: text

  firewalld_log:
    type: file
    inputs:
      - firewalld_logs
    path: "/var/log/firewalld.log"
    buffer:
      type: disk
      max_size: 268435488
    encoding:
      codec: text

  feralos_json_log:
    type: file
    inputs:
      - feralos_json
    path: "/var/log/feralos.json.log"
    buffer:
      type: disk
      max_size: 268435488
    encoding:
      codec: json
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
	// dinit-chimera early chain; vector-setup stays OpenRC-only. The
	// dinit-boot-log package (feralos-aports) ships the tmpfiles (/var/log/
	// dinit), the mirror service and /usr/libexec/dinit-boot-log.

	// Logrotate: vector does NOT rotate files — it only tails rotated ones
	// (copytruncate + delaycompress recommended by vector docs). Covers the
	// vector sinks AND the dinit service logs; works for OpenRC and dinit.
	// Retention split (2026-09-30): auth/sudo 30d (security), general logs
	// 14d, tool logs 7d.
	fmt.Println("[INFO] Writing logrotate configuration...")
	logrotateConf := `/var/log/messages /var/log/daemon.log /var/log/kern.log {
    daily
    rotate 14
    compress
    delaycompress
    missingok
    notifempty
    copytruncate
}

/var/log/auth.log /var/log/sudo.log {
    daily
    rotate 30
    compress
    delaycompress
    missingok
    notifempty
    copytruncate
}

/var/log/vector.log /var/log/firewalld.log /var/log/feralos.json.log {
    daily
    rotate 7
    compress
    delaycompress
    missingok
    notifempty
    copytruncate
}
` + dinitLogrotate
	escapedLogrotate := strings.ReplaceAll(logrotateConf, "'", "'\\''")
	if _, err := shell.ExecuteCmd(chroot, fmt.Sprintf("mkdir -p /etc/logrotate.d && echo '%s' > /etc/logrotate.d/feralos", escapedLogrotate)); err != nil {
		fmt.Printf("[WARNING] Failed to write logrotate config: %v\n", err)
	}

	return nil
}
