package features

import (
	"feralos/src/utils/config"
	"feralos/src/utils/shell"
	"fmt"
)

// SetupEdgeRepos installs the extraPackagesEdge packages from the @edge
// tagged repositories, in a controlled 3-step sequence at the very end of
// the package flow:
//  1. append the tagged @edge repositories to /etc/apk/repositories
//  2. install the extraPackagesEdge packages
//  3. comment the @edge repositories back out
//
// The installed system ends up with all stable packages + only the
// explicitly requested edge ones, and the edge repos disabled (apk v3
// masks tagged-repo packages from dependency resolution — installing
// edge packages mid-install breaks the stable batch solver).
// Requires enableEdgeRepos: true.
func SetupEdgeRepos(conf *config.Config) error {
	if !conf.User.EnableEdgeRepos {
		return nil
	}
	if len(conf.User.ExtraPackagesEdge) == 0 {
		return nil
	}

	chroot := conf.Internal.ChrootDir

	// 1. add the tagged @edge repositories
	fmt.Println("[INFO] Adding @edge repositories...")
	edgeRepos := fmt.Sprintf("@edge %s/edge/main\n@edge %s/edge/community\n@edge %s/edge/testing", conf.User.Mirror, conf.User.Mirror, conf.User.Mirror)
	if _, err := shell.ExecuteCmd(chroot, fmt.Sprintf("printf '%%s\n' '%s' >> /etc/apk/repositories", edgeRepos)); err != nil {
		return fmt.Errorf("failed to add edge repositories: %w", err)
	}

	// 2. install the edge packages
	fmt.Println("[INFO] Installing edge packages (extraPackagesEdge)...")
	args := append([]string{"add"}, conf.User.ExtraPackagesEdge...)
	if _, err := shell.ExecuteCmdArgs(chroot, "apk", args...); err != nil {
		return fmt.Errorf("failed to install edge packages: %w", err)
	}

	// 3. comment the @edge repositories out
	if _, err := shell.ExecuteCmd(chroot, "sed -i 's/^@edge /# @edge /' /etc/apk/repositories"); err != nil {
		return fmt.Errorf("failed to comment edge repositories: %w", err)
	}

	return nil
}
