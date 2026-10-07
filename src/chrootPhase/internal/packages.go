package internal

import (
	"alpineInstaller/chrootPhase/features"
	"alpineInstaller/utils/config"
	"alpineInstaller/utils/shell"
	"fmt"
	"strings"
)

// InstallDummyPackages creates dummy packages for replaced C packages.
func InstallDummyPackages(conf *config.Config) error {
	chroot := conf.Internal.ChrootDir
	dummies := conf.Internal.DummyPackages
	provides := conf.Internal.ProvidePackages
	if len(dummies) == 0 && len(provides) == 0 {
		return nil
	}

	fmt.Println("[INFO] Installing dummy packages (memory-safe replacements)...")

	for replacement, rule := range dummies {
		replaced := rule.Replace
		binName := replacement
		if rule.Bin != "" {
			binName = rule.Bin
		}
		replacementBin := "/usr/bin/" + binName

		if err := createDummySymlinkPackage(chroot, replaced, replacementBin); err != nil {
			fmt.Printf("[WARNING] Failed to create dummy %s: %v\n", replaced, err)
			continue
		}
	}

	for _, pkgName := range provides {
		if err := createProvideDummy(chroot, pkgName); err != nil {
			fmt.Printf("[WARNING] Failed to create provide dummy %s: %v\n", pkgName, err)
			continue
		}
	}

	return nil
}

// appendUnique adds items to slice if not already present.
func appendUnique(slice []string, items ...string) []string {
	for _, item := range items {
		if item == "" {
			continue
		}
		found := false
		for _, s := range slice {
			if s == item {
				found = true
				break
			}
		}
		if !found {
			slice = append(slice, item)
		}
	}
	return slice
}

func createProvideDummy(chroot, pkgName string) error {
	fmt.Printf("[INFO] Creating provide dummy: %s v999 (empty)\n", pkgName)

	apkPath := fmt.Sprintf("/tmp/provide-%s.apk", pkgName)
	_, _ = shell.ExecuteCmdArgs(chroot, "rm", "-rf", apkPath)

	if _, err := shell.ExecuteCmdArgs(chroot, "apk", "mkpkg", "--info", "name:"+pkgName, "--info", "version:999", "--info", "description:Dummy "+pkgName+" for dependency satisfaction", "--info", "arch:noarch", "--output", apkPath); err != nil {
		return fmt.Errorf("apk mkpkg failed for %s: %w", pkgName, err)
	}

	if _, err := shell.ExecuteCmdArgs(chroot, "apk", "add", "--allow-untrusted", apkPath); err != nil {
		return fmt.Errorf("failed to install provide dummy %s: %w", pkgName, err)
	}

	verify, _ := shell.ExecuteCmd(chroot, "apk info -e "+pkgName+" 2>/dev/null && echo INSTALLED || echo NOT_INSTALLED")
	if strings.Contains(verify, "INSTALLED") {
		fmt.Printf("[INFO] Provide dummy %s v999 installed\n", pkgName)
	} else {
		fmt.Printf("[WARNING] Provide dummy %s may not be properly installed\n", pkgName)
	}

	_, _ = shell.ExecuteCmdArgs(chroot, "rm", "-rf", apkPath)
	return nil
}

// createDummySymlinkPackage creates a symlink from replaced to replacement binary inside an APK package.
func createDummySymlinkPackage(chroot, replacedPkg, replacementBin string) error {
	tmpDir := fmt.Sprintf("/tmp/dummy-%s", replacedPkg)
	apkPath := fmt.Sprintf("/tmp/dummy-%s.apk", replacedPkg)
	replacedBin := fmt.Sprintf("/usr/bin/%s", replacedPkg)

	_, _ = shell.ExecuteCmdArgs(chroot, "rm", "-rf", tmpDir, apkPath)

	if _, err := shell.ExecuteCmdArgs(chroot, "mkdir", "-p", tmpDir+"/usr/bin"); err != nil {
		return fmt.Errorf("failed to create dir: %w", err)
	}

	// Create symlink inside package content
	if _, err := shell.ExecuteCmdArgs(chroot, "ln", "-s", replacementBin, tmpDir+replacedBin); err != nil {
		return fmt.Errorf("failed to create symlink %s → %s: %w", replacedBin, replacementBin, err)
	}

	desc := fmt.Sprintf("Dummy %s replaced by %s", replacedPkg, replacementBin)

	infoName := fmt.Sprintf("name:%s", replacedPkg)
	if _, err := shell.ExecuteCmdArgs(chroot, "apk", "mkpkg", "--files", tmpDir, "--info", infoName, "--info", "version:999", "--info", "description:"+desc, "--info", "arch:noarch", "--output", apkPath); err != nil {
		return fmt.Errorf("apk mkpkg failed for %s: %w", replacedPkg, err)
	}

	if _, err := shell.ExecuteCmdArgs(chroot, "apk", "add", "--allow-untrusted", apkPath); err != nil {
		return fmt.Errorf("failed to install dummy %s: %w", replacedPkg, err)
	}

	verify, _ := shell.ExecuteCmd(chroot, "apk info -e "+replacedPkg+" 2>/dev/null && echo INSTALLED || echo NOT_INSTALLED")
	if strings.Contains(verify, "INSTALLED") {
		fmt.Printf("[INFO] Dummy %s v999 installed (symlink → %s)\n", replacedPkg, replacementBin)
	} else {
		fmt.Printf("[WARNING] Dummy %s may not be properly installed\n", replacedPkg)
	}

	_, _ = shell.ExecuteCmdArgs(chroot, "rm", "-rf", tmpDir, apkPath)
	return nil
}

// SetupPackages installs all required packages inside the chroot.
func SetupPackages(conf *config.Config) error {
	shell.SetCaller("packages")
	chroot := conf.Internal.ChrootDir
	u := conf.User

	fmt.Println("[INFO] Installing packages...")

	// Ensure /lib/modules exists before apk add (flatpak and others check it)
	if _, err := shell.ExecuteCmd(chroot, "mkdir -p /lib/modules"); err != nil {
		fmt.Printf("[WARNING] Failed to create /lib/modules: %v\n", err)
	}

	// /tmp + /var/tmp FHS modes (1777) are enforced at layout time
	// (misc.SetTempDirModes — fresh subvolume roots come up 0755).

	// FeralOS repo: bootstrap ALWAYS (not dinit-gated) — desktop packages
	// (dms-shell, quickshell, cli11) also come from feralos-aports. The
	// keyring adds the signing key + repositories.d entry; the following
	// `apk update` picks the new repo up (apk-tools 3 resolves from cache).
	if err := bootstrapFeralosRepo(chroot); err != nil {
		return fmt.Errorf("feralos repo bootstrap failed: %w", err)
	}

	if _, err := shell.ExecuteCmdArgs(chroot, "apk", "update"); err != nil {
		return fmt.Errorf("apk update failed: %w", err)
	}

	if err := InstallDummyPackages(conf); err != nil {
		fmt.Printf("[WARNING] Dummy package installation failed: %v\n", err)
	}

	var allPkgs []string
	allPkgs = append(allPkgs, u.AllPackages...)

	cpuInfo, _ := shell.ExecuteCmdArgs(chroot, "cat", "/proc/cpuinfo")
	if strings.Contains(cpuInfo, "AMD") {
		allPkgs = append(allPkgs, "amd-ucode")
	} else if strings.Contains(cpuInfo, "GenuineIntel") {
		allPkgs = append(allPkgs, "intel-ucode")
	}

	firmwarePkgs := DetectFirmwarePackages(chroot)
	allPkgs = append(allPkgs, firmwarePkgs...)

	// Nix packages (owned by features/nix.go — auto-enables if rules request nixPackages)
	allPkgs = append(allPkgs, features.NixPackages(conf)...)

	// Hardening packages (owned by features/hardening.go)
	allPkgs = append(allPkgs, features.HardeningPackages(conf)...)

	// Zswap modules (owned by features/zswap.go)
	u.Modules = appendUnique(u.Modules, features.ZswapModules(conf)...)

	// Podman packages (owned by features/podman.go)
	allPkgs = append(allPkgs, features.PodmanPackages(conf)...)

	// Webmin packages (owned by features/webmin.go)
	allPkgs = append(allPkgs, features.WebminPackages(conf)...)

	// Flatpak packages (owned by features/flatpak.go)
	allPkgs = append(allPkgs, features.FlatpakPackages(conf)...)

	if len(allPkgs) > 0 {
		args := append([]string{"add"}, allPkgs...)
		if _, err := shell.ExecuteCmdArgs(chroot, "apk", args...); err != nil {
			return fmt.Errorf("failed to install packages: %w", err)
		}
	}

	// Dinit: install dinit packages (after standard packages so we can
	// detect which -openrc subpackages were pulled as dependencies)
	if u.EnableDinit {
		if err := installDinitPackages(conf); err != nil {
			return fmt.Errorf("dinit package installation failed: %w", err)
		}
	} else {
		// OpenRC: klogd is shipped as the klogd-openrc package (989 series) —
		// the vendor busybox-openrc script needs `logger` (busybox syslogd),
		// which vector replaced. No runtime file writes.
		if out, err := shell.ExecuteCmdArgs(chroot, "apk", "add", "klogd-openrc"); err != nil {
			fmt.Printf("[WARNING] klogd-openrc install failed: %v (%s)\n", err, strings.TrimSpace(out))
		}
	}

	// Final APK upgrade
	if err := FinalUpgrade(chroot); err != nil {
		fmt.Printf("[WARNING] Final upgrade failed: %v\n", err)
	}

	return nil
}

// bootstrapFeralosRepo downloads and installs feralos-keyring, which adds
// both the signing key and the repository definition to the chroot.
func bootstrapFeralosRepo(chroot string) error {
	fmt.Println("[INFO] Bootstrapping FeralOS package repository...")
	url := "https://skea999.github.io/feralos-aports/feralos-aports/x86_64/feralos-keyring-1-r0.apk"
	tmp := "/tmp/feralos-keyring.apk"
	if _, err := shell.ExecuteCmdArgs(chroot, "wget", "-q", url, "-O", tmp); err != nil {
		return fmt.Errorf("failed to download feralos-keyring: %w", err)
	}
	if _, err := shell.ExecuteCmdArgs(chroot, "apk", "add", "--allow-untrusted", tmp); err != nil {
		return fmt.Errorf("failed to install feralos-keyring: %w", err)
	}
	fmt.Println("[INFO] FeralOS repository bootstrapped.")
	return nil
}

// installDinitPackages installs the dinit init system and converts all
// installed -openrc subpackages to their -dinit equivalents.
// Core packages (dinit, dinit-chimera, getty-dinit) are installed first.
// Bases listed in dinitReplacedByChimera are skipped: their service is
// already provided by the dinit-chimera suite (see internal_rules.yaml).
// Each remaining -openrc package is converted to its -dinit equivalent
// individually, so a missing -dinit package doesn't block the others.
// NOTE: SKEA_NONINTERACTIVE makes ExecuteCmdArgs swallow the exit error, so
// "no such package" is detected from the output text, not from err.
func installDinitPackages(conf *config.Config) error {
	fmt.Println("[INFO] Installing dinit packages...")
	chroot := conf.Internal.ChrootDir

	// core packages — always installed first
	// (networking-dinit: nothing else brings eth0 up under dinit — chimera's
	// network.target is only an ordering milestone; dinit-boot-log mirrors
	// the PID1 status trail to /var/log/dinit/boot.log; klogd-dinit forwards
	// the kernel ring to /dev/log (vector kern.log) — dinit-only, the Alpine
	// OpenRC klogd needs the `logger` service vector replaced; dinit-shutdown
	// ships dinit-poweroff/reboot/halt and the finalizer the daemon execs to
	// complete any shutdown — without it dinit can never power the machine off;
	// busybox-loadkmap-dinit: console keymap at boot — the chimera
	// early-console hook reads /etc/conf.d/keymaps (paths that don't exist
	// with kbd-bkeymaps), this service consumes /etc/conf.d/loadkmap)
	corePkgs := []string{"dinit", "dinit-chimera", "getty-dinit", "networking-dinit", "dinit-boot-log", "klogd-dinit", "dinit-shutdown", "busybox-loadkmap-dinit"}
	if _, err := shell.ExecuteCmdArgs(chroot, "apk", append([]string{"add"}, corePkgs...)...); err != nil {
		return fmt.Errorf("failed to install core dinit packages %v: %w", corePkgs, err)
	}
	// SKEA_NONINTERACTIVE swallows the apk exit error — verify the core
	// actually landed, otherwise the target cannot boot (hard failure)
	for _, pkg := range corePkgs {
		out, _ := shell.ExecuteCmd(chroot, "apk info -e "+pkg)
		if strings.TrimSpace(out) == "" {
			return fmt.Errorf("core dinit package %s is not installed (apk add failed — check for dependency conflicts above)", pkg)
		}
	}

	// detect installed -openrc subpackages
	out, _ := shell.ExecuteCmd(chroot, "apk info 2>/dev/null | grep -- '-openrc'")

	replaced := make(map[string]bool, len(conf.Internal.DinitReplacedByChimera))
	for _, base := range conf.Internal.DinitReplacedByChimera {
		replaced[base] = true
	}

	// Daemons managed ELSEWHERE — no system dinit service:
	//   pipewire/pipewire-pulse/wireplumber — user dinit (niri-session
	//     wrapper starts the user manager; system instances would run as
	//     root and fight the session ones)
	//   alsa-utils (alsasound mixer state) — session audio is pipewire's
	//   pulseaudio — replaced by pipewire-pulse (user session)
	//   gnome-keyring — per-login-session daemon, never a system service
	//   wpa_supplicant — NetworkManager spawns and supervises it itself
	//   wireguard-tools — wg-quick is per-interface; WireGuard goes through
	//     NetworkManager (native support) or manual config
	//   openconnect/vpnc — VPN clients spawned by NetworkManager on demand
	sessionSkip := map[string]bool{
		"pipewire":        true,
		"pipewire-pulse":  true,
		"wireplumber":     true,
		"alsa-utils":      true,
		"pulseaudio":      true,
		"gnome-keyring":   true,
		"wpa_supplicant":  true,
		"wireguard-tools": true,
		"openconnect":     true,
		"vpnc":            true,
	}

	installed, skipped, managedSkipped := 0, 0, 0
	var missing, failed []string
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || !strings.HasSuffix(line, "-openrc") {
			continue
		}
		base := strings.TrimSuffix(line, "-openrc")

		if replaced[base] {
			skipped++
			continue
		}
		if sessionSkip[base] {
			managedSkipped++
			continue
		}

		dinitPkg := base + "-dinit"
		outAdd, _ := shell.ExecuteCmdArgs(chroot, "apk", "add", dinitPkg)
		if strings.Contains(outAdd, "no such package") {
			missing = append(missing, dinitPkg)
			continue
		}
		if outAdd == "" {
			// empty output with a swallowed error = unexpected failure
			failed = append(failed, dinitPkg)
			continue
		}
		installed++
	}

	if skipped > 0 {
		fmt.Printf("[INFO] Dinit conversion: %d -openrc packages covered by dinit-chimera (not converted)\n", skipped)
	}
	if managedSkipped > 0 {
		fmt.Printf("[INFO] Dinit conversion: %d session/NM-managed services skipped (pipewire stack, wpa_supplicant, wireguard-tools, alsa-utils, pulseaudio, gnome-keyring, openconnect, vpnc)\n", managedSkipped)
	}
	if len(missing) > 0 {
		fmt.Printf("[WARNING] %d -dinit packages missing from repo (create them):\n", len(missing))
		for _, m := range missing {
			fmt.Printf("  - %s\n", m)
		}
	}
	if len(failed) > 0 {
		fmt.Printf("[WARNING] %d -dinit packages failed to install:\n", len(failed))
		for _, f := range failed {
			fmt.Printf("  - %s\n", f)
		}
	}
	fmt.Printf("[INFO] Dinit conversion: %d installed, %d replaced-by-chimera, %d missing, %d failed\n",
		installed, skipped, len(missing), len(failed))
	return nil
}

// FinalUpgrade runs apk update && apk upgrade as the last step.
func FinalUpgrade(chroot string) error {
	fmt.Println("[INFO] Final APK upgrade...")
	if _, err := shell.ExecuteCmd(chroot, "apk update && apk upgrade"); err != nil {
		fmt.Printf("[WARNING] Final apk upgrade failed: %v\n", err)
	}

	return nil
}
