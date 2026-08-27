package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"skeaAlpineInstaller/tests/harness/internal/mcp"
	"skeaAlpineInstaller/tests/harness/internal/service"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		printUsage(os.Stderr)
		return errors.New("missing command")
	}

	repoRoot, err := detectRepoRoot()
	if err != nil {
		return err
	}

	artifactsRoot := filepath.Join(repoRoot, "tests", "artifacts", "harness")
	harnessService := service.New()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	switch args[0] {
	case "prepare-runner":
		cfg, err := parsePrepareRunnerArgs(args[1:], repoRoot, artifactsRoot)
		if err != nil {
			return err
		}
		result, err := harnessService.PrepareRunner(ctx, &cfg)
		if err != nil {
			return err
		}
		return printJSON(result)
	case "run":
		cfg, err := parseRunArgs(args[1:], repoRoot, artifactsRoot)
		if err != nil {
			return err
		}
		// Clean up any leftover VM processes from a previous failed run.
		if !cfg.KeepTargetRunning {
			cleanupVMs()
		}
		// Always clean up after the test completes or fails.
		defer func() {
			if !cfg.KeepTargetRunning {
				cleanupVMs()
			}
		}()
		result, err := harnessService.RunInstall(ctx, &cfg)
		if err != nil {
			return err
		}
		return printJSON(result)
	case "verify":
		cfg, err := parseVerifyArgs(args[1:], artifactsRoot)
		if err != nil {
			return err
		}
		result, err := harnessService.Verify(ctx, &cfg)
		if err != nil {
			return err
		}
		return printJSON(result)
	case "status":
		result, err := harnessService.Status(ctx, artifactsRoot)
		if err != nil {
			return err
		}
		return printJSON(result)
	case "stop":
		cfg, err := parseStopArgs(args[1:], artifactsRoot)
		if err != nil {
			return err
		}
		result, err := harnessService.Stop(ctx, cfg)
		if err != nil {
			return err
		}
		return printJSON(result)
	case "cleanup":
		cfg, err := parseCleanupArgs(args[1:], artifactsRoot)
		if err != nil {
			return err
		}
		result, err := harnessService.Cleanup(ctx, cfg)
		if err != nil {
			return err
		}
		return printJSON(result)
	case "logs":
		cfg, err := parseLogsArgs(args[1:], artifactsRoot)
		if err != nil {
			return err
		}
		result, err := harnessService.Logs(ctx, cfg)
		if err != nil {
			return err
		}
		return printJSON(result)
	case "mcp":
		cfg, err := parseMCPArgs(args[1:], repoRoot, artifactsRoot)
		if err != nil {
			return err
		}
		server := mcp.NewServer(harnessService, cfg.RepoRoot, cfg.ArtifactsRoot)
		return server.Serve(ctx, os.Stdin, os.Stdout)
	case "help", "--help", "-h":
		printUsage(os.Stdout)
		return nil
	default:
		printUsage(os.Stderr)
		return fmt.Errorf("unknown command %q", args[0])
	}
}

type mcpArgs struct {
	RepoRoot      string
	ArtifactsRoot string
}

func parsePrepareRunnerArgs(args []string, repoRoot, artifactsRoot string) (service.PrepareRunnerConfig, error) {
	defaults := service.PrepareRunnerConfig{
		RepoRoot:        repoRoot,
		ArtifactsRoot:   artifactsRoot,
		CacheDir:        filepath.Join(artifactsRoot, "cache"),
		RunnerImage:     filepath.Join(artifactsRoot, "cache", "runner-images", "skea-runner.qcow2"),
		QemuBinary:      "qemu-system-x86_64",
		QemuImgBinary:   "qemu-img",
		CloudLocaldsBin: "cloud-localds",
		SSHPort:         2222,
		SSHUser:         "skea",
		MemoryMiB:       4096,
		CPUs:            2,
		EnableKVM:       true,
		BootTimeout:     5 * time.Minute,
	}

	bootTimeoutSeconds := 300
	fs := flag.NewFlagSet("prepare-runner", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.StringVar(&defaults.ArtifactsRoot, "artifacts-root", defaults.ArtifactsRoot, "root directory for harness artifacts")
	fs.StringVar(&defaults.CacheDir, "cache-dir", defaults.CacheDir, "cache directory for images and keys")
	fs.StringVar(&defaults.RunnerImage, "runner-image", defaults.RunnerImage, "path to the prepared runner qcow2 image")
	fs.StringVar(&defaults.BaseImageURL, "base-image-url", defaults.BaseImageURL, "URL of the cloud image used to build the runner")
	fs.StringVar(&defaults.BaseImagePath, "base-image-path", defaults.BaseImagePath, "local qcow2 cloud image path")
	fs.StringVar(&defaults.QemuBinary, "qemu-binary", defaults.QemuBinary, "QEMU system binary")
	fs.StringVar(&defaults.QemuImgBinary, "qemu-img-binary", defaults.QemuImgBinary, "qemu-img binary")
	fs.StringVar(&defaults.CloudLocaldsBin, "cloud-localds-binary", defaults.CloudLocaldsBin, "cloud-localds binary")
	fs.StringVar(&defaults.AptProxy, "apt-proxy", defaults.AptProxy, "optional APT proxy URL for runner preparation")
	fs.StringVar(&defaults.SSHUser, "ssh-user", defaults.SSHUser, "runner SSH user created by cloud-init")
	fs.IntVar(&defaults.SSHPort, "ssh-port", defaults.SSHPort, "host port forwarded to runner guest port 22")
	fs.IntVar(&defaults.MemoryMiB, "memory", defaults.MemoryMiB, "runner memory in MiB")
	fs.IntVar(&defaults.CPUs, "cpus", defaults.CPUs, "runner vCPU count")
	fs.IntVar(&bootTimeoutSeconds, "boot-timeout-seconds", bootTimeoutSeconds, "seconds to wait for runner SSH readiness")
	fs.BoolVar(&defaults.EnableKVM, "enable-kvm", defaults.EnableKVM, "enable KVM acceleration (default true; pass --enable-kvm=false to disable)")
	fs.BoolVar(&defaults.Force, "force", false, "rebuild the runner image even if it already exists")
	if err := fs.Parse(args); err != nil {
		return service.PrepareRunnerConfig{}, err
	}
	defaults.BootTimeout = time.Duration(bootTimeoutSeconds) * time.Second
	return defaults, nil
}

func parseRunArgs(args []string, repoRoot, artifactsRoot string) (service.RunConfig, error) {
	defaults := service.RunConfig{
		RepoRoot:            repoRoot,
		ArtifactsRoot:       artifactsRoot,
		RunnerImage:         filepath.Join(artifactsRoot, "cache", "runner-images", "skea-runner.qcow2"),
		QemuBinary:          "qemu-system-x86_64",
		QemuImgBinary:       "qemu-img",
		RunnerSSHUser:       "skea",
		RunnerSSHPort:       2222,
		TargetSSHPort:       2223,
		MemoryMiB:           4096,
		TargetMemoryMiB:     4096,
		CPUs:                2,
		DiskSize:            "16G",
		DiskLayout:          "btrfsStandardServer",
		InstalledUser:       "skea",
		InstalledPassword:   "Alpine.1",
		EnableTPM:           true,
		EnableSecureboot:    true,
		EnableHardening:     true,
		EnableZswap:         true,
		EnableNix:           true,
		EnableKVM:           true,
		BootTimeout:         5 * time.Minute,
		VerificationTimeout: 2 * time.Minute,
	}

	var variablesFile string
	bootTimeoutSeconds := 600
	verifyTimeoutSeconds := 120
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.StringVar(&variablesFile, "f", "", "path to variables.yaml (same as installer)")
	fs.StringVar(&defaults.ArtifactsRoot, "artifacts-root", defaults.ArtifactsRoot, "root directory for harness artifacts")
	fs.StringVar(&defaults.RunnerImage, "runner-image", defaults.RunnerImage, "prepared runner qcow2 image")
	fs.StringVar(&defaults.QemuBinary, "qemu-binary", defaults.QemuBinary, "QEMU system binary")
	fs.StringVar(&defaults.QemuImgBinary, "qemu-img-binary", defaults.QemuImgBinary, "qemu-img binary")
	fs.StringVar(&defaults.RunnerSSHUser, "runner-ssh-user", defaults.RunnerSSHUser, "runner SSH user")
	fs.IntVar(&defaults.RunnerSSHPort, "runner-ssh-port", defaults.RunnerSSHPort, "host port forwarded to runner guest SSH")
	fs.IntVar(&defaults.TargetSSHPort, "target-ssh-port", defaults.TargetSSHPort, "host port forwarded to installed target guest SSH")
	fs.IntVar(&defaults.MemoryMiB, "memory", defaults.MemoryMiB, "runner memory in MiB")
	fs.IntVar(&defaults.TargetMemoryMiB, "target-memory", defaults.TargetMemoryMiB, "installed target memory in MiB")
	fs.IntVar(&defaults.CPUs, "cpus", defaults.CPUs, "vCPU count")
	fs.StringVar(&defaults.DiskSize, "disk-size", defaults.DiskSize, "size of each install target disk")
	fs.IntVar(&bootTimeoutSeconds, "boot-timeout-seconds", bootTimeoutSeconds, "seconds to wait for SSH during runner and target boots")
	fs.IntVar(&verifyTimeoutSeconds, "verify-timeout-seconds", verifyTimeoutSeconds, "seconds to wait for verification commands")
	fs.BoolVar(&defaults.EnableKVM, "enable-kvm", defaults.EnableKVM, "enable KVM acceleration (default true; pass --enable-kvm=false to disable)")
	fs.BoolVar(&defaults.KeepTargetRunning, "keep-target-running", false, "leave the installed target VM running after verification")
	if err := fs.Parse(args); err != nil {
		return service.RunConfig{}, err
	}
	defaults.BootTimeout = time.Duration(bootTimeoutSeconds) * time.Second
	defaults.VerificationTimeout = time.Duration(verifyTimeoutSeconds) * time.Second

	// Load variables.yaml if -f flag provided (required)
	if variablesFile == "" {
		return service.RunConfig{}, fmt.Errorf("usage: skea-harness run -f <path-to-variables.yaml>")
	}
	if err := service.LoadVariablesFile(&defaults, variablesFile); err != nil {
		return service.RunConfig{}, fmt.Errorf("load variables file: %w", err)
	}

	return defaults, nil
}

func parseVerifyArgs(args []string, artifactsRoot string) (service.VerifyConfig, error) {
	defaults := service.VerifyConfig{
		ArtifactsRoot: artifactsRoot,
		SSHPort:       2223,
		SSHUser:       "root",
		PrivateKey:    filepath.Join(artifactsRoot, "cache", "keys", "id_ed25519"),
		InstalledUser: "skea",
		Timeout:       2 * time.Minute,
	}

	timeoutSeconds := 120
	fs := flag.NewFlagSet("verify", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.StringVar(&defaults.ArtifactsRoot, "artifacts-root", defaults.ArtifactsRoot, "root directory for harness artifacts")
	fs.StringVar(&defaults.RunDir, "run-dir", defaults.RunDir, "specific run directory to associate with the verification result")
	fs.IntVar(&defaults.SSHPort, "ssh-port", defaults.SSHPort, "host port forwarded to the installed target guest SSH")
	fs.StringVar(&defaults.SSHUser, "ssh-user", defaults.SSHUser, "SSH user to use for verification")
	fs.StringVar(&defaults.PrivateKey, "private-key", defaults.PrivateKey, "SSH private key for guest access")
	fs.StringVar(&defaults.InstalledUser, "installed-user", defaults.InstalledUser, "expected installed Alpine user")
	fs.IntVar(&timeoutSeconds, "timeout-seconds", timeoutSeconds, "seconds to wait for verification")
	if err := fs.Parse(args); err != nil {
		return service.VerifyConfig{}, err
	}
	defaults.Timeout = time.Duration(timeoutSeconds) * time.Second
	return defaults, nil
}

func parseStopArgs(args []string, artifactsRoot string) (service.StopConfig, error) {
	defaults := service.StopConfig{ArtifactsRoot: artifactsRoot}
	fs := flag.NewFlagSet("stop", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.StringVar(&defaults.ArtifactsRoot, "artifacts-root", defaults.ArtifactsRoot, "root directory for harness artifacts")
	fs.BoolVar(&defaults.Force, "force", false, "use QMP quit instead of system_powerdown")
	if err := fs.Parse(args); err != nil {
		return service.StopConfig{}, err
	}
	return defaults, nil
}

func parseLogsArgs(args []string, artifactsRoot string) (service.LogsConfig, error) {
	defaults := service.LogsConfig{ArtifactsRoot: artifactsRoot, LogName: "installer", Lines: 200}
	fs := flag.NewFlagSet("logs", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.StringVar(&defaults.ArtifactsRoot, "artifacts-root", defaults.ArtifactsRoot, "root directory for harness artifacts")
	fs.StringVar(&defaults.RunDir, "run-dir", defaults.RunDir, "specific run directory to read logs from")
	fs.StringVar(&defaults.LogName, "log", defaults.LogName, "log name: installer, post-install, runner-serial, runner-qemu, target-serial, target-qemu, verify, harness")
	fs.IntVar(&defaults.Lines, "lines", defaults.Lines, "number of trailing lines to return")
	if err := fs.Parse(args); err != nil {
		return service.LogsConfig{}, err
	}
	return defaults, nil
}

func parseCleanupArgs(args []string, artifactsRoot string) (service.CleanupConfig, error) {
	defaults := service.CleanupConfig{ArtifactsRoot: artifactsRoot}
	fs := flag.NewFlagSet("cleanup", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.StringVar(&defaults.ArtifactsRoot, "artifacts-root", defaults.ArtifactsRoot, "root directory for harness artifacts")
	fs.BoolVar(&defaults.Force, "force", false, "kill tracked harness VMs even when the active run still appears healthy")
	if err := fs.Parse(args); err != nil {
		return service.CleanupConfig{}, err
	}
	return defaults, nil
}

func parseMCPArgs(args []string, repoRoot, artifactsRoot string) (mcpArgs, error) {
	defaults := mcpArgs{RepoRoot: repoRoot, ArtifactsRoot: artifactsRoot}
	fs := flag.NewFlagSet("mcp", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.StringVar(&defaults.RepoRoot, "repo-root", defaults.RepoRoot, "repository root for harness operations")
	fs.StringVar(&defaults.ArtifactsRoot, "artifacts-root", defaults.ArtifactsRoot, "root directory for harness artifacts")
	if err := fs.Parse(args); err != nil {
		return mcpArgs{}, err
	}
	return defaults, nil
}

func printJSON(value any) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}

func detectRepoRoot() (string, error) {
	cwd, err := os.Getwd()
	if err == nil {
		if repoRoot, detectErr := detectRepoRootFrom(cwd); detectErr == nil {
			return repoRoot, nil
		}
	}

	executablePath, err := os.Executable()
	if err != nil {
		if cwd != "" {
			return "", fmt.Errorf("could not detect repository root from %s", cwd)
		}
		return "", fmt.Errorf("detect executable path: %w", err)
	}

	resolvedExecutablePath, err := filepath.EvalSymlinks(executablePath)
	if err != nil {
		resolvedExecutablePath = executablePath
	}

	if repoRoot, detectErr := detectRepoRootFrom(filepath.Dir(resolvedExecutablePath)); detectErr == nil {
		return repoRoot, nil
	}

	if cwd != "" {
		return "", fmt.Errorf("could not detect repository root from %s or %s", cwd, resolvedExecutablePath)
	}
	return "", fmt.Errorf("could not detect repository root from %s", resolvedExecutablePath)
}

func detectRepoRootFrom(start string) (string, error) {
	current := start
	for {
		// Check for our setup.sh or the old installer/start.sh
		if _, err := os.Stat(filepath.Join(current, "setup.sh")); err == nil {
			return current, nil
		}
		if _, err := os.Stat(filepath.Join(current, "installer", "start.sh")); err == nil {
			return current, nil
		}

		parent := filepath.Dir(current)
		if parent == current {
			return "", fmt.Errorf("could not detect repository root from %s", start)
		}

		current = parent
	}
}

func cleanupVMs() {
	// Kill any lingering QEMU or swtpm processes from failed runs.
	for _, name := range []string{"qemu-system", "swtpm"} {
		if _, err := exec.LookPath(name); err == nil {
			killByName(name)
		}
	}
}

func killByName(name string) {
	// Find all PIDs for the given process name and kill them.
	out, err := exec.Command("pgrep", "-f", name).Output()
	if err != nil {
		return // no processes found (pgrep exits 1)
	}
	for _, pidStr := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if pid, err := strconv.Atoi(pidStr); err == nil {
			_ = syscall.Kill(pid, syscall.SIGKILL)
		}
	}
}

func printUsage(stream *os.File) {
	fmt.Fprintln(stream, "Usage:")
	fmt.Fprintln(stream, "  skea-harness prepare-runner [flags]")
	fmt.Fprintln(stream, "  skea-harness run [flags]")
	fmt.Fprintln(stream, "  skea-harness verify [flags]")
	fmt.Fprintln(stream, "  skea-harness status")
	fmt.Fprintln(stream, "  skea-harness stop [--force]")
	fmt.Fprintln(stream, "  skea-harness cleanup [--force]")
	fmt.Fprintln(stream, "  skea-harness logs [flags]")
	fmt.Fprintln(stream, "  skea-harness mcp [flags]")
	fmt.Fprintln(stream, "")
	fmt.Fprintln(stream, "Commands:")
	fmt.Fprintln(stream, "  prepare-runner  Download or reuse a cloud image and prepare a reusable runner VM image")
	fmt.Fprintln(stream, "  run             Boot the runner VM, execute the installer, boot the installed target, and verify it")
	fmt.Fprintln(stream, "  verify          Run verification checks against a reachable installed target VM")
	fmt.Fprintln(stream, "  status          Show the persisted active run state and current QMP status")
	fmt.Fprintln(stream, "  stop            Stop the active runner or target VM via QMP")
	fmt.Fprintln(stream, "  cleanup         Kill stale harness-owned QEMU processes and clear stale active-run state")
	fmt.Fprintln(stream, "  logs            Return the tail of a named log file")
	fmt.Fprintln(stream, "  mcp             Start the harness MCP server over stdio")
}
