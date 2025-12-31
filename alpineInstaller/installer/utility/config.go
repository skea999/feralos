package utility

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	User     UserVariables `yaml:"variables"`
	Internal InternalRules
}

type UserVariables struct {
	Unattended         bool     `yaml:"unattended"`
	Disks              []string `yaml:"disks"`
	UserName           string   `yaml:"userName"`
	Password           string   `yaml:"password"`
	Hostname           string   `yaml:"hostname"`
	Timezone           string   `yaml:"timezone"`
	Locale             string   `yaml:"locale"`
	Mirror             string   `yaml:"mirror"`
	Arch               string   `yaml:"arch"`
	Branch             string   `yaml:"branch"`
	ExtraPackages      []string `yaml:"extraPackages"`
	DiskLayout         string   `yaml:"diskLayout"`
	DesktopEnvironment string   `yaml:"desktopEnvironment"`
	VideoDriver        string   `yaml:"videoDriver"`

	// Runtime fields
	UserGroups       []string
	Runlevels        map[string][]string
	AllPackages      []string
	Modules          []string
	GrubCmdline      []string
	MkinitfsFeatures []string
}

type InternalRules struct {
	ChrootDir           string                         `yaml:"chrootDir"`
	StandardPackages    []string                       `yaml:"standardPackages"`
	DefaultRunlevels    map[string][]string            `yaml:"defaultRunlevels"`
	DiskLayouts         map[string]DynamicPackageRules `yaml:"diskLayouts"`
	DesktopEnvironments map[string]DynamicPackageRules `yaml:"desktopEnvironments"`
	VideoDrivers        map[string]DynamicPackageRules `yaml:"videoDrivers"`
	PackageLogic        map[string]DynamicPackageRules `yaml:"packageLogic"`
}

// Ensure the field is named "Packages" to match logic.go
type DynamicPackageRules struct {
	Packages         []string          `yaml:"packages"` // <--- Field must exist here
	Groups           []string          `yaml:"groups"`
	Services         map[string]string `yaml:"services"`
	Modules          []string          `yaml:"modules"`
	GrubCmdline      []string          `yaml:"grubCmdline"`
	MkinitfsFeatures []string          `yaml:"mkinitfsFeatures"`
}

func LoadConfig(userPath string) (*Config, error) {
	conf := &Config{}
	uData, err := os.ReadFile(userPath)
	if err != nil {
		return nil, fmt.Errorf("error reading %s: %w", userPath, err)
	}
	if err := yaml.Unmarshal(uData, conf); err != nil {
		return nil, err
	}

	iData, err := os.ReadFile("internalRules.yaml")
	if err != nil {
		return nil, fmt.Errorf("error reading internalRules.yaml: %w", err)
	}
	if err := yaml.Unmarshal(iData, &conf.Internal); err != nil {
		return nil, err
	}

	conf.User.Runlevels = make(map[string][]string)
	for k, v := range conf.Internal.DefaultRunlevels {
		conf.User.Runlevels[k] = v
	}

	return conf, nil
}
