package features

import (
	"feralos/src/utils/config"
	"feralos/src/utils/shell"
	"fmt"
	"strings"
)

// apkRepositoriesFile is the guest apk repository list this feature appends
// to and comments back out (P3-7: repeated path → const).
const apkRepositoriesFile = "/etc/apk/repositories"

// SetupEdgeRepos installs the extraPackagesEdge packages from the @edge
// tagged repositories, in a controlled 3-step sequence as the LAST install
// of chroot_phase.Run (step 8b, right after SetupDbusAudit — the audit's
// apk fallback must run while the repos are still stable-only):
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
	// Feature-gated rules contract: the repo list comes from internal_rules
	// (edgeRepositories — appended as `@edge <mirror>/<repo>`); an empty
	// list would install nothing while claiming the repos were added.
	if len(conf.Internal.EdgeRepositories) == 0 {
		return fmt.Errorf("edgeRepositories missing from internal_rules (enableEdgeRepos requires edgeRepositories)")
	}

	chroot := conf.Internal.ChrootDir

	// 1. add the tagged @edge repositories
	fmt.Println("[INFO] Adding @edge repositories...")
	edgeLines := make([]string, 0, len(conf.Internal.EdgeRepositories))
	for _, repo := range conf.Internal.EdgeRepositories {
		edgeLines = append(edgeLines, fmt.Sprintf("@edge %s/%s", conf.User.Mirror, repo))
	}
	edgeRepos := strings.Join(edgeLines, "\n")
	if _, err := shell.ExecuteCmd(chroot, fmt.Sprintf("printf '%%s\n' '%s' >> %s", edgeRepos, apkRepositoriesFile)); err != nil {
		return fmt.Errorf("failed to add edge repositories: %w", err)
	}

	// 2. install the edge packages
	fmt.Println("[INFO] Installing edge packages (extraPackagesEdge)...")
	args := append([]string{"add"}, conf.User.ExtraPackagesEdge...)
	if _, err := shell.ExecuteCmdArgs(chroot, "apk", args...); err != nil {
		return fmt.Errorf("failed to install edge packages: %w", err)
	}

	// 3. comment the @edge repositories out
	if _, err := shell.ExecuteCmd(chroot, fmt.Sprintf("sed -i 's/^@edge /# @edge /' %s", apkRepositoriesFile)); err != nil {
		return fmt.Errorf("failed to comment edge repositories: %w", err)
	}

	return nil
}
