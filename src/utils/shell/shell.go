package shell

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// caller is the current module label for log lines
var (
	currentCaller string
	callerMu      sync.RWMutex
)

// SetCaller sets the module label for subsequent log lines.
// Call at the start of each module's main function.
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

// formatDuration formats duration in human-readable format
func formatDuration(d time.Duration) string {
	if d < time.Millisecond {
		return fmt.Sprintf("%dus", d.Microseconds())
	}
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	return fmt.Sprintf("%dm%ds", int(d.Minutes()), int(d.Seconds())%60)
}

// logPrefix returns the formatted prefix for a log line
func logPrefix(tag string) string {
	caller := GetCaller()
	if caller != "" {
		return fmt.Sprintf("[%s][%s]", caller, tag)
	}
	return fmt.Sprintf("[%s]", tag)
}

// logCommand logs the command being executed
func logCommand(isChroot bool, name string, args []string) {
	tag := "CMD"
	if isChroot {
		tag = "CHROOT"
	}
	prefix := logPrefix(tag)

	// Build command string
	cmdStr := name
	if len(args) > 0 {
		cmdStr += " " + strings.Join(args, " ")
	}

	fmt.Printf("%s %s\n", prefix, cmdStr)
}

// logResult logs the command result with timing and exit code
func logResult(stdout, stderr *bytes.Buffer, err error, duration time.Duration) {
	// Exit code
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1
		}
	}

	// Status indicator
	status := "OK"
	if exitCode != 0 {
		status = "FAIL"
	}

	prefix := logPrefix("RESULT")
	fmt.Printf("%s %s exit=%d time=%s\n", prefix, status, exitCode, formatDuration(duration))

	// Log stdout if present
	if stdout.Len() > 0 {
		out := strings.TrimSpace(stdout.String())
		if len(out) > 2000 {
			out = out[:2000] + fmt.Sprintf("...(%d bytes truncated)", len(stdout.String())-2000)
		}
		for _, line := range strings.Split(out, "\n") {
			fmt.Printf("[stdout] %s\n", line)
		}
	}

	// Log stderr if present
	if stderr.Len() > 0 {
		errOut := strings.TrimSpace(stderr.String())
		if len(errOut) > 2000 {
			errOut = errOut[:2000] + "...(truncated)"
		}
		for _, line := range strings.Split(errOut, "\n") {
			fmt.Printf("[stderr] %s\n", line)
		}
	}
}

// logError logs command failure details
func logError(name string, args []string, stderr *bytes.Buffer, err error) {
	prefix := logPrefix("ERROR")
	fmt.Printf("%s Command failed: %s %v\n", prefix, name, args)
	fmt.Printf("%s Details: %s\n", prefix, strings.TrimSpace(stderr.String()))
	fmt.Printf("%s System: %v\n", prefix, err)
}

// handleNonInteractive handles non-interactive mode decisions
func handleNonInteractive(name string, args []string, stdout *bytes.Buffer, err error) (string, error) {
	decision := strings.ToLower(os.Getenv("SKEA_NONINTERACTIVE_DECISION"))
	switch decision {
	case "stop", "abort":
		fmt.Printf("[CRITICAL] Non-interactive mode: aborting on error.\n")
		return "", fmt.Errorf("command failed in non-interactive mode: %s %v: %w", name, args, err)
	case "ignore", "skip":
		fmt.Printf("[WARNING] Non-interactive mode: ignoring error and proceeding.\n")
		return stdout.String(), nil
	default:
		fmt.Printf("[CRITICAL] Non-interactive mode: aborting on error.\n")
		return "", fmt.Errorf("command failed in non-interactive mode: %s %v: %w", name, args, err)
	}
}

// executeCommand executes a command and returns the result
func executeCommand(name string, args []string) (string, *bytes.Buffer, *bytes.Buffer, error) {
	var stdout, stderr bytes.Buffer
	cmd := exec.Command(name, args...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	return stdout.String(), &stdout, &stderr, err
}

// RunCommand executes a shell command. If it fails, it asks the user to Retry or Ignore.
func RunCommand(name string, args ...string) (string, error) {
	reader := bufio.NewReader(os.Stdin)

	for {
		logCommand(false, name, args)
		start := time.Now()

		stdoutStr, stdout, stderr, err := executeCommand(name, args)
		duration := time.Since(start)

		logResult(stdout, stderr, err, duration)

		if err == nil {
			return stdoutStr, nil
		}

		// Check for non-interactive mode
		if os.Getenv("SKEA_NONINTERACTIVE") == "1" {
			return handleNonInteractive(name, args, stdout, err)
		}

		// Interactive error handling
		logError(name, args, stderr, err)
		fmt.Print("Options: [r]etry, [i]gnore, [a]bort: ")

		input, _ := reader.ReadString('\n')
		choice := strings.ToLower(strings.TrimSpace(input))

		switch choice {
		case "r", "retry":
			fmt.Printf("[INFO] Retrying command...\n")
			continue
		case "i", "ignore":
			fmt.Printf("[WARNING] Ignoring error and proceeding...\n")
			return stdoutStr, nil
		case "a", "abort":
			fmt.Printf("[CRITICAL] Installation aborted by user.\n")
			os.Exit(1)
		default:
			fmt.Printf("[INFO] Invalid choice. Please type 'r', 'i', or 'a'.\n")
			continue
		}
	}
}

// RunChrootCommand executes a command inside the specified chroot directory
func RunChrootCommand(chroot string, command string) (string, error) {
	return RunCommand("chroot", chroot, "/bin/sh", "-c", command)
}

// RunCommandStrict executes a shell command and ALWAYS returns errors,
// even in non-interactive mode. Use for critical steps (bootstrap, etc.)
func RunCommandStrict(name string, args ...string) (string, error) {
	logCommand(false, name, args)
	start := time.Now()

	stdoutStr, stdout, stderr, err := executeCommand(name, args)
	duration := time.Since(start)

	logResult(stdout, stderr, err, duration)

	if err != nil {
		logError(name, args, stderr, err)
		return stdoutStr, fmt.Errorf("command failed: %s %v: %w", name, args, err)
	}
	return stdoutStr, nil
}

// RunCommandWithChroot logs as chroot command but executes directly
func RunCommandWithChroot(chroot string, name string, args ...string) (string, error) {
	logCommand(true, name, args)
	start := time.Now()

	stdoutStr, stdout, stderr, err := executeCommand(name, args)
	duration := time.Since(start)

	logResult(stdout, stderr, err, duration)

	if err == nil {
		return stdoutStr, nil
	}

	// Check for non-interactive mode
	if os.Getenv("SKEA_NONINTERACTIVE") == "1" {
		return handleNonInteractive(name, args, stdout, err)
	}

	// Interactive error handling
	reader := bufio.NewReader(os.Stdin)

	for {
		logError(name, args, stderr, err)
		fmt.Print("Options: [r]etry, [i]gnore, [a]bort: ")

		input, _ := reader.ReadString('\n')
		choice := strings.ToLower(strings.TrimSpace(input))

		switch choice {
		case "r", "retry":
			fmt.Printf("[INFO] Retrying command...\n")
			// Re-execute the command
			start = time.Now()
			stdoutStr, stdout, stderr, err = executeCommand(name, args)
			duration = time.Since(start)
			logResult(stdout, stderr, err, duration)

			if err == nil {
				return stdoutStr, nil
			}
			// Continue the loop to show error again
			continue

		case "i", "ignore":
			fmt.Printf("[WARNING] Ignoring error and proceeding...\n")
			return stdoutStr, nil
		case "a", "abort":
			fmt.Printf("[CRITICAL] Installation aborted by user.\n")
			os.Exit(1)
		default:
			fmt.Printf("[INFO] Invalid choice. Please type 'r', 'i', or 'a'.\n")
			continue
		}
	}
}
