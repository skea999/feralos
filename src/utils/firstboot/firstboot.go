package firstboot

import (
	"feralos/src/utils/config"
	"feralos/src/utils/shell"
	"fmt"
)

// buildBinary compiles a firstBoot binary and copies it into the chroot.
func buildBinary(conf *config.Config, name, buildPath string) error {
	chroot := conf.Internal.ChrootDir

	binPath := chroot + "/opt/feralos_" + name
	buildCmd := fmt.Sprintf("cd src && go build -o %s ./firstBoot/%s/", binPath, buildPath)
	if _, err := shell.ExecuteCmd("", buildCmd); err != nil {
		return fmt.Errorf("failed to build %s binary: %w", name, err)
	}

	if _, err := shell.ExecuteCmd("", fmt.Sprintf("chmod 755 %s", binPath)); err != nil {
		return fmt.Errorf("failed to chmod %s binary: %w", name, err)
	}

	fmt.Printf("[INFO] %s binary: %s\n", name, binPath)
	return nil
}
