package internal

import (
	"feralos/src/utils/config"
	"feralos/src/utils/shell"
	"fmt"
	"strings"
)

// SetupMisc configures editor, PS1, timezone, pager,
// subuid/subgid, limits, sudoers, LXC, mkinitfs removal, smartd, cron, keymap.
func SetupMisc(conf *config.Config) error {
	chroot := conf.Internal.ChrootDir
	u := &conf.User

	if err := setupEditorConf(chroot); err != nil {
		return err
	}
	if err := setupConsolePS1(chroot); err != nil {
		return err
	}
	if err := setupTimezone(chroot, u); err != nil {
		return err
	}
	if err := setupLocaleConf(chroot, u); err != nil {
		return err
	}
	if err := setupKeymapConf(chroot, u); err != nil {
		return err
	}
	if err := setupPager(chroot); err != nil {
		return err
	}
	if err := setupSubuidSubgid(chroot); err != nil {
		return err
	}

	// 10. System limits — NOT written here: SetupUsers (users.go) owns
	// /etc/security/limits.conf. This file used to write an
	// identical copy that users.go immediately overwrote.

	if err := setupSudoers(chroot); err != nil {
		return err
	}
	if err := setupLXCConf(chroot); err != nil {
		return err
	}
	removeMkinitfs(chroot)
	if err := setupSmartd(chroot); err != nil {
		return err
	}
	setupAuditCron(chroot)
	generateSSHHostKeys(chroot)

	// 17. FeralOS update script — wired to the DMS System Update button
	// (updaterCustomCommand in the niri settings seed) and usable manually:
	// `feralos-update`. Sequence: apk → flatpak (if installed) → nix (if
	// installed). Best-effort: a failing phase bumps the error counter,
	// later phases still run; exit code = error count (DMS reports failure
	// on non-zero). Privileged steps use sudo — DMS opens this in a
	// terminal, so the password prompt is visible there.
	return writeFeralosUpdateScript(chroot)
}

// setupEditorConf sets micro as the default editor (/etc/profile.d/editor.sh).
func setupEditorConf(chroot string) error {
	// 1. Editor
	fmt.Println("[INFO] Setting editor to micro...")
	if err := shell.WriteFile(chroot, "/etc/profile.d/editor.sh", []byte("export EDITOR=micro\n"), 0644); err != nil {
		return fmt.Errorf("failed to write editor.sh: %w", err)
	}
	return nil
}

// setupConsolePS1 writes the root/user PS1 prompt (/etc/profile.d/console.sh).
func setupConsolePS1(chroot string) error {
	// 3. PS1
	ps1Content := `if [ "$(whoami)" = "root" ] ; then
    PS1="\[\033[31;1m\]\u\[\033[0m\]@\[\033[1;33m\]\h\[\033[0m\]:\w# "
else
    PS1="\[\033[32;1m\]\u\[\033[0m\]@\[\033[1;33m\]\h\[\033[0m\]:\w> "
fi`
	if err := shell.WriteFile(chroot, "/etc/profile.d/console.sh", []byte(ps1Content+"\n"), 0644); err != nil {
		return fmt.Errorf("failed to write console.sh: %w", err)
	}
	return nil
}

// setupTimezone links /etc/localtime to the configured zoneinfo entry.
func setupTimezone(chroot string, u *config.UserVariables) error {
	// 4. Timezone
	fmt.Printf("[INFO] Setting timezone to %s\n", u.Timezone)
	if _, err := shell.ExecuteCmd(chroot, fmt.Sprintf("ln -sf /usr/share/zoneinfo/%s /etc/localtime", u.Timezone)); err != nil {
		return fmt.Errorf("failed to set timezone: %w", err)
	}
	return nil
}

// setupLocaleConf writes the locale environment (/etc/profile.d/20locale.sh).
// "it" → "it_IT.UTF-8" (same expansion as the shell reference; bare "it" is
// invalid for musl setlocale and crashes conmon).
func setupLocaleConf(chroot string, u *config.UserVariables) error {
	// 5. Locale
	fmt.Println("[INFO] Setting up locale...")
	localeFull := fmt.Sprintf("%s_%s.UTF-8", u.Locale, strings.ToUpper(u.Locale))
	localeContent := fmt.Sprintf("export CHARSET=%s\nexport LANG=%s\nLC_COLLATE=%s\n", localeFull, localeFull, localeFull)
	if err := shell.WriteFile(chroot, "/etc/profile.d/20locale.sh", []byte(localeContent), 0644); err != nil {
		return fmt.Errorf("failed to write locale config: %w", err)
	}
	return nil
}

// setupKeymapConf writes the console keymap config (/etc/conf.d/loadkmap),
// consumed by the loadkmap service (busybox-loadkmap-dinit under dinit,
// busybox-initscripts under OpenRC). kbd-bkeymaps layout:
// /usr/share/bkeymaps/<code>/<code>.bmap.gz (keymap == layout dir).
func setupKeymapConf(chroot string, u *config.UserVariables) error {
	// 6. Keymap
	keymapPath := fmt.Sprintf("/usr/share/bkeymaps/%s/%s.bmap.gz", u.Keymap, u.Keymap)
	fmt.Printf("[INFO] Setting keyboard layout to %s...\n", keymapPath)
	keymapConf := fmt.Sprintf("KEYMAP=%q\n", keymapPath)
	if err := shell.WriteFile(chroot, "/etc/conf.d/loadkmap", []byte(keymapConf), 0644); err != nil {
		return fmt.Errorf("failed to write keymap config: %w", err)
	}
	return nil
}

// setupPager points the pager at moor (profile sed, best-effort) and writes
// the fish equivalent (/etc/fish/conf.d/pager.fish).
func setupPager(chroot string) error {
	// 7. Pager (fish only)
	fmt.Println("[INFO] Configuring pager...")
	if _, err := shell.ExecuteCmd(chroot, "sed -i 's/PAGER=less/PAGER=moor/g' /etc/profile"); err != nil {
		fmt.Printf("[WARNING] Failed to set pager: %v\n", err)
	}
	if err := shell.WriteFile(chroot, "/etc/fish/conf.d/pager.fish", []byte("set -gx PAGER moor\n"), 0644); err != nil {
		return fmt.Errorf("failed to write pager.fish: %w", err)
	}
	return nil
}

// setupSubuidSubgid writes the root subordinate id ranges.
func setupSubuidSubgid(chroot string) error {
	// 8. Subuid/subgid
	fmt.Println("[INFO] Configuring subuid/subgid...")
	if err := shell.WriteFile(chroot, "/etc/subuid", []byte("root:100000:65536\n"), 0644); err != nil {
		return fmt.Errorf("failed to write subuid: %w", err)
	}
	if err := shell.WriteFile(chroot, "/etc/subgid", []byte("root:100000:65536\n"), 0644); err != nil {
		return fmt.Errorf("failed to write subgid: %w", err)
	}
	return nil
}

// setupSudoers creates /etc/sudoers.d, grants wheel and enables the
// sudoers @includedir (Alpine ships it commented — without it the drop-in
// is never loaded and wheel sudo was denied).
func setupSudoers(chroot string) error {
	// 11. Sudoers
	fmt.Println("[INFO] Configuring sudo-rs permissions...")
	if _, err := shell.ExecuteCmd(chroot, "mkdir -p /etc/sudoers.d"); err != nil {
		return fmt.Errorf("failed to create /etc/sudoers.d directory: %w", err)
	}
	if err := shell.WriteFile(chroot, "/etc/sudoers.d/wheel", []byte("%wheel ALL=(ALL) ALL\n"), 0644); err != nil {
		return fmt.Errorf("failed to write sudoers wheel config: %w", err)
	}
	if _, err := shell.ExecuteCmd(chroot, "sed -i 's/^# @includedir/@includedir/' /etc/sudoers"); err != nil {
		return fmt.Errorf("failed to enable sudoers includedir: %w", err)
	}
	return nil
}

// setupLXCConf enables systemd container mode for LXC (/etc/conf.d/lxc).
func setupLXCConf(chroot string) error {
	// 12. LXC
	fmt.Println("[INFO] Writing LXC service configuration...")
	if err := shell.WriteFile(chroot, "/etc/conf.d/lxc", []byte("systemd_container=yes\n"), 0644); err != nil {
		return fmt.Errorf("failed to write LXC config: %w", err)
	}
	return nil
}

// removeMkinitfs drops mkinitfs (booster replaces it). Best-effort.
func removeMkinitfs(chroot string) {
	// 13. Remove mkinitfs
	fmt.Println("[INFO] Removing mkinitfs...")
	if _, err := shell.ExecuteCmd(chroot, "apk del mkinitfs 2>/dev/null || true"); err != nil {
		fmt.Printf("[WARNING] Failed to remove mkinitfs: %v\n", err)
	}
}

// setupSmartd writes the smartd device scan configuration.
func setupSmartd(chroot string) error {
	// 14. Smartd config
	fmt.Println("[INFO] Configuring smartd...")
	if err := shell.WriteFile(chroot, "/etc/smartd.conf", []byte("DEVICESCAN -a -o on -S on -s (S/../.././02|L/../../6/03) -W 4,40,45\n"), 0644); err != nil {
		return fmt.Errorf("failed to write smartd.conf: %w", err)
	}
	return nil
}

// setupAuditCron appends the weekly APK audit job to the root crontab
// (best-effort).
func setupAuditCron(chroot string) {
	// 15. APK audit cron
	fmt.Println("[INFO] Configuring APK audit cron...")
	if err := shell.WriteFileAppend(chroot, "/etc/crontabs/root", []byte("0 4 * * 0 /usr/bin/apk audit --full --quiet || true\n"), 0644); err != nil {
		fmt.Printf("[WARNING] Failed to write cron job: %v\n", err)
	}
}

// generateSSHHostKeys bakes the SSH host keys at install time. The dinit
// sshd service runs plain `sshd -D -e` (no shell wrapper: dinit does not
// parse quotes), so the OpenRC initd key generation never runs.
// ssh-keygen -A is idempotent. Best-effort.
func generateSSHHostKeys(chroot string) {
	// 16. SSH host keys
	if out, _ := shell.ExecuteCmd(chroot, "apk info -e openssh >/dev/null 2>&1 && echo INSTALLED || echo NOT_INSTALLED"); strings.Contains(out, "INSTALLED") && !strings.Contains(out, "NOT_INSTALLED") {
		fmt.Println("[INFO] Generating SSH host keys...")
		if _, err := shell.ExecuteCmd(chroot, "ssh-keygen -A"); err != nil {
			fmt.Printf("[WARNING] Failed to generate SSH host keys: %v\n", err)
		}
	}
}

// writeFeralosUpdateScript installs /usr/local/bin/feralos-update (apk →
// flatpak → nix update sequence, wired to the DMS System Update button).
func writeFeralosUpdateScript(chroot string) error {
	fmt.Println("[INFO] Writing feralos-update script...")
	updateScript := `#!/bin/sh
# FeralOS system update — wired to the DMS System Update button
# (updaterCustomCommand) and usable manually: feralos-update
# Sequence: apk -> flatpak (if installed) -> nix (if installed).
# Best-effort: a failing phase bumps the error counter, later phases still
# run; exit code = error count. Privileged steps use sudo (prompt in this
# terminal — DMS opens one with the update banner).
echo "=== FeralOS update ==="
errors=0
run() {
    echo
    echo "--- $1 ---"
    shift
    "$@" || { echo "!!! FAILED: $*"; errors=$((errors + 1)); }
}

# --- 1. APK (always) ---
run "APK: refresh indexes" sudo apk update
run "APK: upgrade system" sudo apk upgrade --available

# --- 2. Flatpak (if installed) ---
if command -v flatpak >/dev/null 2>&1; then
    echo
    echo "=== Flatpak ==="
    run "Flatpak: remove unused (user)" flatpak uninstall --unused -y
    run "Flatpak: remove unused (system)" sudo flatpak uninstall --unused --system -y
    run "Flatpak: update (user)" flatpak update --user -y
    run "Flatpak: update (system)" sudo flatpak update --system -y
else
    echo
    echo "=== Flatpak not installed - skipping ==="
fi

# --- 3. Nix (if installed) ---
if command -v nix >/dev/null 2>&1; then
    echo
    echo "=== Nix ==="
    run "Nix: refresh channels" nix-channel --update
    run "Nix: upgrade profile packages" nix profile upgrade '.*'
    run "Nix: trim generations (keep last 5)" nix-env --delete-generations +5
    run "Nix: garbage collect" nix-collect-garbage
    if [ "$(id -u)" != 0 ] && sudo test -e /root/.local/state/nix/profiles/profile; then
        echo
        echo "=== Nix (root profile) ==="
        run "Nix: trim root generations (keep last 5)" sudo nix-env --delete-generations +5
        run "Nix: garbage collect (root)" sudo nix-collect-garbage
    fi
else
    echo
    echo "=== Nix not installed - skipping ==="
fi

echo
if [ "$errors" -eq 0 ]; then
    echo "=== Update completed with no errors ==="
else
    echo "=== Update completed with $errors error(s) ==="
fi
exit "$errors"
`
	// Direct write: the log carries path + size only — the old ExecSilent
	// rationale (the echoed body's "error"/"FAILED" words tripping the harness
	// log_errors sweep) is obsolete with the logged-write helper.
	if err := shell.WriteFile(chroot, "/usr/local/bin/feralos-update", []byte(updateScript+"\n"), 0644); err != nil {
		return fmt.Errorf("failed to write feralos-update script: %w", err)
	}
	if _, err := shell.ExecuteCmd(chroot, "chmod 755 /usr/local/bin/feralos-update"); err != nil {
		return fmt.Errorf("failed to chmod feralos-update script: %w", err)
	}

	return nil
}
