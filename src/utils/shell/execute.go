package shell

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

const MaxRetries = 3

var (
	currentCaller string
	callerMu      sync.RWMutex
	stdinReader   = bufio.NewReader(os.Stdin)
)

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
	return ExecuteCmdWithAllowFail(chroot, cmd, false)
}

// ExecuteCmdWithAllowFail executes a command and optionally treats exit code != 0 as success.
// Use allowFail=true for commands like "apk del" that may legitimately fail.
func ExecuteCmdWithAllowFail(chroot string, cmd string, allowFail bool) (string, error) {
	tag := "CMD"
	if chroot != "" {
		tag = "CHROOT"
	}

	caller := GetCaller()
	prefix := ""
	if caller != "" {
		prefix = fmt.Sprintf("[%s]", caller)
	}

	var lastErr error
	var lastStdout string

	for attempt := 1; attempt <= MaxRetries; attempt++ {
		// Log timestamp + command
		timestamp := time.Now().Format("2006-01-02 15:04:05")
		fmt.Printf("\n%s[%s] %s: %s", prefix, timestamp, tag, cmd)

		// Execute command
		start := time.Now()
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
			time.Sleep(3 * time.Second)
			continue
		}

		// After 3 failures
		if os.Getenv("SKEA_NONINTERACTIVE") == "1" {
			decision := strings.ToLower(os.Getenv("SKEA_NONINTERACTIVE_DECISION"))
			switch decision {
			case "ignore", "skip":
				fmt.Printf("[WARNING] Non-interactive mode: ignoring error and continuing.\n")
				return lastStdout, nil
			default:
				fmt.Printf("[CRITICAL] Non-interactive mode: aborting script.\n")
				return "", fmt.Errorf("command failed after %d attempts: %s: %w", MaxRetries, cmd, lastErr)
			}
		}

		// Ask user
		for {
			fmt.Printf("Options: [s]top, [i]gnore, [r]etry: ")
			input, _ := stdinReader.ReadString('\n')
			choice := strings.ToLower(strings.TrimSpace(input))

			switch choice {
			case "s":
				fmt.Printf("[CRITICAL] Script aborted by user.\n")
				os.Exit(1)
			case "i":
				fmt.Printf("[WARNING] Error ignored. Continuing...\n")
				return lastStdout, nil
			case "r":
				fmt.Printf("[INFO] Retrying command...\n")
				attempt = 0 // Will be incremented to 1
			default:
				fmt.Printf("[INFO] Invalid input. Please enter 's', 'i', or 'r'.\n")
			}
		}
	}

	return "", fmt.Errorf("unreachable")
}
