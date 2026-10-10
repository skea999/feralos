package firstboot

import (
	"feralos/src/utils/config"
	"feralos/src/utils/shell"
	"fmt"
	"regexp"

	"gopkg.in/yaml.v3"
)

// InstallConfig is the configuration written to /opt/feralos/config.yaml
// during installation. FirstBoot binaries read this to understand the
// disk layout and LUKS configuration without parsing kernel cmdline.
type InstallConfig struct {
	DiskLayout      string   `yaml:"disk_layout"`
	Disks           []string `yaml:"disks"`
	LuksUUIDs       []string `yaml:"luks_uuids"`
	UserName        string   `yaml:"user_name"`
	Password        string   `yaml:"password"`
	Hostname        string   `yaml:"hostname"`
	Mirror          string   `yaml:"mirror"`
	Branch          string   `yaml:"branch"`
	EnablePodman    bool     `yaml:"enable_podman"`
	EnableWebmin    bool     `yaml:"enable_webmin"`
	WebminListenAll bool     `yaml:"webmin_listen_all"`
}

// WriteInstallConfig writes /opt/feralos/config.yaml into the mounted chroot.
// Called after chrootPhase so /etc/kernel/cmdline is already written by SetupBoot.
func WriteInstallConfig(conf *config.Config) error {
	chroot := conf.Internal.ChrootDir
	u := &conf.User

	cfg := InstallConfig{
		DiskLayout:      u.DiskLayout,
		Disks:           u.Disks,
		UserName:        u.UserName,
		Password:        u.Password,
		Hostname:        u.Hostname,
		Mirror:          u.Mirror,
		Branch:          u.Branch,
		EnablePodman:    u.EnablePodman,
		EnableWebmin:    u.EnableWebmin,
		WebminListenAll: u.WebminListenAll,
	}

	// Extract LUKS UUIDs from kernel cmdline (written by chrootPhase/internal/boot.go)
	if cmdlineStr, err := shell.ExecuteCmd("", fmt.Sprintf("cat %s 2>/dev/null || true", chroot+"/etc/kernel/cmdline")); err == nil {
		re := regexp.MustCompile(`rd\.luks\.name=([0-9a-f-]+)=`)
		matches := re.FindAllStringSubmatch(cmdlineStr, -1)
		for _, m := range matches {
			if len(m) >= 2 {
				cfg.LuksUUIDs = append(cfg.LuksUUIDs, m[1])
			}
		}
	}

	// Ensure /opt/feralos/ exists
	if _, err := shell.ExecuteCmd("", fmt.Sprintf("mkdir -p %s && chmod 0755 %s", chroot+"/opt/feralos", chroot+"/opt/feralos")); err != nil {
		return fmt.Errorf("failed to create /opt/feralos: %w", err)
	}

	data, err := yaml.Marshal(&cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal install config: %w", err)
	}

	// Write config via heredoc + chmod 0600
	cfgPath := chroot + "/opt/feralos/config.yaml"
	writeCmd := fmt.Sprintf("cat > %s <<'FERA_EOF'\n%s\nFERA_EOF", cfgPath, string(data))
	if _, err := shell.ExecuteCmd("", writeCmd); err != nil {
		return fmt.Errorf("failed to write install config: %w", err)
	}
	if _, err := shell.ExecuteCmd("", fmt.Sprintf("chmod 0600 %s", cfgPath)); err != nil {
		return fmt.Errorf("failed to chmod install config: %w", err)
	}

	fmt.Printf("[INFO] Install config written: %s/opt/feralos/config.yaml\n", chroot)
	return nil
}
