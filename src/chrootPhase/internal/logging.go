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

	// Dinit: equivalent oneshot (vector-dinit depends-on vector-setup)
	if conf.User.EnableDinit {
		fmt.Println("[INFO] Creating vector-setup dinit service...")
		vectorSetupDinit := `# dinit service: vector-setup (FeralOS installer — mirrors OpenRC vector-setup)
type = scripted
command = /bin/sh -c 'mkdir -p /run/vector && chown vector:vector /run/vector && chmod 755 /run/vector && ln -sf /run/vector/syslog.sock /dev/log && chown vector:vector /var/log; chown btrbk:btrbk /btrbk 2>/dev/null || true'
depends-on = local.target
`
		escapedVectorSetupDinit := strings.ReplaceAll(vectorSetupDinit, "'", "'\\''")
		if _, err := shell.ExecuteCmd(chroot, fmt.Sprintf("echo '%s' > /lib/dinit.d/vector-setup", escapedVectorSetupDinit)); err != nil {
			fmt.Printf("[WARNING] Failed to write vector-setup dinit service: %v\n", err)
		}
	}

	return nil
}
