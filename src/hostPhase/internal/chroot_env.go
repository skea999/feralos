package internal

import (
	"feralos/src/utils/config"
	"feralos/src/utils/shell"
	"fmt"
)

// PrepareChrootEnv writes APK repositories into the chroot.
// Must be called after BootstrapAlpine and before chrootPhase runs.
// Uses host path (no chroot tag) — hostPhase writes via host paths.
func PrepareChrootEnv(conf *config.Config) error {
	chroot := conf.Internal.ChrootDir
	u := conf.User

	fmt.Println("[INFO] Configuring APK repositories...")
	// Direct write — exact bytes, no escaped-echo scaffolding (trailing \n
	// preserved: `echo` used to terminate the last line the same way).
	repoContent := fmt.Sprintf("%s/%s/main\n%s/%s/community\n", u.Mirror, u.Branch, u.Mirror, u.Branch)
	if err := shell.WriteFile("", chroot+"/etc/apk/repositories", []byte(repoContent), 0644); err != nil {
		return fmt.Errorf("failed to write repositories: %w", err)
	}

	return nil
}
