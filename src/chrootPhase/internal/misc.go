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
	u := conf.User

	// 1. Editor
	fmt.Println("[INFO] Setting editor to micro...")
	if _, err := shell.ExecuteCmd(chroot, "echo 'export EDITOR=micro' > /etc/profile.d/editor.sh"); err != nil {
		return fmt.Errorf("failed to write editor.sh: %w", err)
	}

	// 3. PS1
	ps1Content := `echo 'if [ "$(whoami)" = "root" ] ; then
    PS1="\[\033[31;1m\]\u\[\033[0m\]@\[\033[1;33m\]\h\[\033[0m\]:\w# "
else
    PS1="\[\033[32;1m\]\u\[\033[0m\]@\[\033[1;33m\]\h\[\033[0m\]:\w> "
fi' > /etc/profile.d/console.sh`
	if _, err := shell.ExecuteCmd(chroot, ps1Content); err != nil {
		return fmt.Errorf("failed to write console.sh: %w", err)
	}

	// 4. Timezone
	fmt.Printf("[INFO] Setting timezone to %s\n", u.Timezone)
	if _, err := shell.ExecuteCmd(chroot, fmt.Sprintf("ln -sf /usr/share/zoneinfo/%s /etc/localtime", u.Timezone)); err != nil {
		return fmt.Errorf("failed to set timezone: %w", err)
	}

	// 5. Locale — "it" → "it_IT.UTF-8" (same expansion as the shell reference;
	// bare "it" is invalid for musl setlocale and crashes conmon)
	fmt.Println("[INFO] Setting up locale...")
	localeFull := fmt.Sprintf("%s_%s.UTF-8", u.Locale, strings.ToUpper(u.Locale))
	localeContent := fmt.Sprintf("echo 'export CHARSET=%s\nexport LANG=%s\nLC_COLLATE=%s' > /etc/profile.d/20locale.sh", localeFull, localeFull, localeFull)
	if _, err := shell.ExecuteCmd(chroot, localeContent); err != nil {
		return fmt.Errorf("failed to write locale config: %w", err)
	}

	// 6. Keymap — consumed by the loadkmap service (busybox-loadkmap-dinit
	// under dinit, busybox-initscripts under OpenRC). kbd-bkeymaps layout:
	// /usr/share/bkeymaps/<code>/<code>.bmap.gz (keymap == layout dir).
	keymapPath := fmt.Sprintf("/usr/share/bkeymaps/%s/%s.bmap.gz", u.Keymap, u.Keymap)
	fmt.Printf("[INFO] Setting keyboard layout to %s...\n", keymapPath)
	if _, err := shell.ExecuteCmd(chroot, fmt.Sprintf("echo 'KEYMAP=%q' > /etc/conf.d/loadkmap", keymapPath)); err != nil {
		return fmt.Errorf("failed to write keymap config: %w", err)
	}

	// 7. Pager (fish only)
	fmt.Println("[INFO] Configuring pager...")
	if _, err := shell.ExecuteCmd(chroot, "sed -i 's/PAGER=less/PAGER=bat/g' /etc/profile"); err != nil {
		fmt.Printf("[WARNING] Failed to set pager: %v\n", err)
	}
	escapedPagerFish := strings.ReplaceAll("set -gx PAGER bat", "'", "'\\''")
	if _, err := shell.ExecuteCmd(chroot, fmt.Sprintf("echo '%s' > /etc/fish/conf.d/pager.fish", escapedPagerFish)); err != nil {
		return fmt.Errorf("failed to write pager.fish: %w", err)
	}

	// 8. Subuid/subgid
	fmt.Println("[INFO] Configuring subuid/subgid...")
	if _, err := shell.ExecuteCmd(chroot, "echo 'root:100000:65536' > /etc/subuid"); err != nil {
		return fmt.Errorf("failed to write subuid: %w", err)
	}
	if _, err := shell.ExecuteCmd(chroot, "echo 'root:100000:65536' > /etc/subgid"); err != nil {
		return fmt.Errorf("failed to write subgid: %w", err)
	}

	// 10. System limits — NOT written here: SetupUsers (users.go) owns
	// /etc/security/limits.conf (polishCode P2-2). This file used to write an
	// identical copy that users.go immediately overwrote.

	// 11. Sudoers
	fmt.Println("[INFO] Configuring sudo-rs permissions...")
	if _, err := shell.ExecuteCmd(chroot, "mkdir -p /etc/sudoers.d"); err != nil {
		return fmt.Errorf("failed to create /etc/sudoers.d directory: %w", err)
	}
	if _, err := shell.ExecuteCmd(chroot, "echo '%wheel ALL=(ALL) ALL' > /etc/sudoers.d/wheel"); err != nil {
		return fmt.Errorf("failed to write sudoers wheel config: %w", err)
	}
	// Alpine's default /etc/sudoers has @includedir commented out — enable it
	// so /etc/sudoers.d/wheel is actually loaded (wheel sudo was denied).
	if _, err := shell.ExecuteCmd(chroot, "sed -i 's/^# @includedir/@includedir/' /etc/sudoers"); err != nil {
		return fmt.Errorf("failed to enable sudoers includedir: %w", err)
	}

	// 12. LXC
	fmt.Println("[INFO] Writing LXC service configuration...")
	if _, err := shell.ExecuteCmd(chroot, "echo 'systemd_container=yes' > /etc/conf.d/lxc"); err != nil {
		return fmt.Errorf("failed to write LXC config: %w", err)
	}

	// 13. Remove mkinitfs
	fmt.Println("[INFO] Removing mkinitfs...")
	if _, err := shell.ExecuteCmd(chroot, "apk del mkinitfs 2>/dev/null || true"); err != nil {
		fmt.Printf("[WARNING] Failed to remove mkinitfs: %v\n", err)
	}

	// 14. Smartd config
	fmt.Println("[INFO] Configuring smartd...")
	escapedSmartd := strings.ReplaceAll("DEVICESCAN -a -o on -S on -s (S/../.././02|L/../../6/03) -W 4,40,45", "'", "'\\''")
	if _, err := shell.ExecuteCmd(chroot, fmt.Sprintf("echo '%s' > /etc/smartd.conf", escapedSmartd)); err != nil {
		return fmt.Errorf("failed to write smartd.conf: %w", err)
	}

	// 15. APK audit cron
	fmt.Println("[INFO] Configuring APK audit cron...")
	if _, err := shell.ExecuteCmd(chroot, "echo '0 4 * * 0 /usr/bin/apk audit --full --quiet || true' >> /etc/crontabs/root"); err != nil {
		fmt.Printf("[WARNING] Failed to write cron job: %v\n", err)
	}

	// 16. SSH host keys — baked at install time. The dinit sshd service runs
	// plain `sshd -D -e` (no shell wrapper: dinit does not parse quotes), so
	// the OpenRC initd key generation never runs. ssh-keygen -A is idempotent.
	if out, _ := shell.ExecuteCmd(chroot, "apk info -e openssh >/dev/null 2>&1 && echo INSTALLED || echo NOT_INSTALLED"); strings.Contains(out, "INSTALLED") && !strings.Contains(out, "NOT_INSTALLED") {
		fmt.Println("[INFO] Generating SSH host keys...")
		if _, err := shell.ExecuteCmd(chroot, "ssh-keygen -A"); err != nil {
			fmt.Printf("[WARNING] Failed to generate SSH host keys: %v\n", err)
		}
	}

	// 17. FeralOS update script — wired to the DMS System Update button
	// (updaterCustomCommand in the niri settings seed) and usable manually:
	// `feralos-update`. Sequence: apk → flatpak (if installed) → nix (if
	// installed). Best-effort: a failing phase bumps the error counter,
	// later phases still run; exit code = error count (DMS reports failure
	// on non-zero). Privileged steps use sudo — DMS opens this in a
	// terminal, so the password prompt is visible there.
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
	escapedUpdate := strings.ReplaceAll(updateScript, "'", "'\\''")
	// ExecSilent: the echoed script body would land in installer.log and trip
	// the harness log_errors sweep (its words "error"/"FAILED"/"failing" are
	// legitimate script content, not failures) — same rationale as firmware
	// detection, which uses this silent variant.
	if _, err := shell.ExecSilent(chroot, fmt.Sprintf("mkdir -p /usr/local/bin && echo '%s' > /usr/local/bin/feralos-update && chmod 755 /usr/local/bin/feralos-update", escapedUpdate)); err != nil {
		return fmt.Errorf("failed to write feralos-update script: %w", err)
	}

	return nil
}
