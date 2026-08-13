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

	return nil
}
