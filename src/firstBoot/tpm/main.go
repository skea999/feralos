package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"alpineInstaller/utils/shell"
)

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

	// --- 2. Get LUKS UUIDs from kernel cmdline ---
	uuids := getLUKSUUIDs()
	if len(uuids) == 0 {
		fmt.Fprintln(os.Stderr, "Error: No LUKS UUIDs found in kernel cmdline.")
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

	// --- 5. Add @edge repo and install clevis ---
	installClevis()

	// --- 6. Bind each LUKS device to TPM ---
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
		// Use empty PCR policy (TPM identity only)
		if _, err := shell.ExecuteCmd("", fmt.Sprintf("clevis luks bind -y -d %s -k %s tpm2 {}", device, passFile)); err != nil {
			fmt.Fprintf(os.Stderr, "WARNING: failed to bind %s: %v\n", device, err)
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
	fmt.Println("Reboot to test automatic LUKS unlock via Clevis TPM2.")
	fmt.Println()
	fmt.Println("If TPM fails, the system will fall back to passphrase.")
}

func getLUKSUUIDs() []string {
	cmdline, err := os.ReadFile("/etc/kernel/cmdline")
	if err != nil {
		return nil
	}

	re := regexp.MustCompile(`rd\.luks\.name=([0-9a-f-]+)=`)
	matches := re.FindAllStringSubmatch(string(cmdline), -1)

	var uuids []string
	for _, match := range matches {
		if len(match) >= 2 {
			uuids = append(uuids, match[1])
		}
	}
	return uuids
}

func findDeviceByUUID(uuid string) string {
	out, err := exec.Command("findfs", "UUID="+uuid).CombinedOutput() //nolint:gosec // G204: UUID is from blkid, not user input
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

	// Check environment variable
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

	// Interactive prompt
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

	// Clear password from memory
	password = ""
	return tmpFile.Name(), tmpFile.Name()
}

func installClevis() {
	// Read mirror from apk/repositories
	repos, err := os.ReadFile("/etc/apk/repositories")
	if err != nil {
		fmt.Fprintf(os.Stderr, "WARNING: cannot read /etc/apk/repositories: %v\n", err)
		return
	}

	re := regexp.MustCompile(`^https?://.+`)
	lines := strings.Split(string(repos), "\n")
	mirrorBase := ""
	for _, line := range lines {
		if re.MatchString(line) {
			// Strip last two path components
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

	f, err := os.OpenFile("/etc/apk/repositories", os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "WARNING: cannot open repositories: %v\n", err)
		return
	}
	fmt.Fprintln(f, edgeRepos)
	f.Close()

	// Update and install
	_, _ = shell.ExecuteCmd("", "apk update")
	_, _ = shell.ExecuteCmd("", "apk add --no-cache clevis@edge")

	// Remove clevis from world
	removeFromWorld("clevis@edge")

	// Remove @edge repos
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
	data, err := os.ReadFile("/etc/apk/repositories")
	if err != nil {
		return
	}

	var lines []string
	for _, line := range strings.Split(string(data), "\n") {
		if !strings.HasPrefix(strings.TrimSpace(line), "@edge") {
			lines = append(lines, line)
		}
	}
	_ = os.WriteFile("/etc/apk/repositories", []byte(strings.Join(lines, "\n")), 0o600)
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

	// Get kernel version
	entries, err := os.ReadDir("/lib/modules")
	if err != nil || len(entries) == 0 {
		fmt.Fprintln(os.Stderr, "WARNING: cannot find kernel modules")
		return
	}
	kernelVersion := entries[0].Name()

	// Build booster initramfs
	imgPath := filepath.Join("boot", "booster-lts.img")
	if _, err := shell.ExecuteCmd("", fmt.Sprintf("booster build --force --kernel-version %s %s", kernelVersion, imgPath)); err != nil {
		fmt.Fprintf(os.Stderr, "WARNING: booster build failed: %v\n", err)
	}
}

func rebuildSignedUKI() {
	fmt.Println("Rebuilding signed UKI...")

	// Read kernel cmdline
	cmdLine, err := os.ReadFile("/etc/kernel/cmdline")
	if err != nil {
		fmt.Fprintf(os.Stderr, "WARNING: cannot read /etc/kernel/cmdline: %v\n", err)
		return
	}
	cmdline := strings.TrimSpace(string(cmdLine))

	// Build UKI
	ukiPath := "/boot/efi/Alpine/linux-lts.efi"
	vmlinuz := "/boot/vmlinuz-lts"
	boosterImg := "/boot/booster-lts.img"

	if _, err := shell.ExecuteCmd("", fmt.Sprintf("efi-mkuki -c '%s' -o %s %s %s", cmdline, ukiPath, vmlinuz, boosterImg)); err != nil {
		fmt.Fprintf(os.Stderr, "WARNING: efi-mkuki failed: %v\n", err)
		return
	}

	// Sign UKI
	if _, err := shell.ExecuteCmd("", fmt.Sprintf("sbsign --key /etc/uefi-keys/db.key --cert /etc/uefi-keys/db.crt --output %s %s", ukiPath, ukiPath)); err != nil {
		fmt.Fprintf(os.Stderr, "WARNING: sbsign failed: %v\n", err)
		return
	}

	// Copy to fallback ESP
	_ = copyFile(ukiPath, "/boot/efi/EFI/BOOT/BOOTX64.EFI")
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o600)
}
