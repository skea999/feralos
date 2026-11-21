package internal

import (
	"feralos/src/utils/config"
	"feralos/src/utils/shell"
	"fmt"
	"strings"
)

// SetupLogging writes the vector-setup OpenRC boot service (the Vector
// CONFIGURATION is package-shipped — vector-config, see the aports
// package). No-op when vector is not in the package list.
func SetupLogging(conf *config.Config, pkgList string) error {
	chroot := conf.Internal.ChrootDir

	vectorInstalled := strings.Contains(pkgList, "\nvector\n") || strings.HasPrefix(pkgList, "vector\n") || strings.HasSuffix(pkgList, "\nvector")
	if !vectorInstalled {
		fmt.Println("[INFO] Vector not installed, skipping config")
		return nil
	}

	if err := writeVectorSetupService(chroot); err != nil {
		return err
	}

	// OpenRC klogd is shipped as the klogd-openrc package (989 series,
	// feralos-aports): the vendor busybox-openrc script can never start here
	// (`depend() need logger` — busybox syslogd was replaced by vector). The
	// package installs /etc/init.d/klogd (dmesg snapshot + klogd -n after
	// vector); enablement via the boot runlevel (internal_rules). No runtime
	// file writes.

	// vector-setup: the script written above is enabled on BOTH inits
	// (OpenRC enablement is never conditional — fallback readiness); under
	// dinit it is inert and vector-dinit's tmpfiles file (/run/vector,
	// /dev/log, /var/log, /btrbk — processed by the dinit-chimera early
	// chain) remains the active mechanism. The dinit-boot-log package
	// (feralos-aports) ships the tmpfiles (/var/log/dinit), the mirror
	// service and /usr/libexec/dinit-boot-log.

	return nil
}

// The Vector CONFIGURATION is package-shipped (vector-config,
// /etc/vector/vector.yaml — replaces the Alpine demo), and so are the LOG
// SINKS it names (/var/log/{messages,auth.log,...}, materialized by the
// package's post-install with the vector owner — ADR-013 R1: files the
// service must find are configuration). The installer writes only the
// OpenRC-specific glue (vector-setup dirs/socket script — dinit uses the
// vector-dinit tmpfiles instead); /var/log itself stays root-owned 0755.
// writeVectorSetupService writes the vector-setup boot service (OpenRC init
// script; under dinit it is inert and the dinit tmpfiles cover the same
// paths) and makes it executable.
func writeVectorSetupService(chroot string) error {
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
    chown btrbk:btrbk /btrbk 2>/dev/null || true
    eend $?
}
`
	if err := shell.WriteFile(chroot, "/etc/init.d/vector-setup", []byte(vectorSetupInit+"\n"), 0644); err != nil {
		fmt.Printf("[WARNING] Failed to write vector-setup init script: %v\n", err)
	}
	if _, err := shell.ExecuteCmd(chroot, "chmod +x /etc/init.d/vector-setup"); err != nil {
		fmt.Printf("[WARNING] Failed to make vector-setup executable: %v\n", err)
	}

	return nil
}
