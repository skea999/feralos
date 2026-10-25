package shell

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

// maxSilentStderr bounds the stderr snippet attached to ExecSilent errors.
const maxSilentStderr = 500

// ExecSilent runs a command without logging (for internal use like firmware detection).
// On failure the (trimmed, bounded) stderr is included in the returned error; on
// success behavior is unchanged (trimmed stdout, nil error).
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

	if err != nil {
		if errText := strings.TrimSpace(stderr.String()); errText != "" {
			if len(errText) > maxSilentStderr {
				errText = errText[:maxSilentStderr]
			}
			return strings.TrimSpace(stdout.String()), fmt.Errorf("%w: %s", err, errText)
		}
	}
	return strings.TrimSpace(stdout.String()), err
}
