package logic

import (
	"reflect"
	"sort"
	"testing"

	"feralos/src/utils/config"
)

// unknownKey = rule-map key guaranteed absent from every fixture map.
const unknownKey = "bogus"

// equalSetKey = ...
// equalSets compares []string ignoring order (rule.Services / DinitServices
// are Go maps: ApplyRules reaches those slices through map iteration, so the
// VERBATIM order inside one runlevel is nondeterministic — membership is
// what the mechanism guarantees).
func equalSets(a, b []string) bool {
	x := append([]string(nil), a...)
	y := append([]string(nil), b...)
	sort.Strings(x)
	sort.Strings(y)
	return reflect.DeepEqual(x, y)
}

// ─── AppendUnique ────────────────────────────────────────────────────────────

func TestAppendUnique(t *testing.T) {
	t.Parallel()

	t.Run("appends missing items", func(t *testing.T) {
		t.Parallel()
		got := AppendUnique([]string{"a"}, "b", "c")
		want := []string{"a", "b", "c"}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})

	t.Run("skips empty items", func(t *testing.T) {
		t.Parallel()
		got := AppendUnique([]string{"a"}, "", "b", "")
		want := []string{"a", "b"}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})

	t.Run("all items empty leaves slice unchanged", func(t *testing.T) {
		t.Parallel()
		got := AppendUnique([]string{"a"}, "", "")
		want := []string{"a"}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})

	t.Run("duplicate item kept once", func(t *testing.T) {
		t.Parallel()
		got := AppendUnique([]string{"a", "b"}, "b", "c", "a")
		want := []string{"a", "b", "c"}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})

	t.Run("repeated item within one call kept once", func(t *testing.T) {
		t.Parallel()
		got := AppendUnique(nil, "x", "x", "x")
		want := []string{"x"}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})

	t.Run("nil slice accepts items", func(t *testing.T) {
		t.Parallel()
		got := AppendUnique(nil, "a", "b")
		want := []string{"a", "b"}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})

	t.Run("nothing new returns equivalent content", func(t *testing.T) {
		t.Parallel()
		in := []string{"a", "b"}
		got := AppendUnique(in, "a", "b")
		if !reflect.DeepEqual(got, []string{"a", "b"}) {
			t.Errorf("got %v, want unchanged content", got)
		}
	})
}

// ─── applyRule (direct, same-package) ───────────────────────────────────────

func TestApplyRule(t *testing.T) {
	t.Run("appends all rule fields with dedup", func(t *testing.T) {
		u := &config.UserVariables{
			AllPackages:     []string{"base"},
			NixPackages:     []string{"nix-base"},
			FlatpakPackages: []string{"flat-base"},
			UserGroups:      []string{"grp-base"},
			Modules:         []string{"mod-base"},
			KernelCmdline:   []string{"quiet"},
		}
		applyRule(u, &config.DynamicPackageRules{
			Packages:        []string{"base", "pkg-new"},
			NixPackages:     []string{"nix-base", "nix-new"},
			FlatpakPackages: []string{"flat-base", "flat-new"},
			Groups:          []string{"grp-base", "grp-new"},
			Modules:         []string{"mod-base", "mod-new"},
			KernelCmdline:   []string{"quiet", "console=ttyS0"},
		})
		checkField := func(label string, got, want []string) {
			if !reflect.DeepEqual(got, want) {
				t.Errorf("%s: got %v, want %v", label, got, want)
			}
		}
		checkField("AllPackages", u.AllPackages, []string{"base", "pkg-new"})
		checkField("NixPackages", u.NixPackages, []string{"nix-base", "nix-new"})
		checkField("FlatpakPackages", u.FlatpakPackages, []string{"flat-base", "flat-new"})
		checkField("UserGroups", u.UserGroups, []string{"grp-base", "grp-new"})
		checkField("Modules", u.Modules, []string{"mod-base", "mod-new"})
		checkField("KernelCmdline", u.KernelCmdline, []string{"quiet", "console=ttyS0"})
	})

	t.Run("creates missing runlevel inside pre-initialized maps", func(t *testing.T) {
		// CONTRACT: Runlevels/RunlevelsDinit must be pre-initialized (empty
		// map suffices) — LoadConfig does (config.go Runlevels = make(...)).
		// applyRule assigns INTO the maps; a nil map panics. Levels missing
		// from the rules are created on demand.
		u := &config.UserVariables{
			Runlevels:      map[string][]string{},
			RunlevelsDinit: map[string][]string{},
		}
		applyRule(u, &config.DynamicPackageRules{
			Services: map[string]string{"sshd": "default"},
		})
		if got, want := u.Runlevels["default"], []string{"sshd"}; !reflect.DeepEqual(got, want) {
			t.Errorf("Runlevels[default]: got %v, want %v", got, want)
		}
		if u.Runlevels["boot"] != nil {
			t.Errorf("untouched runlevel mutated: %v", u.Runlevels["boot"])
		}
	})

	t.Run("merges services into existing runlevel", func(t *testing.T) {
		u := &config.UserVariables{
			Runlevels: map[string][]string{"default": {"existing"}},
		}
		applyRule(u, &config.DynamicPackageRules{
			Services: map[string]string{"sshd": "default", "duplicate": "default"},
		})
		svc := u.Runlevels["default"]
		sort.Strings(svc)
		if want := []string{"duplicate", "existing", "sshd"}; !reflect.DeepEqual(svc, want) {
			t.Errorf("Runlevels[default]: got %v, want %v", svc, want)
		}
	})

	t.Run("same service twice in one call stays single", func(t *testing.T) {
		u := &config.UserVariables{
			Runlevels:      map[string][]string{},
			RunlevelsDinit: map[string][]string{},
		}
		applyRule(u, &config.DynamicPackageRules{
			Services: map[string]string{"sshd": "default"},
		})
		applyRule(u, &config.DynamicPackageRules{
			Services: map[string]string{"sshd": "default"},
		})
		if got, want := u.Runlevels["default"], []string{"sshd"}; !reflect.DeepEqual(got, want) {
			t.Errorf("Runlevels[default]: got %v, want %v (no duplicates)", got, want)
		}
	})

	t.Run("dinit services land in RunlevelsDinit", func(t *testing.T) {
		u := &config.UserVariables{
			RunlevelsDinit: map[string][]string{},
		}
		applyRule(u, &config.DynamicPackageRules{
			DinitServices: map[string]string{"org.freedesktop.UDisks2": "default"},
		})
		if got, want := u.RunlevelsDinit["default"], []string{"org.freedesktop.UDisks2"}; !reflect.DeepEqual(got, want) {
			t.Errorf("RunlevelsDinit[default]: got %v, want %v", got, want)
		}
	})

	t.Run("dinit and openrc service maps stay separate", func(t *testing.T) {
		u := &config.UserVariables{
			Runlevels:      map[string][]string{},
			RunlevelsDinit: map[string][]string{},
		}
		applyRule(u, &config.DynamicPackageRules{
			Services:      map[string]string{"sshd": "default"},
			DinitServices: map[string]string{"org.freedesktop.UDisks2": "default"},
		})
		if got, want := u.Runlevels["default"], []string{"sshd"}; !reflect.DeepEqual(got, want) {
			t.Errorf("Runlevels[default]: got %v, want %v", got, want)
		}
		if got, want := u.RunlevelsDinit["default"], []string{"org.freedesktop.UDisks2"}; !reflect.DeepEqual(got, want) {
			t.Errorf("RunlevelsDinit[default]: got %v, want %v", got, want)
		}
	})
}

// ─── ApplyRules (full pipeline) ─────────────────────────────────────────────

// newRulesConf builds a minimal Config whose rule maps exercise every
// consumer surface of ApplyRules (mirrors the utils/config testConfig shape:
// direct struct literals, no file I/O).
func newRulesConf() *config.Config {
	c := &config.Config{}
	c.Internal.BaseModules = []string{"fbcon", "bridge"}
	c.Internal.StandardPackages = []string{"std-pkg-a", "std-pkg-b"}
	c.Internal.DiskLayouts = map[string]config.DynamicPackageRules{
		"btrfsStandard": {
			Packages:      []string{"layout-pkg"},
			Groups:        []string{"layout-grp"},
			Modules:       []string{"layout-mod"},
			KernelCmdline: []string{"rootflags=subvol=@"},
			Services:      map[string]string{"sshd": "default"},
			DinitServices: map[string]string{"org.freedesktop.UDisks2": "default"},
			NixPackages:   []string{"nix-stone"},
		},
	}
	c.Internal.DesktopEnvironments = map[string]config.DynamicPackageRules{
		"niri": {
			Packages:        []string{"layout-pkg", "de-pkg"}, // layout-pkg: duplicate append collapses
			FlatpakPackages: []string{"de-flatpak"},
			Services:        map[string]string{"sshd": "default"}, // same entry again: still single
		},
	}
	c.Internal.VideoDrivers = map[string]config.DynamicPackageRules{
		"virtio": {
			KernelCmdline: []string{"rootflags=subvol=@"}, // duplicate: collapses
			Modules:       []string{"gpu-mod"},
		},
	}
	c.Internal.PackageLogic = map[string]config.DynamicPackageRules{
		"pkg-with-rule": {
			// cascade target: pkg-child carries its own rule → fixed-point loop
			Packages: []string{"pkg-child"},
			Groups:   []string{"cascade-grp"},
		},
		"pkg-child": {
			Packages: []string{"std-pkg-a"}, // back-reference: already processed/absent, no dup
			Modules:  []string{"cascade-mod"},
		},
	}
	c.User.DiskLayout = "btrfsStandard"
	c.User.DesktopEnvironment = "niri"
	c.User.VideoDriver = "virtio"
	// Caller contract (LoadConfig pre-initializes both, config.go):
	// applyRule assigns into these maps — nil would panic.
	c.User.Runlevels = map[string][]string{}
	c.User.RunlevelsDinit = map[string][]string{}
	return c
}

func TestApplyRules(t *testing.T) {
	t.Run("applies layout, desktop and gpu rule bodies", func(t *testing.T) {
		conf := newRulesConf()
		ApplyRules(conf)
		u := &conf.User

		check := func(label string, got, want []string) {
			if !reflect.DeepEqual(got, want) {
				t.Errorf("%s: got %v, want %v", label, got, want)
			}
		}
		// Order through the deterministic (slice) paths:
		// BaseModules → layout Mod → gpu Mod; layout → de → gpu.
		// Order: BaseModules → layout → desktop → gpu → standard+extra
		// (category rule packages precede StandardPackages — logic.go applies
		// the rule bodies FIRST, then the unconditional merges).
		check("AllPackages", u.AllPackages,
			[]string{"layout-pkg", "de-pkg", "std-pkg-a", "std-pkg-b"})
		check("UserGroups", u.UserGroups, []string{"layout-grp"})
		check("FlatpakPackages", u.FlatpakPackages, []string{"de-flatpak"})
		check("Modules", u.Modules, []string{"fbcon", "bridge", "layout-mod", "gpu-mod"})
		check("KernelCmdline", u.KernelCmdline, []string{"rootflags=subvol=@"})
		check("NixPackages", u.NixPackages, []string{"nix-stone"})
		check("Services", u.Runlevels["default"], []string{"sshd"})
		check("DinitServices", u.RunlevelsDinit["default"], []string{"org.freedesktop.UDisks2"})
	})

	t.Run("unknown keys are no-ops", func(t *testing.T) {
		conf := newRulesConf()
		u := &conf.User
		u.DiskLayout = unknownKey + "-layout"
		u.DesktopEnvironment = unknownKey + "-de"
		u.VideoDriver = unknownKey
		ApplyRules(conf)

		// No rule bodies applied: only the always-merged surfaces carry data.
		wantPkgs := []string{"std-pkg-a", "std-pkg-b"}
		if !reflect.DeepEqual(u.AllPackages, wantPkgs) {
			t.Errorf("AllPackages: got %v, want %v (unknown keys must not fire)", u.AllPackages, wantPkgs)
		}
		if got := u.UserGroups; len(got) > 0 {
			t.Errorf("UserGroups: got %v, want empty (unknown layout key)", got)
		}
		if got := u.Runlevels; len(got) != 0 {
			t.Errorf("Runlevels: got %v, want empty (unknown DE key)", got)
		}
		if got := u.RunlevelsDinit; len(got) != 0 {
			t.Errorf("RunlevelsDinit: got %v, want empty (unknown layout key)", got)
		}
		if got := u.NixPackages; len(got) > 0 {
			t.Errorf("NixPackages: got %v, want empty (unknown layout key)", got)
		}
	})

	t.Run("empty rule body inserts nothing", func(t *testing.T) {
		conf := newRulesConf()
		conf.Internal.DiskLayouts["empty-layout"] = config.DynamicPackageRules{}
		conf.Internal.DesktopEnvironments["empty-de"] = config.DynamicPackageRules{}
		conf.Internal.PackageLogic["empty-pkg"] = config.DynamicPackageRules{}
		conf.User.DiskLayout = "empty-layout"
		conf.User.DesktopEnvironment = "empty-de"
		conf.User.VideoDriver = unknownKey // no GPU rule either
		conf.User.AllPackages = AppendUnique(conf.User.AllPackages, "empty-pkg")

		ApplyRules(conf)
		u := &conf.User
		// Category rules fire before the standard merge: the seeded
		// empty-pkg leads, the (empty-bodied) rules mutate nothing.
		wantPkgs := []string{"empty-pkg", "std-pkg-a", "std-pkg-b"}
		if !reflect.DeepEqual(u.AllPackages, wantPkgs) {
			t.Errorf("AllPackages: got %v, want %v (empty body = no mutation)", u.AllPackages, wantPkgs)
		}
		for label, got := range map[string][]string{
			"UserGroups":    u.UserGroups,
			"Modules":       u.Modules,
			"KernelCmdline": u.KernelCmdline,
			"NixPackages":   u.NixPackages,
		} {
			if len(got) > 0 {
				// Modules may only carry BaseModules (always-merged surface).
				if label == "Modules" {
					want := []string{"fbcon", "bridge"}
					if !reflect.DeepEqual(got, want) {
						t.Errorf("Modules: got %v, want %v", got, want)
					}
					continue
				}
				t.Errorf("%s: got %v, want empty (empty rule body)", label, got)
			}
		}
	})

	t.Run("packageLogic cascade reaches transitive packages once", func(t *testing.T) {
		conf := newRulesConf()
		u := &conf.User
		u.AllPackages = []string{"pkg-with-rule"} // seeded: ApplyRules appends, does not reset

		ApplyRules(conf)

		// Order: pre-seed → layout-pkg → de-pkg → standard → cascade
		// (pkg-with-rule processed in the fixed-point loop).
		// Cascade: pkg-with-rule → pkg-child → (std-pkg-a back-ref).
		// std-pkg-a is also in StandardPackages: appended BEFORE the loop and
		// already in the list, so the back-reference appends nothing.
		wantPkgs := []string{"pkg-with-rule", "layout-pkg", "de-pkg", "std-pkg-a", "std-pkg-b", "pkg-child"}
		if !reflect.DeepEqual(u.AllPackages, wantPkgs) {
			t.Errorf("AllPackages: got %v, want %v", u.AllPackages, wantPkgs)
		}
		if !reflect.DeepEqual(u.UserGroups, []string{"layout-grp", "cascade-grp"}) {
			t.Errorf("UserGroups: got %v, want [layout-grp cascade-grp] (layout + cascade)", u.UserGroups)
		}
		if !equalSets(u.Modules, []string{"fbcon", "bridge", "layout-mod", "gpu-mod", "cascade-mod"}) {
			t.Errorf("Modules: got %v, want base + layout/gpu/cascade (pkg-child fired once)", u.Modules)
		}
	})

	t.Run("package listed twice still applies its rule once", func(t *testing.T) {
		conf := newRulesConf()
		u := &conf.User
		// Pre-seeded duplicate: AppendUnique does NOT dedupe pre-existing
		// entries (them being duplicated is a caller artifact), but the
		// processed-map must still run pkg-with-rule's rule ONCE.
		u.ExtraPackages = []string{"pkg-with-rule"}
		u.AllPackages = []string{"pkg-with-rule", "pkg-with-rule"}

		ApplyRules(conf)

		if !reflect.DeepEqual(u.AllPackages,
			[]string{"pkg-with-rule", "pkg-with-rule", "layout-pkg", "de-pkg", "std-pkg-a", "std-pkg-b", "pkg-child"}) {
			t.Errorf("AllPackages: got %v, want pre-seeded dups + single-application cascade", u.AllPackages)
		}
		if got := u.UserGroups; !reflect.DeepEqual(got, []string{"layout-grp", "cascade-grp"}) {
			t.Errorf("UserGroups: got %v, want single cascade-grp (no double append)", got)
		}
	})

	t.Run("standard and extra packages always merged, duplicate once", func(t *testing.T) {
		conf := newRulesConf()
		u := &conf.User
		u.ExtraPackages = []string{"std-pkg-b", "extra-1"}

		ApplyRules(conf)

		wantPkgs := []string{"layout-pkg", "de-pkg", "std-pkg-a", "std-pkg-b", "extra-1"}
		if !reflect.DeepEqual(u.AllPackages, wantPkgs) {
			t.Errorf("AllPackages: got %v, want %v", u.AllPackages, wantPkgs)
		}
	})

	t.Run("base modules always merged before rule modules", func(t *testing.T) {
		conf := newRulesConf()
		ApplyRules(conf)
		if got := conf.User.Modules; len(got) < 2 || got[0] != "fbcon" || got[1] != "bridge" {
			t.Errorf("Modules: base modules must lead; got %v", got)
		}
	})
}

// TestApplyRulesUnknownLayoutKeepsEmptyRunlevels is a focused regression on
// the early-return shape: with EVERY category key unknown, no rule fires and
// the runlevel maps stay empty (keys not created).
func TestApplyRulesUnknownLayoutKeepsEmptyRunlevels(t *testing.T) {
	t.Parallel()
	conf := newRulesConf()
	u := &conf.User
	u.DiskLayout = unknownKey
	u.DesktopEnvironment = unknownKey + "-de"
	u.VideoDriver = unknownKey

	ApplyRules(conf)

	if _, ok := u.Runlevels["default"]; ok {
		t.Errorf("Runlevels[default] created without any rule firing: %v", u.Runlevels)
	}
	if _, ok := u.RunlevelsDinit["default"]; ok {
		t.Errorf("RunlevelsDinit[default] created without any rule firing: %v", u.RunlevelsDinit)
	}
}
