package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"alpineInstaller/utils/shell"
)

func main() {
	shell.SetCaller("secureboot")
	fmt.Println("=== FeralOS — Secure Boot Setup ===")

	// --- 1. Install secureboot packages ---
	fmt.Println("\nInstalling secureboot packages...")
	packages := []string{
		"efi-mkkeys",
		"efitools",
		"secureboot-hook",
		"systemd-efistub",
		"sbctl",
	}
	pkgList := strings.Join(packages, " ")
	if _, err := shell.ExecuteCmd("", "apk add "+pkgList); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: failed to install secureboot packages: %v\n", err)
		os.Exit(1)
	}

	// --- 2. Generate EFI keys ---
	fmt.Println("\nGenerating EFI keys...")
	if _, err := shell.ExecuteCmd("", "efi-mkkeys -s 'FeralOS' -o /etc/uefi-keys"); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: efi-mkkeys failed: %v\n", err)
		os.Exit(1)
	}

	// --- 3. Ensure efivarfs is available ---
	ensureEfivarfs()

	// Check if UEFI is actually accessible — sbctl retries infinitely if not
	if _, err := os.Stat("/sys/firmware/efi"); os.IsNotExist(err) {
		fmt.Println("WARNING: /sys/firmware/efi not found — skipping Secure Boot enrollment.")
		fmt.Println("  System may not have booted with UEFI firmware.")
		cleanup()
		return
	}
	if !isMounted("/sys/firmware/efi/efivars") {
		fmt.Println("WARNING: efivarfs not mounted — skipping Secure Boot enrollment.")
		cleanup()
		return
	}

	// --- 4. Check Secure Boot state ---
	fmt.Println("\nCurrent Secure Boot status:")
	if out, err := shell.ExecuteCmd("", "sbctl status"); err != nil {
		fmt.Printf("WARNING: sbctl status failed: %v\n", err)
		_ = out
	}

	// --- 5. Import certs to sbctl ---
	importCerts()

	// --- 6. Enroll keys ---
	fmt.Println("\nEnrolling keys — requires Secure Boot in Setup Mode (no Platform Key).")
	fmt.Println("If this fails, clear Secure Boot keys in BIOS first (Custom Mode → Delete All Keys).")
	if _, err := shell.ExecuteCmd("", "sbctl enroll-keys -m"); err != nil {
		fmt.Printf("WARNING: sbctl enroll-keys failed (Setup Mode needed?): %v\n", err)
	}

	// --- 7. Verify ---
	fmt.Println("\n=== Verification ===")
	_, _ = shell.ExecuteCmd("", "sbctl status")
	fmt.Println()
	_, _ = shell.ExecuteCmd("", "sbctl verify")
	fmt.Println()
	fmt.Println("Signed files:")
	_, _ = shell.ExecuteCmd("", "sbctl list-files")

	// --- 8. Write secureboot hook config ---
	writeSecurebootConf()

	// --- 9. Overwrite booster hook with signed version ---
	writeBoosterHookSigned()

	// --- 10. Build and sign UKI ---
	buildAndSignUKI()

	// --- 11. Cleanup ---
	cleanup()

	fmt.Println("\n=== Setup Complete ===")
	fmt.Println()
	fmt.Println("IMPORTANT: Secure Boot must be enabled in BIOS after enrollment.")
	fmt.Println("  BIOS → Security → Secure Boot → Secure Boot Enable: ON")
	fmt.Println("  Secure Boot Mode: Custom → Save & Exit → Reboot")
}

func ensureEfivarfs() {
	// Check if already mounted
	if isMounted("/sys/firmware/efi/efivars") {
		return
	}

	// Try modprobe
	_, _ = shell.ExecuteCmd("", "modprobe efivarfs")

	// Try mount
	if _, err := shell.ExecuteCmd("", "mount -t efivarfs efivarfs /sys/firmware/efi/efivars"); err != nil {
		fmt.Printf("WARNING: failed to mount efivarfs: %v\n", err)
	}
}

func isMounted(path string) bool {
	data, err := os.ReadFile("/proc/mounts")
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[1] == path {
			return true
		}
	}
	return false
}

func importCerts() {
	fmt.Println("\nImporting certificates to sbctl...")

	// Create sbctl directories
	dirs := []string{
		"/var/lib/sbctl",
		"/var/lib/sbctl/keys",
		"/var/lib/sbctl/keys/PK",
		"/var/lib/sbctl/keys/KEK",
		"/var/lib/sbctl/keys/db",
	}
	for _, dir := range dirs {
		_ = os.MkdirAll(dir, 0o755)
	}

	// Generate GUID
	if _, err := os.Stat("/var/lib/sbctl/GUID"); os.IsNotExist(err) {
		uuidBytes, err := os.ReadFile("/proc/sys/kernel/random/uuid")
		if err == nil {
			uuid := strings.TrimSpace(string(uuidBytes))
			_ = os.WriteFile("/var/lib/sbctl/GUID", []byte(uuid), 0o600)
		}
	}

	// Import certs from /etc/uefi-keys/
	certs := []struct {
		name string
		crt  string
		key  string
		dir  string
	}{
		{"PK", "PK.crt", "PK.key", "/var/lib/sbctl/keys/PK"},
		{"KEK", "KEK.crt", "KEK.key", "/var/lib/sbctl/keys/KEK"},
		{"db", "db.crt", "db.key", "/var/lib/sbctl/keys/db"},
	}

	for _, cert := range certs {
		srcCrt := filepath.Join("etc", "uefi-keys", cert.crt)
		dstPem := filepath.Join(cert.dir, cert.name+".pem")
		dstKey := filepath.Join(cert.dir, cert.name+".key")

		if _, err := os.Stat(srcCrt); os.IsNotExist(err) {
			continue
		}

		// Convert CRT to PEM
		if _, err := os.Stat(dstPem); os.IsNotExist(err) {
			_, _ = shell.ExecuteCmd("", fmt.Sprintf("openssl x509 -in %s -out %s", srcCrt, dstPem))
		}

		// Copy key
		srcKey := filepath.Join("etc", "uefi-keys", cert.key)
		if _, err := os.Stat(dstKey); os.IsNotExist(err) {
			_ = copyFile(srcKey, dstKey)
		}
	}

	fmt.Println("Certs imported to /var/lib/sbctl/keys/")
}

func writeSecurebootConf() {
	fmt.Println("\nWriting secureboot hook config...")

	confContent := `cmdline="$(cat /etc/kernel/cmdline)"
signing_cert="/etc/uefi-keys/db.crt"
signing_key="/etc/uefi-keys/db.key"
output_dir="/boot/efi/Alpine"
output_name="linux-{flavor}.efi"
backup=yes
`
	// Escape single quotes in confContent for shell execution
	escapedConfContent := strings.ReplaceAll(confContent, "'", "'\\''")
	if _, err := shell.ExecuteCmd("", fmt.Sprintf("echo '%s' > /etc/kernel-hooks.d/secureboot.conf", escapedConfContent)); err != nil {
		fmt.Printf("WARNING: failed to write secureboot.conf: %v\n", err)
	}
}

// writeBoosterHookSigned overwrites 50-booster.hook with the signed version.
// This replaces the unsigned hook written during installation.
func writeBoosterHookSigned() {
	fmt.Println("\nOverwriting booster hook with signed version...")

	hookContent := `#!/bin/sh
FLAVOR="$1"
NEW_VERSION="$2"
tmpdir=$(mktemp -d)
trap 'rm -rf "${tmpdir}"' EXIT

# Build booster initramfs
booster build --force --kernel-version "${NEW_VERSION}-${FLAVOR}" "${tmpdir}/initramfs"

# Read secureboot config
signing_key="/etc/uefi-keys/db.key"
signing_cert="/etc/uefi-keys/db.crt"
output_dir="/boot/efi/Alpine"
output_name="linux-${FLAVOR}.efi"

if [ -f /etc/kernel-hooks.d/secureboot.conf ]; then
    . /etc/kernel-hooks.d/secureboot.conf
fi

# Create output directory
mkdir -p "${output_dir}"

# Build signed UKI with optional microcode
if [ -f /boot/amd-ucode.img ]; then
  efi-mkuki \
      -c "${cmdline}" \
      -o "${output_dir}/${output_name}" \
      "/boot/vmlinuz-${FLAVOR}" \
      /boot/amd-ucode.img \
      "${tmpdir}/initramfs"
elif [ -f /boot/intel-ucode.img ]; then
  efi-mkuki \
      -c "${cmdline}" \
      -o "${output_dir}/${output_name}" \
      "/boot/vmlinuz-${FLAVOR}" \
      /boot/intel-ucode.img \
      "${tmpdir}/initramfs"
else
  efi-mkuki \
      -c "${cmdline}" \
      -o "${output_dir}/${output_name}" \
      "/boot/vmlinuz-${FLAVOR}" \
      "${tmpdir}/initramfs"
fi

# Sign the UKI for Secure Boot
sbsign --key "${signing_key}" --cert "${signing_cert}" \
    --output "${output_dir}/${output_name}" \
    "${output_dir}/${output_name}"

# Backup old UKI before overwriting
if [ -f "${output_dir}/${output_name}" ]; then
    cp -af "${output_dir}/${output_name}" "${output_dir}/linux-${FLAVOR}-backup.efi"
fi

# Also create fallback boot entry
mkdir -p "${output_dir}/../EFI/BOOT"
cp -af "${output_dir}/${output_name}" "${output_dir}/../EFI/BOOT/BOOTX64.EFI"

# Mirror to secondary ESP
if [ -d /boot/efi2 ]; then
    cp -af /boot/efi/. /boot/efi2/
    mkdir -p /boot/efi2/EFI/BOOT
    cp -af "${output_dir}/${output_name}" /boot/efi2/EFI/BOOT/BOOTX64.EFI
fi
`
	// Escape single quotes in hookContent for shell execution
	escapedHookContent := strings.ReplaceAll(hookContent, "'", "'\\''")
	if _, err := shell.ExecuteCmd("", fmt.Sprintf("echo '%s' > /etc/kernel-hooks.d/50-booster.hook", escapedHookContent)); err != nil {
		fmt.Printf("WARNING: failed to write signed booster hook: %v\n", err)
	}
	// Make hook executable
	_, _ = shell.ExecuteCmd("", "chmod +x /etc/kernel-hooks.d/50-booster.hook")
}

func buildAndSignUKI() {
	fmt.Println("\nBuilding and signing UKI...")

	// Read kernel cmdline
	cmdLine, err := os.ReadFile("/etc/kernel/cmdline")
	if err != nil {
		fmt.Printf("WARNING: cannot read /etc/kernel/cmdline: %v\n", err)
		return
	}
	cmdline := strings.TrimSpace(string(cmdLine))

	// Get kernel version
	entries, err := os.ReadDir("/lib/modules")
	if err != nil || len(entries) == 0 {
		fmt.Println("WARNING: cannot find kernel modules")
		return
	}
	kernelVersion := entries[0].Name()

	// Build booster initramfs
	tmpdir, err := os.MkdirTemp("", "secureboot-*")
	if err != nil {
		fmt.Printf("WARNING: cannot create temp dir: %v\n", err)
		return
	}
	defer os.RemoveAll(tmpdir)

	initramfsPath := filepath.Join(tmpdir, "initramfs")
	fmt.Printf("Building booster initramfs for %s...\n", kernelVersion)
	if _, err := shell.ExecuteCmd("", fmt.Sprintf("booster build --force --kernel-version %s %s", kernelVersion, initramfsPath)); err != nil {
		fmt.Printf("WARNING: booster build failed: %v\n", err)
		return
	}

	// Build UKI with microcode
	ukiPath := "/boot/efi/Alpine/linux-lts.efi"
	vmlinuz := "/boot/vmlinuz-lts"

	// Detect microcode
	ucodeImg := ""
	if _, err := os.Stat("/boot/amd-ucode.img"); err == nil {
		ucodeImg = "/boot/amd-ucode.img"
	} else if _, err := os.Stat("/boot/intel-ucode.img"); err == nil {
		ucodeImg = "/boot/intel-ucode.img"
	}

	fmt.Println("Building UKI...")
	var ukiCmd string
	if ucodeImg != "" {
		ukiCmd = fmt.Sprintf("efi-mkuki -c '%s' -o %s %s %s %s", cmdline, ukiPath, vmlinuz, ucodeImg, initramfsPath)
	} else {
		ukiCmd = fmt.Sprintf("efi-mkuki -c '%s' -o %s %s %s", cmdline, ukiPath, vmlinuz, initramfsPath)
	}
	if _, err := shell.ExecuteCmd("", ukiCmd); err != nil {
		fmt.Printf("WARNING: efi-mkuki failed: %v\n", err)
		return
	}

	// Sign UKI
	fmt.Println("Signing UKI...")
	if _, err := shell.ExecuteCmd("", fmt.Sprintf("sbsign --key /etc/uefi-keys/db.key --cert /etc/uefi-keys/db.crt --output %s %s", ukiPath, ukiPath)); err != nil {
		fmt.Printf("WARNING: sbsign failed: %v\n", err)
		return
	}

	// Copy to fallback ESP
	_ = copyFile(ukiPath, "/boot/efi/EFI/BOOT/BOOTX64.EFI")

	// Mirror to secondary EFI
	_, _ = shell.ExecuteCmd("", "cp -af /boot/efi/. /boot/efi2/ 2>/dev/null || true")

	fmt.Println("UKI built and signed successfully.")
}

func cleanup() {
	fmt.Println("\nCleaning up...")
	// Remove sbctl (no longer needed after enrollment)
	_, _ = shell.ExecuteCmd("", "apk del sbctl")
	// Remove efi-mkkeys (only needed for key generation)
	_, _ = shell.ExecuteCmd("", "apk del efi-mkkeys")
	// Keep efitools (provides sbsign for kernel upgrades)
	// Keep secureboot-hook (kernel hook)
	// Keep systemd-efistub (provides efi-mkuki for kernel upgrades)
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o600)
}
