package internal

import (
	"alpineInstaller/utils/config"
	"alpineInstaller/utils/shell"
	"fmt"
)

// SetupServices configures OpenRC service runlevels.
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

	return nil
}
