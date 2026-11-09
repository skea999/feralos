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
	EnableEdgeRepos    bool     `yaml:"enableEdgeRepos"`
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
	ExtraPackagesEdge  []string `yaml:"extraPackagesEdge"`
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

	// Tunables (stage-2 rules-ification): hardcoded Go data moved into the
	// rules file so the installer is tunable by editing ONLY
	// internal_rules.yaml. The mechanism stays Go.
	StandardUserGroups      []string                  `yaml:"standardUserGroups"`
	DinitSkipSessionManaged []string                  `yaml:"dinitSkipSessionManaged"`
	FeralosRepo             FeralosRepoRules          `yaml:"feralosRepo"`
	Kernel                  KernelRules               `yaml:"kernel"`
	Microcode               map[string]MicrocodeRules `yaml:"microcode"`
	EdgeRepositories        []string                  `yaml:"edgeRepositories"`
	ResolvNameservers       []string                  `yaml:"resolvNameservers"`
	MaintenanceCron         MaintenanceCronRules      `yaml:"maintenanceCron"`
	Limits                  LimitsRules               `yaml:"limits"`
	Nix                     NixRules                  `yaml:"nix"`
	BoosterConfig           BoosterRules              `yaml:"boosterConfig"`
}

// FeralosRepoRules bootstraps the FeralOS apk repository (signing key).
type FeralosRepoRules struct {
	KeyringURL string `yaml:"keyringUrl"`
}

// KernelRules selects the kernel package + UKI flavor naming.
type KernelRules struct {
	Package string `yaml:"package"` // apk package name (hooks.go EFI sync hook)
	Flavor  string `yaml:"flavor"`  // vmlinuz-<flavor>, booster-<flavor>.img, linux-<flavor>.efi
}

// MicrocodeRules is one CPU-vendor entry (amd / intel) of the microcode map.
type MicrocodeRules struct {
	Package string `yaml:"package"` // apk package installed at setup
	Image   string `yaml:"image"`   // initramfs image path included in the UKI
}

// MaintenanceCronRules carries the crontab schedules (commands are fixed in Go).
type MaintenanceCronRules struct {
	ApkAudit  string `yaml:"apkAudit"`
	Trim      string `yaml:"trim"`
	Scrub     string `yaml:"scrub"`
	Snapshots string `yaml:"snapshots"`
}

// LimitsRules are the /etc/security/limits.conf nofile values.
type LimitsRules struct {
	NoFileDefault int `yaml:"nofileDefault"`
	NoFileUser    int `yaml:"nofileUser"`
}

// NixRules pins the stable channel + flake registry ref.
type NixRules struct {
	Channel     string `yaml:"channel"`
	RegistryRef string `yaml:"registryRef"`
}

// BoosterRules tunes /etc/booster.yaml (chrootPhase/internal/boot.go).
// Network mirrors booster.yaml's own sub-block shape (network.dhcp:
// off/on — Dhcp=false renders "off").
type BoosterRules struct {
	Compression  string              `yaml:"compression"`  // initramfs compression codec
	ExtraFiles   string              `yaml:"extraFiles"`   // extra_files baked into the image
	MountTimeout string              `yaml:"mountTimeout"` // mount_timeout (booster duration, e.g. 3m)
	Network      BoosterNetworkRules `yaml:"network"`
}

// BoosterNetworkRules is booster.yaml's network sub-block.
type BoosterNetworkRules struct {
	Dhcp bool `yaml:"dhcp"`
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
	// DinitServices enables dinit units whose names differ from the OpenRC
	// service names (ADR-010: activation targets are named after their
	// well-known D-Bus name; `services` keeps the OpenRC fallback names).
	DinitServices map[string]string `yaml:"dinitServices"`
	Modules       []string          `yaml:"modules"`
	KernelCmdline []string          `yaml:"kernelCmdline"`

	// Desktop-owned tunables (consumed by chrootPhase/desktops):
	// UserServices  — user dinit boot.d infrastructure unit names, verbatim
	//                 INCLUDING the `.user` suffix (joined with spaces into
	//                 the shell loop by the niri/niriWayle setup).
	// GreeterGroups — supplementary groups the `greeter` user joins.
	UserServices  []string `yaml:"userServices"`
	GreeterGroups []string `yaml:"greeterGroups"`
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

	applyImpliedFlags(conf)

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

	// Input validation — reject shell metacharacters in fields that touch the shell.
	// Returns the partially-populated conf alongside the error: the caller
	// (main.go) feeds it to the wizard pre-filled so the values can be fixed
	// interactively instead of hard-stopping.
	if err := validateConfig(conf); err != nil {
		return conf, fmt.Errorf("input validation: %w", err)
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

// InternalRulesCandidatePaths is the search order for internal_rules.yaml,
// all relative to the process CWD (repo root: src/variables/...; src/:
// variables/...; harness/test dirs: ../variables/...). Single source for
// the probe list: LoadInternalRules, the rules guard tests and the wizard
// generator (utils/createConfig) all read through FindInternalRulesPath —
// the list exists exactly once. A FUNCTION returning a fresh slice per
// call (was an exported mutable var): callers cannot mutate a shared
// package-level slice, and no one can observe the import-time value.
func InternalRulesCandidatePaths() []string {
	return []string{
		"src/variables/internal_rules.yaml",
		"variables/internal_rules.yaml",
		"../variables/internal_rules.yaml",
	}
}

// FindInternalRulesPath returns the first candidate path that exists, or
// an error listing the probed paths.
func FindInternalRulesPath() (string, error) {
	for _, p := range InternalRulesCandidatePaths() {
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	return "", fmt.Errorf("internal_rules.yaml not found (probed: %v)", InternalRulesCandidatePaths())
}

// TemplateCandidatePaths is the search order for the variables.yaml
// template the wizard generator (utils/createConfig/createValues) starts
// from — the same CWD-relative shape as InternalRulesCandidatePaths (repo
// root, src/, src/utils/createConfig/, src/utils/createConfig/createValues/).
// The hypothetical packaged system path is gone: no packaged install
// exists and the repo paths are the only consumers. A function returning
// a fresh slice per call, mirroring
// InternalRulesCandidatePaths: the former local copy in createValues was
// the same drift surface the rules list had before its resolver.
func TemplateCandidatePaths() []string {
	return []string{
		"src/variables/variables.yaml",   // If run from project root
		"variables/variables.yaml",       // If run from src/
		"../variables/variables.yaml",    // If run from src/utils/createConfig/
		"../../variables/variables.yaml", // If run from src/utils/createConfig/createValues/
	}
}

// FindTemplatePath resolves TemplateCandidatePaths: first os.Stat-success
// wins, otherwise an error listing the probed paths (createValues Fatalf's
// with its own message — byte-identical CLI output preserved; the resolver
// only owns WHERE to look, not the CLI's failure wording).
func FindTemplatePath() (string, error) {
	for _, p := range TemplateCandidatePaths() {
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	return "", fmt.Errorf("variables.yaml template not found (probed: %v)", TemplateCandidatePaths())
}

// LoadInternalRules loads internal_rules.yaml into conf.Internal.
// Resolved via FindInternalRulesPath (single probe list, shared with the
// guard tests and the generator). Used by LoadConfig and by the generator
// (enum options for the wizard).
func LoadInternalRules(conf *Config) error {
	iPath, err := FindInternalRulesPath()
	if err != nil {
		return err
	}
	iData, err := os.ReadFile(iPath)
	if err != nil {
		return err
	}

	if err := yaml.Unmarshal(iData, &conf.Internal); err != nil {
		return err
	}
	return validateInternalRules(&conf.Internal)
}

// validateInternalRules enforces the ALWAYS-CONSUMED tunables: every install
// path reads them regardless of feature flags, so empty means a hand-edited
// rules file lost content and would silently produce a broken target (no
// DNS in resolv.conf, wrong UKI naming, zeroed limits.conf, no user groups,
// every -openrc package force-converted). Feature-gated keys are NOT
// validated here (edgeRepositories / nix / microcode are consumed only under
// enableEdgeRepos / enableNix / CPU detection — SetupEdgeRepos and SetupNix
// enforce their own contracts, and empty microcode gracefully means "no
// microcode").
func validateInternalRules(i *InternalRules) error {
	if i.Kernel.Package == "" || i.Kernel.Flavor == "" {
		return fmt.Errorf("internal_rules: kernel.package and kernel.flavor are required (always consumed by the boot chain)")
	}
	if i.FeralosRepo.KeyringURL == "" {
		return fmt.Errorf("internal_rules: feralosRepo.keyringUrl is required (repo bootstrap)")
	}
	if len(i.ResolvNameservers) == 0 {
		return fmt.Errorf("internal_rules: resolvNameservers is required (chroot resolv.conf)")
	}
	if i.MaintenanceCron.ApkAudit == "" || i.MaintenanceCron.Trim == "" || i.MaintenanceCron.Scrub == "" || i.MaintenanceCron.Snapshots == "" {
		return fmt.Errorf("internal_rules: maintenanceCron apkAudit/trim/scrub/snapshots are all required")
	}
	if i.Limits.NoFileDefault == 0 || i.Limits.NoFileUser == 0 {
		return fmt.Errorf("internal_rules: limits.nofileDefault and limits.nofileUser are required (limits.conf)")
	}
	if i.BoosterConfig.Compression == "" || i.BoosterConfig.ExtraFiles == "" || i.BoosterConfig.MountTimeout == "" {
		return fmt.Errorf("internal_rules: boosterConfig compression/extraFiles/mountTimeout are required (booster.yaml)")
	}
	if len(i.StandardUserGroups) == 0 {
		return fmt.Errorf("internal_rules: standardUserGroups is required (user supplementary groups)")
	}
	if len(i.DinitSkipSessionManaged) == 0 {
		return fmt.Errorf("internal_rules: dinitSkipSessionManaged is required (dinit conversion skips)")
	}
	return nil
}

// applyImpliedFlags derives feature flags from the values that require them,
// so a single YAML field is enough (no error asking for a second one).
// extraPackagesEdge installs from the @edge tagged repos — those only exist
// when enableEdgeRepos wrote them into /etc/apk/repositories, so the package
// list implies the flag.
func applyImpliedFlags(conf *Config) {
	u := &conf.User
	if len(u.ExtraPackagesEdge) > 0 && !u.EnableEdgeRepos {
		u.EnableEdgeRepos = true
		fmt.Println("[INFO] extraPackagesEdge set: enableEdgeRepos enabled automatically")
	}
}

// ValidateConfig re-checks a configuration (exported for main.go, which runs
// it after the wizard so invalid values never reach the disk phase).
func ValidateConfig(conf *Config) error {
	return validateConfig(conf)
}

// layoutUnencrypted is the only disk layout without a LUKS layer (the
// password policy is skipped for it in validateConfig).
const layoutUnencrypted = "btrfsUnencrypted"

// validEnumValues returns the accepted values for a rules-driven enum field:
// the internal_rules map keys (the wizard's option list) plus "none" and the
// empty zero value — both mean "feature off" (logic.ApplyRules skips absent
// map keys; desktops.Setup returns early for ""/none).
func validEnumValues(m map[string]DynamicPackageRules) []string {
	values := make([]string, 0, len(m)+2)
	values = append(values, "", "none")
	for key := range m {
		values = append(values, key)
	}
	return values
}

// validateConfig rejects invalid values in user-provided fields.
// Delegates to per-field validators in validators.go (shared with the wizard).
// A hand-edited YAML gets the same scrutiny as the wizard:
// enum fields, password policy, swap size and the required identity fields
// are all enforced at load time instead of dying mid-install.
func validateConfig(conf *Config) error {
	u := &conf.User

	// Enum fields. diskLayout must name a layout the installer switch
	// implements; desktopEnvironment/videoDriver must be internal_rules keys
	// or the synthetic off values. The rules-driven checks are guarded on a
	// non-empty map: every real path (LoadConfig, main.go, the generator)
	// loads internal_rules first — the guard only keeps validateConfig
	// usable on a bare Config{} in tests.
	if err := ValidateDiskLayout(u.DiskLayout); err != nil {
		return err
	}
	if len(conf.Internal.DesktopEnvironments) > 0 {
		if err := ValidateEnum("desktopEnvironment", u.DesktopEnvironment, validEnumValues(conf.Internal.DesktopEnvironments)); err != nil {
			return err
		}
	}
	if len(conf.Internal.VideoDrivers) > 0 {
		if err := ValidateEnum("videoDriver", u.VideoDriver, validEnumValues(conf.Internal.VideoDrivers)); err != nil {
			return err
		}
	}

	// Required non-empty (both validators reject the empty string).
	if err := ValidateUserName(u.UserName); err != nil {
		return err
	}
	if err := ValidateHostname(u.Hostname); err != nil {
		return err
	}

	// Password policy on encrypted layouts: the login password doubles as
	// the LUKS passphrase fallback, so it must meet the strength rules and
	// the effective LUKS passphrase (explicit or fallback) must be meta-free.
	// btrfsUnencrypted has no LUKS layer — skipped there.
	if u.DiskLayout != layoutUnencrypted {
		if err := ValidatePassword(u.Password); err != nil {
			return err
		}
		effectiveLuks := u.LuksPassword
		if effectiveLuks == "" {
			effectiveLuks = u.Password
		}
		if err := ValidateLuksPassword(effectiveLuks); err != nil {
			return err
		}
	}

	// Swap size syntax when swap is actually enabled (""/auto stay legal).
	if u.EnableSwap {
		if err := ValidateSwapSize(u.SwapSize); err != nil {
			return err
		}
	}

	if err := ValidateDisks(u.Disks); err != nil {
		return err
	}
	// locale/keymap via the shared validators (same meta rule — single
	// source instead of the inline loop entries).
	if err := ValidateLocale(u.Locale); err != nil {
		return err
	}
	if err := ValidateKeymap(u.Keymap); err != nil {
		return err
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
		{"password", u.Password},
		{"luksPassword", u.LuksPassword},
		{"sshAuthorizedKey", u.SshAuthorizedKey},
	} {
		if err := hasShellMeta(v.name, v.value); err != nil {
			return err
		}
	}
	if err := ValidateExtraPackages(u.ExtraPackages); err != nil {
		return err
	}
	if err := ValidateExtraPackages(u.ExtraPackagesEdge); err != nil {
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
