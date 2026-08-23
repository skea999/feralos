package features

import (
	"alpineInstaller/utils/config"
	"fmt"
	"os"
	"regexp"

	"gopkg.in/yaml.v3"
)

// InstallConfig is the configuration written to /opt/feralos/config.yaml
// during installation. FirstBoot binaries read this to understand the
// disk layout and LUKS configuration without parsing kernel cmdline.
type InstallConfig struct {
	DiskLayout string   `yaml:"disk_layout"`
	Disks      []string `yaml:"disks"`
	LuksUUIDs  []string `yaml:"luks_uuids"`
	UserName   string   `yaml:"user_name"`
	Hostname   string   `yaml:"hostname"`
	Mirror     string   `yaml:"mirror"`
	Branch     string   `yaml:"branch"`
}

// WriteInstallConfig writes /opt/feralos/config.yaml into the mounted chroot.
// Called during hostPhase so firstBoot binaries can read it post-install.
func WriteInstallConfig(conf *config.Config) error {
	chroot := conf.Internal.ChrootDir
	u := &conf.User

	cfg := InstallConfig{
		DiskLayout: u.DiskLayout,
		Disks:      u.Disks,
		UserName:   u.UserName,
		Hostname:   u.Hostname,
		Mirror:     u.Mirror,
		Branch:     u.Branch,
	}

	// Extract LUKS UUIDs from kernel cmdline (already written by disk layout)
	cmdline, err := os.ReadFile(chroot + "/etc/kernel/cmdline")
	if err == nil {
		re := regexp.MustCompile(`rd\.luks\.name=([0-9a-f-]+)=`)
		matches := re.FindAllStringSubmatch(string(cmdline), -1)
		for _, m := range matches {
			if len(m) >= 2 {
				cfg.LuksUUIDs = append(cfg.LuksUUIDs, m[1])
			}
		}
	}

	// Ensure /opt/feralos/ exists
	if mkdirErr := os.MkdirAll(chroot+"/opt/feralos", 0755); mkdirErr != nil {
		return fmt.Errorf("failed to create /opt/feralos: %w", mkdirErr)
	}

	data, err := yaml.Marshal(&cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal install config: %w", err)
	}

	if err := os.WriteFile(chroot+"/opt/feralos/config.yaml", data, 0600); err != nil {
		return fmt.Errorf("failed to write install config: %w", err)
	}

	fmt.Printf("[INFO] Install config written: %s/opt/feralos/config.yaml\n", chroot)
	return nil
}
