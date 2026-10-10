package internal

import (
	"feralos/src/utils/shell"
	"fmt"
	"sort"
	"strings"
)

// execSilent runs a command without logging (for firmware detection).
func execSilent(chroot string, cmd string) (string, error) {
	return shell.ExecSilent(chroot, cmd)
}

// DetectFirmwarePackages detects hardware and returns minimal firmware packages.
func DetectFirmwarePackages(chroot string) []string {
	modules := detectActiveModules(chroot)
	if len(modules) == 0 {
		return []string{"linux-firmware-none"}
	}

	firmwareFiles := extractNeededFirmware(chroot, modules)
	if len(firmwareFiles) == 0 {
		return []string{"linux-firmware-none"}
	}

	index := buildFirmwarePackageIndex(chroot)
	if len(index) == 0 {
		return []string{"linux-firmware-none"}
	}

	packages := mapFirmwareToPackages(firmwareFiles, index)
	if len(packages) == 0 {
		return []string{"linux-firmware-none"}
	}

	sort.Strings(packages)
	packages = deduplicate(packages)

	fmt.Printf("[INFO] Firmware packages: %s\n", strings.Join(packages, ", "))
	return packages
}

func detectActiveModules(chroot string) []string {
	moduleMap := make(map[string]bool)

	driverLinksCmd := "ls /sys/bus/*/devices/*/driver 2>/dev/null || true"
	if out, err := execSilent(chroot, driverLinksCmd); err == nil {
		for _, line := range strings.Split(out, "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			readlinkCmd := fmt.Sprintf("readlink %s 2>/dev/null || true", line)
			if target, err := execSilent(chroot, readlinkCmd); err == nil {
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

	busDirsCmd := "ls -d /sys/bus/*/drivers/*/ 2>/dev/null || true"
	if out, err := execSilent(chroot, busDirsCmd); err == nil {
		for _, busDir := range strings.Split(out, "\n") {
			busDir = strings.TrimSpace(busDir)
			if busDir == "" {
				continue
			}

			mod := busDir[strings.LastIndex(busDir[:len(busDir)-1], "/")+1 : len(busDir)-1]
			if mod == "" {
				continue
			}

			entriesCmd := fmt.Sprintf("ls %s 2>/dev/null || true", busDir)
			if entries, err := execSilent(chroot, entriesCmd); err == nil {
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

	modules := make([]string, 0, len(moduleMap))
	for mod := range moduleMap {
		modules = append(modules, mod)
	}
	sort.Strings(modules)
	return modules
}

func extractNeededFirmware(chroot string, modules []string) []string {
	firmwareMap := make(map[string]bool)

	for _, mod := range modules {
		cmd := fmt.Sprintf("modinfo -F firmware %s 2>/dev/null || true", mod)
		if out, err := execSilent(chroot, cmd); err == nil {
			for _, fw := range strings.Split(out, "\n") {
				fw = strings.TrimSpace(fw)
				if fw != "" {
					firmwareMap[fw] = true
				}
			}
		}
	}

	firmwareFiles := make([]string, 0, len(firmwareMap))
	for fw := range firmwareMap {
		firmwareFiles = append(firmwareFiles, fw)
	}
	sort.Strings(firmwareFiles)
	return firmwareFiles
}

func buildFirmwarePackageIndex(chroot string) map[string]string {
	index := make(map[string]string)

	_, _ = execSilent(chroot, "apk update >/dev/null 2>&1 || true")

	searchCmd := "apk search -q 'linux-firmware-*' 2>/dev/null || true"
	pkgList, err := execSilent(chroot, searchCmd)
	if err != nil {
		return index
	}

	for _, pkg := range strings.Split(pkgList, "\n") {
		pkg = strings.TrimSpace(pkg)
		if pkg == "" || strings.Contains(pkg, " ") {
			continue
		}

		fetchCmd := fmt.Sprintf("apk fetch -q --stdout %s 2>/dev/null | tar -tz 2>/dev/null | grep '^lib/firmware/' || true", pkg)
		if out, err := execSilent(chroot, fetchCmd); err == nil {
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

	packages := make([]string, 0, len(packageMap))
	for pkg := range packageMap {
		packages = append(packages, pkg)
	}
	return packages
}

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
