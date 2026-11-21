package internal

import (
	"errors"
	"fmt"
	"strings"

	"feralos/src/utils/config"
	"feralos/src/utils/shell"
)

// SetupServices configures service enablement.
// OpenRC: rc-update runlevels. Dinit: rules-driven boot.d symlinks
// (packages ship service files, this only enables) + shutdown commands.
func SetupServices(conf *config.Config) error {
	shell.SetCaller("services")
	chroot := conf.Internal.ChrootDir

	// NOTE: the /var/log/ursine belt-and-braces mkdir moved to misc.go
	// (setupUrsineLogDir, with the other daemon-dir setups).

	// OpenRC runlevels — ALWAYS configured, NEVER conditional: a dinit
	// install must remain bootable under OpenRC (fallback), so the rc-update
	// links are created regardless of the active init. They are inert under
	// dinit (PID1 never reads /etc/runlevels) and required for the fallback.
	fmt.Println("[INFO] Configuring OpenRC services...")
	for level, services := range conf.User.Runlevels {
		for _, svc := range services {
			if _, err := shell.ExecuteCmd(chroot, fmt.Sprintf("rc-update add %s %s", svc, level)); err != nil {
				return fmt.Errorf("failed to enable %s: %w", svc, err)
			}
		}
	}

	// swap service: 'before localmount' → 'need localmount'
	// before = wrong stop order (localmount unmounts /swap while swapfile still active)
	// need = hard dependency → swapoff runs BEFORE localmount unmounts /swap
	// NOTE: apk upgrade of openrc overwrites /etc/init.d/swap (sed is lost)
	if _, err := shell.ExecuteCmd(chroot, "sed -i 's/before localmount/need localmount/' /etc/init.d/swap"); err != nil {
		return err
	}

	// polkit power rule: package-shipped (polkit-power-config, ADR-013).

	// Dinit: enablement is data-driven (Alpine model: packages ship service
	// files, rules decide what runs), mirroring the OpenRC flow:
	//   - defaultRunlevelsDinit = base services chimera doesn't cover
	//     (system: processes boot.d; getty: console login)
	//   - packageLogic.services (merged into Runlevels) = per-package services
	// Uses direct symlinks (Alpine dinitctl 0.21 --offline crashes on
	// every service — upstream cpbuffer assert bug):
	//   ln -sf /lib/dinit.d/<svc> /etc/dinit.d/boot.d/<svc>
	// The 'system' service is critical: boot depends on system, and system
	// processes /usr/lib/dinit.d/boot.d/ — without it, no boot.d services
	// (networking, sshd, etc.) ever start.
	// Services in dinitSkipServices are provided by the dinit-chimera
	// early chain (or replaced by tmpfiles, e.g. vector-setup) and are
	// never enabled.
	if conf.User.EnableDinit {
		fmt.Println("[INFO] Enabling Dinit services (rules-driven boot.d symlinks)...")
		// defensive: boot.d dirs are shipped by dinit-chimera, but the
		// enablement must not fail if the packaging changes
		if _, err := shell.ExecuteCmd(chroot, "mkdir -p /etc/dinit.d/boot.d /lib/dinit.d/boot.d"); err != nil {
			return err
		}
		skip := make(map[string]bool, len(conf.Internal.DinitSkipServices))
		for _, s := range conf.Internal.DinitSkipServices {
			skip[s] = true
		}

		enabled, skipped := 0, 0
		// P2-3: enablement errors are collected and returned as one joined
		// error after the loops (a failed symlink must not be invisible to
		// the exit code). Under SKEA_IGNORE_ERRORS=1 shell.ExecuteCmd returns
		// nil on failure — best-effort behavior unchanged. The
		// "could not enable" WARNING stays informational (pre-existing
		// semantics: service file genuinely absent is a packaging fact).
		var enableErrs []error
		// dinitctl enable (from-service defaults to boot) creates the symlink
		// in boot's waits-for.d = /etc/dinit.d/boot.d (the ADMIN dir) — kept
		// separate from /lib/dinit.d/boot.d where packages drop their links;
		// both are waited for at boot.
		linkExists := func(svc string) bool {
			out, _ := shell.ExecuteCmd(chroot, fmt.Sprintf(
				"test -e /etc/dinit.d/boot.d/%s -o -e /lib/dinit.d/boot.d/%s && echo YES || echo NO", svc, svc))
			return strings.Contains(out, "YES")
		}
		enableDinitSvc := func(svc string) {
			// Alpine dinitctl 0.21.0: `--offline enable` CRASHES on every
			// service (unique_ptr assert in cpbuffer — upstream bug), so the
			// symlink is created directly. This is byte-for-byte what a
			// working `dinitctl --offline -d /lib/dinit.d enable <svc>`
			// would do: a link named <svc> in boot's waits-for.d
			// (/etc/dinit.d/boot.d, the admin dir) targeting the description
			// in /lib/dinit.d. Template instances carry the argument in the
			// name (getty@tty1 -> $1 = tty1), target = template body.
			// TARGET RESOLUTION: most -dinit packages install to
			// /lib/dinit.d/<unit>, but some units live in /etc/dinit.d/
			// (e.g. turnstiled ships in the main turnstile package at
			// /etc/dinit.d/turnstiled) — without the /etc fallback the
			// boot.d link dangles and the installer logs
			// "[WARNING] dinit: could not enable <svc>" for a service that
			// dinit would actually resolve by name.
			base := svc
			if idx := strings.Index(svc, "@"); idx > 0 {
				base = svc[:idx]
			}
			if _, err := shell.ExecuteCmd(chroot, fmt.Sprintf("t=/lib/dinit.d/%s; [ -e \"$t\" ] || t=/etc/dinit.d/%s; ln -sf \"$t\" /etc/dinit.d/boot.d/%s", base, base, svc)); err != nil {
				enableErrs = append(enableErrs, fmt.Errorf("failed to enable %s: %w", svc, err))
				return
			}
			if linkExists(svc) {
				enabled++
			} else {
				fmt.Printf("[WARNING] dinit: could not enable %s (service file not installed?)\n", svc)
			}
		}

		for _, services := range conf.User.RunlevelsDinit {
			for _, svc := range services {
				enableDinitSvc(svc)
			}
		}
		for level, services := range conf.User.Runlevels {
			if level == "sysinit" || level == "shutdown" {
				continue // dinit-chimera handles these phases entirely
			}
			for _, svc := range services {
				if skip[svc] {
					skipped++
					continue
				}
				enableDinitSvc(svc)
			}
		}
		if len(enableErrs) > 0 {
			return fmt.Errorf("dinit service enablement failed: %w", errors.Join(enableErrs...))
		}
		fmt.Printf("[INFO] Dinit: %d services enabled, %d chimera-covered skipped\n", enabled, skipped)

		if err := setupDinitShutdownCommands(chroot); err != nil {
			return fmt.Errorf("dinit shutdown commands failed: %w", err)
		}
	}

	return nil
}

// setupDinitShutdownCommands wires the conventional shutdown commands to
// dinit's shutdown helpers so poweroff/reboot/halt/shutdown behave like on
// Artix/Chimera (their dinit packages own /usr/bin/{poweroff,reboot,...}).
// Alpine builds dinit with --shutdown-prefix=dinit- → helpers in /usr/sbin/.
// The helpers derive the action from the invocation name (base_name(argv[0])):
// plain symlinks work for poweroff/halt/shutdown (no reboot match → default
// power down), but reboot/soft-reboot MUST exec the prefixed binary — a
// "reboot"-named link matches neither "dinit-reboot" nor "dinit-soft-reboot"
// and would power down instead of rebooting. /usr/bin precedes /sbin in PATH
// → these shadow the busybox applets, which dinit cannot use (busybox
// poweroff sends SIGUSR2 — dinit ignores it).
func setupDinitShutdownCommands(chroot string) error {
	type commandLink struct{ name, target string }

	for _, link := range []commandLink{
		{"poweroff", "/usr/sbin/dinit-poweroff"},
		{"halt", "/usr/sbin/dinit-halt"},
		{"shutdown", "/usr/sbin/dinit-shutdown"},
	} {
		if _, err := shell.ExecuteCmd(chroot, fmt.Sprintf("ln -sf %s /usr/bin/%s", link.target, link.name)); err != nil {
			return fmt.Errorf("failed to link /usr/bin/%s: %w", link.name, err)
		}
	}

	for _, wrapper := range []commandLink{
		{"reboot", "/usr/sbin/dinit-reboot"},
		{"soft-reboot", "/usr/sbin/dinit-soft-reboot"},
	} {
		// rm first: if the path ever exists as a symlink (e.g. a busybox
		// applet), redirection would follow it and overwrite the target.
		if _, err := shell.ExecuteCmd(chroot, "rm -f /usr/bin/"+wrapper.name); err != nil {
			return fmt.Errorf("failed to remove stale /usr/bin/%s: %w", wrapper.name, err)
		}
		content := fmt.Sprintf("#!/bin/sh\nexec %s \"$@\"\n", wrapper.target)
		if err := shell.WriteFile(chroot, "/usr/bin/"+wrapper.name, []byte(content), 0644); err != nil {
			return fmt.Errorf("failed to write /usr/bin/%s wrapper: %w", wrapper.name, err)
		}
		if _, err := shell.ExecuteCmd(chroot, "chmod 755 /usr/bin/"+wrapper.name); err != nil {
			return fmt.Errorf("failed to chmod /usr/bin/%s wrapper: %w", wrapper.name, err)
		}
	}

	// Deterministic output check via allowFail (same pattern as
	// SetupDbusAudit): the probe cannot change across retries, so the
	// ExecuteCmd retry/interactive machinery is pointless — the `&& echo OK`
	// sentinel carries the result on stdout in EVERY mode.
	out, _ := shell.ExecuteCmdWithAllowFail(chroot, "test -x /usr/bin/poweroff -a -x /usr/bin/reboot -a -x /usr/bin/halt -a -x /usr/bin/shutdown -a -x /usr/bin/soft-reboot && echo OK", true, false)
	if strings.TrimSpace(out) != "OK" {
		return fmt.Errorf("dinit shutdown commands were not installed correctly")
	}

	// elogind shutdown hook (DMS "Power off" / "Reboot" fix): DMS runs
	// `loginctl poweroff|reboot` (elogind branch); elogind bypasses PATH and
	// execs the ABSOLUTE /sbin/poweroff (busybox) — busybox sends SIGUSR2,
	// which dinit IGNORES, so the machine never goes down. elogind has no
	// config override for the binary path, but it runs
	// /etc/elogind/system-shutdown hooks BEFORE the exec: trigger the real
	// dinit shutdown from the hook (exec — elogind waits on this script and
	// the machine tears down; if elogind were to proceed anyway, the
	// subsequent busybox exec is a harmless no-op).
	hookDir := "/etc/elogind/system-shutdown"
	hookContent := `#!/bin/sh
# FeralOS: elogind execs the absolute /sbin/poweroff (busybox, SIGUSR2 —
# ignored by dinit) AFTER these hooks. Trigger the real dinit shutdown here.
case "$1" in
poweroff) exec /usr/sbin/dinit-poweroff ;;
reboot)   exec /usr/sbin/dinit-reboot ;;
halt)     exec /usr/sbin/dinit-halt ;;
esac
`
	if err := shell.WriteFile(chroot, hookDir+"/50-feralos-dinit.sh", []byte(hookContent+"\n"), 0644); err != nil {
		return fmt.Errorf("failed to install elogind shutdown hook: %w", err)
	}
	if _, err := shell.ExecuteCmd(chroot, fmt.Sprintf("chmod 755 '%s/50-feralos-dinit.sh'", hookDir)); err != nil {
		return fmt.Errorf("failed to chmod elogind shutdown hook: %w", err)
	}

	fmt.Println("[INFO] Dinit shutdown commands installed (poweroff/reboot/halt/shutdown/soft-reboot)")
	fmt.Println("[INFO] elogind shutdown hook installed (loginctl poweroff/reboot → dinit)")

	return nil
}
