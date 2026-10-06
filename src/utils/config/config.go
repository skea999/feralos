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
	IgnoreErrors       bool     `yaml:"ignoreErrors"`
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
	EnableDinit      bool   `yaml:"enableDinit"`

	// Network
	DefaultNetworkInterface string `yaml:"defaultNetworkInterface"`

	// Runtime fields
	UserGroups      []string
	Runlevels       map[string][]string
	RunlevelsDinit  map[string][]string
	AllPackages     []string
	NixPackages     []string
	FlatpakPackages []string `yaml:"flatpakPackages"`
	Modules         []string
	KernelCmdline   []string
}

type InternalRules struct {
	ChrootDir              string                         `yaml:"chrootDir"`
	StandardPackages       []string                       `yaml:"standardPackages"`
	DefaultRunlevels       map[string][]string            `yaml:"defaultRunlevels"`
	DefaultRunlevelsDinit  map[string][]string            `yaml:"defaultRunlevelsDinit"`
	DinitReplacedByChimera []string                       `yaml:"dinitReplacedByChimera"`
	DinitSkipServices      []string                       `yaml:"dinitSkipServices"`
	DiskLayouts            map[string]DynamicPackageRules `yaml:"diskLayouts"`
	DesktopEnvironments    map[string]DynamicPackageRules `yaml:"desktopEnvironments"`
	VideoDrivers           map[string]DynamicPackageRules `yaml:"videoDrivers"`
	PackageLogic           map[string]DynamicPackageRules `yaml:"packageLogic"`
	DummyPackages          map[string]DummyPackageRule    `yaml:"dummyPackages"`
	ProvidePackages        []string                       `yaml:"providePackages"`
	BaseModules            []string                       `yaml:"baseModules"`
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

	if err := LoadInternalRules(conf); err != nil {
		return nil, err
	}

	conf.User.Runlevels = make(map[string][]string)
	for k, v := range conf.Internal.DefaultRunlevels {
		conf.User.Runlevels[k] = v
	}

	conf.User.RunlevelsDinit = make(map[string][]string)
	for k, v := range conf.Internal.DefaultRunlevelsDinit {
		conf.User.RunlevelsDinit[k] = v
	}

	// Input validation — reject shell metacharacters in fields that touch the shell
	if err := validateConfig(conf); err != nil {
		return nil, fmt.Errorf("input validation: %w", err)
	}

	// LuksPassword fallback: use Login password if LUKS passphrase not set
	if conf.User.LuksPassword == "" {
		conf.User.LuksPassword = conf.User.Password
	}

	// ignoreErrors: best-effort mode — command failures are logged and the
	// installation continues (SKEA_IGNORE_ERRORS is honored by shell.ExecuteCmd).
	// unattended only skips manual input; this flag governs error handling.
	if conf.User.IgnoreErrors {
		os.Setenv("SKEA_IGNORE_ERRORS", "1")
	}

	return conf, nil
}

// LoadInternalRules loads internal_rules.yaml into conf.Internal.
// Looks in standard locations relative to the working directory.
// Used by LoadConfig and by the generator (enum options for the wizard).
func LoadInternalRules(conf *Config) error {
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
		return fmt.Errorf("cannot find internal_rules.yaml in any expected location")
	}

	return yaml.Unmarshal(iData, &conf.Internal)
}

// validateConfig rejects invalid values in user-provided fields.
// Delegates to per-field validators in validators.go (shared with the wizard).
func validateConfig(conf *Config) error {
	u := &conf.User

	if err := ValidateDisks(u.Disks); err != nil {
		return err
	}
	if u.UserName != "" {
		if err := ValidateUserName(u.UserName); err != nil {
			return err
		}
	}
	if u.Hostname != "" {
		if err := ValidateHostname(u.Hostname); err != nil {
			return err
		}
	}
	if u.Timezone != "" {
		if err := ValidateTimezone(u.Timezone); err != nil {
			return err
		}
	}
	// Reject shell metacharacters in all shell-touching fields
	for _, v := range []struct{ name, value string }{
		{"userName", u.UserName},
		{"hostname", u.Hostname},
		{"timezone", u.Timezone},
		{"locale", u.Locale},
		{"keymap", u.Keymap},
		{"password", u.Password},
		{"luksPassword", u.LuksPassword},
	} {
		if err := hasShellMeta(v.name, v.value); err != nil {
			return err
		}
	}
	if err := ValidateExtraPackages(u.ExtraPackages); err != nil {
		return err
	}

	// Runtime rule fields (populated by logic.ApplyRules, not wizard-editable)
	for _, pkg := range u.NixPackages {
		if !pkgRe.MatchString(pkg) {
			return fmt.Errorf("invalid nix package %q: must match [a-zA-Z0-9._+-]+", pkg)
		}
	}
	for _, pkg := range u.FlatpakPackages {
		if !flatpakRe.MatchString(pkg) {
			return fmt.Errorf("invalid flatpak package %q: must match [a-zA-Z0-9._+:/-]+", pkg)
		}
	}

	if u.Mirror != "" {
		if err := ValidateMirror(u.Mirror); err != nil {
			return err
		}
	}
	if u.Branch != "" {
		if err := ValidateBranch(u.Branch); err != nil {
			return err
		}
	}
	if u.Arch != "" {
		if err := ValidateArch(u.Arch); err != nil {
			return err
		}
	}
	if u.DefaultNetworkInterface != "" {
		if err := ValidateInterface(u.DefaultNetworkInterface); err != nil {
			return err
		}
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
