package internal

import (
	"alpineInstaller/utils/config"
	"alpineInstaller/utils/shell"
	"fmt"
)

// PrepareChrootEnv writes APK repositories into the chroot.
// Must be called after BootstrapAlpine and before chrootPhase runs.
func PrepareChrootEnv(conf *config.Config) error {
	chroot := conf.Internal.ChrootDir
	u := conf.User

	fmt.Println("[INFO] Configuring APK repositories...")
	repoContent := fmt.Sprintf("echo '%s/%s/main\n%s/%s/community' > /etc/apk/repositories", u.Mirror, u.Branch, u.Mirror, u.Branch)
	if _, err := shell.ExecuteCmd(chroot, repoContent); err != nil {
		return fmt.Errorf("failed to write repositories: %w", err)
	}

	return nil
}
