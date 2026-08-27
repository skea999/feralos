package cleanup

import (
	"alpineInstaller/utils/config"
	"alpineInstaller/utils/shell"
	"fmt"
)

// Cleanup performs a full disk cleanup: stops swaps, unmounts chroot
// (including virtual FS), stops MD RAID, closes LUKS, removes device maps.
// Called before hostPhase (start) and after firstboot (shutdown).
// Always continues — not all combinations exist on every system.
func Cleanup(conf *config.Config) error {
	chroot := conf.Internal.ChrootDir
	fmt.Println("[INFO] Stopping existing RAIDs and swaps...")
	_, _ = shell.ExecuteCmd("", "swapoff -a")
	_, _ = shell.ExecuteCmd("", fmt.Sprintf("mountpoint -q %s && umount -fR %s || true", chroot, chroot))

	// Unmount virtual FS (dev, proc, sys) — no-op if not mounted
	for _, dir := range []string{"/dev", "/proc", "/sys"} {
		_, _ = shell.ExecuteCmd("", fmt.Sprintf("umount -R %s 2>/dev/null || true", chroot+dir))
	}

	_, _ = shell.ExecuteCmd("", "mdadm --stop /dev/md* 2>/dev/null || true")
	_, _ = shell.ExecuteCmd("", "for m in $(dmsetup ls --target crypt 2>/dev/null | awk '{print $1}'); do cryptsetup close \"$m\" 2>/dev/null; done; true")
	_, _ = shell.ExecuteCmd("", "dmsetup remove_all --force 2>/dev/null || true")

	// Final unmount of chroot root
	_, _ = shell.ExecuteCmd("", fmt.Sprintf("umount %s 2>/dev/null || true", chroot))

	fmt.Println("[INFO] Disk cleanup complete.")
	return nil
}
