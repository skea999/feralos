package desktops

import (
	"feralos/src/utils/config"
	"feralos/src/utils/shell"
	"fmt"
)

// Setup dispatches to the appropriate desktop environment setup.
// Registry map: desktop name → setup function. Unknown DEs are skipped.
var setupRegistry = map[string]func(*config.Config) error{
	"niri": SetupNiri,
}

// Setup dispatches to the appropriate desktop environment setup.
func Setup(conf *config.Config) error {
	de := conf.User.DesktopEnvironment
	if de == "" || de == "none" {
		return nil
	}

	chroot := conf.Internal.ChrootDir

	// Enable Unicode in rc.conf — required for all desktop environments on
	// Alpine (musl disables unicode by default).
	if _, err := shell.ExecuteCmd(chroot, `sed -i 's/^#unicode="YES"$/unicode="YES"/' /etc/rc.conf`); err != nil {
		fmt.Printf("[WARNING] Failed to enable unicode in rc.conf: %v\n", err)
	}

	fn, ok := setupRegistry[de]
	if !ok {
		return nil
	}

	return fn(conf)
}
