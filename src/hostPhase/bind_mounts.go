package hostPhase

import (
	"alpineInstaller/utils/shell"
	"fmt"
	"os"
)

// MountVirtualFilesystems performs rbind and make-rslave for dev, proc and sys
func MountVirtualFilesystems(chroot string) error {
	fmt.Printf("[INFO] Mounting virtual filesystems to %s...\n", chroot)

	dirs := []string{"/dev", "/proc", "/sys"}

	for _, dir := range dirs {
		target := chroot + dir
		if err := os.MkdirAll(target, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", target, err)
		}

		// We use rbind to ensure all sub-mounts are included
		if _, err := shell.RunCommand("mount", "--rbind", dir, target); err != nil {
			return err
		}

		// Rslave ensures that mount changes in host don't propagate to chroot and vice-versa in a way that breaks things
		if _, err := shell.RunCommand("mount", "--make-rslave", target); err != nil {
			return err
		}
	}

	return nil
}

// UmountVirtualFilesystems can be used for cleanup
func UmountVirtualFilesystems(chroot string) error {
	fmt.Println("[INFO] Unmounting virtual filesystems...")
	dirs := []string{"/dev", "/proc", "/sys"}
	for i := len(dirs) - 1; i >= 0; i-- {
		// Best effort — cleanup may fail for various reasons
		_, _ = shell.RunCommand("umount", "-R", chroot+dirs[i])
	}
	return nil
}
