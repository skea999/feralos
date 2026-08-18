package config

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type RunConfig struct {
	RepoRoot      string
	RunnerImage   string
	QemuBinary    string
	QemuImgBinary string
	ArtifactsRoot string
	DiskSize      string
	SSHPort       int
	MemoryMiB     int
	CPUs          int
	EnableKVM     bool
	DryRun        bool
}

func ParseRunArgs(args []string) (RunConfig, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return RunConfig{}, fmt.Errorf("get working directory: %w", err)
	}

	repoRoot, err := detectRepoRoot(cwd)
	if err != nil {
		return RunConfig{}, err
	}

	defaults := RunConfig{
		RepoRoot:      repoRoot,
		QemuBinary:    "qemu-system-x86_64",
		QemuImgBinary: "qemu-img",
		ArtifactsRoot: filepath.Join(repoRoot, "tests", "artifacts", "harness"),
		DiskSize:      "16G",
		SSHPort:       2222,
		MemoryMiB:     4096,
		CPUs:          2,
	}

	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	fs.StringVar(&defaults.RunnerImage, "runner-image", "", "path to the reusable runner qcow2 image")
	fs.StringVar(&defaults.QemuBinary, "qemu-binary", defaults.QemuBinary, "QEMU system binary")
	fs.StringVar(&defaults.QemuImgBinary, "qemu-img-binary", defaults.QemuImgBinary, "qemu-img binary")
	fs.StringVar(&defaults.ArtifactsRoot, "artifacts-root", defaults.ArtifactsRoot, "root directory for harness artifacts")
	fs.StringVar(&defaults.DiskSize, "disk-size", defaults.DiskSize, "size of each disposable install target disk")
	fs.IntVar(&defaults.SSHPort, "ssh-port", defaults.SSHPort, "host port forwarded to guest port 22")
	fs.IntVar(&defaults.MemoryMiB, "memory", defaults.MemoryMiB, "guest memory in MiB")
	fs.IntVar(&defaults.CPUs, "cpus", defaults.CPUs, "guest virtual CPU count")
	fs.BoolVar(&defaults.EnableKVM, "enable-kvm", false, "enable KVM acceleration")
	fs.BoolVar(&defaults.DryRun, "dry-run", false, "print the QEMU launch plan without starting the VM")

	if parseErr := fs.Parse(args); parseErr != nil {
		return RunConfig{}, parseErr
	}

	if validateErr := validate(&defaults); validateErr != nil {
		return RunConfig{}, validateErr
	}

	defaults.RunnerImage, err = filepath.Abs(defaults.RunnerImage)
	if err != nil {
		return RunConfig{}, fmt.Errorf("resolve runner image path: %w", err)
	}

	defaults.ArtifactsRoot, err = filepath.Abs(defaults.ArtifactsRoot)
	if err != nil {
		return RunConfig{}, fmt.Errorf("resolve artifacts root: %w", err)
	}

	return defaults, nil
}

func validate(cfg *RunConfig) error {
	if strings.TrimSpace(cfg.RunnerImage) == "" {
		return fmt.Errorf("--runner-image is required")
	}

	if cfg.MemoryMiB <= 0 {
		return fmt.Errorf("--memory must be greater than zero")
	}

	if cfg.CPUs <= 0 {
		return fmt.Errorf("--cpus must be greater than zero")
	}

	if cfg.SSHPort <= 0 || cfg.SSHPort > 65535 {
		return fmt.Errorf("--ssh-port must be between 1 and 65535")
	}

	if strings.TrimSpace(cfg.DiskSize) == "" {
		return fmt.Errorf("--disk-size is required")
	}

	return nil
}

func detectRepoRoot(start string) (string, error) {
	current := start

	for {
		candidate := filepath.Join(current, "start.sh")
		if _, err := os.Stat(candidate); err == nil {
			return current, nil
		}

		parent := filepath.Dir(current)
		if parent == current {
			return "", fmt.Errorf("could not detect repository root from %s", start)
		}

		current = parent
	}
}
