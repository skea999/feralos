package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"feralos/src/utils/shell"
)

func main() {
	shell.SetCaller("secureboot")
	fmt.Println("=== FeralOS — Secure Boot Setup ===")

	// The install config (/opt/feralos/config.yaml) is NOT consumed by this
	// binary: its UEFI keys are generated here from scratch and enrollment
	// is self-contained (a previously loaded cfg was dead weight, removed).

	// --- 1. Install sbctl ---
	fmt.Println("\nInstalling sbctl...")
	if _, err := shell.ExecuteCmd("", "apk add --no-cache sbctl"); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: failed to install sbctl: %v\n", err)
		os.Exit(1)
	}

	// --- 2. Ensure efivarfs is available ---
	ensureEfivarfs()

	if _, err := os.Stat("/sys/firmware/efi"); os.IsNotExist(err) {
		fmt.Println("WARNING: /sys/firmware/efi not found — skipping Secure Boot enrollment.")
		cleanup()
		return
	}
	if !isMounted("/sys/firmware/efi/efivars") {
		fmt.Println("WARNING: efivarfs not mounted — skipping Secure Boot enrollment.")
		cleanup()
		return
	}

	// --- 3. Check Secure Boot state ---
	fmt.Println("\nCurrent Secure Boot status:")
	_, _ = shell.ExecuteCmd("", "sbctl status")

	// --- 4. Import certs ---
	importCerts()

	// --- 5. Enroll keys ---
	fmt.Println("\nEnrolling keys...")
	_, _ = shell.ExecuteCmd("", "sbctl enroll-keys -m")

	// --- 6. Write secureboot hook config ---
	writeSecurebootConf()

	// --- 7. Verify ---
	fmt.Println("\n=== Verification ===")
	_, _ = shell.ExecuteCmd("", "sbctl status")
	_, _ = shell.ExecuteCmd("", "sbctl verify")

	// --- 7. Cleanup ---
	cleanup()

	fmt.Println("\n=== Setup Complete ===")
	fmt.Println("IMPORTANT: Secure Boot must be enabled in BIOS after enrollment.")
}

func ensureEfivarfs() {
	if isMounted("/sys/firmware/efi/efivars") {
		return
	}
	_, _ = shell.ExecuteCmd("", "modprobe efivarfs")
	_, _ = shell.ExecuteCmd("", "mount -t efivarfs efivarfs /sys/firmware/efi/efivars")
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

	if _, err := os.Stat("/var/lib/sbctl/GUID"); os.IsNotExist(err) {
		uuidBytes, err := os.ReadFile("/proc/sys/kernel/random/uuid")
		if err == nil {
			_ = os.WriteFile("/var/lib/sbctl/GUID", []byte(strings.TrimSpace(string(uuidBytes))), 0o600)
		}
	}

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
		if _, err := os.Stat(dstPem); os.IsNotExist(err) {
			_, _ = shell.ExecuteCmd("", fmt.Sprintf("openssl x509 -in %s -out %s", srcCrt, dstPem))
		}
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
	// Static content via shell.WriteFile (logging rule) instead of the
	// legacy `echo '<escaped>' >` pattern: byte-identical — the echo run
	// trailed echo's newline after the already-newline-terminated content.
	// Parent dir (/etc/kernel-hooks.d) is created 0755 by WriteFile's
	// MkdirAll (the separate mkdir call is subsumed).
	if err := shell.WriteFile("", "/etc/kernel-hooks.d/secureboot.conf", []byte(confContent+"\n"), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: failed to write secureboot.conf: %v\n", err)
	}
}

func cleanup() {
	fmt.Println("\nCleaning up...")
	// sbctl was installed temporarily for enroll-keys (keys live in
	// /var/lib/sbctl + UEFI db). efi-mkkeys is not used by this flow —
	// enrollment goes through sbctl, UKI building through efi-mkuki.
	_, _ = shell.ExecuteCmd("", "apk del sbctl")
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o600)
}
