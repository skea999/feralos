package shell

import (
	"bytes"
	"os/exec"
	"strings"
)

// ExecSilent runs a command without logging (for internal use like firmware detection).
func ExecSilent(chroot string, cmd string) (string, error) {
	var stdout, stderr bytes.Buffer
	var err error

	if chroot != "" {
		execCmd := exec.Command("chroot", chroot, "/bin/sh", "-c", cmd)
		execCmd.Stdout = &stdout
		execCmd.Stderr = &stderr
		err = execCmd.Run()
	} else {
		execCmd := exec.Command("/bin/sh", "-c", cmd)
		execCmd.Stdout = &stdout
		execCmd.Stderr = &stderr
		err = execCmd.Run()
	}

	_ = stderr
	return strings.TrimSpace(stdout.String()), err
}
