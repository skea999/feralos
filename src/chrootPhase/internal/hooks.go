package internal

import (
	"alpineInstaller/utils/shell"
	"fmt"
	"strings"
)

// InjectEfiSyncHook writes an APK commit hook that syncs /boot/efi to /boot/efi2
// after kernel upgrades. Only for dual-disk layouts (checks /boot/efi2 exists).
// Uses chrootPhase contract: shell.ExecuteCmd(chroot, ...) for all operations.
func InjectEfiSyncHook(chroot string) error {
	// Check if dual-disk (EFI2 exists)
	checkCmd := fmt.Sprintf("test -d %s/boot/efi2 && echo yes || echo no", chroot)
	out, _ := shell.ExecuteCmd("", checkCmd)
	if strings.TrimSpace(out) != "yes" {
		return nil // single-disk, no hook needed
	}

	fmt.Println("[INFO] Injecting EFI sync APK hook (dual-disk)...")

	efiHook := `#!/bin/sh

STAGE="${1}"
targetCmd="upgrade"
targetPkg="linux-lts"

if [ "${STAGE}" = "post-commit" ] && [ "${APK_COMMAND}" = "${targetCmd}" ]; then
    echo "Executing ${targetPkg}-raid-efi.sh post-commit hook because apk is doing the command ${targetCmd}... "

    latestLog=$(ls -t /var/log/apk/* | head -n 1)

    if [ -f "${latestLog}" ]; then
        if grep -qE "install|upgrade" "${latestLog}" | grep -q "${targetPkg}"; then
            echo "Found ${targetPkg}. Executing script now... "
            rsync -a --delete /boot/efi/ /boot/efi2/
        else
            echo "${targetPkg} not found. "
        fi
    else
        echo "Warning: Log file not found. "
    fi
fi

exit 0`
	escapedHook := strings.ReplaceAll(efiHook, "'", "'\\''")
	hookCmd := fmt.Sprintf("mkdir -p /etc/apk/commit_hooks.d && echo '%s' > /etc/apk/commit_hooks.d/copy-efi.sh && chmod 755 /etc/apk/commit_hooks.d/copy-efi.sh", escapedHook)
	if _, err := shell.ExecuteCmd(chroot, hookCmd); err != nil {
		return fmt.Errorf("failed to write EFI sync hook: %w", err)
	}

	fmt.Println("[INFO] EFI sync hook written (dual-disk post-upgrade sync)")
	return nil
}
