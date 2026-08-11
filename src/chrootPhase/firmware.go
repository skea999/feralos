package chrootPhase

import (
	"alpineInstaller/utils/shell"
	"fmt"
	"sort"
	"strings"
)

// DetectFirmwarePackages detects hardware and returns minimal firmware packages.
// Instead of installing the full linux-firmware (100MB+), we install only what's needed.
// Call this BEFORE apk add — packages are added to the install list.
func DetectFirmwarePackages(chroot string) []string {
	fmt.Println("[INFO] Detecting hardware for firmware packages...")

	// Step 1: Find active hardware drivers
	modules := detectActiveModules(chroot)
	if len(modules) == 0 {
		fmt.Println("[INFO] No active hardware drivers found, installing linux-firmware-none")
		return []string{"linux-firmware-none"}
	}
	fmt.Printf("[INFO] Found %d active hardware drivers\n", len(modules))

	// Step 2: Extract required firmware files
	firmwareFiles := extractNeededFirmware(chroot, modules)
	if len(firmwareFiles) == 0 {
		fmt.Println("[INFO] No firmware files required, installing linux-firmware-none")
		return []string{"linux-firmware-none"}
	}
	fmt.Printf("[INFO] Found %d unique firmware files required\n", len(firmwareFiles))

	// Step 3: Build firmware-to-package index
	index := buildFirmwarePackageIndex(chroot)
	if len(index) == 0 {
		fmt.Println("[INFO] Could not build firmware index, installing linux-firmware-none")
		return []string{"linux-firmware-none"}
	}
	fmt.Printf("[INFO] Built firmware index with %d mappings\n", len(index))

	// Step 4: Map firmware files to packages
	packages := mapFirmwareToPackages(firmwareFiles, index)
	if len(packages) == 0 {
		fmt.Println("[INFO] No matching packages found, installing linux-firmware-none")
		return []string{"linux-firmware-none"}
	}

	// Sort and deduplicate
	sort.Strings(packages)
	packages = deduplicate(packages)

	fmt.Printf("[INFO] Will install %d firmware packages: %s\n", len(packages), strings.Join(packages, ", "))
	return packages
}

// detectActiveModules finds modules actively bound to hardware devices.
func detectActiveModules(chroot string) []string {
	moduleMap := make(map[string]bool)

	// Method 1: device driver links in /sys/bus
	// This runs inside chroot to access the target system's /sys
	driverLinksCmd := "ls /sys/bus/*/devices/*/driver 2>/dev/null || true"
	if out, err := shell.RunChrootCommand(chroot, driverLinksCmd); err == nil {
		for _, line := range strings.Split(out, "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			// Read the symlink target to get module name
			readlinkCmd := fmt.Sprintf("readlink %s 2>/dev/null || true", line)
			if target, err := shell.RunChrootCommand(chroot, readlinkCmd); err == nil {
				target = strings.TrimSpace(target)
				if target != "" {
					mod := target[strings.LastIndex(target, "/")+1:]
					if mod != "" {
						moduleMap[mod] = true
					}
				}
			}
		}
	}

	// Method 2: bus driver directories with bound devices
	busDirsCmd := "ls -d /sys/bus/*/drivers/*/ 2>/dev/null || true"
	if out, err := shell.RunChrootCommand(chroot, busDirsCmd); err == nil {
		for _, busDir := range strings.Split(out, "\n") {
			busDir = strings.TrimSpace(busDir)
			if busDir == "" {
				continue
			}

			// Get module name from directory basename
			mod := busDir[strings.LastIndex(busDir[:len(busDir)-1], "/")+1 : len(busDir)-1]
			if mod == "" {
				continue
			}

			// Check if this driver has any devices (skip metadata files)
			entriesCmd := fmt.Sprintf("ls %s 2>/dev/null || true", busDir)
			if entries, err := shell.RunChrootCommand(chroot, entriesCmd); err == nil {
				hasDevices := false
				for _, entry := range strings.Split(entries, "\n") {
					entry = strings.TrimSpace(entry)
					switch entry {
					case mod, "uevent", "module", "new_id", "bind", "unbind", "probe", "remove_id":
						continue
					default:
						if entry != "" {
							hasDevices = true
							break
						}
					}
				}
				if hasDevices {
					moduleMap[mod] = true
				}
			}
		}
	}

	// Convert map to slice
	var modules []string
	for mod := range moduleMap {
		modules = append(modules, mod)
	}
	sort.Strings(modules)
	return modules
}

// extractNeededFirmware extracts firmware file requirements from active modules.
func extractNeededFirmware(chroot string, modules []string) []string {
	firmwareMap := make(map[string]bool)

	for _, mod := range modules {
		// Use modinfo to get firmware requirements
		cmd := fmt.Sprintf("modinfo -F firmware %s 2>/dev/null || true", mod)
		if out, err := shell.RunChrootCommand(chroot, cmd); err == nil {
			for _, fw := range strings.Split(out, "\n") {
				fw = strings.TrimSpace(fw)
				if fw != "" {
					firmwareMap[fw] = true
				}
			}
		}
	}

	// Convert map to sorted slice
	var firmwareFiles []string
	for fw := range firmwareMap {
		firmwareFiles = append(firmwareFiles, fw)
	}
	sort.Strings(firmwareFiles)
	return firmwareFiles
}

// buildFirmwarePackageIndex builds a reverse index: firmware file -> package name.
// This scans the Alpine repository without installing packages.
func buildFirmwarePackageIndex(chroot string) map[string]string {
	index := make(map[string]string)

	// Update package index first
	shell.RunChrootCommand(chroot, "apk update >/dev/null 2>&1 || true")

	// Search for linux-firmware-* packages
	searchCmd := "apk search -q 'linux-firmware-*' 2>/dev/null || true"
	pkgList, err := shell.RunChrootCommand(chroot, searchCmd)
	if err != nil {
		return index
	}

	// For each package, fetch its file list and build mapping
	for _, pkg := range strings.Split(pkgList, "\n") {
		pkg = strings.TrimSpace(pkg)
		if pkg == "" || strings.Contains(pkg, " ") {
			continue
		}

		// Fetch package contents and extract firmware paths
		fetchCmd := fmt.Sprintf("apk fetch -q --stdout %s 2>/dev/null | tar -tz 2>/dev/null | grep '^lib/firmware/' || true", pkg)
		if out, err := shell.RunChrootCommand(chroot, fetchCmd); err == nil {
			for _, line := range strings.Split(out, "\n") {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, "lib/firmware/") {
					fw := strings.TrimPrefix(line, "lib/firmware/")
					if fw != "" {
						index[fw] = pkg
					}
				}
			}
		}
	}

	return index
}

// mapFirmwareToPackages maps firmware files to their corresponding Alpine packages.
func mapFirmwareToPackages(firmwareFiles []string, index map[string]string) []string {
	packageMap := make(map[string]bool)
	var unmatched []string

	for _, fw := range firmwareFiles {
		if pkg, ok := index[fw]; ok {
			packageMap[pkg] = true
		} else {
			unmatched = append(unmatched, fw)
		}
	}

	if len(unmatched) > 0 {
		fmt.Printf("[WARNING] %d firmware files have no matching package\n", len(unmatched))
	}

	// Convert map to slice
	var packages []string
	for pkg := range packageMap {
		packages = append(packages, pkg)
	}
	return packages
}

// deduplicate removes duplicate strings from a sorted slice.
func deduplicate(slice []string) []string {
	if len(slice) == 0 {
		return slice
	}
	result := []string{slice[0]}
	for i := 1; i < len(slice); i++ {
		if slice[i] != slice[i-1] {
			result = append(result, slice[i])
		}
	}
	return result
}
