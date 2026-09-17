package config

import (
	"fmt"
	"os"
	"regexp"
	"strings"

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
	LuksPassword       string   `yaml:"luksPassword"`
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
	EnableSecureboot bool   `yaml:"enableSecureboot"`
	EnableTPM        bool   `yaml:"enableTPM"`
	EnableHardening  bool   `yaml:"enableHardening"`
	EnableZswap      bool   `yaml:"enableZswap"`
	EnableNix        bool   `yaml:"enableNix"`
	EnableSwap       bool   `yaml:"enableSwap"`
	SwapSize         string `yaml:"swapSize"`
	EnableDebug      bool   `yaml:"enableDebug"`
	EnablePodman     bool   `yaml:"enablePodman"`
	EnableWebmin     bool   `yaml:"enableWebmin"`
	WebminListenAll  bool   `yaml:"webminListenAll"`

	// Network
	DefaultNetworkInterface string `yaml:"defaultNetworkInterface"`

	// Runtime fields
	UserGroups      []string
	Runlevels       map[string][]string
	AllPackages     []string
	NixPackages     []string
	FlatpakPackages []string `yaml:"flatpakPackages"`
	Modules         []string
	KernelCmdline   []string
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
// If the key (replacement) is installed, creates a symlink for replaced package.
// Bin is the replacement binary name (defaults to map key if empty).
type DummyPackageRule struct {
	Replace string `yaml:"replace"`       // package name to replace
	Bin     string `yaml:"bin,omitempty"` // replacement binary name (default: map key)
}

// Ensure the field is named "Packages" to match logic.go
type DynamicPackageRules struct {
	Packages        []string          `yaml:"packages"`
	NixPackages     []string          `yaml:"nixPackages"`
	FlatpakPackages []string          `yaml:"flatpakPackages"`
	Groups          []string          `yaml:"groups"`
	Services        map[string]string `yaml:"services"`
	Modules         []string          `yaml:"modules"`
	KernelCmdline   []string          `yaml:"kernelCmdline"`
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

	// Look for internal_rules.yaml in src/variables/ directory
	// This file contains the dynamic rules and is not user-editable
	iPaths := []string{
		"src/variables/internal_rules.yaml",
		"variables/internal_rules.yaml",
		"../variables/internal_rules.yaml",
	}

	var iData []byte
	for _, p := range iPaths {
		if data, err := os.ReadFile(p); err == nil {
			iData = data
			break
		}
	}

	if iData == nil {
		return nil, fmt.Errorf("cannot find internal_rules.yaml in any expected location")
	}

	if err := yaml.Unmarshal(iData, &conf.Internal); err != nil {
		return nil, err
	}

	conf.User.Runlevels = make(map[string][]string)
	for k, v := range conf.Internal.DefaultRunlevels {
		conf.User.Runlevels[k] = v
	}

	// Input validation — reject shell metacharacters in fields that touch the shell
	if err := validateConfig(conf); err != nil {
		return nil, fmt.Errorf("input validation: %w", err)
	}

	// LuksPassword fallback: use Login password if LUKS passphrase not set
	if conf.User.LuksPassword == "" {
		conf.User.LuksPassword = conf.User.Password
	}

	return conf, nil
}

// validateConfig rejects shell metacharacters in user-provided fields.
func validateConfig(conf *Config) error {
	u := &conf.User

	diskRe := regexp.MustCompile(`^/dev/(sd[a-z]+|vd[a-z]+|nvme\d+n\d+|mmcblk\d+)$`)
	for _, disk := range u.Disks {
		if !diskRe.MatchString(disk) {
			return fmt.Errorf("invalid disk path %q: must match /dev/sdX, /dev/vdX, /dev/nvmeXnX, or /dev/mmcblkX", disk)
		}
	}

	userRe := regexp.MustCompile(`^[a-z_][a-z0-9_-]{0,31}$`)
	if u.UserName != "" && !userRe.MatchString(u.UserName) {
		return fmt.Errorf("invalid username %q: must be [a-z_][a-z0-9_-]{0,31}", u.UserName)
	}

	hostRe := regexp.MustCompile(`^[a-zA-Z0-9-]{1,63}$`)
	if u.Hostname != "" && !hostRe.MatchString(u.Hostname) {
		return fmt.Errorf("invalid hostname %q: must be [a-zA-Z0-9-]{1,63}", u.Hostname)
	}

	tzRe := regexp.MustCompile(`^[A-Za-z0-9/_+-]+$`)
	if u.Timezone != "" && !tzRe.MatchString(u.Timezone) {
		return fmt.Errorf("invalid timezone %q: must be [A-Za-z0-9/_+-]+", u.Timezone)
	}

	// Reject shell metacharacters in all shell-touching fields
	shellMeta := regexp.MustCompile(`[;&|$\` + "`" + `\n'"]`)
	for _, field := range []struct {
		name, value string
	}{
		{"userName", u.UserName},
		{"hostname", u.Hostname},
		{"timezone", u.Timezone},
		{"locale", u.Locale},
		{"keymap", u.Keymap},
		{"password", u.Password},
		{"luksPassword", u.LuksPassword},
	} {
		if shellMeta.MatchString(field.value) {
			return fmt.Errorf("invalid %s %q: contains shell metacharacters", field.name, field.value)
		}
	}

	// Validate ExtraPackages
	pkgRe := regexp.MustCompile(`^[a-zA-Z0-9._+-]+$`)
	for _, pkg := range u.ExtraPackages {
		if !pkgRe.MatchString(pkg) {
			return fmt.Errorf("invalid extra package %q: must match [a-zA-Z0-9._+-]+", pkg)
		}
	}
	for _, pkg := range u.NixPackages {
		if !pkgRe.MatchString(pkg) {
			return fmt.Errorf("invalid nix package %q: must match [a-zA-Z0-9._+-]+", pkg)
		}
	}
	flatpakRe := regexp.MustCompile(`^[a-zA-Z0-9._+:/-]+$`)
	for _, pkg := range u.FlatpakPackages {
		if !flatpakRe.MatchString(pkg) {
			return fmt.Errorf("invalid flatpak package %q: must match [a-zA-Z0-9._+:/-]+", pkg)
		}
	}

	// Validate Mirror
	if u.Mirror != "" {
		if !strings.HasPrefix(u.Mirror, "http://") && !strings.HasPrefix(u.Mirror, "https://") {
			return fmt.Errorf("invalid mirror %q: must start with http:// or https://", u.Mirror)
		}
		if shellMeta.MatchString(u.Mirror) {
			return fmt.Errorf("invalid mirror %q: contains shell metacharacters", u.Mirror)
		}
	}
	// Validate Branch / Arch (used in shell curl pipe)
	branchRe := regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)
	if u.Branch != "" && !branchRe.MatchString(u.Branch) {
		return fmt.Errorf("invalid branch %q: must match [a-zA-Z0-9._-]+", u.Branch)
	}
	archRe := regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
	if u.Arch != "" && !archRe.MatchString(u.Arch) {
		return fmt.Errorf("invalid arch %q: must match [a-zA-Z0-9_-]+", u.Arch)
	}
	// Validate DefaultNetworkInterface (used in shell echo > file)
	ifaceRe := regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)
	if u.DefaultNetworkInterface != "" && !ifaceRe.MatchString(u.DefaultNetworkInterface) {
		return fmt.Errorf("invalid defaultNetworkInterface %q: must match [a-zA-Z0-9._-]+", u.DefaultNetworkInterface)
	}

	// LuksPassword fallback: if not set, use the login password
	if conf.User.LuksPassword == "" {
		conf.User.LuksPassword = conf.User.Password
	}

	return nil
}

// LoadTemplate loads only the user variables from a template file.
// Used by the generator to populate defaults without needing internal_rules.yaml.
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
