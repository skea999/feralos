package shell

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

const MaxRetries = 3

// retryBackoff is the fixed delay between retry attempts (unchanged semantics:
// 3 attempts, 3s apart).
const retryBackoff = 3 * time.Second

// commandTimeout bounds every single command execution (P1-2). On expiry the
// process is killed and the timeout-wrapped error flows through the standard
// retry path (3 attempts, then fail/ignore like any failure). The default of
// 30 minutes covers every legitimate installer command (bulk apk add peaks
// around 10 minutes even in slow QEMU); tests override this var.
var commandTimeout = 30 * time.Minute

// osExit is the process-exit hook (P2-1): the library keeps a single abort
// point and tests override this var to capture the call instead of killing
// the test process. Production behavior is identical (os.Exit(1)).
var osExit = os.Exit

var (
	currentCaller string
	callerMu      sync.RWMutex

	// stdinReader is the package-global input for the interactive ask loop
	// (P2-1). Kept global deliberately (single-threaded installer flow, at
	// most one prompt per failed command) and initialized lazily on first
	// interactive read, so importing the package never binds os.Stdin.
	// Tests swap it via setTestStdin (execute_test.go).
	stdinReader *bufio.Reader
)

// promptInput reads one line for the ask loop, lazily binding os.Stdin.
func promptInput() (string, error) {
	if stdinReader == nil {
		stdinReader = bufio.NewReader(os.Stdin)
	}
	return stdinReader.ReadString('\n')
}

// SetCaller sets the module label for subsequent log lines.
func SetCaller(caller string) {
	callerMu.Lock()
	currentCaller = caller
	callerMu.Unlock()
}

// GetCaller returns the current caller label
func GetCaller() string {
	callerMu.RLock()
	defer callerMu.RUnlock()
	return currentCaller
}

// ExecuteCmd executes a shell command with logging, retry, and user feedback.
// Behavior identical to the old bash executeCmd function.
//
// Parameters:
//   - chroot: if non-empty, command runs inside this chroot directory
//   - cmd: the shell command to execute
//
// Returns:
//   - stdout output
//   - error if command failed after all retries
func ExecuteCmd(chroot string, cmd string) (string, error) {
	return ExecuteCmdWithAllowFail(chroot, cmd, false, false)
}

// ExecuteCmdWithAllowFail executes a command and optionally treats exit code != 0 as success.
// Use allowFail=true for commands like "apk del" that may legitimately fail.
// Use critical=true to force failure even when SKEA_NONINTERACTIVE=ignore (for luksFormat, passwd, etc.).
func ExecuteCmdWithAllowFail(chroot string, cmd string, allowFail bool, critical bool) (string, error) {
	tag, prefix := logContext(chroot)
	return runWithRetry(cmd, tag, prefix, func(ctx context.Context) *exec.Cmd {
		if chroot != "" {
			return exec.CommandContext(ctx, "chroot", chroot, "/bin/sh", "-c", cmd)
		}
		return exec.CommandContext(ctx, "/bin/sh", "-c", cmd)
	}, allowFail, critical)
}

// ExecuteCmdArgs executes a command without shell (no sh -c) — safe for variable inputs.
// Use for single commands that don't need shell features ( >, |, &&, ;).
// For host: exec.Command(name, args...)
// For chroot: exec.Command("chroot", chroot, name, args...)
func ExecuteCmdArgs(chroot, name string, args ...string) (string, error) {
	return ExecuteCmdArgsWithAllowFail(chroot, name, args, false, false)
}

// ExecuteCmdArgsWithAllowFail — same as ExecuteCmdArgs but with allowFail/critical.
func ExecuteCmdArgsWithAllowFail(chroot, name string, args []string, allowFail, critical bool) (string, error) {
	tag, prefix := logContext(chroot)
	// Build log string like "adduser -D bob"
	logCmd := name
	if len(args) > 0 {
		logCmd += " " + strings.Join(args, " ")
	}
	if chroot != "" {
		logCmd = fmt.Sprintf("chroot %s %s", chroot, logCmd)
	}
	return runWithRetry(logCmd, tag, prefix, func(ctx context.Context) *exec.Cmd {
		if chroot != "" {
			allArgs := append([]string{chroot, name}, args...)
			return exec.CommandContext(ctx, "chroot", allArgs...)
		}
		return exec.CommandContext(ctx, name, args...)
	}, allowFail, critical)
}

// logContext computes the tag ([CMD] / [CHROOT]) and the caller log prefix
// shared by both command flavors.
func logContext(chroot string) (tag, prefix string) {
	tag = "CMD"
	if chroot != "" {
		tag = "CHROOT"
	}
	caller := GetCaller()
	if caller != "" {
		prefix = fmt.Sprintf("[%s]", caller)
	}
	return tag, prefix
}

// runWithRetry is the single core implementation (P2-1) behind every exported
// Execute* function: logs the command, runs buildCmd(ctx) with a per-command
// timeout (P1-2), streams stdout/stderr to the log, then applies the shared
// retry / ignoreErrors / noninteractive / interactive decision flow. All log
// line formats are byte-identical to the pre-refactor implementation.
func runWithRetry(logCmd, tag, prefix string, buildCmd func(context.Context) *exec.Cmd, allowFail, critical bool) (string, error) {
	var lastErr error
	var lastStdout string

	for attempt := 1; attempt <= MaxRetries; attempt++ {
		// Log timestamp + command
		timestamp := time.Now().Format("2006-01-02 15:04:05")
		fmt.Printf("\n%s[%s] %s: %s", prefix, timestamp, tag, logCmd)

		// Execute command (P1-2: bounded by commandTimeout — on expiry the
		// process is killed and the wrapped error flows through the standard
		// retry path below, like any other failure)
		start := time.Now()
		var stdout, stderr bytes.Buffer
		ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
		execCmd := buildCmd(ctx)
		execCmd.Stdout = &stdout
		execCmd.Stderr = &stderr
		err := execCmd.Run()
		if err != nil && ctx.Err() == context.DeadlineExceeded {
			err = fmt.Errorf("command timed out after %s: %w", commandTimeout, err)
		}
		cancel()
		duration := time.Since(start)
		stdoutStr := stdout.String()

		// Log stdout
		if stdout.Len() > 0 {
			for _, line := range strings.Split(stdoutStr, "\n") {
				if line != "" {
					fmt.Printf("[stdout] %s\n", line)
				}
			}
		}

		// Log stderr
		if stderr.Len() > 0 {
			for _, line := range strings.Split(stderr.String(), "\n") {
				if line != "" {
					fmt.Printf("[stderr] %s\n", line)
				}
			}
		}

		// Log exit code
		exitCode := 0
		if err != nil {
			var exitErr *exec.ExitError
			if errors.As(err, &exitErr) {
				exitCode = exitErr.ExitCode()
			}
		}
		status := "OK"
		if exitCode != 0 {
			status = "FAIL"
		}
		fmt.Printf("[%s][RESULT] %s exit=%d time=%v\n\n", prefix, status, exitCode, duration.Round(time.Millisecond))

		// If OK or allowFail, return
		if err == nil || allowFail {
			return stdoutStr, nil
		}

		lastErr = err
		lastStdout = stdoutStr

		// If not last attempt, retry
		if attempt < MaxRetries {
			fmt.Printf("[%s][RETRY] Attempt %d/%d failed, retrying in 3s...\n", prefix, attempt, MaxRetries)
			time.Sleep(retryBackoff)
			continue
		}

		// After 3 failures
		if os.Getenv("SKEA_IGNORE_ERRORS") == "1" {
			fmt.Printf("[ERROR][IGNORED] command failed (ignoreErrors=true): %s: %v\n", logCmd, lastErr)
			return lastStdout, nil
		}

		if os.Getenv("SKEA_NONINTERACTIVE") == "1" && !critical {
			decision := strings.ToLower(os.Getenv("SKEA_NONINTERACTIVE_DECISION"))
			switch decision {
			case "ignore", "skip":
				fmt.Printf("[WARNING] Non-interactive mode: ignoring error and continuing.\n")
				return lastStdout, nil
			default:
				fmt.Printf("[CRITICAL] Non-interactive mode: aborting script.\n")
				return "", fmt.Errorf("command failed after %d attempts: %s: %w", MaxRetries, logCmd, lastErr)
			}
		}

		// Ask user
		if askUserDecision() {
			attempt = 0 // outer for increments back to 1
		} else {
			return lastStdout, nil
		}
	}

	return "", fmt.Errorf("unreachable")
}

// askUserDecision is THE single interactive decision loop (P2-1 — the former
// askLoop existed twice, once per command flavor). Prompts [s]top/[i]gnore/
// [r]etry until a valid answer; returns true to retry the command, false when
// the error was ignored. "s" never returns — it aborts via the osExit hook.
func askUserDecision() (retry bool) {
	for {
		fmt.Printf("Options: [s]top, [i]gnore, [r]etry: ")
		input, _ := promptInput()
		choice := strings.ToLower(strings.TrimSpace(input))

		switch choice {
		case "s":
			fmt.Printf("[CRITICAL] Script aborted by user.\n")
			osExit(1)
		case "i":
			fmt.Printf("[WARNING] Error ignored. Continuing...\n")
			return false
		case "r":
			fmt.Printf("[INFO] Retrying command...\n")
			return true
		default:
			fmt.Printf("[INFO] Invalid input. Please enter 's', 'i', or 'r'.\n")
		}
	}
}
