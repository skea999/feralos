package desktops

import (
	"alpineInstaller/utils/config"
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

	fn, ok := setupRegistry[de]
	if !ok {
		fmt.Printf("[WARNING] Desktop environment %q not implemented, skipping\n", de)
		return nil
	}

	return fn(conf)
}
