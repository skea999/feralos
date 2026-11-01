package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestApplyImpliedFlagsEdgeRepos(t *testing.T) {
	tests := []struct {
		name        string
		edgePkgs    []string
		edgeEnabled bool
		wantEnabled bool
	}{
		{"edge packages imply repos", []string{"mesa@edge"}, false, true},
		{"already enabled stays", []string{"mesa@edge"}, true, true},
		{"no edge packages no flag", nil, false, false},
		{"no edge packages flag untouched", nil, true, true},
		{"empty list no flag", []string{}, false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conf := &Config{}
			conf.User.ExtraPackagesEdge = tt.edgePkgs
			conf.User.EnableEdgeRepos = tt.edgeEnabled
			applyImpliedFlags(conf)
			if conf.User.EnableEdgeRepos != tt.wantEnabled {
				t.Errorf("EnableEdgeRepos = %v, want %v", conf.User.EnableEdgeRepos, tt.wantEnabled)
			}
		})
	}
}

func TestValidateConfigEdgePackagesNoRepoFlagRequired(t *testing.T) {
	// extraPackagesEdge without enableEdgeRepos must NOT be a validation
	// error anymore — applyImpliedFlags derives the flag (LoadConfig path).
	// Base fields valid (P1-1 enforces the full field set) — only the edge
	// flag interplay is under test here.
	conf := testConfig()
	conf.User.ExtraPackagesEdge = []string{"mesa@edge", "mesa-dri-gallium@edge"}
	conf.User.EnableEdgeRepos = false // simulates pre-implied state
	if err := validateConfig(conf); err != nil {
		t.Errorf("validateConfig with edge packages and no flag returned error: %v", err)
	}
}

// --- P1-1: validateConfig enforcement at load ------------------------------

// testConfig returns a fully valid config (rules maps populated with the real
// internal_rules key sets) as the base for per-field mutation tests.
func testConfig() *Config {
	c := &Config{}
	c.Internal.DesktopEnvironments = map[string]DynamicPackageRules{
		"kde": {}, "niri": {}, "niriWayle": {}, "cosmic": {},
	}
	c.Internal.VideoDrivers = map[string]DynamicPackageRules{
		"amdgpu": {}, "nvidia": {}, "intel": {}, "virtio": {},
	}
	u := &c.User
	u.DiskLayout = "btrfsStandard"
	u.Disks = []string{"/dev/sda"}
	u.DesktopEnvironment = "none"
	u.VideoDriver = ""
	u.UserName = "testuser"
	u.Password = "Str0ng1"
	u.LuksPassword = ""
	u.Hostname = "test-host"
	u.Timezone = "Europe/Rome"
	u.Locale = "it"
	u.Keymap = "it"
	u.EnableSwap = true
	u.SwapSize = "auto"
	return c
}

func TestValidateConfigDiskLayout(t *testing.T) {
	valid := []string{"btrfsStandard", "btrfsStandardServer", "btrfsRaid0", layoutUnencrypted}
	for _, layout := range valid {
		conf := testConfig()
		conf.User.DiskLayout = layout
		if err := validateConfig(conf); err != nil {
			t.Errorf("layout %q: unexpected error %v", layout, err)
		}
	}
	for _, layout := range []string{"", "lvm", "ext4standard", "BtrfsStandard"} {
		conf := testConfig()
		conf.User.DiskLayout = layout
		if err := validateConfig(conf); err == nil {
			t.Errorf("layout %q: expected error, got nil", layout)
		}
	}
}

func TestValidateConfigDesktopEnvironment(t *testing.T) {
	valid := []string{"", "none", "kde", "niri", "niriWayle", "cosmic"}
	for _, de := range valid {
		conf := testConfig()
		conf.User.DesktopEnvironment = de
		if err := validateConfig(conf); err != nil {
			t.Errorf("desktopEnvironment %q: unexpected error %v", de, err)
		}
	}
	for _, de := range []string{"gnome", "KDE", "plasma"} {
		conf := testConfig()
		conf.User.DesktopEnvironment = de
		if err := validateConfig(conf); err == nil {
			t.Errorf("desktopEnvironment %q: expected error, got nil", de)
		}
	}
}

func TestValidateConfigVideoDriver(t *testing.T) {
	valid := []string{"", "none", "virtio", "amdgpu", "nvidia", "intel"}
	for _, vd := range valid {
		conf := testConfig()
		conf.User.VideoDriver = vd
		if err := validateConfig(conf); err != nil {
			t.Errorf("videoDriver %q: unexpected error %v", vd, err)
		}
	}
	for _, vd := range []string{"vesa", "Virtio", "nouveau"} {
		conf := testConfig()
		conf.User.VideoDriver = vd
		if err := validateConfig(conf); err == nil {
			t.Errorf("videoDriver %q: expected error, got nil", vd)
		}
	}
}

func TestValidateConfigPasswordPolicy(t *testing.T) {
	t.Run("encrypted short password rejected", func(t *testing.T) {
		conf := testConfig()
		conf.User.Password = "abc"
		if err := validateConfig(conf); err == nil {
			t.Error("expected error for short password on encrypted layout")
		}
	})
	t.Run("encrypted empty password rejected", func(t *testing.T) {
		conf := testConfig()
		conf.User.Password = ""
		if err := validateConfig(conf); err == nil {
			t.Error("expected error for empty password on encrypted layout")
		}
	})
	t.Run("luks empty falls back to password", func(t *testing.T) {
		conf := testConfig()
		conf.User.LuksPassword = ""
		if err := validateConfig(conf); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
	t.Run("luks with shell meta rejected (effective value)", func(t *testing.T) {
		conf := testConfig()
		conf.User.LuksPassword = "pa$word"
		if err := validateConfig(conf); err == nil {
			t.Error("expected error for shell meta in luksPassword")
		}
	})
	t.Run("unencrypted skips password policy", func(t *testing.T) {
		conf := testConfig()
		conf.User.DiskLayout = layoutUnencrypted
		conf.User.Password = ""
		conf.User.LuksPassword = ""
		if err := validateConfig(conf); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
	t.Run("unencrypted short password also skipped", func(t *testing.T) {
		conf := testConfig()
		conf.User.DiskLayout = layoutUnencrypted
		conf.User.Password = "abc"
		if err := validateConfig(conf); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestValidateConfigSwapSize(t *testing.T) {
	for _, size := range []string{"", "auto", "8G", "4096M"} {
		conf := testConfig()
		conf.User.SwapSize = size
		if err := validateConfig(conf); err != nil {
			t.Errorf("swapSize %q: unexpected error %v", size, err)
		}
	}
	conf := testConfig()
	conf.User.SwapSize = "banana"
	if err := validateConfig(conf); err == nil {
		t.Error("expected error for invalid swapSize with swap enabled")
	}
	conf = testConfig()
	conf.User.SwapSize = "banana"
	conf.User.EnableSwap = false
	if err := validateConfig(conf); err != nil {
		t.Errorf("swap disabled: unexpected error %v", err)
	}
}

func TestValidateConfigRequiredFields(t *testing.T) {
	conf := testConfig()
	conf.User.UserName = ""
	if err := validateConfig(conf); err == nil {
		t.Error("expected error for empty userName")
	}
	conf = testConfig()
	conf.User.Hostname = ""
	if err := validateConfig(conf); err == nil {
		t.Error("expected error for empty hostname")
	}
	conf = testConfig()
	conf.User.Hostname = "bad_host!"
	if err := validateConfig(conf); err == nil {
		t.Error("expected error for invalid hostname charset")
	}
}

func TestValidateConfigSSHKeyWired(t *testing.T) {
	conf := testConfig()
	conf.User.SshAuthorizedKey = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5 test@example"
	if err := validateConfig(conf); err != nil {
		t.Errorf("valid ssh key: unexpected error %v", err)
	}
	conf = testConfig()
	conf.User.SshAuthorizedKey = "ssh-ed25519 AAAA $(rm -rf)"
	if err := validateConfig(conf); err == nil {
		t.Error("expected error for shell meta in sshAuthorizedKey")
	}
}

func TestValidateConfigLocaleKeymapMeta(t *testing.T) {
	conf := testConfig()
	conf.User.Locale = "it; rm -rf"
	if err := validateConfig(conf); err == nil {
		t.Error("expected error for shell meta in locale")
	}
	conf = testConfig()
	conf.User.Keymap = "us`id`"
	if err := validateConfig(conf); err == nil {
		t.Error("expected error for shell meta in keymap")
	}
}

// chdirSrc moves the test CWD to src/ (LoadConfig resolves internal_rules
// relative to it) and restores it on cleanup.
func chdirSrc(t *testing.T) {
	t.Helper()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	// package dir = <repo>/src/utils/config → <repo>/src
	if err := os.Chdir(filepath.Join("..", "..")); err != nil {
		t.Fatalf("chdir src: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })
}

// restoreEnv registers cleanup restoring an env var to its prior state
// (LoadConfig sets SKEA_IGNORE_ERRORS=1 for ignoreErrors fixtures).
func restoreEnv(t *testing.T, key string) {
	t.Helper()
	if prior, ok := os.LookupEnv(key); ok {
		t.Cleanup(func() { _ = os.Setenv(key, prior) })
		return
	}
	t.Cleanup(func() { _ = os.Unsetenv(key) })
}

func TestTemplateVariablesValidate(t *testing.T) {
	chdirSrc(t)
	conf, err := LoadConfig(filepath.Join("variables", "variables.yaml"))
	if err != nil {
		t.Fatalf("template variables.yaml invalid: %v", err)
	}
	// LuksPassword fallback still applied after validation.
	if conf.User.LuksPassword == "" {
		t.Error("luksPassword fallback not applied by LoadConfig")
	}
}

// TestAllFixturesValidate is the safety net for the new strictness: every
// harness fixture (full variables.yaml template mirror) must load AND
// validate with zero errors.
func TestAllFixturesValidate(t *testing.T) {
	chdirSrc(t)
	restoreEnv(t, "SKEA_IGNORE_ERRORS")
	files, err := filepath.Glob(filepath.Join("..", "tests", "fixtures", "variables", "*.yaml"))
	if err != nil {
		t.Fatalf("glob fixtures: %v", err)
	}
	if len(files) < 28 {
		t.Fatalf("expected the full fixture suite (>= 28 files), found %d", len(files))
	}
	for _, f := range files {
		name := filepath.Base(f)
		t.Run(name, func(t *testing.T) {
			if _, err := LoadConfig(f); err != nil {
				t.Errorf("LoadConfig = %v, want nil", err)
			}
		})
	}
}
