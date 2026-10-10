package internal

import (
	"feralos/src/utils/shell"
	"fmt"
)

// MountVirtualFilesystems performs rbind and make-rslave for dev, proc and sys
func MountVirtualFilesystems(chroot string) error {
	shell.SetCaller("mount")
	fmt.Printf("[INFO] Mounting virtual filesystems to %s...\n", chroot)

	dirs := []string{"/dev", "/proc", "/sys"}

	for _, dir := range dirs {
		target := chroot + dir
		if _, err := shell.ExecuteCmd("", fmt.Sprintf("mkdir -p %s && chmod 0755 %s", target, target)); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", target, err)
		}

		if _, err := shell.ExecuteCmd("", fmt.Sprintf("mount --rbind %s %s", dir, target)); err != nil {
			return err
		}

		if _, err := shell.ExecuteCmd("", fmt.Sprintf("mount --make-rslave %s", target)); err != nil {
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
		_, _ = shell.ExecuteCmd("", fmt.Sprintf("umount -R %s", chroot+dirs[i]))
	}
	return nil
}
