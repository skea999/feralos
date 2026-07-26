package shell

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// RunCommand executes a shell command. If it fails, it asks the user to Retry or Ignore.
func RunCommand(name string, args ...string) (string, error) {
	reader := bufio.NewReader(os.Stdin)

	for {
		fmt.Printf("[DEBUG] Executing: %s %v\n", name, args)
		var stdout, stderr bytes.Buffer
		cmd := exec.Command(name, args...)
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		err := cmd.Run()

		// Log result in debug mode
		if stdout.Len() > 0 {
			out := strings.TrimSpace(stdout.String())
			if len(out) > 500 {
				out = out[:500] + "...(truncated)"
			}
			fmt.Printf("[DEBUG] Stdout: %s\n", out)
		}
		if stderr.Len() > 0 {
			fmt.Printf("[DEBUG] Stderr: %s\n", strings.TrimSpace(stderr.String()))
		}

		if err == nil {
			return stdout.String(), nil
		}

		// Check for non-interactive mode
		if os.Getenv("SKEA_NONINTERACTIVE") == "1" {
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

		// If command fails, show error and ask for user input
		fmt.Printf("\n[ERROR] Command failed: %s %v\n", name, args)
		fmt.Printf("[ERROR] Details: %s\n", strings.TrimSpace(stderr.String()))
		fmt.Printf("[ERROR] System Message: %v\n", err)
		fmt.Println("--------------------------------------------------")
		fmt.Print("Options: [r]etry, [i]gnore, [a]bort: ")

		input, _ := reader.ReadString('\n')
		choice := strings.ToLower(strings.TrimSpace(input))

		switch choice {
		case "r", "retry":
			fmt.Println("[INFO] Retrying command...")
			continue // Restarts the loop
		case "i", "ignore":
			fmt.Println("[WARNING] Ignoring error and proceeding...")
			return stdout.String(), nil // Returns what it has, but no error
		case "a", "abort":
			fmt.Println("[CRITICAL] Installation aborted by user.")
			os.Exit(1)
		default:
			fmt.Println("[INFO] Invalid choice. Please type 'r', 'i', or 'a'.")
			continue
		}
	}
}

// RunChrootCommand executes a command inside the specified chroot directory
func RunChrootCommand(chroot string, command string) (string, error) {
	fmt.Printf("[CHROOT] Executing: %s\n", command)
	// Costruiamo il comando: chroot /mnt /bin/sh -c "comando"
	return RunCommand("chroot", chroot, "/bin/sh", "-c", command)
}

// RunCommandStrict executes a shell command and ALWAYS returns errors,
// even in non-interactive mode. Use for critical steps (bootstrap, etc.)
func RunCommandStrict(name string, args ...string) (string, error) {
	fmt.Printf("[DEBUG] Executing: %s %v\n", name, args)
	var stdout, stderr bytes.Buffer
	cmd := exec.Command(name, args...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	// Log result in debug mode
	if stdout.Len() > 0 {
		out := strings.TrimSpace(stdout.String())
		if len(out) > 500 {
			out = out[:500] + "...(truncated)"
		}
		fmt.Printf("[DEBUG] Stdout: %s\n", out)
	}
	if stderr.Len() > 0 {
		fmt.Printf("[DEBUG] Stderr: %s\n", strings.TrimSpace(stderr.String()))
	}

	if err != nil {
		fmt.Printf("\n[ERROR] Command failed: %s %v\n", name, args)
		fmt.Printf("[ERROR] Details: %s\n", strings.TrimSpace(stderr.String()))
		fmt.Printf("[ERROR] System Message: %v\n", err)
		return stdout.String(), fmt.Errorf("command failed: %s %v: %w", name, args, err)
	}
	return stdout.String(), nil
}
