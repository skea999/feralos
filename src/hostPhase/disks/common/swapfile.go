package common

import (
	"feralos/src/utils/shell"
	"fmt"
	"strings"
)

// CreateSwapfile creates a swapfile using btrfs native command.
// Debian guide: chattr +C on the @swap directory BEFORE mount (Step 3) makes
// new files inherit NODATACOW, so mkswapfile doesn't need to set it itself.
// No 'btrfs property set compression none' anywhere: the 'm' attribute it
// sets would make chattr +C fail (NixOS/nixpkgs #384047).
func CreateSwapfile(chroot string, sizeMiB int) error {
	swapDir := chroot + "/swap"
	swapFile := swapDir + "/swapfile"

	// Ensure swap directory exists
	if _, err := shell.ExecuteCmdArgs("", "mkdir", "-p", swapDir); err != nil {
		return fmt.Errorf("failed to create swap directory: %w", err)
	}

	// Remove old swapfile if exists
	_, _ = shell.ExecuteCmdArgs("", "rm", "-f", swapFile)

	// Create swapfile using btrfs native command
	fmt.Println("[INFO] Creating swapfile with btrfs filesystem mkswapfile...")
	if _, err := shell.ExecuteCmdArgs("", "btrfs", "filesystem", "mkswapfile", "--size", fmt.Sprintf("%dM", sizeMiB), swapFile); err != nil {
		return fmt.Errorf("failed to create swapfile: %w", err)
	}

	// Verify
	lsattrOut, _ := shell.ExecuteCmdArgs("", "lsattr", swapFile)
	fmt.Printf("[INFO] lsattr swapfile: %s\n", strings.TrimSpace(lsattrOut))

	return nil
}
