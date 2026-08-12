package config

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
	Keymap             string   `yaml:"keymap"`
	Mirror             string   `yaml:"mirror"`
	Arch               string   `yaml:"arch"`
	Branch             string   `yaml:"branch"`
	ExtraPackages      []string `yaml:"extraPackages"`
	DiskLayout         string   `yaml:"diskLayout"`
	DesktopEnvironment string   `yaml:"desktopEnvironment"`
	VideoDriver        string   `yaml:"videoDriver"`
	SshAuthorizedKey   string   `yaml:"sshAuthorizedKey"`

	// Feature toggles
	EnableEncryption  bool `yaml:"enableEncryption"`
	EnableSecureboot  bool `yaml:"enableSecureboot"`
	EnableTPM         bool `yaml:"enableTPM"`
	EnableHardening   bool `yaml:"enableHardening"`
	EnableZswap       bool `yaml:"enableZswap"`
	EnableNix         bool `yaml:"enableNix"`
	EnableDebug       bool `yaml:"enableDebug"`

	// Network
	DefaultNetworkInterface string `yaml:"defaultNetworkInterface"`

	// Runtime fields
	UserGroups    []string
	Runlevels     map[string][]string
	AllPackages   []string
	Modules       []string
	KernelCmdline []string
}

type InternalRules struct {
	ChrootDir           string                         `yaml:"chrootDir"`
	StandardPackages    []string                       `yaml:"standardPackages"`
	DefaultRunlevels    map[string][]string            `yaml:"defaultRunlevels"`
	DiskLayouts         map[string]DynamicPackageRules `yaml:"diskLayouts"`
	DesktopEnvironments map[string]DynamicPackageRules `yaml:"desktopEnvironments"`
	VideoDrivers        map[string]DynamicPackageRules `yaml:"videoDrivers"`
	PackageLogic        map[string]DynamicPackageRules `yaml:"packageLogic"`
	DummyPackages       map[string]DummyPackageRule    `yaml:"dummyPackages"`
	ProvidePackages     []string                       `yaml:"providePackages"`
	BaseModules         []string                       `yaml:"baseModules"`
}

// DummyPackageRule defines a package to replace with a memory-safe dummy.
// If the key (replacement) is installed, creates dummy for replaced package.
// Binary path is inferred as /usr/bin/<replaced>.
type DummyPackageRule struct {
	Replace string `yaml:"replace"` // package name to replace
}

// Ensure the field is named "Packages" to match logic.go
type DynamicPackageRules struct {
	Packages      []string          `yaml:"packages"`
	Groups        []string          `yaml:"groups"`
	Services      map[string]string `yaml:"services"`
	Modules       []string          `yaml:"modules"`
	KernelCmdline []string          `yaml:"kernelCmdline"`
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

	// Look for internalRules.yaml in src/variables/ directory
	// This file contains the dynamic rules and is not user-editable
	iPaths := []string{
		"src/variables/internalRules.yaml",  // From project root
		"variables/internalRules.yaml",       // From src/
		"../variables/internalRules.yaml",    // From src/utils/
	}
	
	var iData []byte
	for _, p := range iPaths {
		if data, err := os.ReadFile(p); err == nil {
			iData = data
			break
		}
	}
	
	if iData == nil {
		return nil, fmt.Errorf("cannot find internalRules.yaml in any expected location")
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

// LoadTemplate loads only the user variables from a template file.
// Used by the generator to populate defaults without needing internalRules.yaml.
func LoadTemplate(templatePath string) (*Config, error) {
	conf := &Config{}
	uData, err := os.ReadFile(templatePath)
	if err != nil {
		return nil, fmt.Errorf("error reading %s: %w", templatePath, err)
	}
	if err := yaml.Unmarshal(uData, conf); err != nil {
		return nil, err
	}
	return conf, nil
}
