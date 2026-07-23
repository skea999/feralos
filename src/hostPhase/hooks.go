package hostPhase

import (
	"alpineInstaller/utility"
	"fmt"
	"os"
	"path/filepath"
)

// InjectApkHooks writes APK hooks from host to guest system.
// Handles kernel fallback and EFI sync for multi-disk setups.
func InjectApkHooks(chrootDir string) error {
	fmt.Println("[INFO] HostPhase: Injecting APK commit hooks...")

	hookDir := filepath.Join(chrootDir, "/etc/apk/commit_hooks.d")

	// Ensure hook directory exists
	if err := os.MkdirAll(hookDir, 0755); err != nil {
		return fmt.Errorf("failed to create commit_hooks.d: %w", err)
	}

	// 1. Kernel Fallback Hook (Pre-commit)
	// Saves current working kernel before apk overwrites it during upgrade.
	kernelHook := `#!/bin/sh

STAGE="${1}"
targetCmd="upgrade"
targetPkg="linux-lts" # Could be a placeholder for the installation script

# This hook make a copy of the old kernel to fall back in case something appen

# Execute the script before apk. Check if apk is doing a command defined in ${targetCmd} (possible command are: upgrade add del fix)
if [ "${STAGE}" = "pre-commit" ] && [ "${APK_COMMAND}" = "${targetCmd}" ]; then
    echo "Executing ${targetPkg}-keep-old-kernel.sh pre-commit hook because apk is doing the command ${targetCmd}..."
    
    # Simulate upgrade and if the package is upgraded than process the hook
    if apk upgrade --simulate 2>&1 | grep -qE "(will be installed|is now available)" | grep -q "${targetPkg}"; then
        echo "Package ${targetPkg} is beeing upgraded, proceed with the hook:"

        # ---------- Functions here ----------

        # -----------------------------------------------------------

        # ---------- Script here ----------

        # Check the current version
        kernelVersion=$(uname -r)
        # Only act if there are the files to copy
        if [ -f "/boot/vmlinuz-${kernelVersion}" ] && [ -f "${CURRENT_INITRD}" ]; then
            # Overwrite any previous fallback copy
            cp -afX "/boot/vmlinuz-${kernelVersion}" "/boot/vmlinuz-fallback"
            cp -afX "/boot/initramfs-${kernelVersion}.img" "/boot/initramfs-fallback.img"
        else
            echo "Warning: current kernel files not found - skipping fallback creation." 
        fi
        
        # -----------------------------------------------------------
        
    else
        echo "Package ${targetPkg} is not beeing upgraded."
    fi

fi
exit 0`

	if err := utility.WriteFileInChroot(chrootDir, "/etc/apk/commit_hooks.d/keep-old-kernel.sh", kernelHook); err != nil {
		return fmt.Errorf("failed to write kernel hook: %w", err)
	}
	if err := os.Chmod(filepath.Join(hookDir, "keep-old-kernel.sh"), 0755); err != nil {
		return fmt.Errorf("failed to chmod kernel hook: %w", err)
	}

	// 2. EFI Sync Hook (Post-commit) - CONDITIONAL LOGIC
	// Check if /boot/efi2 directory exists in chroot
	efi2Path := filepath.Join(chrootDir, "/boot/efi2")
	if _, err := os.Stat(efi2Path); err == nil {
		fmt.Println("[DEBUG] Dual EFI setup detected, injecting sync hook.")

		efiHook := `#!/bin/sh

STAGE="${1}"
targetCmd="upgrade"
targetPkg="linux-lts"


# This hook will do an rsync from /boot/efi to /boot/efi2 to make a sort of raid 1 on the efi partition

# Execute the script after apk. Check if apk is doing a command defined in ${targetCmd} (possible command are: upgrade add del fix)
if [ "${STAGE}" = "post-commit" ] && [ "${APK_COMMAND}" = "${targetCmd}" ]; then
    echo "Executing ${targetPkg}-raid-efi.sh post-commit hook because apk is doing the command ${targetCmd}... "
    
    # Check apk log
    latestLog=$(ls -t /var/log/apk/* | head -n 1)

    # If the file exists
    if [ -f "${latestLog}" ]; then
        
        # Select oly lines containing install or upgrade and than if those lines contain $targetPkg
        if grep -qE "install|upgrade" "${latestLog}" | grep -q "${targetPkg}"; then
            echo "Found ${targetPkg}. Executing script now... "
            
            # ---------- Functions here ----------

            # -----------------------------------------------------------

            # ---------- Script here ----------

            # Check the current version
            kernelVersion=$(uname -r)
            # Only act if there are the files to copy
            if [ -f "/boot/vmlinuz-${kernelVersion}" ] && [ -f "${CURRENT_INITRD}" ]; then
                # Overwrite any previous fallback copy
                rsync -a --delete /boot/efi/ /boot/efi2/
            else
                echo "Warning: current kernel files not found - skipping fallback creation." 
            fi
            
            # -----------------------------------------------------------
        else
            echo "${targetPkg} not found. "
        fi
    else
        echo "Warning: Log file not found. "
    fi
fi

exit 0`

		if err := utility.WriteFileInChroot(chrootDir, "/etc/apk/commit_hooks.d/copy-efi.sh", efiHook); err != nil {
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
