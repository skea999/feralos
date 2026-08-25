package common

import (
	"alpineInstaller/utils/config"
	"alpineInstaller/utils/shell"
	"fmt"
	"strings"
)

// WriteEfiSyncHook writes an APK commit hook that syncs /boot/efi to /boot/efi2
// after kernel upgrades. Only for dual-disk layouts. Writes via host path.
func WriteEfiSyncHook(conf *config.Config) error {
	chroot := conf.Internal.ChrootDir
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
	escapedEfiHook := strings.ReplaceAll(efiHook, "'", "'\\''")
	hookCmd := fmt.Sprintf("mkdir -p %s/etc/apk/commit_hooks.d && echo '%s' > %s/etc/apk/commit_hooks.d/copy-efi.sh && chmod 755 %s/etc/apk/commit_hooks.d/copy-efi.sh", chroot, escapedEfiHook, chroot, chroot)
	if _, err := shell.ExecuteCmd("", hookCmd); err != nil {
		return fmt.Errorf("failed to write EFI sync hook: %w", err)
	}
	fmt.Println("[INFO] EFI sync hook written (dual-disk post-upgrade sync)")
	return nil
}
