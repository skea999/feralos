package internal

import (
	"alpineInstaller/utils/config"
	"alpineInstaller/utils/shell"
	"fmt"
	"strings"
)

// SetupServices configures OpenRC service runlevels and Dinit enablement.
func SetupServices(conf *config.Config) error {
	shell.SetCaller("services")
	chroot := conf.Internal.ChrootDir

	fmt.Println("[INFO] Configuring OpenRC services...")
	for level, services := range conf.User.Runlevels {
		for _, svc := range services {
			_, _ = shell.ExecuteCmd(chroot, fmt.Sprintf("rc-update add %s %s", svc, level))
		}
	}

	// swap service: 'before localmount' → 'need localmount'
	// before = wrong stop order (localmount unmounts /swap while swapfile still active)
	// need = hard dependency → swapoff runs BEFORE localmount unmounts /swap
	// NOTE: apk upgrade of openrc overwrites /etc/init.d/swap (sed is lost)
	if _, err := shell.ExecuteCmd(chroot, "sed -i 's/before localmount/need localmount/' /etc/init.d/swap"); err != nil {
		return err
	}

	// Dinit: enablement is data-driven (Alpine model: packages ship service
	// files, rules decide what runs), mirroring the OpenRC flow:
	//   - defaultRunlevelsDinit = base services chimera doesn't cover (getty)
	//   - packageLogic.services (merged into Runlevels) = per-package services
	// Uses the upstream offline command (no running daemon needed in chroot):
	//   dinitctl --offline -d /lib/dinit.d enable <svc>
	// Services in dinitChimeraServices are provided by the dinit-chimera early
	// chain (only OpenRC descriptions exist) and are never enabled.
	if conf.User.EnableDinit {
		fmt.Println("[INFO] Enabling Dinit services (dinitctl --offline, rules-driven)...")
		// defensive: boot.d dirs are shipped by dinit-chimera, but the
		// enablement must not fail if the packaging changes
		if _, err := shell.ExecuteCmd(chroot, "mkdir -p /etc/dinit.d/boot.d /lib/dinit.d/boot.d"); err != nil {
			return err
		}
		chimera := make(map[string]bool, len(conf.Internal.DinitChimeraServices))
		for _, s := range conf.Internal.DinitChimeraServices {
			chimera[s] = true
		}

		enabled, skipped := 0, 0
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
			base := svc
			if idx := strings.Index(svc, "@"); idx > 0 {
				base = svc[:idx]
			}
			_, _ = shell.ExecuteCmd(chroot, fmt.Sprintf("ln -sf /lib/dinit.d/%s /etc/dinit.d/boot.d/%s", base, svc))
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
				if chimera[svc] {
					skipped++
					continue
				}
				enableDinitSvc(svc)
			}
		}
		fmt.Printf("[INFO] Dinit: %d services enabled, %d chimera-covered skipped\n", enabled, skipped)
	}

	return nil
}
