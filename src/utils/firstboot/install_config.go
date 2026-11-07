package firstboot

import (
	"feralos/src/utils/config"
	"feralos/src/utils/shell"
	"fmt"
	"regexp"

	"gopkg.in/yaml.v3"
)

// InstallConfigPath is the in-guest location of the install config
// (/opt/feralos/config.yaml). The firstBoot standalone binaries duplicate
// the const by policy (self-sufficient binaries) — keep the values in sync.
const InstallConfigPath = "/opt/feralos/config.yaml"

// InstallConfig is the configuration written to /opt/feralos/config.yaml
// during installation. FirstBoot binaries read this to understand the
// disk layout, LUKS configuration and kernel flavor/naming (kernel.package
// + kernel.flavor from internal_rules.yaml — the tpm binary derives
// booster-<flavor>.img / linux-<flavor>.efi / vmlinuz-<flavor> from the
// flavor, no hardcoded literals) without parsing kernel cmdline.
type InstallConfig struct {
	DiskLayout      string   `yaml:"disk_layout"`
	Disks           []string `yaml:"disks"`
	LuksUUIDs       []string `yaml:"luks_uuids"`
	UserName        string   `yaml:"user_name"`
	Password        string   `yaml:"password"`
	Hostname        string   `yaml:"hostname"`
	Mirror          string   `yaml:"mirror"`
	Branch          string   `yaml:"branch"`
	KernelPackage   string   `yaml:"kernel_package"`
	KernelFlavor    string   `yaml:"kernel_flavor"`
	EnablePodman    bool     `yaml:"enable_podman"`
	EnableWebmin    bool     `yaml:"enable_webmin"`
	WebminListenAll bool     `yaml:"webmin_listen_all"`
}

// luksUUIDRe matches the rd.luks.name=<uuid>= parameters of the kernel
// cmdline. Canonical UUID shape (8-4-4-4-12 lowercase hex) — the former
// `[0-9a-f-]+` also accepted hyphens anywhere, turning any garbage echo of
// the parameter prefix into a "UUID" (a findfs lookup on it just wastes a
// boot-time exec; cryptsetup would fail later). Real samples
// (tests/artifacts/harness runs 2026-11-07T16-26-43Z / 15-56-12Z):
// rd.luks.name=c572e494-fe65-4ad5-89c3-9cdd9ef93e35=luks-c572e494-….
var luksUUIDRe = regexp.MustCompile(`rd\.luks\.name=([0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12})=`)

// ExtractLUKSUUIDs returns every LUKS UUID from rd.luks.name= cmdline
// parameters in order of appearance (FindAllStringSubmatch: all matches,
// appended in order); nil when none match. Pure function — shared with the
// firstBoot/tpm binary, which used to run a byte-identical local regex.
func ExtractLUKSUUIDs(cmdline string) []string {
	var uuids []string
	for _, m := range luksUUIDRe.FindAllStringSubmatch(cmdline, -1) {
		if len(m) >= 2 {
			uuids = append(uuids, m[1])
		}
	}
	return uuids
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
		KernelPackage:   conf.Internal.Kernel.Package,
		KernelFlavor:    conf.Internal.Kernel.Flavor,
		EnablePodman:    u.EnablePodman,
		EnableWebmin:    u.EnableWebmin,
		WebminListenAll: u.WebminListenAll,
	}

	// Extract LUKS UUIDs from kernel cmdline (written by chrootPhase/internal/boot.go)
	if cmdlineStr, err := shell.ExecuteCmd("", fmt.Sprintf("cat %s 2>/dev/null || true", chroot+"/etc/kernel/cmdline")); err == nil {
		cfg.LuksUUIDs = append(cfg.LuksUUIDs, ExtractLUKSUUIDs(cmdlineStr)...)
	}

	// Ensure /opt/feralos/ exists
	if _, err := shell.ExecuteCmd("", fmt.Sprintf("mkdir -p %s && chmod 0755 %s", chroot+"/opt/feralos", chroot+"/opt/feralos")); err != nil {
		return fmt.Errorf("failed to create /opt/feralos: %w", err)
	}

	data, err := yaml.Marshal(&cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal install config: %w", err)
	}

	// Direct write (0600): the file carries the user Password + LUKS
	// passphrase — the old heredoc form put the WHOLE command line (secrets
	// included) into installer.log; WriteFile logs path + size only.
	// Trailing \n preserved (the heredoc body ended with a blank line).
	cfgPath := chroot + InstallConfigPath
	if err := shell.WriteFile("", cfgPath, append(data, '\n'), 0600); err != nil {
		return fmt.Errorf("failed to write install config: %w", err)
	}

	fmt.Printf("[INFO] Install config written: %s%s\n", chroot, InstallConfigPath)
	return nil
}
