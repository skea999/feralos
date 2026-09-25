package config

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	"alpineInstaller/utils/swap"
)

// Per-field validators — single source of truth for both validateConfig
// (LoadConfig, installer path) and the createConfig wizard (per-field
// re-ask loop). Each validator returns nil for a valid value.

// MinPasswordLen is the minimum accepted length for user passwords.
const MinPasswordLen = 6

var (
	diskRe    = regexp.MustCompile(`^/dev/(sd[a-z]+|vd[a-z]+|nvme\d+n\d+|mmcblk\d+)$`)
	userRe    = regexp.MustCompile(`^[a-z_][a-z0-9_-]{0,31}$`)
	hostRe    = regexp.MustCompile(`^[a-zA-Z0-9-]{1,63}$`)
	tzRe      = regexp.MustCompile(`^[A-Za-z0-9/_+-]+$`)
	pkgRe     = regexp.MustCompile(`^[a-zA-Z0-9._+-]+$`)
	flatpakRe = regexp.MustCompile(`^[a-zA-Z0-9._+:/-]+$`)
	branchRe  = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)
	archRe    = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
	ifaceRe   = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)
	// shellMeta rejects characters that would break shell quoting in
	// ExecuteCmd echo/cat writes (config.go ADR-004 pattern).
	shellMeta = regexp.MustCompile("[;&|$\n'\"`]")
)

func hasShellMeta(field, value string) error {
	if shellMeta.MatchString(value) {
		return fmt.Errorf("invalid %s: contains shell metacharacters", field)
	}
	return nil
}

// ValidateDisks validates each disk path (/dev/sdX, /dev/vdX, /dev/nvmeXnX, /dev/mmcblkX).
func ValidateDisks(disks []string) error {
	for _, disk := range disks {
		if !diskRe.MatchString(disk) {
			return fmt.Errorf("invalid disk path %q: must match /dev/sdX, /dev/vdX, /dev/nvmeXnX, or /dev/mmcblkX", disk)
		}
	}
	return nil
}

// ValidateUserName validates a login name: [a-z_][a-z0-9_-]{0,31}.
func ValidateUserName(s string) error {
	if s == "" {
		return fmt.Errorf("username must not be empty")
	}
	if !userRe.MatchString(s) {
		return fmt.Errorf("invalid username %q: must be [a-z_][a-z0-9_-]{0,31}", s)
	}
	return nil
}

// ValidateHostname validates [a-zA-Z0-9-]{1,63}.
func ValidateHostname(s string) error {
	if s == "" {
		return fmt.Errorf("hostname must not be empty")
	}
	if !hostRe.MatchString(s) {
		return fmt.Errorf("invalid hostname %q: must be [a-zA-Z0-9-]{1,63}", s)
	}
	return nil
}

// ValidateTimezone validates [A-Za-z0-9/_+-]+ (e.g. Europe/Rome).
func ValidateTimezone(s string) error {
	if s == "" {
		return fmt.Errorf("timezone must not be empty")
	}
	if !tzRe.MatchString(s) {
		return fmt.Errorf("invalid timezone %q: must be [A-Za-z0-9/_+-]+", s)
	}
	return nil
}

// ValidateLocale rejects shell metacharacters (e.g. "it", "en").
func ValidateLocale(s string) error { return hasShellMeta("locale", s) }

// ValidateKeymap rejects shell metacharacters (e.g. "it", "us").
func ValidateKeymap(s string) error { return hasShellMeta("keymap", s) }

// ValidatePassword enforces no shell metacharacters + MinPasswordLen runes.
func ValidatePassword(s string) error {
	if err := hasShellMeta("password", s); err != nil {
		return err
	}
	if utf8.RuneCountInString(s) < MinPasswordLen {
		return fmt.Errorf("password too short (%d chars): minimum is %d", utf8.RuneCountInString(s), MinPasswordLen)
	}
	return nil
}

// ValidateLuksPassword rejects shell metacharacters. Empty is allowed —
// the installer falls back to the login password.
func ValidateLuksPassword(s string) error { return hasShellMeta("luksPassword", s) }

// ValidateMirror requires http:// or https:// and no shell metacharacters.
func ValidateMirror(s string) error {
	if s == "" {
		return fmt.Errorf("mirror must not be empty")
	}
	if !strings.HasPrefix(s, "http://") && !strings.HasPrefix(s, "https://") {
		return fmt.Errorf("invalid mirror %q: must start with http:// or https://", s)
	}
	return hasShellMeta("mirror", s)
}

// ValidateBranch validates [a-zA-Z0-9._-]+ (latest-stable, edge, 3.22).
func ValidateBranch(s string) error {
	if s == "" {
		return fmt.Errorf("branch must not be empty")
	}
	if !branchRe.MatchString(s) {
		return fmt.Errorf("invalid branch %q: must match [a-zA-Z0-9._-]+", s)
	}
	return nil
}

// ValidateArch validates [a-zA-Z0-9_-]+.
func ValidateArch(s string) error {
	if s == "" {
		return fmt.Errorf("arch must not be empty")
	}
	if !archRe.MatchString(s) {
		return fmt.Errorf("invalid arch %q: must match [a-zA-Z0-9_-]+", s)
	}
	return nil
}

// ValidateInterface validates the network interface name [a-zA-Z0-9._-]+.
func ValidateInterface(s string) error {
	if s == "" {
		return fmt.Errorf("network interface must not be empty")
	}
	if !ifaceRe.MatchString(s) {
		return fmt.Errorf("invalid defaultNetworkInterface %q: must match [a-zA-Z0-9._-]+", s)
	}
	return nil
}

// ValidateSSHKey rejects shell metacharacters in the authorized key line.
func ValidateSSHKey(s string) error { return hasShellMeta("sshAuthorizedKey", s) }

// ValidateExtraPackages validates each space-separated APK package name.
func ValidateExtraPackages(pkgs []string) error {
	for _, pkg := range pkgs {
		if !pkgRe.MatchString(pkg) {
			return fmt.Errorf("invalid extra package %q: must match [a-zA-Z0-9._+-]+", pkg)
		}
	}
	return nil
}

// ValidateSwapSize accepts "auto", empty, or explicit sizes (8G, 4096M).
// Delegates to utils/swap without touching /proc/meminfo.
func ValidateSwapSize(s string) error { return swap.ValidSwapSize(s) }

// ValidateEnum checks value against the allowed list (case-sensitive).
func ValidateEnum(field, value string, allowed []string) error {
	for _, a := range allowed {
		if value == a {
			return nil
		}
	}
	return fmt.Errorf("invalid %s %q: valid options are: %s", field, value, strings.Join(allowed, ", "))
}

// DiskCountForLayout returns the required number of target disks per layout.
// Mirrors the layout packages' runtime validation (hostPhase/disks/*).
func DiskCountForLayout(layout string) (int, error) {
	switch layout {
	case "btrfsStandardServer", "btrfsRaid0":
		return 2, nil
	case "btrfsStandard", "btrfsUnencrypted":
		return 1, nil
	default:
		return 0, fmt.Errorf("unknown disk layout %q", layout)
	}
}

// ValidateDisksForLayout cross-checks disk count against the layout requirement.
func ValidateDisksForLayout(layout string, disks []string) error {
	want, err := DiskCountForLayout(layout)
	if err != nil {
		return err
	}
	if len(disks) != want {
		return fmt.Errorf("%s requires exactly %d disk(s), got %d", layout, want, len(disks))
	}
	return nil
}
