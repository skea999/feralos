package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"skeaAlpineInstaller/tests/harness/internal/cache"
	"skeaAlpineInstaller/tests/harness/internal/guest"
	"skeaAlpineInstaller/tests/harness/internal/qemu"
	"skeaAlpineInstaller/tests/harness/internal/runstate"

	"gopkg.in/yaml.v3"
)

const defaultRunnerImageURL = "https://cloud.debian.org/images/cloud/trixie/latest/debian-13-genericcloud-amd64.qcow2"
const defaultAlpineMirrorURL = "https://dl-cdn.alpinelinux.org/alpine"

const (
	statusRunning             = "running"
	statusFailed              = "failed"
	layoutBtrfsStandardServer = "btrfsStandardServer"
	layoutBtrfsRaid0          = "btrfsRaid0"
	sshUserSkea               = "skea"
	desktopNiri               = "niri"
	// targetShutdownTimeout bounds the wait for the target VM to power off
	// after a shutdown request — fail the run instead of hanging forever.
	targetShutdownTimeout = 5 * time.Minute
)

type Service struct{}

// harnessLog is the run-level log file. Set at start of RunInstall, nil elsewhere.
var harnessLog *os.File

// harnessProgress receives run progress/phase messages (CLI: stdout). The
// MCP server switches it to stderr to keep the stdio JSON-RPC stream clean.
var harnessProgress io.Writer = os.Stdout

// SetProgressWriter overrides where run progress messages are written.
func SetProgressWriter(w io.Writer) {
	if w != nil {
		harnessProgress = w
	}
}

// logHarness writes to harness.log (when a run is active) and the progress
// writer (stdout by default).
func logHarness(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	if harnessLog != nil {
		fmt.Fprint(harnessLog, msg)
		_ = harnessLog.Sync()
	}
	fmt.Fprint(harnessProgress, msg)
}

// runPhaseTotal is the number of phases in the `run` flow, shown by phase().
const runPhaseTotal = 9

// phase announces the current run phase on stdout and in harness.log so a
// stuck run is diagnosable without attaching to the VMs.
func phase(n int, name string) {
	logHarness("[harness] phase %d/%d: %s\n", n, runPhaseTotal, name)
}

type PrepareRunnerConfig struct {
	ArtifactsRoot   string
	RunnerImage     string
	BaseImageURL    string
	BaseImagePath   string
	CacheDir        string
	QemuBinary      string
	QemuImgBinary   string
	CloudLocaldsBin string
	SSHPort         int
	SSHUser         string
	MemoryMiB       int
	CPUs            int
	EnableKVM       bool
	Force           bool
	AptProxy        string
	Packages        []string
	RunnerImageSize string
	WaitTimeout     time.Duration
	BootTimeout     time.Duration
	RepoRoot        string
}

type PrepareRunnerResult struct {
	RunDir        string        `json:"runDir"`
	RunnerImage   string        `json:"runnerImage"`
	BaseImagePath string        `json:"baseImagePath"`
	SSHPort       int           `json:"sshPort"`
	SSHUser       string        `json:"sshUser"`
	KeyPair       guest.KeyPair `json:"keyPair"`
	SerialLog     string        `json:"serialLog"`
	QemuLog       string        `json:"qemuLog"`
}

type RunConfig struct {
	ArtifactsRoot       string
	RepoRoot            string
	RunnerImage         string
	QemuBinary          string
	QemuImgBinary       string
	RunnerSSHUser       string
	RunnerSSHPort       int
	TargetSSHPort       int
	MemoryMiB           int
	TargetMemoryMiB     int
	CPUs                int
	EnableTPM           bool
	EnableSecureboot    bool
	EnableHardening     bool
	EnablePodman        bool `yaml:"enablePodman"`
	EnableWebmin        bool `yaml:"enableWebmin"`
	EnableDinit         bool `yaml:"enableDinit"`
	EnableZswap         bool
	EnableNix           bool
	EnableSwap          bool
	SwapSize            string
	EnableKVM           bool
	DiskSize            string
	DiskLayout          string
	DesktopEnvironment  string
	AlpineMirror        string
	InstalledUser       string
	InstalledPassword   string
	NixPackages         []string
	FlatpakPackages     []string
	KeepTargetRunning   bool
	WaitTimeout         time.Duration
	BootTimeout         time.Duration
	VerificationTimeout time.Duration
	VariablesFile       string // -f flag: path to variables.yaml
}

// VariablesYAML represents the structure of variables.yaml
type VariablesYAML struct {
	Variables struct {
		Unattended              bool     `yaml:"unattended"`
		Disks                   []string `yaml:"disks"`
		UserName                string   `yaml:"userName"`
		Password                string   `yaml:"password"`
		Hostname                string   `yaml:"hostname"`
		Timezone                string   `yaml:"timezone"`
		Locale                  string   `yaml:"locale"`
		Keymap                  string   `yaml:"keymap"`
		Mirror                  string   `yaml:"mirror"`
		Arch                    string   `yaml:"arch"`
		Branch                  string   `yaml:"branch"`
		DiskLayout              string   `yaml:"diskLayout"`
		DesktopEnvironment      string   `yaml:"desktopEnvironment"`
		VideoDriver             string   `yaml:"videoDriver"`
		SSHAuthorizedKey        string   `yaml:"sshAuthorizedKey"`
		DefaultNetworkInterface string   `yaml:"defaultNetworkInterface"`
		EnableSecureboot        bool     `yaml:"enableSecureboot"`
		EnableTPM               bool     `yaml:"enableTPM"`
		EnableHardening         bool     `yaml:"enableHardening"`
		EnableZswap             bool     `yaml:"enableZswap"`
		EnableNix               bool     `yaml:"enableNix"`
		EnableSwap              bool     `yaml:"enableSwap"`
		SwapSize                string   `yaml:"swapSize"`
		EnableDebug             bool     `yaml:"enableDebug"`
		EnablePodman            bool     `yaml:"enablePodman"`
		EnableWebmin            bool     `yaml:"enableWebmin"`
		EnableDinit             bool     `yaml:"enableDinit"`
		ExtraPackages           []string `yaml:"extraPackages"`
		NixPackages             []string `yaml:"nixPackages"`
		FlatpakPackages         []string `yaml:"flatpakPackages"`
	} `yaml:"variables"`
}

// LoadVariablesFile loads a variables.yaml and applies values to RunConfig
func LoadVariablesFile(cfg *RunConfig, filePath string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("read variables file: %w", err)
	}

	var vars VariablesYAML
	if err := yaml.Unmarshal(data, &vars); err != nil {
		return fmt.Errorf("parse variables file: %w", err)
	}

	v := vars.Variables

	// Apply values from YAML
	if v.UserName != "" {
		cfg.InstalledUser = v.UserName
	}
	if v.Password != "" {
		cfg.InstalledPassword = v.Password
	}
	if v.DiskLayout != "" {
		cfg.DiskLayout = v.DiskLayout
	}
	if v.DesktopEnvironment != "" {
		cfg.DesktopEnvironment = v.DesktopEnvironment
	}
	if v.Mirror != "" {
		cfg.AlpineMirror = v.Mirror
	}
	// Hostname and default network interface are consumed by the installer;
	// the harness does not need them for verification.

	// Feature toggles
	cfg.EnableSecureboot = v.EnableSecureboot
	cfg.EnableTPM = v.EnableTPM
	cfg.EnableHardening = v.EnableHardening
	cfg.EnableZswap = v.EnableZswap
	cfg.EnableNix = v.EnableNix
	cfg.EnableSwap = v.EnableSwap
	cfg.SwapSize = v.SwapSize
	cfg.EnablePodman = v.EnablePodman
	cfg.EnableWebmin = v.EnableWebmin
	cfg.EnableDinit = v.EnableDinit
	if len(v.NixPackages) > 0 {
		cfg.NixPackages = v.NixPackages
	}
	if len(v.FlatpakPackages) > 0 {
		cfg.FlatpakPackages = v.FlatpakPackages
	}

	return nil
}

type RunResult struct {
	RunID             string        `json:"runId"`
	RunDir            string        `json:"runDir"`
	Disk1Path         string        `json:"disk1Path"`
	Disk2Path         string        `json:"disk2Path"`
	RunnerSerialLog   string        `json:"runnerSerialLog"`
	TargetSerialLog   string        `json:"targetSerialLog,omitempty"`
	InstallerLogPath  string        `json:"installerLogPath"`
	VerifyLogPath     string        `json:"verifyLogPath,omitempty"`
	RunnerQemuLog     string        `json:"runnerQemuLog"`
	TargetQemuLog     string        `json:"targetQemuLog,omitempty"`
	TargetFirmwareLog string        `json:"targetFirmwareLog,omitempty"`
	VerifyResult      *VerifyResult `json:"verifyResult,omitempty"`
	KeyPair           guest.KeyPair `json:"keyPair"`
	TargetRunning     bool          `json:"targetRunning"`
	OVMFVarsPath      string        `json:"ovmfVarsPath,omitempty"`
	CacheDir          string        `json:"cacheDir,omitempty"`
	CacheMirrorURL    string        `json:"cacheMirrorUrl,omitempty"`
	CacheUpstreamURL  string        `json:"cacheUpstreamUrl,omitempty"`
	StartedAt         time.Time     `json:"startedAt"`
	FinishedAt        time.Time     `json:"finishedAt"`
}

type VerifyConfig struct {
	ArtifactsRoot      string
	RunDir             string
	SSHPort            int
	SSHUser            string
	PrivateKey         string
	InstalledUser      string
	DesktopEnvironment string
	EnableSwap         bool
	EnableSecureboot   bool
	EnableTPM          bool
	EnablePodman       bool
	EnableWebmin       bool
	EnableDinit        bool
	EnableNix          bool
	SwapSize           string
	DiskLayout         string
	NixPackages        []string
	FlatpakPackages    []string
	Timeout            time.Duration
}

type VerifyResult struct {
	RunDir  string                 `json:"runDir"`
	Success bool                   `json:"success"`
	Checks  map[string]CheckResult `json:"checks"`
}

type CheckResult struct {
	Pass   bool   `json:"pass"`
	Detail string `json:"detail"`
}

type StatusResult struct {
	Active      *runstate.ActiveRun `json:"active,omitempty"`
	QMPStatus   *qemu.Status        `json:"qmpStatus,omitempty"`
	LatestRun   string              `json:"latestRun,omitempty"`
	ActiveState string              `json:"activeStateFile"`
}

type StopConfig struct {
	ArtifactsRoot string
	Force         bool
}

type StopResult struct {
	Stopped bool   `json:"stopped"`
	Method  string `json:"method"`
	RunID   string `json:"runId,omitempty"`
}

type CleanupConfig struct {
	ArtifactsRoot string
	RunnerSSHPort int
	TargetSSHPort int
	Force         bool
}

type CleanupResult struct {
	ActiveRunID  string `json:"activeRunId,omitempty"`
	ClearedState bool   `json:"clearedState"`
	KilledPIDs   []int  `json:"killedPids,omitempty"`
	BusyPorts    []int  `json:"busyPorts,omitempty"`
	Skipped      bool   `json:"skipped"`
	Reason       string `json:"reason,omitempty"`
}

type LogsConfig struct {
	ArtifactsRoot string
	RunDir        string
	LogName       string
	Lines         int
}

type LogsResult struct {
	Path    string `json:"path"`
	Content string `json:"content"`
	Lines   int    `json:"lines"`
}

const clevisRebootSerialOffsetFile = "target-serial-clevis-reboot.offset"

type SerialWriteConfig struct {
	ArtifactsRoot string
	RunDir        string
	Data          string
}

type SerialWriteResult struct {
	RunID        string `json:"runId,omitempty"`
	RunDir       string `json:"runDir"`
	SocketPath   string `json:"socketPath"`
	BytesWritten int    `json:"bytesWritten"`
	Target       string `json:"target"`
}

func New() *Service {
	return &Service{}
}

func (service *Service) PrepareRunner(ctx context.Context, cfg *PrepareRunnerConfig) (PrepareRunnerResult, error) {
	if err := ensurePrepareDefaults(cfg); err != nil {
		return PrepareRunnerResult{}, err
	}
	if err := service.cleanupBeforeLaunch(ctx, cfg.ArtifactsRoot, cfg.SSHPort, 0); err != nil {
		return PrepareRunnerResult{}, err
	}

	// Clean old prepare directories, keep last 1
	cleanOldRuns(filepath.Join(cfg.ArtifactsRoot, "prepare"), 1)

	keyPair, err := guest.EnsureKeyPair(filepath.Join(cfg.CacheDir, "keys"))
	if err != nil {
		return PrepareRunnerResult{}, err
	}

	baseImagePath := cfg.BaseImagePath
	if baseImagePath == "" {
		baseImagePath = filepath.Join(cfg.CacheDir, "base-images", filepath.Base(cfg.BaseImageURL))
		if _, dlErr := cache.EnsureDownloaded(ctx, cfg.BaseImageURL, baseImagePath); dlErr != nil {
			return PrepareRunnerResult{}, dlErr
		}
	}

	if _, statErr := os.Stat(baseImagePath); statErr != nil {
		return PrepareRunnerResult{}, fmt.Errorf("base image %s: %w", baseImagePath, statErr)
	}

	if cfg.RunnerImage == "" {
		cfg.RunnerImage = filepath.Join(cfg.CacheDir, "runner-images", "skea-runner.qcow2")
	}

	if cfg.Force {
		_ = os.Remove(cfg.RunnerImage)
	}

	runDir := filepath.Join(cfg.ArtifactsRoot, "prepare", time.Now().UTC().Format("2006-01-02T15-04-05Z"))
	serialLog := filepath.Join(runDir, "serial.log")
	qemuLog := filepath.Join(runDir, "qemu.log")
	qmpPath := filepath.Join(runDir, "qmp.sock")
	seedISO := filepath.Join(runDir, "seed.iso")
	userData := filepath.Join(runDir, "user-data")
	metaData := filepath.Join(runDir, "meta-data")

	if mkdirErr := os.MkdirAll(runDir, 0o755); mkdirErr != nil {
		return PrepareRunnerResult{}, fmt.Errorf("create runner work directory: %w", mkdirErr)
	}

	if _, statErr := os.Stat(cfg.RunnerImage); os.IsNotExist(statErr) {
		if overlayErr := qemu.CreateOverlay(cfg.QemuImgBinary, baseImagePath, cfg.RunnerImage); overlayErr != nil {
			return PrepareRunnerResult{}, overlayErr
		}
		// Base image is 327MB; resize overlay so cloud-init growpart/resizefs
		// can expand the filesystem to fit (cloud-init + apt need headroom).
		if resizeErr := qemu.ResizeDisk(cfg.QemuImgBinary, cfg.RunnerImage, cfg.RunnerImageSize); resizeErr != nil {
			return PrepareRunnerResult{}, resizeErr
		}
	}

	if writeErr := writeCloudInitFiles(userData, metaData, cfg, keyPair); writeErr != nil {
		return PrepareRunnerResult{}, writeErr
	}

	if seedErr := buildSeedISO(cfg.CloudLocaldsBin, seedISO, userData, metaData); seedErr != nil {
		return PrepareRunnerResult{}, seedErr
	}

	stateStore := runstate.New(cfg.ArtifactsRoot)
	active := runstate.ActiveRun{
		RunID:           filepath.Base(runDir),
		Mode:            "prepare-runner",
		Status:          statusRunning,
		Phase:           "booting-runner",
		RunDir:          runDir,
		RunnerQMPPath:   qmpPath,
		RunnerSerialLog: serialLog,
		RunnerQemuLog:   qemuLog,
		RunnerSSHPort:   cfg.SSHPort,
	}
	_ = stateStore.Save(&active)

	args := []string{
		"-name", "skea-runner-prep",
		"-m", fmt.Sprintf("%d", cfg.MemoryMiB),
		"-smp", fmt.Sprintf("%d", cfg.CPUs),
		"-nographic",
		"-serial", "file:" + serialLog,
		"-qmp", "unix:" + qmpPath + ",server=on,wait=off",
		"-netdev", fmt.Sprintf("user,id=net0,hostfwd=tcp::%d-:22", cfg.SSHPort),
		"-device", "virtio-net-pci,netdev=net0",
		"-drive", "if=virtio,format=qcow2,file=" + cfg.RunnerImage,
		"-cdrom", seedISO,
	}
	if cfg.EnableKVM {
		args = append(args, "-enable-kvm")
	}

	instance, err := qemu.Start(ctx, &qemu.LaunchConfig{
		Binary:       cfg.QemuBinary,
		Args:         args,
		LogPath:      qemuLog,
		QMPPath:      qmpPath,
		SerialLog:    serialLog,
		StartTimeout: 10 * time.Second,
	})
	if err != nil {
		active.Status = statusFailed
		active.Error = err.Error()
		_ = stateStore.Save(&active)
		return PrepareRunnerResult{}, err
	}
	defer func() {
		_ = instance.Kill()
	}()

	runnerGuest := guest.Options{User: cfg.SSHUser, Port: cfg.SSHPort, PrivateKey: keyPair.PrivateKeyPath, Shell: "bash"}
	active.Phase = "waiting-for-ssh"
	_ = stateStore.Save(&active)
	if err := guest.WaitForSSH(ctx, runnerGuest, cfg.BootTimeout); err != nil {
		_ = instance.Kill()
		active.Status = statusFailed
		active.Error = err.Error()
		_ = stateStore.Save(&active)
		return PrepareRunnerResult{}, err
	}

	active.Phase = "waiting-for-cloud-init"
	_ = stateStore.Save(&active)
	if err := waitForGuestCommand(ctx, runnerGuest, "test -f /var/lib/cloud/instance/boot-finished", cfg.WaitTimeout); err != nil {
		_ = instance.Kill()
		active.Status = statusFailed
		active.Error = err.Error()
		_ = stateStore.Save(&active)
		return PrepareRunnerResult{}, err
	}

	active.Phase = "powering-off"
	_ = stateStore.Save(&active)
	if err := guest.Run(ctx, runnerGuest, "sudo poweroff", io.Discard, io.Discard); err != nil {
		_ = qemu.SystemPowerdown(qmpPath)
	}

	if err := instance.Wait(); err != nil {
		active.Status = statusFailed
		active.Error = err.Error()
		_ = stateStore.Save(&active)
		return PrepareRunnerResult{}, fmt.Errorf("runner preparation VM failed: %w", err)
	}
	instance = nil

	_ = stateStore.Clear()

	return PrepareRunnerResult{
		RunDir:        runDir,
		RunnerImage:   cfg.RunnerImage,
		BaseImagePath: baseImagePath,
		SSHPort:       cfg.SSHPort,
		SSHUser:       cfg.SSHUser,
		KeyPair:       keyPair,
		SerialLog:     serialLog,
		QemuLog:       qemuLog,
	}, nil
}

func (service *Service) RunInstall(ctx context.Context, cfg *RunConfig) (RunResult, error) {
	if err := ensureRunDefaults(cfg); err != nil {
		return RunResult{}, err
	}
	if err := service.cleanupBeforeLaunch(ctx, cfg.ArtifactsRoot, cfg.RunnerSSHPort, cfg.TargetSSHPort); err != nil {
		return RunResult{}, err
	}

	keyPair, err := guest.EnsureKeyPair(filepath.Join(cfg.ArtifactsRoot, "cache", "keys"))
	if err != nil {
		return RunResult{}, err
	}

	runID := time.Now().UTC().Format("2006-01-02T15-04-05Z")
	runDir := filepath.Join(cfg.ArtifactsRoot, "runs", runID)
	if mkdirErr := os.MkdirAll(runDir, 0o755); mkdirErr != nil {
		return RunResult{}, fmt.Errorf("create run directory: %w", mkdirErr)
	}

	// Capture harness output to harness.log in the run directory.
	var logErr error
	harnessLog, logErr = os.Create(filepath.Join(runDir, "harness.log"))
	if logErr == nil {
		fmt.Fprintf(harnessLog, "run: %s\nlayout: %s\n", runID, cfg.DiskLayout)
		_ = harnessLog.Sync()
	}

	// Clean up old runs, keep only the last 2
	cleanOldRuns(filepath.Join(cfg.ArtifactsRoot, "runs"), 2)

	disk1Path := filepath.Join(runDir, "install-disk-1.qcow2")
	disk2Path := filepath.Join(runDir, "install-disk-2.qcow2")
	runnerSerialLog := filepath.Join(runDir, "runner-serial.log")
	runnerQemuLog := filepath.Join(runDir, "runner-qemu.log")
	runnerQMPPath := filepath.Join(runDir, "runner-qmp.sock")
	installerLogPath := filepath.Join(runDir, "installer.log")
	verifyLogPath := filepath.Join(runDir, "verify.md")
	tpmSocketPath := filepath.Join(runDir, "tpm.sock")
	tpmControlPath := filepath.Join(runDir, "tpm.sock.ctrl")
	tpmStateDir := filepath.Join(runDir, "tpm-state")
	tpmPIDPath := filepath.Join(runDir, "tpm.pid")
	tpmLogPath := filepath.Join(runDir, "tpm.log")
	alpineCacheDir := filepath.Join(cfg.ArtifactsRoot, "cache", "web", "alpine")
	alpineUpstream, err := resolveHarnessAlpineMirror(cfg)
	if err != nil {
		return RunResult{}, err
	}
	alpineCacheServer, err := cache.StartHTTPMirrorServer(ctx, cache.HTTPMirrorServerConfig{
		CacheDir:    alpineCacheDir,
		UpstreamURL: alpineUpstream,
	})
	if err != nil {
		return RunResult{}, fmt.Errorf("start alpine cache server: %w", err)
	}
	cfg.AlpineMirror = alpineCacheServer.GuestURL

	if diskErr := qemu.CreateDisk(cfg.QemuImgBinary, disk1Path, cfg.DiskSize); diskErr != nil {
		return RunResult{}, diskErr
	}
	// Only create second install disk for multi-disk layouts (btrfsStandardServer, btrfsRaid0)
	if cfg.DiskLayout == layoutBtrfsStandardServer || cfg.DiskLayout == layoutBtrfsRaid0 {
		if disk2Err := qemu.CreateDisk(cfg.QemuImgBinary, disk2Path, cfg.DiskSize); disk2Err != nil {
			return RunResult{}, disk2Err
		}
	}

	stopTPM := false
	if tpmErr := qemu.StartSoftwareTPM(&qemu.SoftwareTPMConfig{
		SocketPath:   tpmSocketPath,
		ControlPath:  tpmControlPath,
		StateDir:     tpmStateDir,
		PIDFile:      tpmPIDPath,
		LogPath:      tpmLogPath,
		StartTimeout: 10 * time.Second,
	}); tpmErr != nil {
		return RunResult{}, tpmErr
	}
	stopTPM = true
	defer func() {
		if stopTPM {
			_ = qemu.StopSoftwareTPM(tpmPIDPath)
		}
	}()

	stateStore := runstate.New(cfg.ArtifactsRoot)
	active := runstate.ActiveRun{
		RunID:            runID,
		Mode:             "run-install",
		Status:           statusRunning,
		Phase:            "booting-runner",
		RunDir:           runDir,
		RunnerQMPPath:    runnerQMPPath,
		RunnerSerialLog:  runnerSerialLog,
		RunnerQemuLog:    runnerQemuLog,
		InstallerLogPath: installerLogPath,
		VerifyLogPath:    verifyLogPath,
		RunnerSSHPort:    cfg.RunnerSSHPort,
		TargetSSHPort:    cfg.TargetSSHPort,
	}
	_ = stateStore.Save(&active)

	runnerArgs := []string{
		"-name", "skea-runner",
		"-m", fmt.Sprintf("%d", cfg.MemoryMiB),
		"-smp", fmt.Sprintf("%d", cfg.CPUs),
		"-nographic",
		"-serial", "file:" + runnerSerialLog,
		"-qmp", "unix:" + runnerQMPPath + ",server=on,wait=off",
		"-netdev", fmt.Sprintf("user,id=net0,hostfwd=tcp::%d-:22", cfg.RunnerSSHPort),
		"-device", "virtio-net-pci,netdev=net0",
		"-drive", "if=virtio,format=qcow2,file=" + cfg.RunnerImage,
		"-drive", "if=virtio,format=qcow2,file=" + disk1Path,
	}
	if cfg.DiskLayout == layoutBtrfsStandardServer {
		runnerArgs = append(runnerArgs,
			"-drive", "if=virtio,format=qcow2,file="+disk2Path,
		)
	} else if cfg.DiskLayout == layoutBtrfsRaid0 {
		runnerArgs = append(runnerArgs,
			"-drive", "if=virtio,format=qcow2,file="+disk2Path,
		)
	}
	if cfg.EnableKVM {
		runnerArgs = append(runnerArgs, "-enable-kvm")
	}
	// Always pass swtpm to runner — binaries need /dev/tpmrm0 during install
	runnerArgs = append(runnerArgs, tpmQEMUArgs(tpmControlPath)...)

	phase(1, "boot runner")
	runnerInstance, err := qemu.Start(ctx, &qemu.LaunchConfig{
		Binary:       cfg.QemuBinary,
		Args:         runnerArgs,
		LogPath:      runnerQemuLog,
		QMPPath:      runnerQMPPath,
		SerialLog:    runnerSerialLog,
		StartTimeout: 10 * time.Second,
	})
	if err != nil {
		active.Status = statusFailed
		active.Error = err.Error()
		_ = stateStore.Save(&active)
		return RunResult{}, err
	}
	defer func() {
		_ = runnerInstance.Kill()
	}()

	runnerGuest := guest.Options{User: cfg.RunnerSSHUser, Port: cfg.RunnerSSHPort, PrivateKey: keyPair.PrivateKeyPath, Shell: "bash"}
	active.Phase = "waiting-for-runner-ssh"
	_ = stateStore.Save(&active)
	if sshErr := guest.WaitForSSH(ctx, runnerGuest, cfg.BootTimeout); sshErr != nil {
		_ = runnerInstance.Kill()
		active.Status = statusFailed
		active.Error = sshErr.Error()
		_ = stateStore.Save(&active)
		return RunResult{}, sshErr
	}

	active.Phase = "copying-repository"
	_ = stateStore.Save(&active)
	phase(2, "run installer")
	remoteRepoRoot := fmt.Sprintf("/home/%s/skeaAlpineInstaller", cfg.RunnerSSHUser)
	remoteRepoArchive := remoteRepoRoot + ".tar.gz"
	repoArchivePath := filepath.Join(runDir, "repo.tgz")

	if archiveErr := createRepositoryArchive(cfg.RepoRoot, repoArchivePath); archiveErr != nil {
		_ = runnerInstance.Kill()
		active.Status = statusFailed
		active.Error = archiveErr.Error()
		_ = stateStore.Save(&active)
		return RunResult{}, archiveErr
	}

	if copyErr := guest.CopyToGuest(ctx, runnerGuest, repoArchivePath, remoteRepoArchive, false); copyErr != nil {
		_ = runnerInstance.Kill()
		active.Status = statusFailed
		active.Error = copyErr.Error()
		_ = stateStore.Save(&active)
		return RunResult{}, copyErr
	}

	unpackCommand := strings.Join([]string{
		"sudo rm -rf " + shellQuote(remoteRepoRoot),
		"mkdir -p " + shellQuote(remoteRepoRoot),
		"tar -xzf " + shellQuote(remoteRepoArchive) + " -C " + shellQuote(remoteRepoRoot),
		"rm -f " + shellQuote(remoteRepoArchive),
	}, " && ")
	if _, unpackErr := guest.Output(ctx, runnerGuest, unpackCommand); unpackErr != nil {
		_ = runnerInstance.Kill()
		active.Status = statusFailed
		active.Error = unpackErr.Error()
		_ = stateStore.Save(&active)
		return RunResult{}, unpackErr
	}

	varsPath := filepath.Join(runDir, "variables.yaml")
	if varsErr := writeHarnessVariables(cfg, keyPair, varsPath, alpineCacheServer.GuestURL); varsErr != nil {
		_ = runnerInstance.Kill()
		active.Status = statusFailed
		active.Error = varsErr.Error()
		_ = stateStore.Save(&active)
		return RunResult{}, varsErr
	}

	if copyVarsErr := guest.CopyToGuest(ctx, runnerGuest, varsPath, remoteRepoRoot, false); copyVarsErr != nil {
		_ = runnerInstance.Kill()
		active.Status = statusFailed
		active.Error = copyVarsErr.Error()
		_ = stateStore.Save(&active)
		return RunResult{}, copyVarsErr
	}

	setupGitShimPath := filepath.Join(runDir, "git")
	if gitShimErr := writeHarnessSetupGitShim(setupGitShimPath); gitShimErr != nil {
		_ = runnerInstance.Kill()
		active.Status = statusFailed
		active.Error = gitShimErr.Error()
		_ = stateStore.Save(&active)
		return RunResult{}, gitShimErr
	}

	remoteHarnessBinDir := filepath.Join(remoteRepoRoot, ".skea-harness-bin")
	if _, mkdirBinErr := guest.Output(ctx, runnerGuest, "mkdir -p "+shellQuote(remoteHarnessBinDir)); mkdirBinErr != nil {
		_ = runnerInstance.Kill()
		active.Status = statusFailed
		active.Error = mkdirBinErr.Error()
		_ = stateStore.Save(&active)
		return RunResult{}, mkdirBinErr
	}
	if copyGitErr := guest.CopyToGuest(ctx, runnerGuest, setupGitShimPath, remoteHarnessBinDir, false); copyGitErr != nil {
		_ = runnerInstance.Kill()
		active.Status = statusFailed
		active.Error = copyGitErr.Error()
		_ = stateStore.Save(&active)
		return RunResult{}, copyGitErr
	}

	setupBashShimPath := filepath.Join(runDir, "bash")
	if bashShimErr := writeHarnessSetupBashShim(setupBashShimPath); bashShimErr != nil {
		_ = runnerInstance.Kill()
		active.Status = statusFailed
		active.Error = bashShimErr.Error()
		_ = stateStore.Save(&active)
		return RunResult{}, bashShimErr
	}
	if copyBashErr := guest.CopyToGuest(ctx, runnerGuest, setupBashShimPath, remoteHarnessBinDir, false); copyBashErr != nil {
		_ = runnerInstance.Kill()
		active.Status = statusFailed
		active.Error = copyBashErr.Error()
		_ = stateStore.Save(&active)
		return RunResult{}, copyBashErr
	}

	setupFdiskShimPath := filepath.Join(runDir, "fdisk")
	if fdiskShimErr := writeHarnessSetupFdiskShim(setupFdiskShimPath); fdiskShimErr != nil {
		_ = runnerInstance.Kill()
		active.Status = statusFailed
		active.Error = fdiskShimErr.Error()
		_ = stateStore.Save(&active)
		return RunResult{}, fdiskShimErr
	}
	if copyFdiskErr := guest.CopyToGuest(ctx, runnerGuest, setupFdiskShimPath, remoteHarnessBinDir, false); copyFdiskErr != nil {
		_ = runnerInstance.Kill()
		active.Status = statusFailed
		active.Error = copyFdiskErr.Error()
		_ = stateStore.Save(&active)
		return RunResult{}, copyFdiskErr
	}

	active.Phase = "running-installer"
	_ = stateStore.Save(&active)
	installerLogFile, err := os.Create(installerLogPath)
	if err != nil {
		_ = runnerInstance.Kill()
		active.Status = statusFailed
		active.Error = err.Error()
		_ = stateStore.Save(&active)
		return RunResult{}, fmt.Errorf("create installer log: %w", err)
	}
	defer installerLogFile.Close()

	installScript := `set -eu
cd ` + shellQuote(remoteRepoRoot) + `
chmod +x ./setup.sh

echo '=== harness preflight ==='
ls -l /dev/vdb
echo 'harness preflight: disk(s) available'

setupStatus=0
env -u http_proxy -u https_proxy -u HTTP_PROXY -u HTTPS_PROXY -u ALL_PROXY -u all_proxy -u no_proxy -u NO_PROXY \
	sudo env PATH="$(pwd)/.skea-harness-bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin" \
	SKEA_HARNESS_LOCAL_REPO="$(pwd)" \
	SKEA_NONINTERACTIVE=1 SKEA_NONINTERACTIVE_DECISION=ignore \
	bash ./setup.sh --test "$(pwd)" -config "$(pwd)/variables.yaml" || setupStatus=$?

if [ "$setupStatus" -ne 0 ]; then
	exit "$setupStatus"
fi
`
	if err := guest.Run(ctx, runnerGuest, "sh -s -- <<'SCRIPT'\n"+installScript+"\nSCRIPT", installerLogFile, installerLogFile); err != nil {
		_ = runnerInstance.Kill()
		active.Status = statusFailed
		active.Error = err.Error()
		_ = stateStore.Save(&active)
		return RunResult{}, err
	}

	active.Phase = "powering-off-runner"
	_ = stateStore.Save(&active)
	phase(3, "reboot (runner)")
	// Flush guest page caches (ESP is vfat — a hard kill loses dirty
	// entries). 'sync' returns immediately, unlike 'poweroff' which hangs.
	_ = guest.Run(ctx, runnerGuest, "sync", io.Discard, io.Discard)
	// The runner is disposable — kill it instantly. A graceful SSH
	// 'poweroff' + ACPI wait could hang and eat the run budget.
	_ = runnerInstance.Kill()
	runnerInstance = nil
	// The runner's swtpm daemon is independent of QEMU (it does NOT die with
	// CMD_SHUTDOWN on an instant kill) and holds the NVRAM lockfile — stop it
	// before restarting swtpm for the target.
	_ = qemu.StopSoftwareTPM(tpmPIDPath)

	// The runner VM now has a TPM device. When it powers off QEMU sends
	// CMD_SHUTDOWN to swtpm, causing swtpm to exit and remove the ctrl
	// socket. Restart swtpm (preserving tpm-state/) before booting the
	// target so it can reconnect to the ctrl socket.
	if cfg.shouldBootInstalledTarget() {
		if err := qemu.StartSoftwareTPM(&qemu.SoftwareTPMConfig{
			SocketPath:   tpmSocketPath,
			ControlPath:  tpmControlPath,
			StateDir:     tpmStateDir,
			PIDFile:      tpmPIDPath,
			LogPath:      tpmLogPath,
			StartTimeout: 10 * time.Second,
		}); err != nil {
			active.Status = statusFailed
			active.Error = err.Error()
			_ = stateStore.Save(&active)
			return RunResult{}, fmt.Errorf("restart swtpm after runner shutdown: %w", err)
		}
	}

	result := RunResult{
		RunID:            runID,
		RunDir:           runDir,
		Disk1Path:        disk1Path,
		Disk2Path:        disk2Path,
		RunnerSerialLog:  runnerSerialLog,
		InstallerLogPath: installerLogPath,
		RunnerQemuLog:    runnerQemuLog,
		KeyPair:          keyPair,
		CacheDir:         alpineCacheDir,
		CacheMirrorURL:   alpineCacheServer.GuestURL,
		CacheUpstreamURL: alpineUpstream,
		StartedAt:        active.StartedAt,
	}

	if !cfg.KeepTargetRunning && !cfg.shouldBootInstalledTarget() {
		active.Status = "completed"
		active.Phase = "completed"
		_ = stateStore.Clear()
		result.FinishedAt = time.Now().UTC()
		return result, nil
	}

	// Delay before booting the target to let the installed disk images settle.
	time.Sleep(10 * time.Second)

	targetResult, targetErr := service.bootAndVerifyTarget(ctx, cfg, runID, runDir, disk1Path, disk2Path, keyPair, verifyLogPath, stateStore, &active)
	if targetErr != nil {
		return result, targetErr
	}

	result.TargetSerialLog = targetResult.TargetSerialLog
	result.TargetQemuLog = targetResult.TargetQemuLog
	result.VerifyLogPath = verifyLogPath
	result.VerifyResult = targetResult.VerifyResult
	result.TargetRunning = targetResult.TargetRunning
	result.OVMFVarsPath = targetResult.OVMFVarsPath
	if cfg.KeepTargetRunning {
		stopTPM = false
	}
	result.FinishedAt = time.Now().UTC()

	// Write run summary to harness.log
	if harnessLog != nil {
		fmt.Fprintf(harnessLog, "\n=== RUN SUMMARY ===\n")
		fmt.Fprintf(harnessLog, "status: completed\n")
		fmt.Fprintf(harnessLog, "layout: %s\n", cfg.DiskLayout)
		fmt.Fprintf(harnessLog, "disk1: %s\n", disk1Path)
		fmt.Fprintf(harnessLog, "disk2: %s\n", disk2Path)
		fmt.Fprintf(harnessLog, "started: %s\n", active.StartedAt.Format(time.RFC3339))
		fmt.Fprintf(harnessLog, "finished: %s\n", result.FinishedAt.Format(time.RFC3339))
		if result.VerifyResult != nil {
			passed, failed := 0, 0
			for _, c := range result.VerifyResult.Checks {
				if c.Pass {
					passed++
				} else {
					failed++
				}
			}
			fmt.Fprintf(harnessLog, "checks: %d passed, %d failed, %d total\n", passed, failed, passed+failed)
			fmt.Fprintf(harnessLog, "verify: %s\n", boolStr(result.VerifyResult.Success, "PASS", "FAIL"))
		}
		_ = harnessLog.Sync()
		harnessLog.Close()
		harnessLog = nil
	}
	return result, nil
}

func (service *Service) bootAndVerifyTarget(ctx context.Context, cfg *RunConfig, runID, runDir, disk1Path, disk2Path string, keyPair guest.KeyPair, verifyLogPath string, stateStore runstate.Store, active *runstate.ActiveRun) (RunResult, error) {
	ovmfPaths, err := qemu.FindOVMF()
	if err != nil {
		active.Status = statusFailed
		active.Error = err.Error()
		_ = stateStore.Save(active)
		return RunResult{}, err
	}

	ovmfVarsPath := filepath.Join(runDir, "OVMF_VARS.fd")
	if copyErr := qemu.CopyFile(ovmfPaths.Vars, ovmfVarsPath); copyErr != nil {
		active.Status = statusFailed
		active.Error = copyErr.Error()
		_ = stateStore.Save(active)
		return RunResult{}, copyErr
	}
	// First boot has no TPM (buildTargetArgs(false)) so OVMF stays in
	// Setup Mode, allowing sbctl to enroll Secure Boot certificates.

	targetSerialLog := filepath.Join(runDir, "target-serial.log")
	targetSerialSocketPath := filepath.Join(runDir, "tser.sock")
	targetQemuLog := filepath.Join(runDir, "target-qemu.log")
	targetQMPPath := filepath.Join(runDir, "target-qmp.sock")
	targetFirmwareLog := filepath.Join(runDir, "target-firmware.log")
	postInstallLogPath := filepath.Join(runDir, "post-install.log")
	buildTargetArgs := func(includeTPM bool) []string {
		args := []string{
			"-name", "skea-installed-target",
			"-machine", "q35,smm=on",
			"-m", fmt.Sprintf("%d", cfg.TargetMemoryMiB),
			"-smp", fmt.Sprintf("%d", cfg.CPUs),
			"-nographic",
			"-chardev", "socket,id=targetserial,path=" + targetSerialSocketPath + ",server=on,wait=off,logfile=" + targetSerialLog + ",logappend=on",
			"-serial", "chardev:targetserial",
			"-qmp", "unix:" + targetQMPPath + ",server=on,wait=off",
			"-global", "driver=cfi.pflash01,property=secure,value=off",
			"-global", "isa-debugcon.iobase=0x402",
			"-debugcon", "file:" + targetFirmwareLog,
			"-drive", "if=pflash,format=raw,readonly=on,file=" + ovmfPaths.Code,
			"-drive", "if=pflash,format=raw,file=" + ovmfVarsPath,
			"-netdev", fmt.Sprintf("user,id=net0,hostfwd=tcp::%d-:22", cfg.TargetSSHPort),
			"-device", "virtio-net-pci,netdev=net0",
			"-drive", "if=none,id=targetdisk1,format=qcow2,file=" + disk1Path,
			"-device", "virtio-blk-pci,drive=targetdisk1,bootindex=1",
		}
		if cfg.DiskLayout == layoutBtrfsStandardServer {
			args = append(args,
				"-drive", "if=none,id=targetdisk2,format=qcow2,file="+disk2Path,
				"-device", "virtio-blk-pci,drive=targetdisk2,bootindex=2",
			)
		} else if cfg.DiskLayout == layoutBtrfsRaid0 {
			args = append(args,
				"-drive", "if=none,id=targetdisk2,format=qcow2,file="+disk2Path,
				"-device", "virtio-blk-pci,drive=targetdisk2,bootindex=2",
			)
		}
		if cfg.DesktopEnvironment == desktopNiri {
			// niri needs a DRM render node (/dev/dri/renderD128) — virtio-gpu
			// provides it headless (like VMSVGA+3D on VirtualBox).
			args = append(args, "-device", "virtio-gpu-pci")
		}
		if includeTPM {
			// QEMU 10 tpmdev emulator connects to the swtpm --ctrl socket (not
			// the --server socket). QEMU provides its data fd via CMD_SET_DATAFD.
			args = append(args, tpmQEMUArgs(filepath.Join(runDir, "tpm.sock.ctrl"))...)
		}
		if cfg.EnableKVM {
			args = append(args, "-enable-kvm")
		}
		return args
	}

	active.Phase = "booting-installed-target"
	active.TargetQMPPath = targetQMPPath
	active.TargetSerialLog = targetSerialLog
	active.TargetQemuLog = targetQemuLog
	active.TargetFirmwareLog = targetFirmwareLog
	_ = stateStore.Save(active)
	phase(4, "boot target")

	// Boot target — always with TPM (even if EnableTPM=false, swtpm is
	// always present). firstBoot binaries use /dev/tpmrm0 if available.
	targetArgs := buildTargetArgs(true)
	targetGuest := guest.Options{User: "root", Port: cfg.TargetSSHPort, PrivateKey: keyPair.PrivateKeyPath, Shell: "sh"}
	var allowPassphraseFallback bool

	// Retry target boot up to 3 times in case of intermittent
	// btrfs RAID1 mount race (dm-0 not ready in time).
	var targetInstance *qemu.Instance
	const maxBootRetries = 3
	for attempt := 1; attempt <= maxBootRetries; attempt++ {
		if attempt > 1 {
			logHarness("target boot attempt %d/%d\n", attempt, maxBootRetries)
		}
		var startErr error
		targetInstance, startErr = qemu.Start(ctx, &qemu.LaunchConfig{
			Binary:       cfg.QemuBinary,
			Args:         targetArgs,
			LogPath:      targetQemuLog,
			QMPPath:      targetQMPPath,
			SerialLog:    targetSerialLog,
			StartTimeout: 10 * time.Second,
			Persistent:   cfg.KeepTargetRunning,
		})
		if startErr != nil {
			active.Status = statusFailed
			active.Error = startErr.Error()
			_ = stateStore.Save(active)
			return RunResult{}, startErr
		}

		active.Phase = "waiting-for-target-ssh"
		_ = stateStore.Save(active)
		sshErr := waitForTargetSSH(ctx, targetInstance, targetGuest, cfg.BootTimeout, targetSerialLog, targetSerialSocketPath, targetQMPPath, cfg.InstalledPassword, true, 2, func() {
			active.Status = statusRunning
			active.Phase = "waiting-for-target-ssh"
			_ = stateStore.Save(active)
		})
		if sshErr == nil {
			break // Success
		}
		_ = targetInstance.Kill()
		targetInstance = nil
		if attempt == maxBootRetries {
			active.Status = statusFailed
			active.Error = sshErr.Error()
			_ = stateStore.Save(active)
			return RunResult{}, sshErr
		}
		time.Sleep(2 * time.Second)
	}
	defer func() {
		_ = targetInstance.Kill()
	}()

	// Run post-install scripts.
	// Target has TPM available (always passed) — binaries can bind LUKS to TPM2.
	active.Phase = "running-post-install"
	_ = stateStore.Save(active)
	phase(5, "run sbctl")

	// Run sbctl enrollment (non-fatal in QEMU — no real efivarfs)
	sbctlLogPath := filepath.Join(runDir, "sbctl.log")
	sbctlDiag, sbctlErr := runSbctlOnTarget(ctx, targetGuest, cfg.RepoRoot)
	if sbctlDiag != "" {
		_ = os.WriteFile(sbctlLogPath, []byte(sbctlDiag), 0o600)
	}
	if sbctlErr != nil {
		logHarness("WARNING: sbctl enrollment failed (non-fatal in QEMU): %v\n", sbctlErr)
	}

	// Run TPM binary — binds LUKS to TPM2 if TPM is available
	phase(6, "run tpm")
	tpmLogPath := filepath.Join(runDir, "tpm-post-install.log")
	tpmDiag, tpmErr := runTPMOnTarget(ctx, targetGuest, cfg.InstalledPassword)
	_ = tpmErr // logged above
	if tpmDiag != "" {
		_ = os.WriteFile(tpmLogPath, []byte(tpmDiag), 0o600)
	}
	tpmBindingSuccessful := false
	if strings.Contains(tpmDiag, "TPM binding complete") && !strings.Contains(tpmDiag, "No LUKS devices were bound") {
		tpmBindingSuccessful = true
	}

	// Run Webmin binary — installs Webmin container (podman compose).
	// webmin-ensure oneshot restarts it on the verification boot.
	if cfg.EnableWebmin {
		webminLogPath := filepath.Join(runDir, "webmin-post-install.log")
		webminDiag, webminErr := runWebminOnTarget(ctx, targetGuest)
		if webminDiag != "" {
			_ = os.WriteFile(webminLogPath, []byte(webminDiag), 0o600)
		}
		if webminErr != nil {
			logHarness("WARNING: webmin setup failed: %v\n", webminErr)
		}
	}

	// Run diagnostics
	postInstallDiag, diagErr := runPostInstallDiag(ctx, targetGuest)
	if postInstallDiag != "" {
		_ = os.WriteFile(postInstallLogPath, []byte(postInstallDiag), 0o600)
	}
	if diagErr != nil {
		_ = targetInstance.Kill()
		active.Status = statusFailed
		active.Error = diagErr.Error()
		_ = stateStore.Save(active)
		return RunResult{}, diagErr
	}

	// Power off and restart swtpm for verification boot
	phase(7, "reboot (target)")
	if poweroffErr := powerOffTarget(ctx, targetGuest, targetInstance, cfg, targetQMPPath); poweroffErr != nil {
		active.Status = statusFailed
		active.Error = poweroffErr.Error()
		_ = stateStore.Save(active)
		return RunResult{}, poweroffErr
	}
	targetInstance = nil

	// Restart swtpm (killed by QEMU CMD_SHUTDOWN on poweroff)
	tpmSocketPath := filepath.Join(runDir, "tpm.sock")
	tpmControlPath := filepath.Join(runDir, "tpm.sock.ctrl")
	tpmStateDir := filepath.Join(runDir, "tpm-state")
	tpmPIDPath := filepath.Join(runDir, "tpm.pid")
	_ = qemu.StopSoftwareTPM(tpmPIDPath)
	if tpmRestartErr := qemu.StartSoftwareTPM(&qemu.SoftwareTPMConfig{
		SocketPath:   tpmSocketPath,
		ControlPath:  tpmControlPath,
		StateDir:     tpmStateDir,
		PIDFile:      tpmPIDPath,
		LogPath:      tpmLogPath,
		StartTimeout: 10 * time.Second,
	}); tpmRestartErr != nil {
		active.Status = statusFailed
		active.Error = fmt.Sprintf("restart swtpm for verification: %s", tpmRestartErr.Error())
		_ = stateStore.Save(active)
		return RunResult{}, fmt.Errorf("restart swtpm for verification: %w", tpmRestartErr)
	}

	// Verification boot — boot with TPM, check LUKS auto-unlock
	phase(8, "boot target (verification)")
	targetArgs = buildTargetArgs(true)
	var startErr error
	targetInstance, startErr = qemu.Start(ctx, &qemu.LaunchConfig{
		Binary:       cfg.QemuBinary,
		Args:         targetArgs,
		LogPath:      targetQemuLog,
		QMPPath:      targetQMPPath,
		SerialLog:    targetSerialLog,
		StartTimeout: 10 * time.Second,
		Persistent:   cfg.KeepTargetRunning,
	})
	if startErr != nil {
		active.Status = statusFailed
		active.Error = startErr.Error()
		_ = stateStore.Save(active)
		return RunResult{}, startErr
	}

	active.Phase = "waiting-for-target-ssh-verification"
	_ = stateStore.Save(active)
	allowPassphraseFallback = !tpmBindingSuccessful
	if waitErr := waitForTargetSSH(ctx, targetInstance, targetGuest, cfg.BootTimeout, targetSerialLog, targetSerialSocketPath, targetQMPPath, cfg.InstalledPassword, allowPassphraseFallback, 2, func() {
		active.Status = statusRunning
		active.Phase = "waiting-for-target-ssh-verification"
		_ = stateStore.Save(active)
	}); waitErr != nil {
		_ = targetInstance.Kill()
		active.Status = statusFailed
		active.Error = waitErr.Error()
		_ = stateStore.Save(active)
		return RunResult{}, waitErr
	}

	active.Phase = "verifying"
	_ = stateStore.Save(active)
	phase(9, "verify target")
	verifyResult, err := service.Verify(ctx, &VerifyConfig{
		ArtifactsRoot:      cfg.ArtifactsRoot,
		RunDir:             runDir,
		SSHPort:            cfg.TargetSSHPort,
		SSHUser:            "root",
		PrivateKey:         keyPair.PrivateKeyPath,
		InstalledUser:      cfg.InstalledUser,
		DesktopEnvironment: cfg.DesktopEnvironment,
		EnableSwap:         cfg.EnableSwap,
		EnableSecureboot:   cfg.EnableSecureboot,
		EnableTPM:          cfg.EnableTPM,
		EnablePodman:       cfg.EnablePodman,
		EnableWebmin:       cfg.EnableWebmin,
		EnableDinit:        cfg.EnableDinit,
		EnableNix:          cfg.EnableNix,
		SwapSize:           cfg.SwapSize,
		DiskLayout:         cfg.DiskLayout,
		NixPackages:        cfg.NixPackages,
		FlatpakPackages:    cfg.FlatpakPackages,
		Timeout:            cfg.VerificationTimeout,
	})
	if err != nil {
		active.Status = statusFailed
		active.Error = err.Error()
		_ = stateStore.Save(active)
		return RunResult{}, err
	}

	verifyFile, err := os.Create(verifyLogPath)
	if err == nil {
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("# Verification: %s\n\n", boolStr(verifyResult.Success, "PASS", "FAIL")))
		sb.WriteString(fmt.Sprintf("**Layout:** %s  \n**Result:** %d/%d checks passed  \n\n", cfg.DiskLayout, countPassed(verifyResult.Checks), len(verifyResult.Checks)))
		sb.WriteString(formatCheckTable(verifyResult.Checks))
		sb.WriteString(fmt.Sprintf("\n**Total:** %d checks, %d passed, %d failed — %s\n",
			len(verifyResult.Checks), countPassed(verifyResult.Checks), countFailed(verifyResult.Checks),
			boolStr(verifyResult.Success, "🟢 ALL PASS", "🔴 FAIL")))
		_, _ = verifyFile.WriteString(sb.String())
		verifyFile.Close()
	}

	// Write individual test results + summary table to tests/ subdirectory
	writeTestResults(runDir, verifyResult)

	result := RunResult{
		RunID:             runID,
		RunDir:            runDir,
		TargetSerialLog:   targetSerialLog,
		VerifyResult:      &verifyResult,
		VerifyLogPath:     verifyLogPath,
		TargetQemuLog:     targetQemuLog,
		TargetFirmwareLog: targetFirmwareLog,
		TargetRunning:     cfg.KeepTargetRunning,
		OVMFVarsPath:      ovmfVarsPath,
	}

	if cfg.KeepTargetRunning {
		active.Status = statusRunning
		active.Phase = "target-active"
		_ = stateStore.Save(active)
		targetInstance = nil
		return result, nil
	}

	// Shut the target down (init-aware) before the run ends.
	if poweroffErr := powerOffTarget(ctx, targetGuest, targetInstance, cfg, targetQMPPath); poweroffErr != nil {
		active.Status = statusFailed
		active.Error = poweroffErr.Error()
		_ = stateStore.Save(active)
		return RunResult{}, poweroffErr
	}
	targetInstance = nil
	_ = stateStore.Clear()
	return result, nil
}

func (service *Service) Verify(ctx context.Context, cfg *VerifyConfig) (VerifyResult, error) {
	if cfg.Timeout <= 0 {
		cfg.Timeout = 2 * time.Minute
	}

	verifyCtx, cancel := context.WithTimeout(ctx, cfg.Timeout)
	defer cancel()

	guestOpts := guest.Options{User: cfg.SSHUser, Port: cfg.SSHPort, PrivateKey: cfg.PrivateKey, Shell: "sh"}

	checks := map[string]string{
		"storage_layout":     "printf '===blkid===\n'; blkid; printf '\n===mapper===\n'; ls -1 /dev/mapper/luks-* 2>/dev/null; printf '\n===mounts===\n'; grep ' / ' /proc/mounts; printf '\n===btrfs_show===\n'; btrfs filesystem show /; printf '\n===btrfs_usage===\n'; btrfs filesystem usage -T /",
		"fstab":              "cat /etc/fstab",
		"fstab_vs_mount":     `echo "=== FSTAB vs MOUNT COMPARISON ===" && for mp in $(awk '/btrfs/ && !/^#/ {print $2}' /etc/fstab | sort -u); do fstab_opts=$(awk -v mp="$mp" '$2==mp && /btrfs/ && !/^#/ {print $4}' /etc/fstab | head -1); mount_opts=$(awk -v mp="$mp" '$2==mp {print $4}' /proc/mounts | head -1); if [ -n "$fstab_opts" ] && [ -n "$mount_opts" ]; then echo "=== $mp ==="; echo "  fstab:  $fstab_opts"; echo "  mount:  $mount_opts"; for opt in $(echo "$fstab_opts" | tr ',' '\n' | sort); do if ! echo "$mount_opts" | tr ',' '\n' | grep -q "^${opt}$"; then echo "  MISSING in mount: $opt"; fi; done; for opt in $(echo "$mount_opts" | tr ',' '\n' | sort); do if ! echo "$fstab_opts" | tr ',' '\n' | grep -q "^${opt}$"; then echo "  EXTRA in mount: $opt"; fi; done; fi; done; echo "=== DONE ==="`,
		"kernel_cmdline":     "cat /etc/kernel/cmdline",
		"booster_conf":       "cat /etc/booster.yaml",
		"uki_artifacts":      "ls -lah /boot/efi/Alpine /boot/efi/EFI/BOOT || true",
		"booster_image":      "ls -lah /boot/booster-lts.img || true",
		"subvolumes":         "btrfs subvolume list / || true",
		"incus_packages":     "apk info | grep '^incus-feature' || true",
		"sshd_service":       "rc-update show | grep sshd || true",
		"installed_user":     "id " + shellQuote(cfg.InstalledUser),
		"hostname":           "cat /etc/hostname",
		"network_interfaces": "ip addr || ifconfig || true",
		"tpm_devices":        "ls -1 /dev/tpm* 2>/dev/null || true",
		"clevis_packages":    "apk info | grep -E '^(bash|clevis|jose|luksmeta|tpm2-tools|tpm2-tss|tpm2-tss-tcti-device)$' || true",
		"clevis_bindings":    "for dev in $(blkid -o device -t TYPE=crypto_LUKS 2>/dev/null); do printf '===%s===\\n' \"$dev\"; clevis luks list -d \"$dev\" 2>/dev/null || cryptsetup token list \"$dev\" || true; done",
		"vector_service":     "if [ -S /run/dinitctl ]; then dinitctl list 2>/dev/null | grep vector || test -e /etc/dinit.d/boot.d/vector && echo \"vector (boot.d)\" || test -e /lib/dinit.d/vector && echo \"vector (service file)\"; else rc-status boot 2>/dev/null | grep vector || rc-update show 2>/dev/null | grep vector || true; fi",
		"vector_logs":        "ls -la /var/log/vector*.log 2>/dev/null || echo 'no vector log files'",
		"vector_running":     "pgrep -f '/usr/bin/vector' 2>/dev/null && echo 'running' || echo 'vector not running'",
		"vector_diag":        "rc-status boot 2>/dev/null | grep vector; pgrep -la vector 2>/dev/null; ls -la /var/log/vector* 2>/dev/null; echo '===vector.log==='; cat /var/log/vector.log 2>/dev/null | tail -30; echo '===end==='; vector --version 2>&1 || true",
		"zswap_status":       "cat /sys/kernel/debug/zswap/ 2>/dev/null || cat /sys/module/zswap/parameters/enabled 2>/dev/null || echo 'zswap not available'",
		"swap_status":        "swapon -s 2>/dev/null || cat /proc/swaps 2>/dev/null || echo 'no swap active'",
		"apk_packages":       "apk info | sort",
		"sudo_installed":     "apk info | grep '^sudo-rs$' && echo 'sudo-rs installed' || echo 'sudo-rs not installed'",
		"sudo_wheel":         "grep '^%wheel' /etc/sudoers.d/wheel 2>/dev/null || echo 'no wheel sudoers config'",
		"sudoers_includedir": "grep -q '^@includedir' /etc/sudoers && echo 'includedir enabled' || echo 'includedir disabled'",
		"sudo_works":         "sudo -n id 2>&1 || echo 'sudo requires password (expected for non-root)'",
		"less_dummy":         "echo '=== file type ==='; ls -la /usr/bin/less 2>&1; echo '=== readlink ==='; readlink -f /usr/bin/less 2>&1; echo '=== version ==='; /usr/bin/less --version 2>&1 | head -1 || echo 'less binary not found'",
		"lsattr_nocow":       "echo '=== NoCoW subvolume attributes ==='; for dir in /swap /srv /var/lib/ex /var/log /var/spool /var/tmp /nix/var/log; do echo \"=== $dir ===\"; lsattr -d \"$dir\" 2>&1; f=$(find \"$dir\" -type f 2>/dev/null | head -1); if [ -n \"$f\" ]; then lsattr \"$f\" 2>&1; else echo 'no files in dir'; fi; done; echo '=== Compressed subvolume attributes ==='; for dir in / /home /opt /root /tmp /var /var/lib/libvirt /var/lib/incus /var/lib/containerd /var/lib/docker /var/lib/containers /var/lib/vector /var/cache /var/opt /usr/local /boot /nix/store /nix/var/nix /nix/var/nix/build /btrbk; do echo \"=== $dir ===\"; lsattr -d \"$dir\" 2>&1; done",
		"services_status":    "if [ -S /run/dinitctl ]; then dinitctl list; else rc-status --all 2>/dev/null || rc-status 2>/dev/null; fi",
		// Security checks — read installer.log from HOST (not target via SSH)
		// Config file markers — verify installer wrote these files (not just Alpine defaults)
		"cmdline_marker":       "grep -c 'Generated by FeralOS' /etc/kernel/cmdline 2>/dev/null || echo 0",
		"booster_marker":       "grep -c 'Generated by FeralOS' /etc/booster.yaml 2>/dev/null || echo 0",
		"fstab_marker":         "grep -c 'Generated by FeralOS' /etc/fstab 2>/dev/null || echo 0",
		"network_marker":       "grep -c 'Generated by FeralOS' /etc/network/interfaces 2>/dev/null || echo 0",
		"trusted_users_clean":  fmt.Sprintf("! grep -q %q /etc/nix/nix.conf", cfg.InstalledUser),
		"nix_packages_missing": fmt.Sprintf("su - %s -c 'nix profile list 2>/dev/null' | grep -oE \"quickshell|dms-shell|rclone-ui|rclone|mission-center|cups-browsed\" | sort -u | wc -l", cfg.InstalledUser),
	}

	// niri desktop environment checks — greetd session, niri configs,
	// DMS (native apk package dms-shell + quickshell, feralos-aports).
	if cfg.DesktopEnvironment == desktopNiri {
		checks["niri_config_valid"] = fmt.Sprintf("niri validate --config /var/lib/greetd/.config/niri/config.kdl && niri validate --config /home/%s/.config/niri/config.kdl", cfg.InstalledUser)
		checks["greetd_config"] = "ls /etc/greetd/config.toml /etc/greetd/greeter.sh /etc/greetd/regreet.toml /var/lib/greetd/.config/niri/config.kdl && test -x /etc/greetd/greeter.sh"
		checks["niri_greeter"] = "pgrep -x niri"
		checks["dms_installed"] = "apk info -e dms-shell quickshell && test -x /usr/bin/dms"
		// user dinit services: supervised session stack (dbus/audio/wayland
		// gate/polkit-agent/dms) shipped by *-user-dinit packages + dms-shell
		checks["dinit_user_services"] = "for s in dbus.user pipewire.user pipewire-pulse.user wireplumber.user wait-wayland.user polkit-agent.user dms.user; do test -f /etc/dinit.d/user/$s || { echo \"missing $s\"; exit 1; }; done; echo 'user services OK'"
		checks["niri_session_wrapper"] = "grep -q 'dinit -d /etc/dinit.d/user' /usr/local/bin/niri-session && test -x /usr/local/bin/niri-session"
	}

	// Flatpak checks — generic, only when a fixture explicitly requests
	// flatpakPackages (none ship by default; niri uses wayvnc, not RustDesk).
	if len(cfg.FlatpakPackages) > 0 {
		checks["flatpak_installed"] = "flatpak list 2>/dev/null | wc -l"
	}

	// Podman checks — container runtime
	if cfg.EnablePodman {
		checks["podman_installed"] = "podman --version"
		checks["podman_running"] = "podman ps"
		checks["docker_symlink"] = "docker ps"
	}

	// Nix tmp exec — nix builds use the upstream default build dir (/tmp),
	// so /tmp must be mounted WITHOUT noexec (old /nix/var/nix/build/tmp
	// workaround is gone).
	if cfg.EnableNix {
		checks["nix_tmp_exec"] = "mount | grep ' /tmp ' | grep -q noexec && echo 'noexec!' || echo 'exec ok'"
		// trusted-users = root @wheel (NixOS default): wheel is trusted,
		// the session user is NEVER individually injected into nix.conf.
		checks["nix_trusted_wheel"] = "grep -q 'trusted-users = root @wheel' /etc/nix/nix.conf"
	}

	// Webmin checks — functional assertions: container running, port mapped,
	// HTTP answering. Binary runs on first boot (runWebminOnTarget);
	// webmin-ensure oneshot restarts the container on this verification boot.
	// No "|| echo" fallbacks — these must FAIL when webmin doesn't work.
	if cfg.EnableWebmin {
		checks["webmin_binary"] = "test -x /opt/feralos_webmin"
		checks["webmin_container"] = "podman ps --format '{{.Names}}' | grep -q '^webmin$'"
		checks["webmin_port"] = "podman port webmin 10000 | grep -q 10000"
		checks["webmin_http"] = "for i in 1 2 3 4 5 6 7 8; do wget -q -T 5 -O /dev/null --no-check-certificate https://localhost:10000/ && exit 0; sleep 5; done; exit 1"
	}

	if cfg.EnableSecureboot {
		checks["secureboot_conf"] = "cat /etc/kernel-hooks.d/secureboot.conf"
	}

	// Dinit checks — verify dinit is PID 1, base services are running,
	// cmdline contains init=, and dinit-chimera suite is installed.
	// These MUST be falsifiable: each command exits non-zero when the
	// condition is false. A bare "&& echo ok || echo fail" fallback always
	// exits 0 and would make the check unconditionally green.
	if cfg.EnableDinit {
		checks["dinit_cmdline"] = "grep -q 'init=/usr/libexec/dinit/init' /proc/cmdline && echo 'init= OK' || { echo 'init= MISSING'; exit 1; }"
		// Busybox ps has no -p flag — read /proc/1/comm directly.
		checks["dinit_pid1"] = "[ \"$(cat /proc/1/comm)\" = dinit ] && echo 'dinit PID1' || { echo 'NOT dinit'; exit 1; }"
		// dinitctl status needs <service-name>; bare `status` is a usage
		// error. `dinitctl list` proves the control socket is functional.
		checks["dinitctl_status"] = "dinitctl list || { echo 'dinitctl failed'; exit 1; }"
		checks["dinit_services"] = "dinitctl list | grep -E 'boot|getty' || { echo 'no dinit services'; exit 1; }"
		checks["dinit_chimera_installed"] = "apk info -e dinit-chimera && echo 'installed' || { echo 'NOT installed'; exit 1; }"
		checks["dinit_getty_installed"] = "apk info -e getty-dinit && echo 'installed' || { echo 'NOT installed'; exit 1; }"
		// feralos-keyring installs the repo definition into
		// /etc/apk/repositories.d/feralos.list (not /etc/apk/repositories).
		checks["dinit_feralos_repo"] = "grep -q 'feralos-aports' /etc/apk/repositories.d/feralos.list && echo 'repo OK' || { echo 'repo MISSING'; exit 1; }"
		checks["dinit_key"] = "test -f /etc/apk/keys/feralos.rsa.pub && echo 'key OK' || { echo 'key MISSING'; exit 1; }"
		// Conventional shutdown commands wired to dinit helpers by the
		// installer (busybox poweroff/reboot do not work under dinit).
		checks["dinit_shutdown_commands"] = "test -x /usr/bin/poweroff -a -x /usr/bin/reboot -a -x /usr/bin/halt -a -x /usr/bin/shutdown -a -x /usr/bin/soft-reboot && echo 'shutdown commands OK' || { echo 'shutdown commands MISSING'; exit 1; }"
	}

	result := VerifyResult{RunDir: cfg.RunDir, Success: true, Checks: map[string]CheckResult{}}
	for name, command := range checks {
		output, err := guest.Output(verifyCtx, guestOpts, command)
		check := CheckResult{Pass: err == nil, Detail: strings.TrimSpace(string(output))}
		if err != nil {
			check.Detail = err.Error()
			result.Success = false
		}

		result.Checks[name] = check
	}

	if storageCheck, ok := result.Checks["storage_layout"]; ok {
		isRaid1 := hasHealthyBtrfsRaid1Layout(storageCheck.Detail)
		isSingle := hasHealthyBtrfsSingleLayout(storageCheck.Detail)
		isRaid0 := hasHealthyBtrfsRaid0Layout(storageCheck.Detail)
		isUnencrypted := hasHealthyBtrfsUnencryptedLayout(storageCheck.Detail)
		if !isRaid1 && !isSingle && !isRaid0 && !isUnencrypted {
			storageCheck.Pass = false
			storageCheck.Detail += "\nExpected a mounted Btrfs root — either RAID1 (2 devices, LUKS), RAID0 (2 devices, LUKS), Single (1 device, LUKS) or Unencrypted (1 device, no LUKS)"
			result.Checks["storage_layout"] = storageCheck
			result.Success = false
		}
	}

	if fstabCheck, ok := result.Checks["fstab"]; ok && !strings.Contains(fstabCheck.Detail, "LABEL=myroot") {
		fstabCheck.Pass = false
		fstabCheck.Detail += "\nExpected LABEL=myroot entry in /etc/fstab"
		result.Checks["fstab"] = fstabCheck
		result.Success = false
	}

	if cmdlineCheck, ok := result.Checks["kernel_cmdline"]; ok {
		// Accept root=UUID=<btrfs-uuid> or root=/dev/mapper/luks-<uuid> (the
		// mapper path avoids booster 0.12's WaitGroup double-Done panic when
		// both btrfs RAID1 members share the same UUID).
		// rd.luks.name is optional — present for encrypted layouts, absent for unencrypted.
		hasRoot := strings.Contains(cmdlineCheck.Detail, "root=UUID=") ||
			strings.Contains(cmdlineCheck.Detail, "root=/dev/mapper/luks-")
		if !hasRoot {
			cmdlineCheck.Pass = false
			cmdlineCheck.Detail += "\nExpected /etc/kernel/cmdline to define root=UUID=<btrfs UUID> or root=/dev/mapper/luks-<uuid>"
			result.Checks["kernel_cmdline"] = cmdlineCheck
			result.Success = false
		}
	}

	// Security check: chmod 777/1777 in installer.log (read from HOST, not target)
	// Excludes dirs where world-writable sticky is REQUIRED:
	//   /nix/var/nix/build/tmp (nix build sandbox), /var/tmp (O_TMPFILE —
	//   fresh subvolume mounts come up 0755 and break flatpak/podman).
	if cfg.RunDir != "" {
		installerLogPath := filepath.Join(cfg.RunDir, "installer.log")
		chmodCheck := CheckResult{Pass: true, Detail: "no insecure 'chmod 777/1777' found in installer.log"}
		if data, err := os.ReadFile(installerLogPath); err == nil {
			count := 0
			for _, line := range strings.Split(string(data), "\n") {
				if strings.Contains(line, "/nix/var/nix/build/tmp") || strings.Contains(line, "/var/tmp") {
					continue
				}
				if strings.Contains(line, "chmod 777") || strings.Contains(line, "chmod 1777") {
					count++
				}
			}
			if count > 0 {
				chmodCheck.Pass = false
				chmodCheck.Detail = fmt.Sprintf("Found %d occurrences of 'chmod 777/1777' in installer.log (excluding nix build tmp + /var/tmp) — insecure permissions detected", count)
				result.Success = false
			}
		}
		result.Checks["chmod_77_check"] = chmodCheck
	}

	// works_in_real_life check — verify installer.log does NOT contain
	// non-interactive warning (read from HOST, not target via SSH)
	if cfg.RunDir != "" {
		installerLogPath := filepath.Join(cfg.RunDir, "installer.log")
		worksCheck := CheckResult{Pass: true, Detail: "no non-interactive warnings in installer.log"}
		if data, err := os.ReadFile(installerLogPath); err == nil {
			if strings.Contains(string(data), "[WARNING] Non-interactive mode: ignoring error and continuing.") {
				worksCheck.Pass = false
				worksCheck.Detail = "Found '[WARNING] Non-interactive mode: ignoring error and continuing.' in installer.log — errors were ignored instead of failing"
				result.Success = false
			}
		}
		result.Checks["works_in_real_life"] = worksCheck
	}

	// Config file marker checks — verify installer actually wrote these files
	// (not just Alpine defaults). The marker "# Generated by FeralOS Installer"
	// is appended at the end of each file by the installer.
	for _, markerName := range []string{"cmdline_marker", "booster_marker", "fstab_marker", "network_marker"} {
		if markerCheck, ok := result.Checks[markerName]; ok {
			// grep -c returns "0" if not found, "1" or more if found
			count := strings.TrimSpace(markerCheck.Detail)
			if count == "0" || count == "" {
				markerCheck.Pass = false
				markerCheck.Detail = "Marker '# Generated by FeralOS Installer' not found — installer did not write this config file"
				result.Checks[markerName] = markerCheck
				result.Success = false
			}
		}
	}

	tpmExpected := false
	if boosterCheck, ok := result.Checks["booster_conf"]; ok && strings.Contains(boosterCheck.Detail, "booster") {
		tpmExpected = true
	}
	if bindingCheck, ok := result.Checks["clevis_bindings"]; ok && strings.Contains(bindingCheck.Detail, "tpm2") {
		tpmExpected = true
	}
	if tpmExpected {
		if deviceCheck, ok := result.Checks["tpm_devices"]; ok && !strings.Contains(deviceCheck.Detail, "/dev/tpm") {
			deviceCheck.Pass = false
			deviceCheck.Detail += "\nExpected a TPM device node when TPM-backed LUKS unlock is enabled"
			result.Checks["tpm_devices"] = deviceCheck
			result.Success = false
		}
		if packageCheck, ok := result.Checks["clevis_packages"]; ok {
			hasClevis := strings.Contains(packageCheck.Detail, "clevis")
			hasJose := strings.Contains(packageCheck.Detail, "jose")
			hasTools := strings.Contains(packageCheck.Detail, "tpm2-tools")
			hasTCTI := strings.Contains(packageCheck.Detail, "tpm2-tss-tcti-device")
			if !hasClevis || !hasJose || !hasTools || !hasTCTI {
				packageCheck.Pass = false
				packageCheck.Detail += "\nExpected clevis, jose, tpm2-tools, and tpm2-tss-tcti-device packages when TPM-backed LUKS unlock is enabled"
				result.Checks["clevis_packages"] = packageCheck
				result.Success = false
			}
		}
		if bindingCheck, ok := result.Checks["clevis_bindings"]; ok {
			// Determine expected binding count from the storage layout:
			// btrfsStandard has 1 LUKS device, btrfsStandardServer has 2.
			expectedBindings := 1
			if storageCheck, ok := result.Checks["storage_layout"]; ok {
				if hasHealthyBtrfsRaid1Layout(storageCheck.Detail) || hasHealthyBtrfsRaid0Layout(storageCheck.Detail) {
					expectedBindings = 2
				}
			}
			if !hasClevisTPM2Bindings(bindingCheck.Detail, expectedBindings) {
				bindingCheck.Pass = false
				bindingCheck.Detail += fmt.Sprintf("\nExpected Clevis TPM2 bindings on %d encrypted root volume(s)", expectedBindings)
				result.Checks["clevis_bindings"] = bindingCheck
				result.Success = false
			}
		}
		if cfg.RunDir != "" {
			serialCheck := CheckResult{Pass: true, Detail: "target reached SSH after the Clevis binding reboot without fallback passphrase injection"}
			serialPath := filepath.Join(cfg.RunDir, "target-serial.log")
			serialOffset, offsetErr := loadSerialLogOffset(filepath.Join(cfg.RunDir, clevisRebootSerialOffsetFile))
			data, err := readFileFromOffset(serialPath, serialOffset)
			switch {
			case offsetErr != nil:
				serialCheck.Pass = false
				serialCheck.Detail = fmt.Sprintf("read serial offset: %v", offsetErr)
				result.Success = false
			case err != nil:
				serialCheck.Pass = false
				serialCheck.Detail = fmt.Sprintf("read %s: %v", serialPath, err)
				result.Success = false
			case strings.Contains(string(data), "Enter passphrase for "):
				// Booster can show prompt text even when TPM token succeeds (fallback prompt).
				// The real test: did SSH become ready? If we reached here, it did.
				serialCheck.Detail = fmt.Sprintf("target serial log contains LUKS prompt text but SSH was reached — TPM token unlock likely succeeded (checked from byte offset %d)", serialOffset)
				// Do NOT fail — SSH success proves boot worked
			case serialOffset > 0:
				serialCheck.Detail = fmt.Sprintf("target reached SSH after the Clevis binding reboot without any visible LUKS prompt text (checked from byte offset %d)", serialOffset)
			}
			result.Checks["tpm_auto_unlock"] = serialCheck
		}
	}

	// Nix packages check — verify expected nix profile packages are installed.
	// Post-processing similar to tpm_devices: compare expected vs actual.
	// Only explicit nixPackages fixtures are gated (enableNix alone means
	// nix infra present, no profile packages expected).
	if cfg.EnableNix || len(cfg.NixPackages) > 0 {
		if nixCheck, ok := result.Checks["nix_packages_missing"]; ok {
			// SSH warning + \r\n prefix the count — strip \r, then take the
			// last line (the wc -l output) before parsing.
			cleaned := strings.TrimSpace(strings.ReplaceAll(nixCheck.Detail, "\r", ""))
			cleanedLines := strings.Split(cleaned, "\n")
			countStr := cleanedLines[len(cleanedLines)-1]
			count, err := strconv.Atoi(countStr)
			if err != nil {
				nixCheck.Pass = false
				nixCheck.Detail = fmt.Sprintf("failed to parse nix package count %q: %v (raw output: %q)", countStr, err, nixCheck.Detail)
				result.Checks["nix_packages_missing"] = nixCheck
				result.Success = false
			} else {
				expected := len(cfg.NixPackages)
				if count < expected {
					nixCheck.Pass = false
					nixCheck.Detail = fmt.Sprintf("only %d/%d nix packages installed (missing %d): count=%d expected=%d (raw output: %q)", count, expected, expected-count, count, expected, nixCheck.Detail)
					result.Checks["nix_packages_missing"] = nixCheck
					result.Success = false
				}
			}
		}
	}

	// Nix tmp exec — verify /tmp is NOT mounted noexec (nix builds there now)
	if execCheck, ok := result.Checks["nix_tmp_exec"]; ok {
		if !strings.Contains(execCheck.Detail, "exec ok") {
			execCheck.Pass = false
			execCheck.Detail += "\nExpected /tmp to be mounted exec (nix upstream build dir)"
			result.Checks["nix_tmp_exec"] = execCheck
			result.Success = false
		}
	}

	if cfg.EnableSecureboot {
		if securebootCheck, ok := result.Checks["secureboot_conf"]; ok && !strings.Contains(securebootCheck.Detail, "cmdline=") {
			securebootCheck.Pass = false
			securebootCheck.Detail += "\nExpected cmdline entry in /etc/kernel-hooks.d/secureboot.conf"
			result.Checks["secureboot_conf"] = securebootCheck
			result.Success = false
		}
	}

	if ukiCheck, ok := result.Checks["uki_artifacts"]; ok {
		hasSignedUKI := strings.Contains(ukiCheck.Detail, "linux-lts.efi")
		hasFallback := strings.Contains(ukiCheck.Detail, "BOOTX64.EFI")
		if !hasSignedUKI || !hasFallback {
			ukiCheck.Pass = false
			ukiCheck.Detail += "\nExpected signed linux-lts.efi and removable fallback BOOTX64.EFI on the ESP"
			result.Checks["uki_artifacts"] = ukiCheck
			result.Success = false
		}
	}

	if lessCheck, ok := result.Checks["less_dummy"]; ok {
		detail := lessCheck.Detail
		// Check 1: /usr/bin/less should be a symlink (not a regular file)
		isSymlink := strings.Contains(detail, "-> /usr/bin/bat") ||
			strings.Contains(detail, "-> /usr/bin/") ||
			strings.Contains(detail, "lrwxrwxrwx")
		// Check 2: readlink should resolve to bat
		resolvesToBat := strings.Contains(detail, "/usr/bin/bat")
		// Check 3: version output should contain "bat" (not "less")
		versionIsBat := strings.Contains(detail, "bat ")
		if !isSymlink || !resolvesToBat || !versionIsBat {
			lessCheck.Pass = false
			lessCheck.Detail = detail + "\nExpected /usr/bin/less → /usr/bin/bat (symlink to memory-safe replacement)"
			result.Checks["less_dummy"] = lessCheck
			result.Success = false
		}
	}

	// Validate NoCoW subvolume attributes (lsattr_nocow)
	// NoCoW subvolumes: dir + first file must have C flag, must NOT have m flag
	// (m = compression attribute, conflicts with C, NixOS/nixpkgs #384047).
	// Compressed subvolumes: dir must NOT have C flag (must stay CoW).
	if lsattrCheck, ok := result.Checks["lsattr_nocow"]; ok {
		detail := lsattrCheck.Detail
		var failures []string
		for _, dir := range []string{"/swap", "/srv", "/var/lib/ex", "/var/log", "/var/spool", "/var/tmp", "/nix/var/log"} {
			section := extractLsattrSection(detail, dir)
			if section == "" {
				failures = append(failures, dir+": section not found in lsattr output")
				continue
			}
			for _, line := range strings.Split(section, "\n") {
				line = strings.TrimSpace(line)
				if line == "" || strings.HasPrefix(line, "===") || strings.HasPrefix(line, "no files") {
					continue
				}
				fields := strings.Fields(line)
				if len(fields) < 2 {
					continue
				}
				attrs := fields[0]
				if !strings.Contains(attrs, "C") {
					failures = append(failures, dir+": missing C (NoCoW) flag on "+line)
				}
				if strings.Contains(attrs, "m") {
					failures = append(failures, dir+": unexpected m (compression) flag on "+line)
				}
			}
		}
		for _, dir := range []string{"/", "/home", "/opt", "/root", "/tmp", "/var", "/var/lib/libvirt", "/var/lib/incus", "/var/lib/containerd", "/var/lib/docker", "/var/lib/containers", "/var/lib/vector", "/var/cache", "/var/opt", "/usr/local", "/boot", "/nix/store", "/nix/var/nix", "/nix/var/nix/build", "/btrbk"} {
			section := extractLsattrSection(detail, dir)
			if section == "" {
				failures = append(failures, dir+": section not found in lsattr output")
				continue
			}
			for _, line := range strings.Split(section, "\n") {
				line = strings.TrimSpace(line)
				if line == "" || strings.HasPrefix(line, "===") {
					continue
				}
				fields := strings.Fields(line)
				if len(fields) < 2 {
					continue
				}
				if strings.Contains(fields[0], "C") {
					failures = append(failures, dir+": unexpected C (NoCoW) flag on compressed subvolume "+line)
				}
			}
		}
		if len(failures) > 0 {
			lsattrCheck.Pass = false
			lsattrCheck.Detail = detail + "\n" + strings.Join(failures, "\n")
			result.Checks["lsattr_nocow"] = lsattrCheck
			result.Success = false
		}
	}

	// Save apk info to files in run directory
	if cfg.RunDir != "" {
		if apkCheck, ok := result.Checks["apk_packages"]; ok && apkCheck.Pass {
			// Save full package list
			allPackagesPath := filepath.Join(cfg.RunDir, "apk-packages-all.txt")
			if err := os.WriteFile(allPackagesPath, []byte(apkCheck.Detail+"\n"), 0o600); err != nil {
				fmt.Printf("[WARNING] Failed to write %s: %v\n", allPackagesPath, err)
			}

			// Save openrc packages only
			openrcPackagesPath := filepath.Join(cfg.RunDir, "apk-packages-openrc.txt")
			var openrcLines []string
			for _, line := range strings.Split(apkCheck.Detail, "\n") {
				if strings.HasSuffix(strings.TrimSpace(line), "-openrc") {
					openrcLines = append(openrcLines, line)
				}
			}
			if err := os.WriteFile(openrcPackagesPath, []byte(strings.Join(openrcLines, "\n")+"\n"), 0o600); err != nil {
				fmt.Printf("[WARNING] Failed to write %s: %v\n", openrcPackagesPath, err)
			}
		}
	}

	// Validate vector service check
	if vectorCheck, ok := result.Checks["vector_service"]; ok {
		if !strings.Contains(vectorCheck.Detail, "vector") {
			vectorCheck.Pass = false
			vectorCheck.Detail += "\nExpected vector service in boot runlevel"
			result.Checks["vector_service"] = vectorCheck
			result.Success = false
		}
	}

	// Validate vector logs check
	if vectorLogsCheck, ok := result.Checks["vector_logs"]; ok {
		if strings.Contains(vectorLogsCheck.Detail, "no vector log files") {
			vectorLogsCheck.Pass = false
			vectorLogsCheck.Detail += "\nExpected vector log files to exist in /var/log/"
			result.Checks["vector_logs"] = vectorLogsCheck
			result.Success = false
		}
	}

	// Validate vector running check
	if vectorRunningCheck, ok := result.Checks["vector_running"]; ok {
		if strings.Contains(vectorRunningCheck.Detail, "vector not running") {
			vectorRunningCheck.Pass = false
			vectorRunningCheck.Detail += "\nExpected vector process to be running"
			result.Checks["vector_running"] = vectorRunningCheck
			result.Success = false
		}
	}

	// Validate zswap check
	if zswapCheck, ok := result.Checks["zswap_status"]; ok {
		// zswap is optional - just log the status
		if strings.Contains(zswapCheck.Detail, "not available") {
			zswapCheck.Detail += "\nzswap not enabled in this configuration"
		} else if strings.Contains(zswapCheck.Detail, "enabled") {
			zswapCheck.Detail += "\nzswap is enabled"
		}
		result.Checks["zswap_status"] = zswapCheck
	}

	// Validate sudo installed check
	if sudoInstalledCheck, ok := result.Checks["sudo_installed"]; ok {
		if strings.Contains(sudoInstalledCheck.Detail, "not installed") {
			sudoInstalledCheck.Pass = false
			sudoInstalledCheck.Detail += "\nExpected sudo-rs to be installed"
			result.Checks["sudo_installed"] = sudoInstalledCheck
			result.Success = false
		}
	}

	// Validate sudo wheel check
	if sudoWheelCheck, ok := result.Checks["sudo_wheel"]; ok {
		if strings.Contains(sudoWheelCheck.Detail, "no wheel sudoers config") {
			sudoWheelCheck.Pass = false
			sudoWheelCheck.Detail += "\nExpected %wheel group in /etc/sudoers.d/wheel"
			result.Checks["sudo_wheel"] = sudoWheelCheck
			result.Success = false
		}
	}

	// Validate sudoers @includedir is enabled (Alpine default comments it out,
	// which would make /etc/sudoers.d/wheel ineffective — wheel sudo denied)
	if sudoersCheck, ok := result.Checks["sudoers_includedir"]; ok {
		if strings.Contains(sudoersCheck.Detail, "includedir disabled") {
			sudoersCheck.Pass = false
			sudoersCheck.Detail += "\nExpected @includedir /etc/sudoers.d enabled in /etc/sudoers"
			result.Checks["sudoers_includedir"] = sudoersCheck
			result.Success = false
		}
	}

	// sudo_works check: just verify the command ran (password expected for non-root)
	// This is informational - sudo requires password which is correct behavior

	// Log error check — non-regression guard: scan boot + installer logs for
	// error/failed markers. Any match fails the run so regressions (e.g.
	// "in use but fuser finds nothing", OpenRC [ !! ], apk failures) surface
	// immediately instead of being masked by passing service checks.
	if cfg.RunDir != "" {
		logCheck := CheckResult{Pass: true, Detail: "no error/failed markers found in logs"}
		logErrRe := regexp.MustCompile(`(?i)(error|failed|critical|\[ !! \]|in use but fuser finds nothing|invalid argument|cannot set)`)
		// Innocuous lines: counters/negations ("0 errors", "no failed", ...),
		// package name libgpg-error and Debian git dependency liberror-perl
		// (apt install git output), efibootmgr "error trace:" on QEMU (no real
		// EFI vars — expected, command runs with || true, exit 0), smartd
		// "start-stop-daemon: no matching processes found" (no SMART hardware
		// in QEMU — daemon never starts), fsck "fsck.vfat: No such file or
		// directory" (fsck.vfat not installed — ESP check cannot run in QEMU),
		// runner-side Debian "dpkg was interrupted" (stale runner image, not
		// the installer) — including the retry lines that contain bare "failed",
		// and the installer's non-interactive "ignoring error and continuing"
		// warning (informational — retries exhausted, continues). Also
		// mdadm "error opening /dev/md?" (no pre-existing RAID) and mkfs.btrfs
		// usage-help example lines (contain "error" in the example text).
		allowRe := regexp.MustCompile(`(?i)(0 errors|no errors|errors: 0|0 failed|no failed|failed: 0|zero errors|libgpg-error|liberror-perl|error trace:|start-stop-daemon: no matching processes found|fsck\.vfat: No such file or directory|Check FAT filesystem on DEVICE for errors|early-sysctl command failed|\[FAILED\] early-sysctl|Failed to connect to system bus|Failed to allocate manager object|install command '/bin/false'|dpkg was interrupted|dpkg --configure -a|failed, retrying in|apt does not have a stable CLI|error opening /dev/md|E\.g\.: dev1|LABEL=EFIVDB|errors=remount-ro|sysctl: error:.*is an unknown key|sysctl: unknown sysctl|icmp_ignore_bogus_error_responses|/nix/store/[^ ]*-error|commitpartial|bwrap.*pivot_root|To enable access to XDG directories|abseil-cpp-strerror)`)
		// BTRFS-specific errors: only for btrfs layouts (btrfsStandard, btrfsStandardServer, btrfsRaid0, btrfsUnencrypted)
		if strings.Contains(cfg.DiskLayout, "btrfs") {
			allowRe = regexp.MustCompile(`(?i)(0 errors|no errors|errors: 0|0 failed|no failed|failed: 0|zero errors|libgpg-error|liberror-perl|error trace:|start-stop-daemon: no matching processes found|fsck\.vfat: No such file or directory|Check FAT filesystem on DEVICE for errors|early-sysctl command failed|\[FAILED\] early-sysctl|Failed to connect to system bus|Failed to allocate manager object|dpkg was interrupted|dpkg --configure -a|failed, retrying in|apt does not have a stable CLI|error opening /dev/md|E\.g\.: dev1|LABEL=EFIVDB|errors=remount-ro|sysctl: error:.*is an unknown key|sysctl: unknown sysctl|icmp_ignore_bogus_error_responses|/nix/store/[^ ]*-error|commitpartial|bwrap.*pivot_root|To enable access to XDG directories|BTRFS error.*is missing|BTRFS error.*failed to read the system array|BTRFS error.*open_ctree failed|BTRFS Transaction aborted|forced readonly|abseil-cpp-strerror)`)
		}
		var problems []string
		for _, logName := range []string{"target-serial.log", "installer.log"} {
			path := filepath.Join(cfg.RunDir, logName)
			data, err := os.ReadFile(path)
			if err != nil {
				if os.IsNotExist(err) {
					continue
				}
				problems = append(problems, fmt.Sprintf("%s: read error: %v", logName, err))
				continue
			}
			for i, line := range strings.Split(string(data), "\n") {
				if logErrRe.MatchString(line) && !allowRe.MatchString(line) {
					problems = append(problems, fmt.Sprintf("%s:%d: %s", logName, i+1, strings.TrimSpace(line)))
				}
			}
		}
		if len(problems) > 0 {
			logCheck.Pass = false
			logCheck.Detail = "error/failed markers found in logs:\n" + strings.Join(problems, "\n")
			result.Success = false
		}
		result.Checks["log_errors"] = logCheck
	}

	// Services active check — verify configured services are actually
	// running (rc-status), not just that their init scripts printed [ ok ].
	// Excluded services that cannot work under QEMU:
	//   - smartd: no SMART-capable hardware in QEMU, daemon never starts
	//   - fsck:   fsck.vfat not installed, ESP check fails silently
	//   - udev-settle: one-shot wait service, not persistent
	if svcCheck, ok := result.Checks["services_status"]; ok {
		// Dinit targets: parse `dinitctl list` (lines like `[[+]     ] name`).
		// Steady state = state group contains '+' and no 'X' (failed).
		// smartd excluded: QEMU has no SMART devices (exit 17, known artifact).
		if cfg.EnableDinit || strings.Contains(svcCheck.Detail, "dinitctl") || strings.Contains(svcCheck.Detail, "[{") {
			expected := []string{
				// entry points + targets
				"boot", "system", "local.target", "pre-local.target",
				"login.target", "network.target", "pre-network.target",
				// console
				"getty@tty1", "getty@ttyS0",
				// daemons (rules-driven boot.d enablement)
				"networking", "incusd", "vector", "cronie", "chronyd", "dbus",
				"sshd", "acpid", "irqbalance", "fwupd", "earlyoom",
			}
			started := map[string]bool{}
			for _, line := range strings.Split(svcCheck.Detail, "\n") {
				line = strings.TrimSpace(line)
				// dinitctl list format: `[[+]     ] boot` — state marker(s)
				// wrapped in an outer bracket pair, then the service name.
				if !strings.HasPrefix(line, "[") {
					continue
				}
				closeB := strings.LastIndex(line, "]")
				if closeB < 0 || closeB+1 >= len(line) {
					continue
				}
				state := line[:closeB+1]
				name := strings.TrimSpace(line[closeB+1:])
				// strip annotations: "chronyd (pid: 1534)", "login.target (has console)"
				if paren := strings.Index(name, "("); paren > 0 {
					name = strings.TrimSpace(name[:paren])
				}
				started[name] = strings.Contains(state, "+") && !strings.Contains(state, "X")
			}
			var notStarted []string
			for _, name := range expected {
				if !started[name] {
					notStarted = append(notStarted, name+"=not-started")
				}
			}
			if len(notStarted) > 0 {
				svcCheck.Pass = false
				svcCheck.Detail += "\nServices not active: " + strings.Join(notStarted, ", ")
				result.Success = false
			}
			result.Checks["services_status"] = svcCheck
		} else {
			expected := []string{
				// sysinit
				"devfs", "dmesg", "sysctl", "sysfs",
				// boot
				"hwclock", "hostname", "bootmisc", "binfmt", "root", "procfs",
				"loadkmap", "localmount", "btrfs-scan", "dmcrypt",
				"udev", "udev-trigger", "vector", "vector-setup",
				// default
				"cgroups", "networking", "cronie", "chronyd", "dbus", "sshd",
				"acpid", "irqbalance", "fwupd", "earlyoom", "incusd", "udev-postmount",
			}
			states := parseRCStatus(svcCheck.Detail)
			// greetd + elogind are only expected when the niri desktop is enabled.
			if cfg.DesktopEnvironment == desktopNiri {
				expected = append(expected, "greetd", "elogind")
			}
			// swap service is conditional: only when swap exists (enableSwap for
			// btrfsStandard/btrfsRaid0, always for btrfsStandardServer LUKS swap).
			if cfg.EnableSwap || cfg.DiskLayout == layoutBtrfsStandardServer {
				expected = append(expected, "swap")
			}
			var notStarted []string
			for _, name := range expected {
				if state, ok := states[name]; !ok {
					notStarted = append(notStarted, name+"=missing")
				} else if state != "started" {
					notStarted = append(notStarted, name+"="+state)
				}
			}
			if len(notStarted) > 0 {
				svcCheck.Pass = false
				svcCheck.Detail += "\nServices not active: " + strings.Join(notStarted, ", ")
				result.Success = false
			}
			result.Checks["services_status"] = svcCheck
		}
	}

	// swap_status must be truthful: when swap is expected (enableSwap for
	// btrfsStandard/btrfsRaid0, always for btrfsStandardServer LUKS swap), the
	// check fails if no swap device is actually active (swapon -s header-only).
	if swapCheck, ok := result.Checks["swap_status"]; ok {
		swapExpected := cfg.EnableSwap || cfg.DiskLayout == layoutBtrfsStandardServer
		hasActiveSwap := strings.Contains(swapCheck.Detail, "/swap/swapfile") ||
			strings.Contains(swapCheck.Detail, "/dev/mapper/swap-") ||
			strings.Contains(swapCheck.Detail, "/dev/zram")
		if swapExpected && !hasActiveSwap {
			swapCheck.Pass = false
			swapCheck.Detail += "\nExpected active swap (enableSwap or btrfsStandardServer) but none found"
			result.Checks["swap_status"] = swapCheck
			result.Success = false
		}
	}

	return result, nil
}

// extractLsattrSection returns the lsattr output section for a given mount
// point (between "=== <dir> ===" markers) from the lsattr_nocow check detail.
func extractLsattrSection(detail, dir string) string {
	startMarker := "=== " + dir + " ==="
	idx := strings.Index(detail, startMarker)
	if idx == -1 {
		return ""
	}
	rest := detail[idx+len(startMarker):]
	endIdx := strings.Index(rest, "===")
	if endIdx == -1 {
		return rest
	}
	return rest[:endIdx]
}

// parseRCStatus parses `rc-status --all` output into a map of service name
// to state. Lines look like: "  sshd  [ started  ]" or, for supervise-daemon
// services: "  vector  [ started 00:00:36 (0) ]" (uptime + exit code).
func parseRCStatus(detail string) map[string]string {
	states := make(map[string]string)
	stateRe := regexp.MustCompile(`^\s*(\S+)\s+\[\s*(\w+)`)
	for _, line := range strings.Split(detail, "\n") {
		m := stateRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		states[m[1]] = m[2]
	}
	return states
}

// hasHealthyBtrfsRaidLayout validates a dual-disk btrfs RAID layout
// (btrfsRaid0 or btrfsRaid1): two LUKS devices, RAID{level} data + metadata profiles.
func hasHealthyBtrfsRaidLayout(detail string, level string) bool {
	lowerDetail := strings.ToLower(detail)
	hasMountedBtrfsRoot := regexp.MustCompile(`(?m)^/dev/(mapper/luks-|dm-)[^ ]+ / btrfs `).MatchString(detail)
	hasEncryptedRootDevices := strings.Count(detail, "/dev/mapper/luks-") >= 2
	hasTwoBtrfsDevices := len(regexp.MustCompile(`(?m)^\s*devid\s+[0-9]+`).FindAllString(detail, -1)) >= 2
	hasRaidProfileHeader := regexp.MustCompile(fmt.Sprintf(`(?m)^Id Path\s+%[1]s\s+%[1]s(?:\s+%[1]s)?(?:\s|$)`, level)).MatchString(detail)
	hasDataRaid := regexp.MustCompile(fmt.Sprintf(`(?m)\bData(?:,\s*%[1]s:|\s+%[1]s\b)`, level)).MatchString(detail) || hasRaidProfileHeader
	hasMetadataRaid := regexp.MustCompile(fmt.Sprintf(`(?m)\bMetadata(?:,\s*%[1]s:|\s+%[1]s\b)`, level)).MatchString(detail) || hasRaidProfileHeader
	hasMissingDevice := strings.Contains(lowerDetail, " is missing")

	return hasMountedBtrfsRoot && hasEncryptedRootDevices && hasTwoBtrfsDevices && hasDataRaid && hasMetadataRaid && !hasMissingDevice
}

// hasHealthyBtrfsRaid1Layout validates a dual-disk btrfs RAID1 layout.
func hasHealthyBtrfsRaid1Layout(detail string) bool {
	return hasHealthyBtrfsRaidLayout(detail, "RAID1")
}

// hasHealthyBtrfsRaid0Layout validates a dual-disk btrfs RAID0 layout
// (btrfsRaid0): two LUKS devices, RAID0 data + metadata profiles.
func hasHealthyBtrfsRaid0Layout(detail string) bool {
	return hasHealthyBtrfsRaidLayout(detail, "RAID0")
}

// hasHealthyBtrfsSingleLayout validates a single-disk btrfs layout (btrfsStandard):
// one LUKS device, single data profile, DUP metadata.
func hasHealthyBtrfsSingleLayout(detail string) bool {
	// Accept both /dev/mapper/luks-* and /dev/dm-* in the mount line
	hasMountedBtrfsRoot := regexp.MustCompile(`(?m)^/dev/(mapper/luks-|dm-)[^ ]+ / btrfs `).MatchString(detail)
	// Extract only the mapper section (between ===mapper=== and ===mounts===)
	mapperStart := strings.Index(detail, "===mapper===")
	mapperEnd := strings.Index(detail, "===mounts===")
	hasOneEncryptedDevice := false
	if mapperStart >= 0 && mapperEnd > mapperStart {
		mapperSection := detail[mapperStart:mapperEnd]
		hasOneEncryptedDevice = strings.Count(mapperSection, "/dev/mapper/luks-") == 1
	}
	hasOneBtrfsDevice := len(regexp.MustCompile(`(?m)^\s*devid\s+[0-9]+`).FindAllString(detail, -1)) == 1

	// Match the btrfs_usage -T table header: "Id Path      single   DUP"
	hasSingleProfile := regexp.MustCompile(`(?m)Id\s+Path\s+single\s+DUP`).MatchString(detail)
	// Also check for "Data ratio:\t\t      1.00" and "Metadata ratio:\t\t      2.00"
	hasSingleDataRatio := strings.Contains(detail, "Data ratio:") && strings.Contains(detail, "1.00")
	hasDupMetadataRatio := strings.Contains(detail, "Metadata ratio:") && strings.Contains(detail, "2.00")

	hasSingleData := hasSingleProfile || hasSingleDataRatio
	hasDupMetadata := hasSingleProfile || hasDupMetadataRatio

	return hasMountedBtrfsRoot && hasOneEncryptedDevice && hasOneBtrfsDevice && hasSingleData && hasDupMetadata
}

// hasHealthyBtrfsUnencryptedLayout validates a single-disk unencrypted btrfs
// layout (btrfsUnencrypted): root mounted directly from /dev/vd* or /dev/sd*
// (no LUKS mapper), single data profile, DUP metadata.
func hasHealthyBtrfsUnencryptedLayout(detail string) bool {
	// Root mounted from a raw partition (not mapper, not dm-)
	hasMountedBtrfsRoot := regexp.MustCompile(`(?m)^/dev/(vd[a-z]+|sd[a-z]+)[0-9]* / btrfs `).MatchString(detail)
	// No LUKS devices in mapper section
	mapperStart := strings.Index(detail, "===mapper===")
	mapperEnd := strings.Index(detail, "===mounts===")
	hasNoEncryptedDevices := true
	if mapperStart >= 0 && mapperEnd > mapperStart {
		mapperSection := detail[mapperStart:mapperEnd]
		hasNoEncryptedDevices = !strings.Contains(mapperSection, "/dev/mapper/luks-")
	}
	hasOneBtrfsDevice := len(regexp.MustCompile(`(?m)^\s*devid\s+[0-9]+`).FindAllString(detail, -1)) == 1
	hasSingleProfile := regexp.MustCompile(`(?m)Id\s+Path\s+single\s+DUP`).MatchString(detail)
	hasSingleDataRatio := strings.Contains(detail, "Data ratio:") && strings.Contains(detail, "1.00")
	hasDupMetadataRatio := strings.Contains(detail, "Metadata ratio:") && strings.Contains(detail, "2.00")
	hasSingleData := hasSingleProfile || hasSingleDataRatio
	hasDupMetadata := hasSingleProfile || hasDupMetadataRatio

	return hasMountedBtrfsRoot && hasNoEncryptedDevices && hasOneBtrfsDevice && hasSingleData && hasDupMetadata
}

func hasClevisTPM2Bindings(detail string, expectedDevices int) bool {
	if expectedDevices <= 0 {
		expectedDevices = 1
	}

	deviceCount := len(regexp.MustCompile(`(?m)^===/dev/[^=]+===\s*$`).FindAllString(detail, -1))
	bindingCount := len(regexp.MustCompile(`(?m)\btpm2\b`).FindAllString(detail, -1))

	return deviceCount >= expectedDevices && bindingCount >= expectedDevices
}

func (service *Service) WriteSerial(_ context.Context, cfg SerialWriteConfig) (SerialWriteResult, error) {
	if strings.TrimSpace(cfg.ArtifactsRoot) == "" {
		return SerialWriteResult{}, fmt.Errorf("artifacts root is required")
	}
	if cfg.Data == "" {
		return SerialWriteResult{}, fmt.Errorf("serial data is required")
	}

	runDir, runID, err := resolveSerialTargetRun(cfg.ArtifactsRoot, cfg.RunDir)
	if err != nil {
		return SerialWriteResult{}, err
	}

	socketPath := filepath.Join(runDir, "tser.sock")
	if _, err := os.Stat(socketPath); err != nil {
		return SerialWriteResult{}, fmt.Errorf("target serial socket %s: %w", socketPath, err)
	}

	if err := qemu.WriteSerial(socketPath, cfg.Data); err != nil {
		return SerialWriteResult{}, err
	}

	return SerialWriteResult{
		RunID:        runID,
		RunDir:       runDir,
		SocketPath:   socketPath,
		BytesWritten: len(cfg.Data),
		Target:       "target-serial",
	}, nil
}

func (service *Service) Status(_ context.Context, artifactsRoot string) (StatusResult, error) {
	store := runstate.New(artifactsRoot)
	result := StatusResult{ActiveState: store.ActivePath(), LatestRun: latestRunDir(filepath.Join(artifactsRoot, "runs"))}

	active, err := store.Load()
	if err == nil {
		result.Active = &active
		qmpPath := active.TargetQMPPath
		if qmpPath == "" {
			qmpPath = active.RunnerQMPPath
		}
		if qmpPath != "" {
			status, qmpErr := qemu.QueryStatus(qmpPath)
			if qmpErr == nil {
				result.QMPStatus = &status
			}
		}
	}

	return result, nil
}

func (service *Service) Stop(_ context.Context, cfg StopConfig) (StopResult, error) {
	store := runstate.New(cfg.ArtifactsRoot)
	active, err := store.Load()
	if err != nil {
		return StopResult{}, err
	}

	qmpPath := active.TargetQMPPath
	if qmpPath == "" {
		qmpPath = active.RunnerQMPPath
	}
	if qmpPath == "" {
		return StopResult{}, fmt.Errorf("no active QMP socket found")
	}

	method := "system_powerdown"
	if cfg.Force {
		method = "quit"
		if err := qemu.Quit(qmpPath); err != nil {
			return StopResult{}, err
		}
	} else {
		if err := qemu.SystemPowerdown(qmpPath); err != nil {
			return StopResult{}, err
		}
	}

	active.Status = "stopping"
	active.Phase = method
	_ = store.Save(&active)

	return StopResult{Stopped: true, Method: method, RunID: active.RunID}, nil
}

func (service *Service) Cleanup(_ context.Context, cfg CleanupConfig) (CleanupResult, error) {
	if cfg.ArtifactsRoot == "" {
		return CleanupResult{}, fmt.Errorf("artifacts root is required")
	}

	store := runstate.New(cfg.ArtifactsRoot)
	result := CleanupResult{}
	ports := uniquePorts(cfg.RunnerSSHPort, cfg.TargetSSHPort)

	activePath := store.ActivePath()
	if _, err := os.Stat(activePath); err != nil {
		if !os.IsNotExist(err) {
			return result, fmt.Errorf("stat active state: %w", err)
		}
	} else {
		active, err := store.Load()
		if err != nil {
			return result, err
		}
		result.ActiveRunID = active.RunID
		ports = uniquePorts(append(ports, active.RunnerSSHPort, active.TargetSSHPort)...)
		if !cfg.Force && activeRunHealthy(&active) {
			result.Skipped = true
			result.Reason = fmt.Sprintf("active harness run %s is still running in phase %s", active.RunID, active.Phase)
			return result, nil
		}

		killedPIDs, killErr := killHarnessQEMUProcesses(cfg.ArtifactsRoot, active.RunDir, ports)
		if killErr != nil {
			return result, killErr
		}
		result.KilledPIDs = appendUniqueInts(result.KilledPIDs, killedPIDs...)

		if err := store.Clear(); err != nil {
			return result, err
		}
		result.ClearedState = true
	}

	killedPIDs, err := killHarnessQEMUProcesses(cfg.ArtifactsRoot, "", ports)
	if err != nil {
		return result, err
	}
	result.KilledPIDs = appendUniqueInts(result.KilledPIDs, killedPIDs...)
	result.BusyPorts = unavailablePorts(ports)
	if len(result.BusyPorts) > 0 {
		result.Reason = fmt.Sprintf("ports still busy after cleanup: %v", result.BusyPorts)
	}

	return result, nil
}

func (service *Service) Logs(_ context.Context, cfg LogsConfig) (LogsResult, error) {
	if cfg.Lines <= 0 {
		cfg.Lines = 200
	}

	runDir := cfg.RunDir
	if runDir == "" {
		runDir = latestRunDir(filepath.Join(cfg.ArtifactsRoot, "runs"))
	}
	if runDir == "" {
		return LogsResult{}, fmt.Errorf("no run directory available")
	}

	path := resolveLogPath(runDir, cfg.LogName)
	if path == "" {
		return LogsResult{}, fmt.Errorf("unknown log name %q", cfg.LogName)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return LogsResult{}, fmt.Errorf("read %s: %w", path, err)
	}

	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	if len(lines) > cfg.Lines {
		lines = lines[len(lines)-cfg.Lines:]
	}

	return LogsResult{Path: path, Content: strings.Join(lines, "\n"), Lines: cfg.Lines}, nil
}

func ensurePrepareDefaults(cfg *PrepareRunnerConfig) error {
	if cfg.ArtifactsRoot == "" {
		return fmt.Errorf("artifacts root is required")
	}
	if cfg.BaseImageURL == "" && cfg.BaseImagePath == "" {
		cfg.BaseImageURL = defaultRunnerImageURL
	}
	if cfg.QemuBinary == "" {
		cfg.QemuBinary = "qemu-system-x86_64"
	}
	if cfg.QemuImgBinary == "" {
		cfg.QemuImgBinary = "qemu-img"
	}
	if cfg.CloudLocaldsBin == "" {
		cfg.CloudLocaldsBin = "cloud-localds"
	}
	if cfg.CacheDir == "" {
		cfg.CacheDir = filepath.Join(cfg.ArtifactsRoot, "cache")
	}
	if cfg.SSHPort == 0 {
		cfg.SSHPort = 2222
	}
	if cfg.SSHUser == "" {
		cfg.SSHUser = sshUserSkea
	}
	if cfg.MemoryMiB == 0 {
		cfg.MemoryMiB = 4096
	}
	if cfg.CPUs == 0 {
		cfg.CPUs = 2
	}
	if cfg.WaitTimeout == 0 {
		cfg.WaitTimeout = 15 * time.Minute
	}
	if cfg.RunnerImageSize == "" {
		// Overlay starts at base image size (327MB) which is too small for
		// cloud-init + apt. Resize so growpart/resizefs can expand the FS.
		cfg.RunnerImageSize = "8G"
	}
	if cfg.BootTimeout == 0 {
		cfg.BootTimeout = 10 * time.Minute
	}
	if len(cfg.Packages) == 0 {
		// Runner minimal: tar (unpack repo), curl (installer downloads), parted (growpart).
		// Base Debian13 has bash, ssh, sudo. Installer installs its own deps.
		cfg.Packages = []string{"tar", "curl", "parted"}
	}
	if _, err := exec.LookPath(cfg.QemuBinary); err != nil {
		return fmt.Errorf("find %s: %w", cfg.QemuBinary, err)
	}
	if _, err := exec.LookPath(cfg.QemuImgBinary); err != nil {
		return fmt.Errorf("find %s: %w", cfg.QemuImgBinary, err)
	}
	if _, err := exec.LookPath(cfg.CloudLocaldsBin); err != nil {
		return fmt.Errorf("find %s: %w", cfg.CloudLocaldsBin, err)
	}
	return nil
}

func (service *Service) cleanupBeforeLaunch(ctx context.Context, artifactsRoot string, runnerSSHPort, targetSSHPort int) error {
	cleanupResult, err := service.Cleanup(ctx, CleanupConfig{
		ArtifactsRoot: artifactsRoot,
		RunnerSSHPort: runnerSSHPort,
		TargetSSHPort: targetSSHPort,
	})
	if err != nil {
		return err
	}
	if cleanupResult.Skipped {
		return errors.New(cleanupResult.Reason)
	}
	if len(cleanupResult.BusyPorts) > 0 {
		return fmt.Errorf("required harness ports are still busy after cleanup: %v", cleanupResult.BusyPorts)
	}
	return nil
}

func ensureRunDefaults(cfg *RunConfig) error {
	if cfg.ArtifactsRoot == "" {
		return fmt.Errorf("artifacts root is required")
	}
	if cfg.RepoRoot == "" {
		return fmt.Errorf("repo root is required")
	}
	if cfg.RunnerImage == "" {
		return fmt.Errorf("runner image is required")
	}
	if cfg.QemuBinary == "" {
		cfg.QemuBinary = "qemu-system-x86_64"
	}
	if cfg.QemuImgBinary == "" {
		cfg.QemuImgBinary = "qemu-img"
	}
	if cfg.RunnerSSHUser == "" {
		cfg.RunnerSSHUser = sshUserSkea
	}
	if cfg.RunnerSSHPort == 0 {
		cfg.RunnerSSHPort = 2222
	}
	if cfg.TargetSSHPort == 0 {
		cfg.TargetSSHPort = 2223
	}
	if cfg.MemoryMiB == 0 {
		cfg.MemoryMiB = 4096
	}
	if cfg.TargetMemoryMiB == 0 {
		cfg.TargetMemoryMiB = 4096
	}
	if cfg.CPUs == 0 {
		cfg.CPUs = 2
	}
	if !cfg.EnableTPM {
		cfg.EnableTPM = false
	} else {
		cfg.EnableTPM = true
	}
	if cfg.DiskSize == "" {
		cfg.DiskSize = "16G"
	}
	if cfg.InstalledUser == "" {
		cfg.InstalledUser = sshUserSkea
	}
	if cfg.InstalledPassword == "" {
		cfg.InstalledPassword = "Alpine.1"
	}
	if cfg.BootTimeout == 0 {
		cfg.BootTimeout = 10 * time.Minute
	}
	if cfg.VerificationTimeout == 0 {
		cfg.VerificationTimeout = 2 * time.Minute
	}
	if _, err := exec.LookPath(cfg.QemuBinary); err != nil {
		return fmt.Errorf("find %s: %w", cfg.QemuBinary, err)
	}
	if _, err := exec.LookPath(cfg.QemuImgBinary); err != nil {
		return fmt.Errorf("find %s: %w", cfg.QemuImgBinary, err)
	}
	if _, err := os.Stat(cfg.RunnerImage); err != nil {
		return fmt.Errorf("runner image %s: %w", cfg.RunnerImage, err)
	}
	// niri DMS is a native apk (dms-shell from feralos-aports) — no nix
	// packages and no flatpak injection for the niri desktop environment
	// (RustDesk removed; wayvnc is the remote-desktop path).
	return nil
}

func writeCloudInitFiles(userDataPath, metaDataPath string, cfg *PrepareRunnerConfig, keyPair guest.KeyPair) error {
	var writeFiles string
	if cfg.AptProxy != "" {
		writeFiles = fmt.Sprintf(`write_files:
  - path: /etc/apt/apt.conf.d/90skea-harness-proxy
    permissions: '0644'
    content: |
      Acquire::HTTP::Proxy "%s";
`, cfg.AptProxy)
	}

	packages := ""
	for _, pkg := range cfg.Packages {
		packages += fmt.Sprintf("  - %s\n", pkg)
	}

	userData := fmt.Sprintf(`#cloud-config
package_update: true
growpart:
  mode: auto
  devices: ['/']
resizefs: true
ssh_pwauth: false
users:
  - default
  - name: %s
    sudo: ALL=(ALL) NOPASSWD:ALL
    groups: sudo,adm
    shell: /bin/bash
    lock_passwd: true
    ssh_authorized_keys:
      - %s
packages:
%s%s`, cfg.SSHUser, keyPair.PublicKey, packages, writeFiles)

	metaData := "instance-id: skea-runner\nlocal-hostname: skea-runner\n"

	if err := os.WriteFile(userDataPath, []byte(userData), 0o600); err != nil {
		return fmt.Errorf("write user-data: %w", err)
	}
	if err := os.WriteFile(metaDataPath, []byte(metaData), 0o600); err != nil {
		return fmt.Errorf("write meta-data: %w", err)
	}

	return nil
}

func buildSeedISO(cloudLocaldsBinary, seedISO, userData, metaData string) error {
	cmd := exec.Command(cloudLocaldsBinary, seedISO, userData, metaData)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("build cloud-init seed ISO: %w: %s", err, string(output))
	}

	return nil
}

func writeHarnessVariables(cfg *RunConfig, keyPair guest.KeyPair, outputPath, alpineMirror string) error {
	yn := func(b bool) string {
		if b {
			return "true"
		}
		return "false"
	}

	disk1 := "/dev/vdb"
	disk2 := ""
	if cfg.DiskLayout == layoutBtrfsStandardServer || cfg.DiskLayout == layoutBtrfsRaid0 {
		disk2 = "/dev/vdc"
	}
	disksYaml := fmt.Sprintf("  disks:\n    - %q", disk1)
	if disk2 != "" {
		disksYaml += fmt.Sprintf("\n    - %q", disk2)
	}

	sshKey := escapeDoubleQuotes(keyPair.PublicKey)
	content := fmt.Sprintf(`variables:
  unattended: true

%s

  userName: "%s"
  password: '%s'
  hostname: "skea-server"
  timezone: "Europe/Rome"
  locale: "it"
  keymap: "it"

  mirror: "%s"
  arch: "x86_64"
  branch: "latest-stable"

  diskLayout: "%s"
  desktopEnvironment: "%s"
  videoDriver: "none"

  sshAuthorizedKey: "%s"
  defaultNetworkInterface: "eth0"

  enableSecureboot: %s
  enableTPM: %s
  enableHardening: %s
  enableZswap: %s
  enableNix: %s
  enableSwap: %s
  swapSize: "%s"
  enableDebug: true

  enablePodman: %s
  enableWebmin: %s
  enableDinit: %s
  webminListenAll: false

  extraPackages:
    - incus-feature
    - incus-feature-vm
    - incus-feature-client
    - incus-feature-utils
    - qemu-system-x86_64
    - qemu-img
    - qemu-modules
`,
		disksYaml,
		cfg.InstalledUser,
		escapeDoubleQuotes(cfg.InstalledPassword),
		alpineMirror,
		cfg.DiskLayout,
		deOrDefault(cfg.DesktopEnvironment),
		sshKey,
		yn(cfg.EnableSecureboot),
		yn(cfg.EnableTPM),
		yn(cfg.EnableHardening),
		yn(cfg.EnableZswap),
		yn(cfg.EnableNix),
		yn(cfg.EnableSwap),
		cfg.SwapSize,
		yn(cfg.EnablePodman),
		yn(cfg.EnableWebmin),
		yn(cfg.EnableDinit),
	)

	// Flatpak packages: if explicitly set (e.g. via -f fixture), propagate
	// to the installer variables.yaml so verification expectations match.
	if len(cfg.FlatpakPackages) > 0 {
		content += "  flatpakPackages:\n"
		for _, pkg := range cfg.FlatpakPackages {
			content += fmt.Sprintf("    - %q\n", pkg)
		}
	}
	if len(cfg.NixPackages) > 0 {
		content += "  nixPackages:\n"
		for _, pkg := range cfg.NixPackages {
			content += fmt.Sprintf("    - %q\n", pkg)
		}
	}

	if err := os.WriteFile(outputPath, []byte(content), 0o600); err != nil {
		return fmt.Errorf("write harness variables: %w", err)
	}
	return nil
}

func writeHarnessSetupGitShim(outputPath string) error {
	content := `#!/bin/sh
set -eu

if [ "${1:-}" != "clone" ]; then
	echo "unsupported harness git command: $*" >&2
	exit 1
fi
shift

if [ "${1:-}" = "--branch" ]; then
	shift 2
fi

[ $# -eq 2 ] || {
	echo "unexpected git clone arguments: $*" >&2
	exit 1
}

local_repo="${SKEA_HARNESS_LOCAL_REPO:-}"
[ -n "$local_repo" ] || {
	echo "SKEA_HARNESS_LOCAL_REPO is not set" >&2
	exit 1
}

destination="$2"
mkdir -p "$destination"

tar \
	--exclude='./.git' \
	--exclude='./tests/artifacts' \
	--exclude='./.skea-harness-bin' \
	--exclude='./.skea-harness-overrides.sh' \
	--exclude='./.skea-harness-setup-input.txt' \
	-C "$local_repo" -cf - . | tar -C "$destination" -xf -
`

	// #nosec G306 -- executable shell shim, 0755 required
	if err := os.WriteFile(outputPath, []byte(content), 0o755); err != nil {
		return fmt.Errorf("write harness git shim: %w", err)
	}

	return nil
}

func writeHarnessSetupBashShim(outputPath string) error {
	content := `#!/bin/sh
set -eu

if [ $# -gt 0 ]; then
	case "$1" in
		./setup.sh|setup.sh)
			exec 3>&2
			export BASH_XTRACEFD=3
			exec /bin/bash -x "$@"
			;;
	esac
fi

exec /bin/bash "$@"
`

	// #nosec G306 -- executable shell shim, 0755 required
	if err := os.WriteFile(outputPath, []byte(content), 0o755); err != nil {
		return fmt.Errorf("write harness bash shim: %w", err)
	}

	return nil
}

func writeHarnessSetupFdiskShim(outputPath string) error {
	content := `#!/bin/sh
set -eu

real_fdisk="/usr/sbin/fdisk"
[ -x "$real_fdisk" ] || real_fdisk="/sbin/fdisk"

if [ "${1:-}" = "-l" ] && [ $# -ge 2 ]; then
	status=0
	output="$($real_fdisk "$@" 2>&1)" || status=$?
	printf '%s\n' "$output"
	if [ "$status" -eq 0 ] && [ -b "$2" ]; then
		if ! printf '%s\n' "$output" | grep -qi "^Disk $2:.*bytes"; then
			size_bytes=$(blockdev --getsize64 "$2" 2>/dev/null || true)
			if [ -n "$size_bytes" ]; then
				printf 'Disk %s: %s bytes\n' "$2" "$size_bytes"
			fi
		fi
	fi
	exit "$status"
fi

exec "$real_fdisk" "$@"
`

	// #nosec G306 -- executable shell shim, 0755 required
	if err := os.WriteFile(outputPath, []byte(content), 0o755); err != nil {
		return fmt.Errorf("write harness setup fdisk shim: %w", err)
	}

	return nil
}

// deOrDefault returns the desktop environment name, defaulting to "none".
func deOrDefault(de string) string {
	if strings.TrimSpace(de) == "" {
		return "none"
	}
	return de
}

func resolveHarnessAlpineMirror(cfg *RunConfig) (string, error) {
	if strings.TrimSpace(cfg.AlpineMirror) != "" {
		return strings.TrimRight(cfg.AlpineMirror, "/"), nil
	}

	baseVariablesPath := filepath.Join(cfg.RepoRoot, "src", "variables", "variables.yaml")
	baseData, err := os.ReadFile(baseVariablesPath)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", baseVariablesPath, err)
	}

	pattern := regexp.MustCompile(`(?m)^  mirror:\s*("([^"]*)"|'([^']*)')\s*$`)
	matches := pattern.FindStringSubmatch(string(baseData))
	if len(matches) == 0 {
		return defaultAlpineMirrorURL, nil
	}

	rawValue := matches[1]
	value, unquoteErr := strconv.Unquote(rawValue)
	if unquoteErr == nil {
		return strings.TrimRight(value, "/"), nil
	}
	// rawValue may be an unquoted mirror string — fall back to trimming quotes manually
	return strings.TrimRight(rawValue, `"'`), nil
}

func cleanOldRuns(runsDir string, keep int) {
	entries, err := os.ReadDir(runsDir)
	if err != nil {
		return
	}

	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			names = append(names, entry.Name())
		}
	}

	sort.Strings(names)
	if len(names) <= keep {
		return
	}

	for _, name := range names[:len(names)-keep] {
		os.RemoveAll(filepath.Join(runsDir, name))
	}
}

func latestRunDir(root string) string {
	entries, err := os.ReadDir(root)
	if err != nil {
		return ""
	}

	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			names = append(names, filepath.Join(root, entry.Name()))
		}
	}

	sort.Strings(names)
	if len(names) == 0 {
		return ""
	}

	return names[len(names)-1]
}

func resolveSerialTargetRun(artifactsRoot, requestedRunDir string) (string, string, error) {
	if strings.TrimSpace(requestedRunDir) != "" {
		runDir, err := filepath.Abs(requestedRunDir)
		if err != nil {
			return "", "", fmt.Errorf("resolve run directory %s: %w", requestedRunDir, err)
		}
		if _, err := os.Stat(runDir); err != nil {
			return "", "", fmt.Errorf("run directory %s: %w", runDir, err)
		}
		return runDir, filepath.Base(runDir), nil
	}

	active, err := runstate.New(artifactsRoot).Load()
	if err != nil {
		return "", "", fmt.Errorf("load active run state: %w", err)
	}
	if strings.TrimSpace(active.RunDir) == "" {
		return "", "", fmt.Errorf("active run state does not include a run directory")
	}

	return active.RunDir, active.RunID, nil
}

func resolveLogPath(runDir, logName string) string {
	switch logName {
	case "runner-serial":
		return filepath.Join(runDir, "runner-serial.log")
	case "runner-qemu":
		return filepath.Join(runDir, "runner-qemu.log")
	case "target-serial":
		return filepath.Join(runDir, "target-serial.log")
	case "target-qemu":
		return filepath.Join(runDir, "target-qemu.log")
	case "target-firmware":
		return filepath.Join(runDir, "target-firmware.log")
	case "installer":
		return filepath.Join(runDir, "installer.log")
	case "post-install":
		return filepath.Join(runDir, "post-install.log")
	case "verify":
		return filepath.Join(runDir, "verify.md")
	case "harness":
		return filepath.Join(runDir, "harness.log")
	default:
		return ""
	}
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}

func escapeDoubleQuotes(value string) string {
	return strings.ReplaceAll(value, "\"", "\\\"")
}

func waitForGuestCommand(ctx context.Context, opts guest.Options, command string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	var lastErr error

	for time.Now().Before(deadline) {
		attemptCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		err := guest.Run(attemptCtx, opts, command, io.Discard, io.Discard)
		cancel()
		if err == nil {
			return nil
		}
		lastErr = err

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}

	if lastErr != nil {
		return fmt.Errorf("guest condition %q not satisfied within %s: %w", command, timeout, lastErr)
	}

	return fmt.Errorf("guest condition %q not satisfied within %s", command, timeout)
}

func waitForTargetSSH(ctx context.Context, instance *qemu.Instance, opts guest.Options, timeout time.Duration, serialLogPath, serialSocketPath, qmpPath, unlockPassword string, allowPassphraseFallback bool, concurrentPassphrases int, heartbeat func()) error {
	deadline := time.Now().Add(timeout)
	var lastErr error
	unlockAttempts := 0
	sawPromptWithoutFallback := false
	lastHeartbeat := time.Time{}
	// Capture serial log size at start of this boot attempt — only check new output
	// so old prompts from previous boots (still in last 4KB) don't trigger false inject.
	serialStartOffset := getSerialLogSize(serialLogPath)

	for time.Now().Before(deadline) {
		if heartbeat != nil && (lastHeartbeat.IsZero() || time.Since(lastHeartbeat) >= 15*time.Second) {
			heartbeat()
			lastHeartbeat = time.Now()
		}

		if exited, exitErr := instance.Exited(); exited {
			if exitErr != nil {
				return fmt.Errorf("target VM exited before SSH became ready: %w", exitErr)
			}
			return fmt.Errorf("target VM exited before SSH became ready")
		}

		attemptCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		err := guest.Run(attemptCtx, opts, "true", io.Discard, io.Discard)
		cancel()
		if err == nil {
			return nil
		}
		lastErr = err

		// Detect CURRENT LUKS prompt after this boot's start offset.
		// Old prompts from previous boots are ignored via minOffset.
		luksPrompt := serialLogHasPrompt(serialLogPath, "Enter passphrase for luks-", serialStartOffset)
		if !allowPassphraseFallback && luksPrompt {
			sawPromptWithoutFallback = true
		}
		if allowPassphraseFallback && strings.TrimSpace(unlockPassword) != "" {
			// Inject only once per boot — if cat succeeds here, LUKS will unlock and prompt disappears.
			// Previous code retried 4× every 5s → "mille volte" spam.
			promptRetryDue := luksPrompt && unlockAttempts < 1
			if promptRetryDue {
				logHarness("harness: LUKS prompt detected (attempt=%d), injecting passphrase\n", unlockAttempts+1)
				// Send one passphrase per LUKS device so all KDF operations start
				// concurrently. Sequential injection creates a seconds-long gap
				// between dm-devices appearing, which causes btrfs RAID1
				// open_ctree to fail on the first device before the second is
				// available, deregistering the first from the btrfs device list.
				n := concurrentPassphrases
				if n < 1 {
					n = 1
				}
				injected := false
				for i := 0; i < n; i++ {
					if err := sendBootPassphrase(serialSocketPath, qmpPath, unlockPassword); err != nil {
						lastErr = fmt.Errorf("send target boot passphrase %d/%d: %w", i+1, n, err)
						break
					}
					injected = true
				}
				if injected {
					unlockAttempts++
				}
			}
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}

	if sawPromptWithoutFallback {
		return fmt.Errorf("target showed a LUKS prompt during TPM-backed auto-unlock boot and SSH never became ready within %s", timeout)
	}

	if lastErr != nil {
		return fmt.Errorf("ssh did not become ready within %s: %w", timeout, lastErr)
	}

	return fmt.Errorf("ssh did not become ready within %s", timeout)
}

// powerOffTarget powers off the installed target with the command matching
// its init system, then waits for QEMU to exit (bounded — a guest that never
// powers off fails the run instead of hanging it).
//
//   - OpenRC (busybox init): `poweroff` works — busybox sends SIGUSR2, which
//     busybox init handles (runs rc.shutdown, then performs the poweroff).
//   - dinit: busybox `poweroff` is a NO-OP — dinit ignores SIGUSR2, and
//     `dinitctl shutdown` sends HALT (QEMU would stay up). The POWEROFF
//     request is `dinit-poweroff` (from the Alpine `dinit-shutdown` package,
//     installed by the installer): the dinit daemon rolls back services, then
//     execs `/usr/sbin/dinit-shutdown --system -p` which performs
//     reboot(RB_POWER_OFF) — QEMU exits.
//
// The SSH connection drops while services stop, so a command error is only
// treated as a failure when the command itself is missing (older install);
// then fall back to the ACPI power-button event.
func powerOffTarget(ctx context.Context, targetGuest guest.Options, targetInstance *qemu.Instance, cfg *RunConfig, qmpPath string) error {
	shutdownCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	if !cfg.EnableDinit {
		if err := guest.Run(shutdownCtx, targetGuest, "poweroff", io.Discard, io.Discard); err != nil {
			_ = qemu.SystemPowerdown(qmpPath)
		}
	} else {
		output, err := guest.Output(shutdownCtx, targetGuest, "/usr/sbin/dinit-poweroff")
		if err != nil && (strings.Contains(string(output), "not found") ||
			strings.Contains(string(output), "No such file")) {
			logHarness("harness: dinit-poweroff not available (%v) — falling back to ACPI powerdown\n", err)
			_ = qemu.SystemPowerdown(qmpPath)
		}
	}

	deadline := time.Now().Add(targetShutdownTimeout)
	for time.Now().Before(deadline) {
		if exited, _ := targetInstance.Exited(); exited {
			_ = targetInstance.Wait()
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	initName := "openrc"
	if cfg.EnableDinit {
		initName = "dinit"
	}
	return fmt.Errorf("target VM did not power off within %s (init: %s)", targetShutdownTimeout, initName)
}

func tpmQEMUArgs(socketPath string) []string {
	return []string{
		"-chardev", "socket,id=chrtpm,path=" + socketPath,
		"-tpmdev", "emulator,id=tpm0,chardev=chrtpm",
		"-device", "tpm-tis,tpmdev=tpm0",
	}
}

func sendBootPassphrase(serialSocketPath, qmpPath, passphrase string) error {
	if strings.TrimSpace(serialSocketPath) != "" {
		if err := qemu.WriteSerialChunks(serialSocketPath, 200*time.Millisecond, passphrase, "\r"); err == nil {
			logHarness("harness: passphrase sent via serial socket\n")
			return nil
		} else {
			logHarness("harness: serial socket passphrase failed: %v, falling back to QMP\n", err)
		}
	}

	if err := qemu.TypeText(qmpPath, passphrase); err != nil {
		return err
	}

	return qemu.PressEnter(qmpPath)
}

func runSbctlOnTarget(ctx context.Context, targetGuest guest.Options, repoRoot string) (string, error) {
	// Copy debug script to target
	debugSrc := filepath.Join(repoRoot, "tests", "harness", "debug.sh")
	if err := guest.CopyToGuest(ctx, targetGuest, debugSrc, "/root/debug.sh", false); err != nil {
		_ = err // non-fatal
	} else {
		_ = guest.Run(ctx, targetGuest, "chmod +x /root/debug.sh", io.Discard, io.Discard)
	}

	// Secureboot binary is pre-installed by the installer at
	// /opt/feralos_secureboot — no build/copy needed.
	script := `set -eu
export SKEA_NONINTERACTIVE=1
export SKEA_NONINTERACTIVE_DECISION=ignore
[ -x /root/debug.sh ] && sh /root/debug.sh || true
[ -x /opt/feralos_secureboot ] && /opt/feralos_secureboot || true
`
	var outBuf, errBuf strings.Builder
	if err := guest.Run(ctx, targetGuest, "sh -s -- <<'SCRIPT'\n"+script+"\nSCRIPT", &outBuf, &errBuf); err != nil {
		detail := strings.TrimSpace(errBuf.String())
		if outBuf.Len() > 0 {
			detail = strings.TrimSpace(outBuf.String()) + "\nstderr: " + detail
		}
		if detail != "" {
			return detail, fmt.Errorf("%w: %s", err, detail)
		}
		return "", err
	}
	return outBuf.String(), nil
}

func runTPMOnTarget(ctx context.Context, targetGuest guest.Options, password string) (string, error) {
	if password == "" {
		password = "Alpine.1"
	}
	// TPM binary is pre-installed by the installer at
	// /opt/feralos_tpm — no build/copy needed.
	script := `set -eu
export SKEA_NONINTERACTIVE=1
export SKEA_NONINTERACTIVE_DECISION=ignore
export SKEA_LUKS_PASSWORD=` + shellQuote(password) + `
/opt/feralos_tpm
`

	var outBuf, errBuf strings.Builder
	if err := guest.Run(ctx, targetGuest, "sh -s -- <<'SCRIPT'\n"+script+"\nSCRIPT", &outBuf, &errBuf); err != nil {
		detail := strings.TrimSpace(errBuf.String())
		if outBuf.Len() > 0 {
			detail = strings.TrimSpace(outBuf.String()) + "\nstderr: " + detail
		}
		if detail != "" {
			return detail, fmt.Errorf("%w: %s", err, detail)
		}
		return "", err
	}
	return outBuf.String(), nil
}

func runScriptOnTarget(ctx context.Context, targetGuest guest.Options, script string) (string, error) {
	var outBuf, errBuf strings.Builder
	if err := guest.Run(ctx, targetGuest, "sh -s -- <<'SCRIPT'\n"+script+"\nSCRIPT", &outBuf, &errBuf); err != nil {
		detail := strings.TrimSpace(errBuf.String())
		if outBuf.Len() > 0 {
			detail = strings.TrimSpace(outBuf.String()) + "\nstderr: " + detail
		}
		if detail != "" {
			return detail, fmt.Errorf("%w: %s", err, detail)
		}
		return "", err
	}
	return outBuf.String(), nil
}

// runWebminOnTarget runs the Webmin firstBoot binary — writes /opt/webmin
// compose + configs, pulls the image and starts the container.
func runWebminOnTarget(ctx context.Context, targetGuest guest.Options) (string, error) {
	script := `set -eu
export SKEA_NONINTERACTIVE=1
/opt/feralos_webmin
`
	return runScriptOnTarget(ctx, targetGuest, script)
}

func runPostInstallDiag(ctx context.Context, targetGuest guest.Options) (string, error) {
	script := `set -eu
echo '=== /etc/kernel/cmdline ==='
cat /etc/kernel/cmdline 2>/dev/null || true
echo '=== /etc/booster.yaml ==='
cat /etc/booster.yaml 2>/dev/null || true
echo '=== LUKS token dump ==='
cryptroots=$(tr ' ' '\n' < /etc/kernel/cmdline | sed -n 's/^rd\.luks\.name=\([^ =]*\)=.*/\1/p' | sort -u)
for cryptroot in $cryptroots; do
	dev=$(findfs "UUID=${cryptroot}" 2>/dev/null || true)
	[ -n "$dev" ] || continue
	echo "=== $dev ==="
	clevis luks list -d "$dev" 2>/dev/null || true
	cryptsetup token list "$dev" 2>/dev/null || true
done
`
	return runScriptOnTarget(ctx, targetGuest, script)
}

func getSerialLogSize(path string) int64 {
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return info.Size()
}

// serialLogHasPrompt checks if the serial log contains a CURRENT LUKS prompt
// after minOffset. minOffset is the file size at start of this boot attempt,
// so old prompts from previous boots (still in last 4KB) don't trigger.
// The LUKS prompt is often interleaved with kernel output on the same line,
// so we check for "Enter passphrase" + "luks" separately rather than the
// exact continuous string "Enter passphrase for luks-".
func serialLogHasPrompt(path, needle string, minOffset int64) bool {
	file, err := os.Open(path)
	if err != nil {
		return false
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return false
	}
	// Only check new data after minOffset, capped to last 4KB
	startOffset := minOffset
	if info.Size()-startOffset > 4096 {
		startOffset = info.Size() - 4096
	}
	if startOffset < 0 {
		startOffset = 0
	}
	if startOffset >= info.Size() {
		return false
	}
	if _, err = file.Seek(startOffset, io.SeekStart); err != nil {
		return false
	}
	data, err := io.ReadAll(file)
	if err != nil {
		return false
	}
	dataStr := string(data)
	// Check for LUKS prompt — may be interleaved with kernel output.
	// Look for "Enter passphrase" + "luks" separately (not continuous).
	if strings.Contains(dataStr, "Enter passphrase") && strings.Contains(dataStr, "luks") {
		return true
	}
	return strings.Contains(dataStr, needle)
}

func loadSerialLogOffset(offsetPath string) (int64, error) {
	data, err := os.ReadFile(offsetPath)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("read %s: %w", offsetPath, err)
	}
	value := strings.TrimSpace(string(data))
	if value == "" {
		return 0, nil
	}
	offset, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse %s: %w", offsetPath, err)
	}
	if offset < 0 {
		return 0, nil
	}
	return offset, nil
}

func readFileFromOffset(path string, offset int64) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	if offset > 0 {
		if _, err := file.Seek(offset, io.SeekStart); err != nil {
			return nil, fmt.Errorf("seek %s to %d: %w", path, offset, err)
		}
	}
	return io.ReadAll(file)
}

func createRepositoryArchive(repoRoot, archivePath string) error {
	if err := os.MkdirAll(filepath.Dir(archivePath), 0o755); err != nil {
		return fmt.Errorf("create repository archive directory: %w", err)
	}

	cmd := exec.Command(
		"tar",
		"--exclude=./tests/artifacts",
		"--exclude=./.git",
		"--exclude=*.sock",
		"--exclude=*.qcow2",
		"--exclude=*.iso",
		"-czf",
		archivePath,
		"-C",
		repoRoot,
		".",
	)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("create repository archive: %w: %s", err, string(output))
	}

	return nil
}

type processInfo struct {
	PID     int
	Command string
}

func activeRunHealthy(active *runstate.ActiveRun) bool {
	qmpPath := active.TargetQMPPath
	if qmpPath == "" {
		qmpPath = active.RunnerQMPPath
	}
	if qmpPath == "" {
		return false
	}

	status, err := qemu.QueryStatus(qmpPath)
	return err == nil && status.Running
}

func killHarnessQEMUProcesses(artifactsRoot, runDir string, ports []int) ([]int, error) {
	processes, err := listProcesses()
	if err != nil {
		return nil, err
	}

	killedPIDs := make([]int, 0)
	for _, process := range processes {
		if !isHarnessQEMUProcess(process.Command, artifactsRoot, runDir, ports) {
			continue
		}
		if err := killProcess(process.PID); err != nil {
			return killedPIDs, fmt.Errorf("kill process %d: %w", process.PID, err)
		}
		killedPIDs = append(killedPIDs, process.PID)
	}

	if err := waitForProcessesToExit(killedPIDs, 5*time.Second); err != nil {
		return killedPIDs, err
	}

	return killedPIDs, nil
}

func listProcesses() ([]processInfo, error) {
	cmd := exec.Command("ps", "-eo", "pid=,args=")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("list processes: %w", err)
	}

	processes := make([]processInfo, 0)
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		pid, err := strconv.Atoi(fields[0])
		if err != nil {
			continue
		}
		command := strings.TrimSpace(line[len(fields[0]):])
		processes = append(processes, processInfo{PID: pid, Command: command})
	}

	return processes, nil
}

func isHarnessQEMUProcess(command, artifactsRoot, runDir string, ports []int) bool {
	isQEMU := strings.Contains(command, "qemu-system")
	isSWTPM := strings.Contains(command, "swtpm socket")
	if !isQEMU && !isSWTPM {
		return false
	}
	if runDir != "" && strings.Contains(command, runDir) {
		return true
	}
	if artifactsRoot != "" && strings.Contains(command, artifactsRoot) {
		return true
	}
	if isQEMU && (strings.Contains(command, "-name skea-runner") || strings.Contains(command, "-name skea-runner-prep") || strings.Contains(command, "-name skea-installed-target")) {
		return true
	}
	for _, port := range ports {
		if isQEMU && strings.Contains(command, fmt.Sprintf("hostfwd=tcp::%d-:22", port)) && strings.Contains(command, "skea-") {
			return true
		}
	}
	return false
}

func killProcess(pid int) error {
	err := syscall.Kill(pid, syscall.SIGKILL)
	if err != nil && !errors.Is(err, syscall.ESRCH) {
		return err
	}
	return nil
}

func waitForProcessesToExit(pids []int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for _, pid := range pids {
		for {
			err := syscall.Kill(pid, 0)
			if errors.Is(err, syscall.ESRCH) {
				break
			}
			if err != nil {
				return fmt.Errorf("probe process %d: %w", pid, err)
			}
			if time.Now().After(deadline) {
				return fmt.Errorf("process %d did not exit after cleanup", pid)
			}
			time.Sleep(100 * time.Millisecond)
		}
	}
	return nil
}

func unavailablePorts(ports []int) []int {
	busy := make([]int, 0)
	for _, port := range uniquePorts(ports...) {
		listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
		if err != nil {
			busy = append(busy, port)
			continue
		}
		_ = listener.Close()
	}
	return busy
}

func uniquePorts(ports ...int) []int {
	seen := map[int]struct{}{}
	result := make([]int, 0, len(ports))
	for _, port := range ports {
		if port <= 0 {
			continue
		}
		if _, ok := seen[port]; ok {
			continue
		}
		seen[port] = struct{}{}
		result = append(result, port)
	}
	return result
}

func appendUniqueInts(existing []int, values ...int) []int {
	seen := map[int]struct{}{}
	result := make([]int, 0, len(existing)+len(values))
	for _, value := range existing {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func (cfg *RunConfig) shouldBootInstalledTarget() bool {
	return true
}

// writeTestResults writes individual check results to tests/<name>.txt
// and a summary table to tests/summary.md inside the run directory.
func writeTestResults(runDir string, result VerifyResult) {
	testDir := filepath.Join(runDir, "tests")
	if err := os.MkdirAll(testDir, 0o755); err != nil {
		return
	}

	// Individual check files
	for name, check := range result.Checks {
		status := "PASS"
		if !check.Pass {
			status = "FAIL"
		}
		content := fmt.Sprintf("%s\n\n%s\n", status, check.Detail)
		_ = os.WriteFile(filepath.Join(testDir, name+".txt"), []byte(content), 0o600)
	}

	// Summary table
	var sb strings.Builder
	sb.WriteString("# Verification Summary\n\n")
	sb.WriteString(fmt.Sprintf("**Result:** %s  \n**Total:** %d checks, %d passed, %d failed — %s\n\n",
		boolStr(result.Success, "PASS", "FAIL"),
		len(result.Checks), countPassed(result.Checks), countFailed(result.Checks),
		boolStr(result.Success, "🟢 ALL PASS", "🔴 FAIL")))
	sb.WriteString(formatCheckTable(result.Checks))

	_ = os.WriteFile(filepath.Join(testDir, "summary.md"), []byte(sb.String()), 0o600)
}

// formatCheckTable renders verify checks as a markdown table.
func formatCheckTable(checks map[string]CheckResult) string {
	var sb strings.Builder
	sb.WriteString("| Check | Status | Detail |\n")
	sb.WriteString("|-------|--------|--------|\n")
	for _, name := range sortedKeys(checks) {
		check := checks[name]
		status := "🟢 PASS"
		if !check.Pass {
			status = "🔴 FAIL"
		}
		detail := strings.ReplaceAll(check.Detail, "Warning: Permanently added '[127.0.0.1]:2223' (ED25519) to the list of known hosts.", "")
		detail = strings.ReplaceAll(detail, "\n", " ")
		detail = strings.Join(strings.Fields(detail), " ")
		detail = strings.TrimSpace(detail)
		if detail == "" {
			detail = "—"
		}
		if len(detail) > 100 {
			detail = detail[:97] + "..."
		}
		detail = strings.ReplaceAll(detail, "|", "\\|")
		sb.WriteString(fmt.Sprintf("| %s | %s | %s |\n", name, status, detail))
	}
	return sb.String()
}

func boolStr(b bool, trueStr, falseStr string) string {
	if b {
		return trueStr
	}
	return falseStr
}

func countPassed(checks map[string]CheckResult) int {
	n := 0
	for _, c := range checks {
		if c.Pass {
			n++
		}
	}
	return n
}

func countFailed(checks map[string]CheckResult) int {
	n := 0
	for _, c := range checks {
		if !c.Pass {
			n++
		}
	}
	return n
}

func sortedKeys(m map[string]CheckResult) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
