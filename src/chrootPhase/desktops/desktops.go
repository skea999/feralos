package desktops

import (
	"feralos/src/utils/config"
	"feralos/src/utils/shell"
	"fmt"
	"strings"
)

// Setup dispatches to the appropriate desktop environment setup.
// Registry map: desktop name → setup function. Unknown DEs are skipped.
var setupRegistry = map[string]func(*config.Config) error{
	"niri":      SetupNiri,
	"niriWayle": SetupNiriWayle,
}

// Setup dispatches to the appropriate desktop environment setup.
func Setup(conf *config.Config) error {
	de := conf.User.DesktopEnvironment
	if de == "" || de == "none" {
		return nil
	}

	chroot := conf.Internal.ChrootDir

	// Enable Unicode in rc.conf — required for all desktop environments on
	// Alpine (musl disables unicode by default).
	if _, err := shell.ExecuteCmd(chroot, `sed -i 's/^#unicode="YES"$/unicode="YES"/' /etc/rc.conf`); err != nil {
		fmt.Printf("[WARNING] Failed to enable unicode in rc.conf: %v\n", err)
	}

	fn, ok := setupRegistry[de]
	if !ok {
		return nil
	}

	return fn(conf)
}

// ensureOO7PamLines makes /etc/pam.d/greetd carry the oo7 keyring lines
// EXACTLY ONCE (both niri-family desktops): the file is read, every
// existing line is NORMALIZED (whitespace-collapsed) and compared against
// the required token sequence — spacing variants can never create a
// second copy — and a missing line is appended at the END of the stack
// (the session line must run after base-session so /run/user exists).
// One read, one write, NO shell appends: duplicate lines are impossible
// by construction. A greetd vendor update that later lands as .apk-new
// cannot duplicate them either (the module lines exist only where we
// wrote them). (User directive 2026-11-28: no appends to vendor PAM
// files; no package takeover of vendor PAM files.)
func ensureOO7PamLines(chroot string) error {
	const path = "/etc/pam.d/greetd"
	required := []string{
		"auth       optional pam_oo7.so",
		"session    optional pam_oo7.so auto_start",
	}
	out, err := shell.ExecuteCmd(chroot, "cat "+path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	norm := func(l string) string { return strings.Join(strings.Fields(l), " ") }
	have := map[string]bool{}
	for _, l := range strings.Split(out, "\n") {
		have[norm(l)] = true
	}
	var add []string
	for _, r := range required {
		if !have[norm(r)] {
			add = append(add, r)
		}
	}
	if len(add) == 0 {
		return nil
	}
	content := strings.TrimRight(out, "\n") + "\n" + strings.Join(add, "\n") + "\n"
	return shell.WriteFile(chroot, path, []byte(content), 0644)
}

// ensureSecretsLogLevel sets RUST_LOG for the oo7 secrets daemon in the
// ADR-008 config the secrets wrapper sources before exec'ing it (survives
// user-dinit respawns — dinitctl setenv would apply too late: boot.d
// starts the daemon before the session scripts run). The harness
// oo7_pam_unlock runtime proof greps the INFO line "Received unlock
// request" that ONLY RUST_LOG=info produces (the file ships with the line
// commented — ERROR-only default); per user directive only enableDebug
// installs carry INFO logging. Deterministic: an existing RUST_LOG line is
// rewritten IN PLACE, extra ones collapse, and the file is written back
// once — nothing is ever appended twice (user directive 2026-11-28).
const defaultSecretsLogLine = "#RUST_LOG=info" //nolint:gosec // env filter value, not a credential

func ensureSecretsLogLevel(chroot string, enableDebug bool) error {
	const path = "/etc/dinit.d/config/secrets.conf"
	out, err := shell.ExecuteCmd(chroot, "cat "+path+" 2>/dev/null || true")
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	want := defaultSecretsLogLine
	if enableDebug {
		want = "RUST_LOG=info"
	}
	lines := make([]string, 0, 32)
	found := false
	for _, l := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
		if strings.HasPrefix(l, "RUST_LOG=") || l == defaultSecretsLogLine {
			if !found {
				lines = append(lines, want)
				found = true
			}
			continue
		}
		lines = append(lines, l)
	}
	if !found {
		lines = append(lines, want)
	}
	content := strings.Join(lines, "\n") + "\n"
	if content == out || (out == "" && want == defaultSecretsLogLine) {
		return nil
	}
	return shell.WriteFile(chroot, path, []byte(content), 0644)
}
