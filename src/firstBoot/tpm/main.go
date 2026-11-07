package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"

	"feralos/src/utils/firstboot"
	"feralos/src/utils/shell"
)

// Standalone binary: duplicated consts are intentional (firstBoot standalone
// policy — each binary is self-sufficient; P3-7 const-ified within the file).
const (
	installConfigPath   = "/opt/feralos/config.yaml"
	apkRepositoriesFile = "/etc/apk/repositories"
	apkWorldFile        = "/etc/apk/world"
)

// InstallConfig matches the YAML written by utils/firstboot/install_config.go
type InstallConfig struct {
	DiskLayout    string   `yaml:"disk_layout"`
	Disks         []string `yaml:"disks"`
	LuksUUIDs     []string `yaml:"luks_uuids"`
	UserName      string   `yaml:"user_name"`
	Hostname      string   `yaml:"hostname"`
	Mirror        string   `yaml:"mirror"`
	Branch        string   `yaml:"branch"`
	KernelPackage string   `yaml:"kernel_package"`
	KernelFlavor  string   `yaml:"kernel_flavor"`
}

// Fallbacks for an install config written by an older build (before
// kernel_package/kernel_flavor existed). The same-build installer ALWAYS
// writes both (utils/firstboot/install_config.go fills them from the
// validated internal_rules kernel block) — these are defensive only.
// KernelPackage is carried for parity with the rules contract; the tpm
// binary derives its boot-image names from the flavor alone.
const (
	defaultKernelPackage = "linux-lts"
	defaultKernelFlavor  = "lts"
)

// kernelNames derives the (kernel package, flavor) pair from the install
// config with the defensive fallbacks above. The names mirror the canonical
// chrootPhase derivation (chrootPhase/internal/uki.go): /boot/booster-<flavor>.img,
// /boot/efi/Alpine/linux-<flavor>.efi, /boot/vmlinuz-<flavor>.
func (c *InstallConfig) kernelNames() (string, string) {
	pkg := c.KernelPackage
	if pkg == "" {
		pkg = defaultKernelPackage
	}
	flavor := c.KernelFlavor
	if flavor == "" {
		flavor = defaultKernelFlavor
	}
	return pkg, flavor
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
	if err := yaml.Unmarshal(data, cfg); err != nil {
		// A corrupt config must NOT hard-fail the first boot: the boot
		// console has no salvage path, and every consumed field has a
		// fallback (LUKS UUIDs from the kernel cmdline, kernel flavor
		// default). But the degradation must be VISIBLE — print to BOTH
		// streams (stdout: SSH/serial-captured logs; stderr: shell
		// debugging convention).
		warn := fmt.Sprintf("[WARNING] %s parse failed: %v (continuing with fallbacks)", installConfigPath, err)
		fmt.Println(warn)
		fmt.Fprintln(os.Stderr, warn)
	}

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
	// Shared parser (utils/firstboot ExtractLUKSUUIDs) — the former local
	// regex copy here was byte-identical to install_config.go's.
	return firstboot.ExtractLUKSUUIDs(string(cmdline))
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
	_, kernelFlavor := cfg.kernelNames()
	rebuildInitramfs(kernelFlavor)

	// --- 9. Rebuild signed UKI ---
	rebuildSignedUKI(kernelFlavor)

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
		// os.CreateTemp always creates 0600 (Go 1.22): no chmod needed —
		// the former by-name chmod was both redundant and racy.
		_, _ = tmpFile.WriteString(pw)
		tmpFile.Close()
		return tmpFile.Name(), tmpFile.Name()
	}

	fmt.Fprint(os.Stderr, "Enter existing LUKS passphrase: ")
	password := readLineNoEcho()

	tmpFile, err := os.CreateTemp("", "luks-pass-*.tmp")
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: failed to create temp file: %v\n", err)
		os.Exit(1)
	}
	_, _ = tmpFile.WriteString(password)
	tmpFile.Close()
	return tmpFile.Name(), tmpFile.Name()
}

// readLineNoEcho reads one full line from stdin. fmt.Scanln is unsuitable
// for a passphrase: it stops at the first space, silently truncating any
// passphrase that contains whitespace. Echo is suppressed best-effort via
// stty (the binary runs at first boot on the console; stty exists in the
// Alpine base install). If stty fails — non-tty stdin, e.g. piped or
// scripted input — the prompt continues WITH echo: the SKEA_LUKS_PASSWORD
// environment variable is the supported non-tty path, and a best-effort
// field-suppressed prompt is better than failing the first boot.
func readLineNoEcho() string {
	// stty must write/restore on the TERMINAL: stdin is passed through
	// explicitly (the exec'd process inherits no tty otherwise).
	echoOff := exec.Command("stty", "-echo")
	echoOff.Stdin = os.Stdin
	echoSuppressed := echoOff.Run() == nil
	if echoSuppressed {
		// restore echo even when the read errors — a terminal left in
		// -echo mode would be unusable for interactive repair
		defer func() {
			echoOn := exec.Command("stty", "echo")
			echoOn.Stdin = os.Stdin
			_ = echoOn.Run()
		}()
	}

	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil && line == "" {
		return ""
	}
	return strings.TrimSpace(line)
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
	// The edge base derives from the MAIN repository line (mirror/branch/
	// main → mirror base): the former first-http-line assumption breaks
	// whenever community (or any other repo) sorts or landed before main.
	// Fallback keeps a mirror alive when no line ends in /main; the
	// resulting URLs are unchanged when main is present.
	trimmed := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed = append(trimmed, strings.TrimSpace(line))
	}
	mainLine := ""
	firstHTTP := ""
	for _, line := range trimmed {
		if !re.MatchString(line) {
			continue
		}
		if strings.HasSuffix(line, "/main") && mainLine == "" {
			mainLine = line
		}
		if firstHTTP == "" {
			firstHTTP = line
		}
	}
	chosen := mainLine
	if chosen == "" {
		if firstHTTP == "" {
			fmt.Fprintln(os.Stderr, "WARNING: cannot determine mirror base URL")
			return
		}
		chosen = firstHTTP
		fmt.Fprintf(os.Stderr, "WARNING: no %s line ending in /main found; deriving the edge base from the first repository line (%s)\n", apkRepositoriesFile, chosen)
	}
	mirrorBase, mirrorOK := repoDirBase(chosen)
	if !mirrorOK {
		fmt.Fprintln(os.Stderr, "WARNING: cannot determine mirror base URL")
		return
	}

	// Add @edge repos
	edgeRepos := fmt.Sprintf("@edge %s/edge/main\n@edge %s/edge/community\n@edge %s/edge/testing",
		mirrorBase, mirrorBase, mirrorBase)

	// O_APPEND|O_WRONLY (no O_CREATE) only ever opens the file
	// repositories ALWAYS pre-exists here: written 0644 host-side by the
	// installer bootstrap (PrepareChrootEnv) and never recreated since, so
	// the 0o644 OpenFile arg is unused (open fails cleanly when absent —
	// the perm is carried for the OpenFile signature only).
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

	// Remove clevis from world.
	// apk world records TAGGED names verbatim — docker-proven on
	// alpine:3.22: with the @edge repo lines appended (the shape this
	// function creates), `apk add --no-cache clevis@edge` leaves
	// /etc/apk/world containing the literal `clevis@edge`, so an
	// untagged "clevis" match would never hit. Once removeEdgeRepos()
	// strips the @edge lines, a leftover tagged world entry references a
	// non-existent tag — remove it to keep the world consistent with the
	// stable-only repos.
	removeFromWorld("clevis@edge")
	removeEdgeRepos()
}

// repoDirBase strips the trailing <branch>/<repo> path segments of an
// Alpine repository URL (…/v3.24/main → … ; …/edge/community → …/edge)
// and reports failure for degenerate URLs (the former inline splice kept
// going silently when the split was too short).
func repoDirBase(repoURL string) (string, bool) {
	parts := strings.Split(repoURL, "/")
	if len(parts) < 3 {
		return "", false
	}
	return strings.Join(parts[:len(parts)-2], "/"), true
}

// removeFromWorld removes an exact world entry. apk world stores the
// TAGGED literal for tagged installs (`clevis@edge`, docker-proven on
// alpine:3.22) — callers must pass the tagged name; an untagged name
// never matches.
func removeFromWorld(pkg string) {
	// The world path is the apkWorldFile const (same file layout as
	// apkRepositoriesFile above) — no inline literal.
	data, err := os.ReadFile(apkWorldFile)
	if err != nil {
		return
	}
	var lines []string
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) != pkg {
			lines = append(lines, line)
		}
	}
	// 0644: docker-verified on alpine:3.22 — /etc/apk/world carries the
	// same system-readable perms as /etc/apk/repositories (apk manages
	// both); the former 0o600 arg contradicted the stock file perms
	// (inert on an existing file — WriteFile perm applies at create —
	// but wrong if the file is ever recreated).
	_ = os.WriteFile(apkWorldFile, []byte(strings.Join(lines, "\n")), 0o644) //nolint:gosec // G306: /etc/apk/world must stay system-readable like repositories
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
	// 0644: apk reads this file as any user (apk-tools stats it in
	// non-root userspace flows); the former 0600 arg contradicted its
	// own system-readable intent (inert on an existing file — WriteFile
	// perm applies at create — but wrong if the file is ever recreated).
	_ = os.WriteFile(apkRepositoriesFile, []byte(strings.Join(lines, "\n")), 0o644) //nolint:gosec // G306: /etc/apk/repositories must stay system-readable
}

func removeTemporaryKey(passFile string, uuids []string) {
	allRemoved := true
	// Best-effort key-file removal at exit, failure paths or not: a
	// per-device cryptsetup failure previously kept the 0600 passphrase
	// file on disk forever (re-runs return early in hasClevisTPM2Binding
	// once TPM2 bindings exist, so nothing else ever cleaned it). The
	// passphrase itself stays valid on every device regardless of the
	// file — deleting the copy sacrifices no boot path, and a plaintext
	// passphrase file left behind is the worse outcome.
	defer func() {
		if err := os.Remove(passFile); err != nil {
			fmt.Fprintf(os.Stderr, "WARNING: failed to remove key file %s: %v\n", passFile, err)
		}
	}()
	for _, uuid := range uuids {
		device := findDeviceByUUID(uuid)
		if device == "" {
			continue
		}
		if _, err := shell.ExecuteCmd("", fmt.Sprintf("cryptsetup luksRemoveKey --key-file %s %s", passFile, device)); err != nil {
			fmt.Fprintf(os.Stderr, "WARNING: failed to remove temporary Clevis binding key from %s: %v (key file deleted anyway; the slot may still exist)\n", device, err)
			allRemoved = false
		}
	}
	if !allRemoved {
		fmt.Fprintln(os.Stderr, "WARNING: some LUKS key-slot removals failed; verify slots with 'cryptsetup luksDump'.")
	}
}

func rebuildInitramfs(kernelFlavor string) {
	fmt.Println("\nRebuilding initramfs...")
	entries, err := os.ReadDir("/lib/modules")
	if err != nil || len(entries) == 0 {
		fmt.Fprintln(os.Stderr, "WARNING: cannot find kernel modules")
		return
	}
	// uki.go parity (chrootPhase/internal/uki.go picks `ls /lib/modules/ |
	// head -1`): os.ReadDir returns the same filename-sorted order, so the
	// lexically-first kernel version dir matches the chroot choice. The
	// entries here are version dirs (e.g. "6.12.115-0-lts") — the
	// vmlinuz-*/booster-* image names live in /boot — so filter to
	// directories as the expected shape and fail clearly instead of
	// silently taking entries[0] whatever it is.
	var kernelVersion string
	for _, entry := range entries {
		if entry.IsDir() {
			kernelVersion = entry.Name()
			break
		}
	}
	if kernelVersion == "" {
		fmt.Fprintln(os.Stderr, "WARNING: no kernel version directories under /lib/modules")
		return
	}
	// Absolute: this binary runs post-install/first-boot (harness SSH and
	// real-hw shells start in a user home, not /), so the former relative
	// "boot" join would have written the initramfs outside the real /boot
	// — keep it consistent with the other /boot paths below
	// (rebuildSignedUKI).
	imgPath := fmt.Sprintf("/boot/booster-%s.img", kernelFlavor)
	if _, err := shell.ExecuteCmd("", fmt.Sprintf("booster build --force --kernel-version %s %s", kernelVersion, imgPath)); err != nil {
		fmt.Fprintf(os.Stderr, "WARNING: booster build failed: %v\n", err)
	}
}

// filterCmdlineComments drops comment lines (leading '#' after optional
// whitespace — the "Generated by FeralOS Installer" marker written under
// the cmdline for re-run detection) before the value reaches efi-mkuki
// -c; remaining lines join with a space. TrimSpace keeps the no-marker
// case byte-identical to the previous plain TrimSpace()+pass-through.
func filterCmdlineComments(content string) string {
	kept := make([]string, 0, strings.Count(content, "\n")+1)
	for _, line := range strings.Split(content, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		kept = append(kept, line)
	}
	return strings.TrimSpace(strings.Join(kept, " "))
}

// shellSingleQuote renders s as a POSIX single-quoted shell literal
// (embedded single quotes become the `'\”` escape). Used at the
// efi-mkuki call only for the cmdline ASSIGNMENT — the -c value itself
// rides through the double-quoted variable (unified embedding form; see
// rebuildSignedUKI).
func shellSingleQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

func rebuildSignedUKI(kernelFlavor string) {
	fmt.Println("Rebuilding signed UKI...")
	cmdC, err := os.ReadFile("/etc/kernel/cmdline")
	if err != nil {
		fmt.Fprintf(os.Stderr, "WARNING: cannot read /etc/kernel/cmdline: %v\n", err)
		return
	}
	cmdline := filterCmdlineComments(string(cmdC))
	ukiPath := fmt.Sprintf("/boot/efi/Alpine/linux-%s.efi", kernelFlavor)
	vmlinuz := fmt.Sprintf("/boot/vmlinuz-%s", kernelFlavor)
	boosterImg := fmt.Sprintf("/boot/booster-%s.img", kernelFlavor)

	// efi-mkuki -c carries the cmdline through a shell VARIABLE in double
	// quotes — the boot.go hook + uki.go embedding form (an expansion is
	// not re-parsed by the shell, so the -c argument is exactly the
	// cmdline whatever its characters). Only the assignment embeds the
	// raw value, single-quoted POSIX-escaped via shellSingleQuote.
	if _, err := shell.ExecuteCmd("", fmt.Sprintf("cmdline=%s; efi-mkuki -c \"${cmdline}\" -o %s %s %s", shellSingleQuote(cmdline), ukiPath, vmlinuz, boosterImg)); err != nil {
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
