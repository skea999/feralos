package logic

import (
	"alpineInstaller/utils/config"
	"fmt"
)

func ApplyRules(conf *config.Config) {
	u := &conf.User
	i := &conf.Internal

	fmt.Println("[DEBUG] Starting dynamic rule application...")

	// Always include base modules (fbcon, bridge, bonding, etc.)
	u.Modules = appendUnique(u.Modules, i.BaseModules...)

	// Add zsmalloc module for zswap
	if u.EnableZswap {
		u.Modules = appendUnique(u.Modules, "zsmalloc")
	}

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

	u.AllPackages = appendUnique(u.AllPackages, i.StandardPackages...)
	u.AllPackages = appendUnique(u.AllPackages, u.ExtraPackages...)

	// If any rule requested Nix packages, enable Nix automatically.
	// Same behavior as apk: data in internal_rules selects the install path.
	if len(u.NixPackages) > 0 && !u.EnableNix {
		fmt.Printf("[WARNING] Nix packages requested (%d), enabling Nix automatically\n", len(u.NixPackages))
		u.EnableNix = true
	}

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
	u.AllPackages = appendUnique(u.AllPackages, rule.Packages...)
	u.NixPackages = appendUnique(u.NixPackages, rule.NixPackages...)
	u.UserGroups = appendUnique(u.UserGroups, rule.Groups...)
	u.Modules = appendUnique(u.Modules, rule.Modules...)
	u.KernelCmdline = appendUnique(u.KernelCmdline, rule.KernelCmdline...)

	for svc, level := range rule.Services {
		if u.Runlevels[level] == nil {
			u.Runlevels[level] = []string{}
		}
		u.Runlevels[level] = appendUnique(u.Runlevels[level], svc)
	}
}

func appendUnique(slice []string, items ...string) []string {
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
