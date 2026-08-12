package hostPhase

import (
	"alpineInstaller/utils/config"
	"alpineInstaller/utils/shell"
	"fmt"
	"strings"
)

// BootstrapAlpine downloads apk-tools-static and initializes the Alpine base system.
func BootstrapAlpine(conf *config.Config) error {
	shell.SetCaller("bootstrap")
	// u = User variables (mirror, branch, arch)
	// i = Internal system rules (chrootDir)
	u := conf.User
	i := conf.Internal

	fmt.Printf("[INFO] Bootstrapping Alpine Linux %s (%s)...\n", u.Branch, u.Arch)

	fmt.Println("[INFO] Fetching latest apk-tools-static version...")
	// repoURL must NOT include architecture — apk.static appends /ARCH/APKINDEX.tar.gz
	repoURL := fmt.Sprintf("%s/%s/main", u.Mirror, u.Branch)
	// archURL includes architecture for fetching the package index listing
	archURL := fmt.Sprintf("%s/%s", repoURL, u.Arch)

	// Command to find the latest version of apk-tools-static from the repository index
	findCmd := fmt.Sprintf("curl -sSf %s/ | sed -n 's/.*apk-tools-static-\\(.*\\)\\.apk.*/\\1/p' | head -n1", archURL)
	apkVersion, err := shell.ExecuteCmd("", findCmd)
	if err != nil {
		return fmt.Errorf("failed to detect apk-tools-static version: %w", err)
	}
	apkVersion = strings.TrimSpace(apkVersion)

	if apkVersion == "" {
		return fmt.Errorf("failed to detect apk-tools-static version: empty result from %s", archURL)
	}

	apkFileName := fmt.Sprintf("apk-tools-static-%s.apk", apkVersion)
	fullURL := fmt.Sprintf("%s/%s", archURL, apkFileName)

	fmt.Printf("[DEBUG] Downloading %s\n", apkFileName)
	if _, err := shell.ExecuteCmd("", fmt.Sprintf("curl -LO %s", fullURL)); err != nil {
		return fmt.Errorf("failed to download apk-static: %w", err)
	}

	fmt.Println("[DEBUG] Extracting apk-tools-static")
	if _, err := shell.ExecuteCmd("", fmt.Sprintf("tar -xzf %s", apkFileName)); err != nil {
		return fmt.Errorf("failed to extract apk-static: %w", err)
	}

	fmt.Println("[INFO] Initializing Alpine base system into target...")
	// We use i.ChrootDir for the destination path (-p)
	// Use ExecuteCmd — bootstrap is critical, must not be ignored
	_, err = shell.ExecuteCmd("", fmt.Sprintf("./sbin/apk.static -X %s -U --allow-untrusted -p %s --initdb add alpine-base", repoURL, i.ChrootDir))

	// Cleanup the downloaded files
	_, _ = shell.ExecuteCmd("", fmt.Sprintf("rm %s", apkFileName))
	_, _ = shell.ExecuteCmd("", "rm -rf sbin")

	return err
}
