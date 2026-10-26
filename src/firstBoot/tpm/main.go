package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"

	"feralos/src/utils/shell"
)

// Standalone binary: duplicated consts are intentional (firstBoot standalone
// policy — each binary is self-sufficient; P3-7 const-ified within the file).
const (
	installConfigPath   = "/opt/feralos/config.yaml"
	apkRepositoriesFile = "/etc/apk/repositories"
)

// InstallConfig matches the YAML written by hostPhase/features/install_config.go
type InstallConfig struct {
	DiskLayout string   `yaml:"disk_layout"`
	Disks      []string `yaml:"disks"`
	LuksUUIDs  []string `yaml:"luks_uuids"`
	UserName   string   `yaml:"user_name"`
	Hostname   string   `yaml:"hostname"`
	Mirror     string   `yaml:"mirror"`
	Branch     string   `yaml:"branch"`
}

func loadConfig() *InstallConfig {
	cfg := &InstallConfig{}
	data, err := os.ReadFile(installConfigPath)
	if err != nil {
		// Fallback: parse kernel cmdline
		uuids := getLUKSUUIDsFromCmdline()
		cfg.LuksUUIDs = uuids
		return cfg
	}
	_ = yaml.Unmarshal(data, cfg)

	// If no LUKS UUIDs in config, fallback to kernel cmdline
	if len(cfg.LuksUUIDs) == 0 {
		cfg.LuksUUIDs = getLUKSUUIDsFromCmdline()
	}
	return cfg
}

func getLUKSUUIDsFromCmdline() []string {
	cmdline, err := os.ReadFile("/etc/kernel/cmdline")
	if err != nil {
		return nil
	}
	re := regexp.MustCompile(`rd\.luks\.name=([0-9a-f-]+)=`)
	var uuids []string
	for _, m := range re.FindAllStringSubmatch(string(cmdline), -1) {
		if len(m) >= 2 {
			uuids = append(uuids, m[1])
		}
	}
	return uuids
}

func main() {
	shell.SetCaller("tpm")
	fmt.Println("=== FeralOS — TPM Enrollment ===")

	// --- 1. Check if TPM is available ---
	if _, err := os.Stat("/dev/tpmrm0"); os.IsNotExist(err) {
		fmt.Println("Warning: TPM not found at /dev/tpmrm0")
		fmt.Println("TPM auto-unlock will not be configured.")
		return
	}
	fmt.Println("TPM detected, binding LUKS volumes...")

	// --- 2. Load config ---
	cfg := loadConfig()
	uuids := cfg.LuksUUIDs
	if len(uuids) == 0 {
		fmt.Fprintln(os.Stderr, "Error: No LUKS UUIDs found in config or kernel cmdline.")
		os.Exit(1)
	}

	// --- 3. Check if any device needs binding ---
	needsBinding := false
	for _, uuid := range uuids {
		device := findDeviceByUUID(uuid)
		if device == "" {
			continue
		}
		if !hasClevisTPM2Binding(device) {
			needsBinding = true
			break
		}
	}
	if !needsBinding {
		fmt.Println("All root LUKS devices already have Clevis TPM2 bindings.")
		return
	}

	// --- 4. Get passphrase ---
	passFile, tempFile := getPassphraseFile()
	if tempFile != "" {
		defer os.Remove(tempFile)
	}

	// --- 5. Install clevis + tpm2-tools ---
	installClevis()

	// --- 6. Bind each LUKS device to TPM ---
	bindingOK := false
	for _, uuid := range uuids {
		device := findDeviceByUUID(uuid)
		if device == "" {
			continue
		}
		if hasClevisTPM2Binding(device) {
			fmt.Printf("%s already has a Clevis TPM2 binding, skipping.\n", device)
			continue
		}

		fmt.Printf("Binding %s to Clevis TPM2...\n", device)
		if _, err := shell.ExecuteCmd("", fmt.Sprintf("clevis luks bind -y -d %s -k %s tpm2 {}", device, passFile)); err != nil {
			fmt.Fprintf(os.Stderr, "WARNING: failed to bind %s: %v\n", device, err)
		} else {
			bindingOK = true
		}
	}

	// --- 7. Remove temporary key if needed ---
	if passFile == "/etc/cryptsetup-tpm2-key" { //nolint:gosec // G101: false positive, checking file path not credential
		removeTemporaryKey(passFile, uuids)
	}

	// --- 8. Rebuild initramfs ---
	rebuildInitramfs()

	// --- 9. Rebuild signed UKI ---
	rebuildSignedUKI()

	fmt.Println("\n=== TPM binding complete ===")
	if bindingOK {
		fmt.Println("Reboot to test automatic LUKS unlock via Clevis TPM2.")
	} else {
		fmt.Println("WARNING: No LUKS devices were bound to TPM2.")
		fmt.Println("Check tpm2-tools and /dev/tpmrm0 availability.")
	}
}

func findDeviceByUUID(uuid string) string {
	out, err := exec.Command("findfs", "UUID="+uuid).CombinedOutput() //nolint:gosec // G204: UUID is from config/cmdline
	if err != nil {
		return ""
	}
	device := strings.TrimSpace(string(out))
	if _, err := os.Stat(device); err != nil {
		return ""
	}
	return device
}

func hasClevisTPM2Binding(device string) bool {
	out, err := exec.Command("clevis", "luks", "list", "-d", device).CombinedOutput()
	if err != nil {
		return false
	}
	return strings.Contains(string(out), "tpm2")
}

func getPassphraseFile() (string, string) {
	passFile := "/etc/cryptsetup-tpm2-key" //nolint:gosec // G101: false positive, this is a file path not a credential
	if _, err := os.Stat(passFile); err == nil {
		return passFile, ""
	}

	if pw := os.Getenv("SKEA_LUKS_PASSWORD"); pw != "" {
		tmpFile, err := os.CreateTemp("", "luks-pass-*.tmp")
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: failed to create temp file: %v\n", err)
			os.Exit(1)
		}
		_, _ = tmpFile.WriteString(pw)
		tmpFile.Close()
		_ = os.Chmod(tmpFile.Name(), 0o600)
		return tmpFile.Name(), tmpFile.Name()
	}

	fmt.Fprint(os.Stderr, "Enter existing LUKS passphrase: ")
	var password string
	_, _ = fmt.Scanln(&password)

	tmpFile, err := os.CreateTemp("", "luks-pass-*.tmp")
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: failed to create temp file: %v\n", err)
		os.Exit(1)
	}
	_, _ = tmpFile.WriteString(password)
	tmpFile.Close()
	_ = os.Chmod(tmpFile.Name(), 0o600)
	password = ""
	return tmpFile.Name(), tmpFile.Name()
}

func installClevis() {
	// Read mirror from apk/repositories
	repos, err := os.ReadFile(apkRepositoriesFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "WARNING: cannot read %s: %v\n", apkRepositoriesFile, err)
		return
	}

	re := regexp.MustCompile(`^https?://.+`)
	lines := strings.Split(string(repos), "\n")
	mirrorBase := ""
	for _, line := range lines {
		if re.MatchString(line) {
			mirrorBase = strings.TrimSpace(line)
			parts := strings.Split(mirrorBase, "/")
			if len(parts) >= 3 {
				mirrorBase = strings.Join(parts[:len(parts)-2], "/")
			}
			break
		}
	}

	if mirrorBase == "" {
		fmt.Fprintln(os.Stderr, "WARNING: cannot determine mirror base URL")
		return
	}

	// Add @edge repos
	edgeRepos := fmt.Sprintf("@edge %s/edge/main\n@edge %s/edge/community\n@edge %s/edge/testing",
		mirrorBase, mirrorBase, mirrorBase)

	f, err := os.OpenFile(apkRepositoriesFile, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "WARNING: cannot open repositories: %v\n", err)
		return
	}
	fmt.Fprintln(f, edgeRepos)
	f.Close()

	// Update and install
	_, _ = shell.ExecuteCmd("", "apk update")
	_, _ = shell.ExecuteCmd("", "apk add --no-cache clevis@edge tpm2-tools tpm2-tss tpm2-tss-tcti-device")

	// Remove clevis from world
	removeFromWorld("clevis@edge")
	removeEdgeRepos()
}

func removeFromWorld(pkg string) {
	data, err := os.ReadFile("/etc/apk/world")
	if err != nil {
		return
	}
	var lines []string
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) != pkg {
			lines = append(lines, line)
		}
	}
	_ = os.WriteFile("/etc/apk/world", []byte(strings.Join(lines, "\n")), 0o600)
}

func removeEdgeRepos() {
	data, err := os.ReadFile(apkRepositoriesFile)
	if err != nil {
		return
	}
	var lines []string
	for _, line := range strings.Split(string(data), "\n") {
		if !strings.HasPrefix(strings.TrimSpace(line), "@edge") {
			lines = append(lines, line)
		}
	}
	_ = os.WriteFile(apkRepositoriesFile, []byte(strings.Join(lines, "\n")), 0o600) //nolint:gosec // /etc/apk/repositories is system-readable
}

func removeTemporaryKey(passFile string, uuids []string) {
	allRemoved := true
	for _, uuid := range uuids {
		device := findDeviceByUUID(uuid)
		if device == "" {
			continue
		}
		if _, err := shell.ExecuteCmd("", fmt.Sprintf("cryptsetup luksRemoveKey --key-file %s %s", passFile, device)); err != nil {
			fmt.Fprintf(os.Stderr, "WARNING: failed to remove temporary Clevis binding key from %s: %v\n", device, err)
			allRemoved = false
		}
	}
	if allRemoved {
		os.Remove(passFile)
	}
}

func rebuildInitramfs() {
	fmt.Println("\nRebuilding initramfs...")
	entries, err := os.ReadDir("/lib/modules")
	if err != nil || len(entries) == 0 {
		fmt.Fprintln(os.Stderr, "WARNING: cannot find kernel modules")
		return
	}
	kernelVersion := entries[0].Name()
	imgPath := filepath.Join("boot", "booster-lts.img")
	if _, err := shell.ExecuteCmd("", fmt.Sprintf("booster build --force --kernel-version %s %s", kernelVersion, imgPath)); err != nil {
		fmt.Fprintf(os.Stderr, "WARNING: booster build failed: %v\n", err)
	}
}

func rebuildSignedUKI() {
	fmt.Println("Rebuilding signed UKI...")
	cmdLine, err := os.ReadFile("/etc/kernel/cmdline")
	if err != nil {
		fmt.Fprintf(os.Stderr, "WARNING: cannot read /etc/kernel/cmdline: %v\n", err)
		return
	}
	cmdline := strings.TrimSpace(string(cmdLine))
	ukiPath := "/boot/efi/Alpine/linux-lts.efi"
	vmlinuz := "/boot/vmlinuz-lts"
	boosterImg := "/boot/booster-lts.img"

	if _, err := shell.ExecuteCmd("", fmt.Sprintf("efi-mkuki -c '%s' -o %s %s %s", cmdline, ukiPath, vmlinuz, boosterImg)); err != nil {
		fmt.Fprintf(os.Stderr, "WARNING: efi-mkuki failed: %v\n", err)
		return
	}
	if _, err := shell.ExecuteCmd("", fmt.Sprintf("sbsign --key /etc/uefi-keys/db.key --cert /etc/uefi-keys/db.crt --output %s %s", ukiPath, ukiPath)); err != nil {
		fmt.Fprintf(os.Stderr, "WARNING: sbsign failed: %v\n", err)
		return
	}
	_ = copyFile(ukiPath, "/boot/efi/EFI/BOOT/BOOTX64.EFI")

	// Mirror to secondary ESP (only if /boot/efi2 exists — dual-disk layouts)
	if _, err := os.Stat("/boot/efi2"); err == nil {
		_ = copyFile(ukiPath, "/boot/efi2/EFI/BOOT/BOOTX64.EFI")
	}
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o600)
}
