package shell

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// maxSilentStderr bounds the stderr snippet attached to ExecSilent errors.
const maxSilentStderr = 500

// ExecSilent runs a command without logging (for internal use like firmware detection).
// Bounded by the package commandTimeout (P1-2) like every other execution path.
// On failure the (trimmed, bounded) stderr is included in the returned error; on
// success behavior is unchanged (trimmed stdout, nil error).
func ExecSilent(chroot string, cmd string) (string, error) {
	var stdout, stderr bytes.Buffer

	ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
	defer cancel()
	var execCmd *exec.Cmd
	if chroot != "" {
		execCmd = exec.CommandContext(ctx, "chroot", chroot, "/bin/sh", "-c", cmd)
	} else {
		execCmd = exec.CommandContext(ctx, "/bin/sh", "-c", cmd)
	}
	execCmd.Stdout = &stdout
	execCmd.Stderr = &stderr
	err := execCmd.Run()
	if err != nil && ctx.Err() == context.DeadlineExceeded {
		err = fmt.Errorf("command timed out after %s: %w", commandTimeout, err)
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
