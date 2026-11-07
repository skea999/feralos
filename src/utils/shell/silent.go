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

// silentRun is the shared core of the silent flavors: builds the command with
// the package commandTimeout context (P1-2 — the ctx-bound command kills the
// process on expiry), captures + trims stdout, attaches a trimmed+bounded
// stderr snippet to errors, logs nothing, and never retries / prompts
// (boot-oneshot safety).
func silentRun(buildCmd func(ctx context.Context) *exec.Cmd) (string, error) {
	var stdout, stderr bytes.Buffer

	ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
	defer cancel()
	execCmd := buildCmd(ctx)
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

// ExecSilent runs a command without logging (for internal use like firmware detection).
// Bounded by the package commandTimeout (P1-2) like every other execution path.
// On failure the (trimmed, bounded) stderr is included in the returned error; on
// success behavior is unchanged (trimmed stdout, nil error).
func ExecSilent(chroot string, cmd string) (string, error) {
	return silentRun(func(ctx context.Context) *exec.Cmd {
		if chroot != "" {
			return exec.CommandContext(ctx, "chroot", chroot, "/bin/sh", "-c", cmd)
		}
		return exec.CommandContext(ctx, "/bin/sh", "-c", cmd)
	})
}

// ExecSilentArgs runs a command WITHOUT a shell (no /bin/sh -c) — safe for
// inputs that must never be reinterpreted by a shell (activation-file
// content). Same silent semantics as ExecSilent (timeout-bounded, no retry /
// interactive prompt, trimmed stdout, bounded stderr snippet on error). dir
// mirrors ExecSilent's chroot parameter: non-empty runs through chroot,
// empty runs on the host.
func ExecSilentArgs(dir string, name string, args ...string) (string, error) {
	return silentRun(func(ctx context.Context) *exec.Cmd {
		if dir != "" {
			return exec.CommandContext(ctx, "chroot", append([]string{dir, name}, args...)...) //nolint:gosec // G204: same posture as ExecuteCmdArgs — silent path, args logged-free
		}
		return exec.CommandContext(ctx, name, args...)
	})
}
