package internal

import (
	"alpineInstaller/utils/config"
	"alpineInstaller/utils/shell"
	"fmt"
)

// PrepareChrootEnv writes APK repositories into the chroot.
// Must be called after BootstrapAlpine and before chrootPhase runs.
// Uses host path (no chroot tag) — hostPhase writes via host paths.
func PrepareChrootEnv(conf *config.Config) error {
	chroot := conf.Internal.ChrootDir
	u := conf.User

	fmt.Println("[INFO] Configuring APK repositories...")
	repoContent := fmt.Sprintf("echo '%s/%s/main\n%s/%s/community' > %s/etc/apk/repositories", u.Mirror, u.Branch, u.Mirror, u.Branch, chroot)
	if _, err := shell.ExecuteCmd("", repoContent); err != nil {
		return fmt.Errorf("failed to write repositories: %w", err)
	}

	if u.EnableEdgeRepos {
		fmt.Println("[INFO] Adding @edge tagged repositories...")
		edgeContent := fmt.Sprintf(
			"printf '%%s\n' '@edge %s/edge/main' '@edge %s/edge/community' '@edge %s/edge/testing' >> %s/etc/apk/repositories",
			u.Mirror, u.Mirror, u.Mirror, chroot)
		if _, err := shell.ExecuteCmd("", edgeContent); err != nil {
			return fmt.Errorf("failed to add edge repositories: %w", err)
		}
	}

	return nil
}
