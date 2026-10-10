package internal

import (
	"feralos/src/utils/config"
	"feralos/src/utils/shell"
	"fmt"
	"strings"
)

// BootstrapAlpine downloads apk-tools-static and initializes the Alpine base system.
func BootstrapAlpine(conf *config.Config) error {
	shell.SetCaller("bootstrap")
	u := conf.User
	i := conf.Internal

	fmt.Printf("[INFO] Bootstrapping Alpine Linux %s (%s)...\n", u.Branch, u.Arch)

	fmt.Println("[INFO] Fetching latest apk-tools-static version...")
	repoURL := fmt.Sprintf("%s/%s/main", u.Mirror, u.Branch)
	archURL := fmt.Sprintf("%s/%s", repoURL, u.Arch)

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
	_, dlErr := shell.ExecuteCmdArgs("", "curl", "-LO", fullURL)
	if dlErr != nil {
		return fmt.Errorf("failed to download apk-static: %w", dlErr)
	}

	fmt.Println("[DEBUG] Extracting apk-tools-static")
	_, extractErr := shell.ExecuteCmdArgs("", "tar", "-xzf", apkFileName)
	if extractErr != nil {
		return fmt.Errorf("failed to extract apk-static: %w", extractErr)
	}

	fmt.Println("[INFO] Initializing Alpine base system into target...")
	_, err = shell.ExecuteCmdArgs("", "./sbin/apk.static", "-X", repoURL, "-U", "--allow-untrusted", "-p", i.ChrootDir, "--initdb", "add", "alpine-base")

	_, _ = shell.ExecuteCmdArgs("", "rm", apkFileName)
	_, _ = shell.ExecuteCmdArgs("", "rm", "-rf", "sbin")

	return err
}
