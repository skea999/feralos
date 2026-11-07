package logic

import (
	"feralos/src/utils/config"
	"fmt"
)

func ApplyRules(conf *config.Config) {
	u := &conf.User
	i := &conf.Internal

	fmt.Println("[DEBUG] Starting dynamic rule application...")

	// Always include base modules (fbcon, bridge, bonding, etc.)
	u.Modules = AppendUnique(u.Modules, i.BaseModules...)

	if layout, ok := i.DiskLayouts[u.DiskLayout]; ok {
		fmt.Printf("[DEBUG] Applying Disk Layout: %s\n", u.DiskLayout)
		applyRule(u, &layout)
	}

	if de, ok := i.DesktopEnvironments[u.DesktopEnvironment]; ok {
		fmt.Printf("[DEBUG] Applying Desktop: %s\n", u.DesktopEnvironment)
		applyRule(u, &de)
	}

	if gpu, ok := i.VideoDrivers[u.VideoDriver]; ok {
		fmt.Printf("[DEBUG] Applying Driver: %s\n", u.VideoDriver)
		applyRule(u, &gpu)
	}

	u.AllPackages = AppendUnique(u.AllPackages, i.StandardPackages...)
	u.AllPackages = AppendUnique(u.AllPackages, u.ExtraPackages...)

	processed := make(map[string]bool)
	for {
		added := false
		for _, pkg := range u.AllPackages {
			if !processed[pkg] {
				if rule, found := i.PackageLogic[pkg]; found {
					fmt.Printf("[DEBUG] Applying logic for package: %s\n", pkg)
					applyRule(u, &rule)
					added = true
				}
				processed[pkg] = true
			}
		}
		if !added {
			break
		}
	}
}

func applyRule(u *config.UserVariables, rule *config.DynamicPackageRules) {
	u.AllPackages = AppendUnique(u.AllPackages, rule.Packages...)
	u.NixPackages = AppendUnique(u.NixPackages, rule.NixPackages...)
	u.FlatpakPackages = AppendUnique(u.FlatpakPackages, rule.FlatpakPackages...)
	u.UserGroups = AppendUnique(u.UserGroups, rule.Groups...)
	u.Modules = AppendUnique(u.Modules, rule.Modules...)
	u.KernelCmdline = AppendUnique(u.KernelCmdline, rule.KernelCmdline...)

	for svc, level := range rule.Services {
		if u.Runlevels[level] == nil {
			u.Runlevels[level] = []string{}
		}
		u.Runlevels[level] = AppendUnique(u.Runlevels[level], svc)
	}

	// ADR-010: dinit unit names for activation targets (they differ from the
	// OpenRC fallback names whenever the unit is named after the well-known
	// D-Bus name).
	for svc, level := range rule.DinitServices {
		if u.RunlevelsDinit[level] == nil {
			u.RunlevelsDinit[level] = []string{}
		}
		u.RunlevelsDinit[level] = AppendUnique(u.RunlevelsDinit[level], svc)
	}
}

// AppendUnique adds items to slice if not already present (empty items are
// skipped). Single shared implementation — the former local copies in
// logic.go and chrootPhase/internal/packages.go were byte-identical.
func AppendUnique(slice []string, items ...string) []string {
	for _, item := range items {
		if item == "" {
			continue
		}
		exists := false
		for _, s := range slice {
			if s == item {
				exists = true
				break
			}
		}
		if !exists {
			slice = append(slice, item)
		}
	}
	return slice
}
