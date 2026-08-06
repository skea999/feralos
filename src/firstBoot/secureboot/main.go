package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"alpineInstaller/utils/shell"
)

func main() {
	shell.SetCaller("secureboot")
	fmt.Println("=== FeralOS — Secure Boot sbctl Setup ===")

	// --- 1. Install sbctl ---
	fmt.Println("\nInstalling sbctl...")
	if _, err := shell.RunCommand("apk", "add", "sbctl"); err != nil {
		fmt.Fprintf(os.Stderr, "WARNING: failed to install sbctl: %v\n", err)
	}

	// --- 2. Ensure efivarfs is available ---
	ensureEfivarfs()

	// --- 3. Check Secure Boot state ---
	fmt.Println("\nCurrent Secure Boot status:")
	if out, err := shell.RunCommand("sbctl", "status"); err != nil {
		fmt.Printf("WARNING: sbctl status failed (efivarfs not mounted?): %v\n", err)
		_ = out
	}

	// --- 4. Convert existing efi-mkkeys certs to DER and copy to ESP ---
	importCerts()

	// --- 5. Enroll keys ---
	fmt.Println("\nEnrolling keys — requires Secure Boot in Setup Mode (no Platform Key).")
	fmt.Println("If this fails, clear Secure Boot keys in BIOS first (Custom Mode → Delete All Keys).")
	if _, err := shell.RunCommand("sbctl", "enroll-keys", "-m"); err != nil {
		fmt.Printf("WARNING: sbctl enroll-keys failed (Setup Mode needed?): %v\n", err)
	}

	// --- 6. Verify ---
	fmt.Println("\n=== Verification ===")
	shell.RunCommand("sbctl", "status")
	fmt.Println()
	shell.RunCommand("sbctl", "verify")
	fmt.Println()
	fmt.Println("Signed files:")
	shell.RunCommand("sbctl", "list-files")

	// Remove sbctl
	fmt.Println("\nRemoving sbctl...")
	shell.RunCommand("apk", "del", "sbctl")

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
	shell.RunCommand("modprobe", "efivarfs")

	// Try mount
	if _, err := shell.RunCommand("mount", "-t", "efivarfs", "efivarfs", "/sys/firmware/efi/efivars"); err != nil {
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
	fmt.Println("\nCopying DER certificates to ESP...")

	// Create sbctl directories
	dirs := []string{
		"/var/lib/sbctl",
		"/var/lib/sbctl/keys",
		"/var/lib/sbctl/keys/PK",
		"/var/lib/sbctl/keys/KEK",
		"/var/lib/sbctl/keys/db",
	}
	for _, dir := range dirs {
		os.MkdirAll(dir, 0o755)
	}

	// Generate GUID
	if _, err := os.Stat("/var/lib/sbctl/GUID"); os.IsNotExist(err) {
		uuidBytes, err := os.ReadFile("/proc/sys/kernel/random/uuid")
		if err == nil {
			uuid := strings.TrimSpace(string(uuidBytes))
			os.WriteFile("/var/lib/sbctl/GUID", []byte(uuid), 0o644)
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
		srcCrt := filepath.Join("/etc/uefi-keys", cert.crt)
		dstPem := filepath.Join(cert.dir, cert.name+".pem")
		dstKey := filepath.Join(cert.dir, cert.name+".key")

		if _, err := os.Stat(srcCrt); os.IsNotExist(err) {
			continue
		}

		// Convert CRT to PEM
		if _, err := os.Stat(dstPem); os.IsNotExist(err) {
			shell.RunCommand("openssl", "x509", "-in", srcCrt, "-out", dstPem)
		}

		// Copy key
		srcKey := filepath.Join("/etc/uefi-keys", cert.key)
		if _, err := os.Stat(dstKey); os.IsNotExist(err) {
			copyFile(srcKey, dstKey)
		}
	}

	fmt.Println("Certs imported to /var/lib/sbctl/keys/")
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o600)
}

// runCommandQuiet runs a command without logging (for verification)
func runCommandQuiet(name string, args ...string) string {
	cmd := exec.Command(name, args...)
	out, _ := cmd.CombinedOutput()
	return strings.TrimSpace(string(out))
}
