package internal

import (
	"alpineInstaller/utils/shell"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// InjectApkHooks writes APK hooks from host to guest system.
func InjectApkHooks(chrootDir string) error {
	fmt.Println("[INFO] Injecting APK commit hooks...")

	hookDir := filepath.Join(chrootDir, "etc", "apk", "commit_hooks.d")
	if err := os.MkdirAll(hookDir, 0755); err != nil {
		return fmt.Errorf("failed to create commit_hooks.d: %w", err)
	}

	efi2Path := filepath.Join(chrootDir, "boot", "efi2")
	if _, err := os.Stat(efi2Path); err == nil {
		fmt.Println("[DEBUG] Dual EFI setup detected, injecting sync hook.")
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
		if _, err := shell.ExecuteCmd(chrootDir, fmt.Sprintf("echo '%s' > /etc/apk/commit_hooks.d/copy-efi.sh", escapedEfiHook)); err != nil {
			return fmt.Errorf("failed to write EFI hook: %w", err)
		}
		if err := os.Chmod(filepath.Join(hookDir, "copy-efi.sh"), 0755); err != nil {
			return fmt.Errorf("failed to chmod EFI hook: %w", err)
		}
	} else {
		fmt.Println("[DEBUG] Single EFI setup (no /boot/efi2), skipping sync hook.")
	}

	fmt.Println("[DEBUG] APK hooks successfully processed.")
	return nil
}
