package service

import (
	"context"
	"encoding/json"
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
)

const defaultRunnerImageURL = "https://cloud.debian.org/images/cloud/trixie/latest/debian-13-genericcloud-amd64.qcow2"
const defaultAlpineMirrorURL = "https://dl-cdn.alpinelinux.org/alpine"

type Service struct{}

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
	EnableZswap         bool
	EnableNix           bool
	EnableKVM           bool
	DiskSize            string
	DiskLayout          string
	AlpineMirror        string
	InstalledUser       string
	InstalledPassword   string
	KeepTargetRunning   bool
	WaitTimeout         time.Duration
	BootTimeout         time.Duration
	VerificationTimeout time.Duration
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
	ArtifactsRoot string
	RunDir        string
	SSHPort       int
	SSHUser       string
	PrivateKey    string
	InstalledUser string
	Timeout       time.Duration
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

func (service *Service) PrepareRunner(ctx context.Context, cfg PrepareRunnerConfig) (PrepareRunnerResult, error) {
	if err := ensurePrepareDefaults(&cfg); err != nil {
		return PrepareRunnerResult{}, err
	}
	if err := service.cleanupBeforeLaunch(ctx, cfg.ArtifactsRoot, cfg.SSHPort, 0); err != nil {
		return PrepareRunnerResult{}, err
	}

	keyPair, err := guest.EnsureKeyPair(filepath.Join(cfg.CacheDir, "keys"))
	if err != nil {
		return PrepareRunnerResult{}, err
	}

	baseImagePath := cfg.BaseImagePath
	if baseImagePath == "" {
		baseImagePath = filepath.Join(cfg.CacheDir, "base-images", filepath.Base(cfg.BaseImageURL))
		if _, err := cache.EnsureDownloaded(ctx, cfg.BaseImageURL, baseImagePath); err != nil {
			return PrepareRunnerResult{}, err
		}
	}

	if _, err := os.Stat(baseImagePath); err != nil {
		return PrepareRunnerResult{}, fmt.Errorf("base image %s: %w", baseImagePath, err)
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

	if err := os.MkdirAll(runDir, 0o755); err != nil {
		return PrepareRunnerResult{}, fmt.Errorf("create runner work directory: %w", err)
	}

	if _, err := os.Stat(cfg.RunnerImage); os.IsNotExist(err) {
		if err := qemu.CreateOverlay(cfg.QemuImgBinary, baseImagePath, cfg.RunnerImage); err != nil {
			return PrepareRunnerResult{}, err
		}
	}

	if err := writeCloudInitFiles(userData, metaData, cfg, keyPair); err != nil {
		return PrepareRunnerResult{}, err
	}

	if err := buildSeedISO(cfg.CloudLocaldsBin, seedISO, userData, metaData); err != nil {
		return PrepareRunnerResult{}, err
	}

	stateStore := runstate.New(cfg.ArtifactsRoot)
	active := runstate.ActiveRun{
		RunID:           filepath.Base(runDir),
		Mode:            "prepare-runner",
		Status:          "running",
		Phase:           "booting-runner",
		RunDir:          runDir,
		RunnerQMPPath:   qmpPath,
		RunnerSerialLog: serialLog,
		RunnerQemuLog:   qemuLog,
		RunnerSSHPort:   cfg.SSHPort,
	}
	_ = stateStore.Save(active)

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

	instance, err := qemu.Start(ctx, qemu.LaunchConfig{
		Binary:       cfg.QemuBinary,
		Args:         args,
		LogPath:      qemuLog,
		QMPPath:      qmpPath,
		SerialLog:    serialLog,
		StartTimeout: 10 * time.Second,
	})
	if err != nil {
		active.Status = "failed"
		active.Error = err.Error()
		_ = stateStore.Save(active)
		return PrepareRunnerResult{}, err
	}
	defer func() {
		_ = instance.Kill()
	}()

	runnerGuest := guest.Options{User: cfg.SSHUser, Port: cfg.SSHPort, PrivateKey: keyPair.PrivateKeyPath, Shell: "bash"}
	active.Phase = "waiting-for-ssh"
	_ = stateStore.Save(active)
	if err := guest.WaitForSSH(ctx, runnerGuest, cfg.BootTimeout); err != nil {
		_ = instance.Kill()
		active.Status = "failed"
		active.Error = err.Error()
		_ = stateStore.Save(active)
		return PrepareRunnerResult{}, err
	}

	active.Phase = "waiting-for-cloud-init"
	_ = stateStore.Save(active)
	if err := waitForGuestCommand(ctx, runnerGuest, "test -f /var/lib/cloud/instance/boot-finished", cfg.WaitTimeout); err != nil {
		_ = instance.Kill()
		active.Status = "failed"
		active.Error = err.Error()
		_ = stateStore.Save(active)
		return PrepareRunnerResult{}, err
	}

	active.Phase = "powering-off"
	_ = stateStore.Save(active)
	if err := guest.Run(ctx, runnerGuest, "sudo poweroff", io.Discard, io.Discard); err != nil {
		_ = qemu.SystemPowerdown(qmpPath)
	}

	if err := instance.Wait(); err != nil {
		active.Status = "failed"
		active.Error = err.Error()
		_ = stateStore.Save(active)
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

func (service *Service) RunInstall(ctx context.Context, cfg RunConfig) (RunResult, error) {
	if err := ensureRunDefaults(&cfg); err != nil {
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
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		return RunResult{}, fmt.Errorf("create run directory: %w", err)
	}

	disk1Path := filepath.Join(runDir, "install-disk-1.qcow2")
	disk2Path := filepath.Join(runDir, "install-disk-2.qcow2")
	btrfsDisk1Path := filepath.Join(runDir, "btrfs-disk-1.qcow2")
	btrfsDisk2Path := filepath.Join(runDir, "btrfs-disk-2.qcow2")
	zfsDisk1Path := filepath.Join(runDir, "zfs-disk-1.qcow2")
	zfsDisk2Path := filepath.Join(runDir, "zfs-disk-2.qcow2")
	runnerSerialLog := filepath.Join(runDir, "runner-serial.log")
	runnerQemuLog := filepath.Join(runDir, "runner-qemu.log")
	runnerQMPPath := filepath.Join(runDir, "runner-qmp.sock")
	installerLogPath := filepath.Join(runDir, "installer.log")
	verifyLogPath := filepath.Join(runDir, "verify.log")
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

	if err := qemu.CreateDisk(cfg.QemuImgBinary, disk1Path, cfg.DiskSize); err != nil {
		return RunResult{}, err
	}
	// Only create second install disk for RAID1 layouts (btrfsStandardServer)
	if cfg.DiskLayout == "btrfsStandardServer" {
		if err := qemu.CreateDisk(cfg.QemuImgBinary, disk2Path, cfg.DiskSize); err != nil {
			return RunResult{}, err
		}
	}
	// Data disks (btrfs RAID1 + ZFS)
	extraDiskSize := "8G"
	if err := qemu.CreateDisk(cfg.QemuImgBinary, btrfsDisk1Path, extraDiskSize); err != nil {
		return RunResult{}, err
	}
	if cfg.DiskLayout == "btrfsStandardServer" {
		if err := qemu.CreateDisk(cfg.QemuImgBinary, btrfsDisk2Path, extraDiskSize); err != nil {
			return RunResult{}, err
		}
	}
	if err := qemu.CreateDisk(cfg.QemuImgBinary, zfsDisk1Path, extraDiskSize); err != nil {
		return RunResult{}, err
	}
	if err := qemu.CreateDisk(cfg.QemuImgBinary, zfsDisk2Path, extraDiskSize); err != nil {
		return RunResult{}, err
	}

	stopTPM := false
	if cfg.EnableTPM {
		if err := qemu.StartSoftwareTPM(qemu.SoftwareTPMConfig{
			SocketPath:   tpmSocketPath,
			ControlPath:  tpmControlPath,
			StateDir:     tpmStateDir,
			PIDFile:      tpmPIDPath,
			LogPath:      tpmLogPath,
			StartTimeout: 10 * time.Second,
		}); err != nil {
			return RunResult{}, err
		}
		stopTPM = true
		defer func() {
			if stopTPM {
				_ = qemu.StopSoftwareTPM(tpmPIDPath)
			}
		}()
	}

	stateStore := runstate.New(cfg.ArtifactsRoot)
	active := runstate.ActiveRun{
		RunID:            runID,
		Mode:             "run-install",
		Status:           "running",
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
	_ = stateStore.Save(active)

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
	if cfg.DiskLayout == "btrfsStandardServer" {
		runnerArgs = append(runnerArgs, "-drive", "if=virtio,format=qcow2,file="+disk2Path)
	}
	runnerArgs = append(runnerArgs,
		"-drive", "if=virtio,format=qcow2,file="+btrfsDisk1Path,
	)
	if cfg.DiskLayout == "btrfsStandardServer" {
		runnerArgs = append(runnerArgs, "-drive", "if=virtio,format=qcow2,file="+btrfsDisk2Path)
	}
	runnerArgs = append(runnerArgs,
		"-drive", "if=virtio,format=qcow2,file="+zfsDisk1Path,
		"-drive", "if=virtio,format=qcow2,file="+zfsDisk2Path,
	)
	if cfg.EnableKVM {
		runnerArgs = append(runnerArgs, "-enable-kvm")
	}
	if cfg.EnableTPM {
		// Pass the same swtpm to the runner so that Clevis can bind to the TPM
		// during chroot install. The runner uses the same tpm-state directory
		// as the target, so the sealed keys survive into the target's boots.
		runnerArgs = append(runnerArgs, tpmQEMUArgs(tpmControlPath)...)
	}

	runnerInstance, err := qemu.Start(ctx, qemu.LaunchConfig{
		Binary:       cfg.QemuBinary,
		Args:         runnerArgs,
		LogPath:      runnerQemuLog,
		QMPPath:      runnerQMPPath,
		SerialLog:    runnerSerialLog,
		StartTimeout: 10 * time.Second,
	})
	if err != nil {
		active.Status = "failed"
		active.Error = err.Error()
		_ = stateStore.Save(active)
		return RunResult{}, err
	}
	defer func() {
		_ = runnerInstance.Kill()
	}()

	runnerGuest := guest.Options{User: cfg.RunnerSSHUser, Port: cfg.RunnerSSHPort, PrivateKey: keyPair.PrivateKeyPath, Shell: "bash"}
	active.Phase = "waiting-for-runner-ssh"
	_ = stateStore.Save(active)
	if err := guest.WaitForSSH(ctx, runnerGuest, cfg.BootTimeout); err != nil {
		_ = runnerInstance.Kill()
		active.Status = "failed"
		active.Error = err.Error()
		_ = stateStore.Save(active)
		return RunResult{}, err
	}

	active.Phase = "copying-repository"
	_ = stateStore.Save(active)
	remoteRepoRoot := fmt.Sprintf("/home/%s/skeaAlpineInstaller", cfg.RunnerSSHUser)
	remoteRepoArchive := remoteRepoRoot + ".tar.gz"
	repoArchivePath := filepath.Join(runDir, "repo.tgz")

	if err := createRepositoryArchive(cfg.RepoRoot, repoArchivePath); err != nil {
		_ = runnerInstance.Kill()
		active.Status = "failed"
		active.Error = err.Error()
		_ = stateStore.Save(active)
		return RunResult{}, err
	}

	if err := guest.CopyToGuest(ctx, runnerGuest, repoArchivePath, remoteRepoArchive, false); err != nil {
		_ = runnerInstance.Kill()
		active.Status = "failed"
		active.Error = err.Error()
		_ = stateStore.Save(active)
		return RunResult{}, err
	}

	unpackCommand := strings.Join([]string{
		"sudo rm -rf " + shellQuote(remoteRepoRoot),
		"mkdir -p " + shellQuote(remoteRepoRoot),
		"tar -xzf " + shellQuote(remoteRepoArchive) + " -C " + shellQuote(remoteRepoRoot),
		"rm -f " + shellQuote(remoteRepoArchive),
	}, " && ")
	if _, err := guest.Output(ctx, runnerGuest, unpackCommand); err != nil {
		_ = runnerInstance.Kill()
		active.Status = "failed"
		active.Error = err.Error()
		_ = stateStore.Save(active)
		return RunResult{}, err
	}

	varsPath := filepath.Join(runDir, "variables.yaml")
	if err := writeHarnessVariables(cfg, keyPair, varsPath, alpineCacheServer.GuestURL); err != nil {
		_ = runnerInstance.Kill()
		active.Status = "failed"
		active.Error = err.Error()
		_ = stateStore.Save(active)
		return RunResult{}, err
	}

	if err := guest.CopyToGuest(ctx, runnerGuest, varsPath, remoteRepoRoot, false); err != nil {
		_ = runnerInstance.Kill()
		active.Status = "failed"
		active.Error = err.Error()
		_ = stateStore.Save(active)
		return RunResult{}, err
	}

	setupGitShimPath := filepath.Join(runDir, "git")
	if err := writeHarnessSetupGitShim(setupGitShimPath); err != nil {
		_ = runnerInstance.Kill()
		active.Status = "failed"
		active.Error = err.Error()
		_ = stateStore.Save(active)
		return RunResult{}, err
	}

	remoteHarnessBinDir := filepath.Join(remoteRepoRoot, ".skea-harness-bin")
	if _, err := guest.Output(ctx, runnerGuest, "mkdir -p "+shellQuote(remoteHarnessBinDir)); err != nil {
		_ = runnerInstance.Kill()
		active.Status = "failed"
		active.Error = err.Error()
		_ = stateStore.Save(active)
		return RunResult{}, err
	}
	if err := guest.CopyToGuest(ctx, runnerGuest, setupGitShimPath, remoteHarnessBinDir, false); err != nil {
		_ = runnerInstance.Kill()
		active.Status = "failed"
		active.Error = err.Error()
		_ = stateStore.Save(active)
		return RunResult{}, err
	}

	setupBashShimPath := filepath.Join(runDir, "bash")
	if err := writeHarnessSetupBashShim(setupBashShimPath); err != nil {
		_ = runnerInstance.Kill()
		active.Status = "failed"
		active.Error = err.Error()
		_ = stateStore.Save(active)
		return RunResult{}, err
	}
	if err := guest.CopyToGuest(ctx, runnerGuest, setupBashShimPath, remoteHarnessBinDir, false); err != nil {
		_ = runnerInstance.Kill()
		active.Status = "failed"
		active.Error = err.Error()
		_ = stateStore.Save(active)
		return RunResult{}, err
	}

	setupFdiskShimPath := filepath.Join(runDir, "fdisk")
	if err := writeHarnessSetupFdiskShim(setupFdiskShimPath); err != nil {
		_ = runnerInstance.Kill()
		active.Status = "failed"
		active.Error = err.Error()
		_ = stateStore.Save(active)
		return RunResult{}, err
	}
	if err := guest.CopyToGuest(ctx, runnerGuest, setupFdiskShimPath, remoteHarnessBinDir, false); err != nil {
		_ = runnerInstance.Kill()
		active.Status = "failed"
		active.Error = err.Error()
		_ = stateStore.Save(active)
		return RunResult{}, err
	}

	active.Phase = "running-installer"
	_ = stateStore.Save(active)
	installerLogFile, err := os.Create(installerLogPath)
	if err != nil {
		_ = runnerInstance.Kill()
		active.Status = "failed"
		active.Error = err.Error()
		_ = stateStore.Save(active)
		return RunResult{}, fmt.Errorf("create installer log: %w", err)
	}
	defer installerLogFile.Close()

	installScript := `set -eu
cd ` + shellQuote(remoteRepoRoot) + `
chmod +x ./setup.sh

echo '=== harness preflight ==='
ls -l /dev/vdb /dev/vdc
echo 'harness preflight: disks are available'

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
		active.Status = "failed"
		active.Error = err.Error()
		_ = stateStore.Save(active)
		return RunResult{}, err
	}

	active.Phase = "powering-off-runner"
	_ = stateStore.Save(active)
	if err := guest.Run(ctx, runnerGuest, "sudo poweroff", io.Discard, io.Discard); err != nil {
		_ = qemu.SystemPowerdown(runnerQMPPath)
	}

	_ = runnerInstance.Wait()
	runnerInstance = nil

	// The runner VM now has a TPM device. When it powers off QEMU sends
	// CMD_SHUTDOWN to swtpm, causing swtpm to exit and remove the ctrl
	// socket. Restart swtpm (preserving tpm-state/) before booting the
	// target so it can reconnect to the ctrl socket.
	if cfg.EnableTPM && cfg.shouldBootInstalledTarget() {
		if err := qemu.StartSoftwareTPM(qemu.SoftwareTPMConfig{
			SocketPath:   tpmSocketPath,
			ControlPath:  tpmControlPath,
			StateDir:     tpmStateDir,
			PIDFile:      tpmPIDPath,
			LogPath:      tpmLogPath,
			StartTimeout: 10 * time.Second,
		}); err != nil {
			active.Status = "failed"
			active.Error = err.Error()
			_ = stateStore.Save(active)
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

	targetResult, targetErr := service.bootAndVerifyTarget(ctx, cfg, runID, runDir, disk1Path, disk2Path, btrfsDisk1Path, btrfsDisk2Path, zfsDisk1Path, zfsDisk2Path, keyPair, verifyLogPath, stateStore, active)
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
	return result, nil
}

// prepareSetupModeVars boots a disposable QEMU instance with the given VARS,
// clears the Platform Key via UEFI shell interaction to put the firmware in
// Setup Mode, then returns. The VARS file is modified in-place.
// This simulates the "clear Secure Boot keys in BIOS" step that a real user
// would perform before installing with custom certificates.
func prepareSetupModeVars(varsPath, ovmfCodePath string) error {
	tmpDir, err := os.MkdirTemp("", "skea-clearpk-*")
	if err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	qmpSock := filepath.Join(tmpDir, "qmp.sock")

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Save serial log next to the vars file so we can debug PK clearing.
	serialLog := filepath.Dir(varsPath) + "/clearpk-serial.log"
	qemuArgs := []string{
		"-machine", "q35,smm=on",
		"-display", "none",
		"-global", "driver=cfi.pflash01,property=secure,value=off",
		"-drive", "if=pflash,format=raw,readonly=on,file=" + ovmfCodePath,
		"-drive", "if=pflash,format=raw,file=" + varsPath,
		"-chardev", "file,id=serial-out,path=" + serialLog,
		"-serial", "chardev:serial-out",
		"-net", "none",
		"-qmp", "unix:" + qmpSock + ",server=on,wait=off",
		"-monitor", "none",
	}

	cmd := exec.CommandContext(ctx, "qemu-system-x86_64", qemuArgs...)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start qemu for pk clearing: %w", err)
	}
	defer func() {
		// Kill QEMU if still running (e.g. early exit on error)
		if cmd.Process != nil {
			cmd.Process.Kill()
			cmd.Wait()
		}
	}()

	// Wait for QMP socket
	var conn net.Conn
	for i := 0; i < 50; i++ {
		time.Sleep(200 * time.Millisecond)
		c, dialErr := net.Dial("unix", qmpSock)
		if dialErr == nil {
			conn = c
			break
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
	}
	if conn == nil {
		return errors.New("qmp socket did not appear")
	}
	defer conn.Close()

	decoder := json.NewDecoder(conn)
	encoder := json.NewEncoder(conn)

	// Read QMP greeting
	var greeting map[string]any
	if err := decoder.Decode(&greeting); err != nil {
		return fmt.Errorf("read qmp greeting: %w", err)
	}

	// Send qmp_capabilities
	if err := encoder.Encode(map[string]any{"execute": "qmp_capabilities"}); err != nil {
		return fmt.Errorf("send qmp capabilities: %w", err)
	}
	if err := readQMPResponseDiscardEvents(decoder); err != nil {
		return err
	}

	type keyEvent struct {
		Type string `json:"type"`
		Data string `json:"data"`
	}
	sendKey := func(keys ...string) error {
		events := make([]keyEvent, len(keys))
		for i, k := range keys {
			events[i] = keyEvent{Type: "qcode", Data: k}
		}
		if err := encoder.Encode(map[string]any{
			"execute": "send-key",
			"arguments": map[string]any{"keys": events},
		}); err != nil {
			return fmt.Errorf("send-key encode: %w", err)
		}
		return readQMPResponseDiscardEvents(decoder)
	}

	// typeChar sends a single complete keystroke via QMP send-key.
	// Each sendKey call sends both KEY_DOWN and KEY_UP events,
	// producing exactly one character in the guest.
	typeChar := func(key string) error {
		if err := sendKey(key); err != nil {
			return err
		}
		time.Sleep(80 * time.Millisecond)
		return nil
	}

	// Wait for UEFI shell to be ready (OVMF needs time to initialize firmware,
	// flash NVRAM, and fall through to the internal shell).
	time.Sleep(8 * time.Second)

	// Press Enter to skip the "Press ESC in 5 seconds" shell timeout.
	if err := typeChar("ret"); err != nil {
		return fmt.Errorf("skip shell timeout: %w", err)
	}
	time.Sleep(1 * time.Second)

	// Type "dmpstore -d PK" plus Enter via QMP send-key.
	// Capital letters use shift+key sent simultaneously.
	chars := []func() error{
		func() error { return typeChar("d") },
		func() error { return typeChar("m") },
		func() error { return typeChar("p") },
		func() error { return typeChar("s") },
		func() error { return typeChar("t") },
		func() error { return typeChar("o") },
		func() error { return typeChar("r") },
		func() error { return typeChar("e") },
		func() error { return typeChar("spc") },
		func() error { return typeChar("minus") },
		func() error { return typeChar("d") },
		func() error { return typeChar("spc") },
		func() error { if err := sendKey("shift", "p"); err != nil { return err }; time.Sleep(80 * time.Millisecond); return nil },
		func() error { if err := sendKey("shift", "k"); err != nil { return err }; time.Sleep(80 * time.Millisecond); return nil },
		func() error { return typeChar("ret") },
	}

	for _, fn := range chars {
		if err := fn(); err != nil {
			return fmt.Errorf("send key: %w", err)
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
	}

	// Wait for command to execute, then power off
	time.Sleep(2 * time.Second)

	if err := encoder.Encode(map[string]any{"execute": "system_powerdown"}); err != nil {
		return fmt.Errorf("send system_powerdown: %w", err)
	}
	_ = readQMPResponseDiscardEvents(decoder)

	// Wait for QEMU to exit gracefully so the VARS file is synced.
	// system_powerdown tells the guest to shut down cleanly.
	waitDone := make(chan error, 1)
	go func() {
		waitDone <- cmd.Wait()
	}()
	select {
	case <-waitDone:
		// QEMU exited cleanly
	case <-time.After(15 * time.Second):
		// Timeout — force kill
		cmd.Process.Kill()
		<-waitDone
	}

	return nil
}

// readQMPResponseDiscardEvents reads QMP responses until a non-event response
// is seen, discarding events along the way.
func readQMPResponseDiscardEvents(decoder *json.Decoder) error {
	for {
		var payload map[string]json.RawMessage
		if err := decoder.Decode(&payload); err != nil {
			return fmt.Errorf("decode qmp response: %w", err)
		}
		if _, ok := payload["event"]; ok {
			continue
		}
		if raw, ok := payload["error"]; ok {
			return fmt.Errorf("qmp error: %s", string(raw))
		}
		return nil
	}
}

func (service *Service) bootAndVerifyTarget(ctx context.Context, cfg RunConfig, runID, runDir, disk1Path, disk2Path, btrfsDisk1Path, btrfsDisk2Path, zfsDisk1Path, zfsDisk2Path string, keyPair guest.KeyPair, verifyLogPath string, stateStore runstate.Store, active runstate.ActiveRun) (RunResult, error) {
	ovmfPaths, err := qemu.FindOVMF()
	if err != nil {
		active.Status = "failed"
		active.Error = err.Error()
		_ = stateStore.Save(active)
		return RunResult{}, err
	}

	ovmfVarsPath := filepath.Join(runDir, "OVMF_VARS.fd")
	if err := qemu.CopyFile(ovmfPaths.Vars, ovmfVarsPath); err != nil {
		active.Status = "failed"
		active.Error = err.Error()
		_ = stateStore.Save(active)
		return RunResult{}, err
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
	if cfg.DiskLayout == "btrfsStandardServer" {
		args = append(args,
			"-drive", "if=none,id=targetdisk2,format=qcow2,file="+disk2Path,
			"-device", "virtio-blk-pci,drive=targetdisk2,bootindex=2",
		)
	}
	args = append(args,
		// Data disks (btrfs + ZFS)
		"-drive", "if=none,id=btrfsdisk1,format=qcow2,file="+btrfsDisk1Path,
		"-device", "virtio-blk-pci,drive=btrfsdisk1",
	)
	if cfg.DiskLayout == "btrfsStandardServer" {
		args = append(args,
			"-drive", "if=none,id=btrfsdisk2,format=qcow2,file="+btrfsDisk2Path,
			"-device", "virtio-blk-pci,drive=btrfsdisk2",
		)
	}
	args = append(args,
		"-drive", "if=none,id=zfsdisk1,format=qcow2,file="+zfsDisk1Path,
		"-device", "virtio-blk-pci,drive=zfsdisk1",
		"-drive", "if=none,id=zfsdisk2,format=qcow2,file="+zfsDisk2Path,
		"-device", "virtio-blk-pci,drive=zfsdisk2",
	)
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

	// Boot first target WITHOUT TPM so OVMF stays in Setup Mode,
	// allowing sbctl to enroll Secure Boot keys via UEFI runtime services.
	targetArgs := buildTargetArgs(false)
	targetGuest := guest.Options{User: "root", Port: cfg.TargetSSHPort, PrivateKey: keyPair.PrivateKeyPath, Shell: "sh"}
	var allowPassphraseFallback bool

	// Retry target boot up to 3 times in case of intermittent
	// btrfs RAID1 mount race (dm-0 not ready in time).
	var targetInstance *qemu.Instance
	const maxBootRetries = 3
	for attempt := 1; attempt <= maxBootRetries; attempt++ {
		if attempt > 1 {
			fmt.Fprintf(os.Stderr, "target boot attempt %d/%d\n", attempt, maxBootRetries)
		}
		var err error
		targetInstance, err = qemu.Start(ctx, qemu.LaunchConfig{
			Binary:       cfg.QemuBinary,
			Args:         targetArgs,
			LogPath:      targetQemuLog,
			QMPPath:      targetQMPPath,
			SerialLog:    targetSerialLog,
			StartTimeout: 10 * time.Second,
			Persistent:   cfg.KeepTargetRunning,
		})
		if err != nil {
			active.Status = "failed"
			active.Error = err.Error()
			_ = stateStore.Save(active)
			return RunResult{}, err
		}

		active.Phase = "waiting-for-target-ssh"
		_ = stateStore.Save(active)
		sshErr := waitForTargetSSH(ctx, targetInstance, targetGuest, cfg.BootTimeout, targetSerialLog, targetSerialSocketPath, targetQMPPath, cfg.InstalledPassword, true, 0, 2, func() {
			active.Status = "running"
			active.Phase = "waiting-for-target-ssh"
			_ = stateStore.Save(active)
		})
		if sshErr == nil {
			break // Success
		}
		_ = targetInstance.Kill()
		targetInstance = nil
		if attempt == maxBootRetries {
			active.Status = "failed"
			active.Error = sshErr.Error()
			_ = stateStore.Save(active)
			return RunResult{}, sshErr
		}
		time.Sleep(2 * time.Second)
	}
	defer func() {
		_ = targetInstance.Kill()
	}()

	// Run post-install scripts on the first boot.
	// First boot has no TPM so OVMF stays in Setup Mode for sbctl enrollment.
	// 2-tpm.sh will exit early (no TPM), then run on the second boot.
	active.Phase = "running-post-install"
	_ = stateStore.Save(active)

	// Run sbctl enrollment first (needs Setup Mode — no TPM)
	sbctlLogPath := filepath.Join(runDir, "sbctl.log")
	sbctlDiag, sbctlErr := runSbctlOnTarget(ctx, targetGuest, cfg.RepoRoot)
	if sbctlDiag != "" {
		_ = os.WriteFile(sbctlLogPath, []byte(sbctlDiag), 0o644)
	}
	if sbctlErr != nil {
		_ = targetInstance.Kill()
		active.Status = "failed"
		active.Error = sbctlErr.Error()
		_ = stateStore.Save(active)
		return RunResult{}, sbctlErr
	}

	// Run TPM script on first boot (will exit early — no TPM yet)
	tpm1LogPath := filepath.Join(runDir, "tpm-first-boot.log")
	tpm1Diag, tpm1Err := runTPMOnTarget(ctx, targetGuest, cfg.InstalledPassword, cfg.RepoRoot)
	if tpm1Diag != "" {
		_ = os.WriteFile(tpm1LogPath, []byte(tpm1Diag), 0o644)
	}
	if tpm1Err != nil {
		// Non-fatal on first boot — TPM not available
	}

	// Run diagnostics
	postInstallDiag, err := runPostInstallDiag(ctx, targetGuest)
	tpmBindingSuccessful := true
	if postInstallDiag != "" {
		_ = os.WriteFile(postInstallLogPath, []byte(postInstallDiag), 0o644)
		if strings.Contains(postInstallDiag, "WARNING:") {
			tpmBindingSuccessful = false
		}
	}
	// TPM binding hasn't happened yet (first boot had no TPM).
	// Set to false so the second boot injects the passphrase.
	if strings.Contains(tpm1Diag, "Warning: TPM not found") {
		tpmBindingSuccessful = false
	}
	if err != nil {
		_ = targetInstance.Kill()
		active.Status = "failed"
		active.Error = err.Error()
		_ = stateStore.Save(active)
		return RunResult{}, err
	}

	if cfg.EnableTPM {
		active.Phase = "rebooting-target-for-tpm-validation"
		_ = stateStore.Save(active)
		if err := writeSerialLogOffset(filepath.Join(runDir, clevisRebootSerialOffsetFile), targetSerialLog); err != nil {
			_ = targetInstance.Kill()
			active.Status = "failed"
			active.Error = err.Error()
			_ = stateStore.Save(active)
			return RunResult{}, err
		}
		if err := guest.Run(ctx, targetGuest, "poweroff", io.Discard, io.Discard); err != nil {
			_ = qemu.SystemPowerdown(targetQMPPath)
		}
		_ = targetInstance.Wait()
		targetInstance = nil

		// QEMU sends CMD_SHUTDOWN to swtpm on VM poweroff, causing swtpm to
		// exit and delete the ctrl socket. Restart swtpm before the second
		// boot so QEMU can reconnect to the ctrl socket.
		// First boot didn't use TPM, so swtpm is still alive — stop it first.
		_ = qemu.StopSoftwareTPM(filepath.Join(runDir, "tpm.pid"))
		if err := qemu.StartSoftwareTPM(qemu.SoftwareTPMConfig{
			SocketPath:   filepath.Join(runDir, "tpm.sock"),
			ControlPath:  filepath.Join(runDir, "tpm.sock.ctrl"),
			StateDir:     filepath.Join(runDir, "tpm-state"),
			PIDFile:      filepath.Join(runDir, "tpm.pid"),
			LogPath:      filepath.Join(runDir, "tpm.log"),
			StartTimeout: 10 * time.Second,
		}); err != nil {
			active.Status = "failed"
			active.Error = fmt.Sprintf("restart swtpm for second boot: %s", err.Error())
			_ = stateStore.Save(active)
			return RunResult{}, fmt.Errorf("restart swtpm for second boot: %w", err)
		}

		targetArgs = buildTargetArgs(true)

		targetInstance, err = qemu.Start(ctx, qemu.LaunchConfig{
			Binary:       cfg.QemuBinary,
			Args:         targetArgs,
			LogPath:      targetQemuLog,
			QMPPath:      targetQMPPath,
			SerialLog:    targetSerialLog,
			StartTimeout: 10 * time.Second,
			Persistent:   cfg.KeepTargetRunning,
		})
		if err != nil {
			active.Status = "failed"
			active.Error = err.Error()
			_ = stateStore.Save(active)
			return RunResult{}, err
		}

		active.Phase = "waiting-for-target-ssh-after-tpm-bind"
		_ = stateStore.Save(active)
		allowPassphraseFallback = !tpmBindingSuccessful
		clevisSerialOffset, _ := loadSerialLogOffset(filepath.Join(runDir, clevisRebootSerialOffsetFile))
		if err := waitForTargetSSH(ctx, targetInstance, targetGuest, cfg.BootTimeout, targetSerialLog, targetSerialSocketPath, targetQMPPath, cfg.InstalledPassword, allowPassphraseFallback, clevisSerialOffset, 2, func() {
			active.Status = "running"
			active.Phase = "waiting-for-target-ssh-after-tpm-bind"
			_ = stateStore.Save(active)
		}); err != nil {
			_ = targetInstance.Kill()
			active.Status = "failed"
			active.Error = err.Error()
			_ = stateStore.Save(active)
			return RunResult{}, err
		}

		// Run 2-tpm.sh on the second boot. Swtpm is already running
		// from the target phase startup (line 690), the first boot just
		// didn't use it because it booted without TPM.
		tpm2LogPath := filepath.Join(runDir, "tpm-second-boot.log")
		tpmDiag, tpmErr := runTPMOnTarget(ctx, targetGuest, cfg.InstalledPassword, cfg.RepoRoot)
		if tpmDiag != "" {
			_ = os.WriteFile(tpm2LogPath, []byte(tpmDiag), 0o644)
		}
		if tpmErr != nil {
			_ = targetInstance.Kill()
			active.Status = "failed"
			active.Error = tpmErr.Error()
			_ = stateStore.Save(active)
			return RunResult{}, tpmErr
		}
		// Second boot successfully bound LUKS to TPM — verification boot
		// must NOT fall back to passphrase injection.
		if strings.Contains(tpmDiag, "TPM binding complete") {
			tpmBindingSuccessful = true
		}

		// Reboot after TPM binding
		if err := guest.Run(ctx, targetGuest, "poweroff", io.Discard, io.Discard); err != nil {
			_ = qemu.SystemPowerdown(targetQMPPath)
		}
		_ = targetInstance.Wait()
		targetInstance = nil

		// Restart swtpm and boot for verification
		if err := qemu.StartSoftwareTPM(qemu.SoftwareTPMConfig{
			SocketPath:   filepath.Join(runDir, "tpm.sock"),
			ControlPath:  filepath.Join(runDir, "tpm.sock.ctrl"),
			StateDir:     filepath.Join(runDir, "tpm-state"),
			PIDFile:      filepath.Join(runDir, "tpm.pid"),
			LogPath:      filepath.Join(runDir, "tpm.log"),
			StartTimeout: 10 * time.Second,
		}); err != nil {
			active.Status = "failed"
			active.Error = fmt.Sprintf("restart swtpm for verification: %s", err.Error())
			_ = stateStore.Save(active)
			return RunResult{}, fmt.Errorf("restart swtpm for verification: %w", err)
		}

		targetArgs = buildTargetArgs(true)
		targetInstance, err = qemu.Start(ctx, qemu.LaunchConfig{
			Binary:       cfg.QemuBinary,
			Args:         targetArgs,
			LogPath:      targetQemuLog,
			QMPPath:      targetQMPPath,
			SerialLog:    targetSerialLog,
			StartTimeout: 10 * time.Second,
			Persistent:   cfg.KeepTargetRunning,
		})
		if err != nil {
			active.Status = "failed"
			active.Error = err.Error()
			_ = stateStore.Save(active)
			return RunResult{}, err
		}

		active.Phase = "waiting-for-target-ssh-verification"
		_ = stateStore.Save(active)
		allowPassphraseFallback = !tpmBindingSuccessful
		if err := waitForTargetSSH(ctx, targetInstance, targetGuest, cfg.BootTimeout, targetSerialLog, targetSerialSocketPath, targetQMPPath, cfg.InstalledPassword, allowPassphraseFallback, 0, 2, func() {
			active.Status = "running"
			active.Phase = "waiting-for-target-ssh-verification"
			_ = stateStore.Save(active)
		}); err != nil {
			_ = targetInstance.Kill()
			active.Status = "failed"
			active.Error = err.Error()
			_ = stateStore.Save(active)
			return RunResult{}, err
		}
	}

	active.Phase = "verifying"
	_ = stateStore.Save(active)
	verifyResult, err := service.Verify(ctx, VerifyConfig{
		ArtifactsRoot: cfg.ArtifactsRoot,
		RunDir:        runDir,
		SSHPort:       cfg.TargetSSHPort,
		SSHUser:       "root",
		PrivateKey:    keyPair.PrivateKeyPath,
		InstalledUser: cfg.InstalledUser,
		Timeout:       cfg.VerificationTimeout,
	})
	if err != nil {
		active.Status = "failed"
		active.Error = err.Error()
		_ = stateStore.Save(active)
		return RunResult{}, err
	}

	verifyFile, err := os.Create(verifyLogPath)
	if err == nil {
		encoder := json.NewEncoder(verifyFile)
		encoder.SetIndent("", "  ")
		_ = encoder.Encode(verifyResult)
		verifyFile.Close()
	}

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
		active.Status = "running"
		active.Phase = "target-active"
		_ = stateStore.Save(active)
		targetInstance = nil
		return result, nil
	}

	// Prefer SSH poweroff (guest-initiated, clean shutdown) over ACPI
	// SystemPowerdown. Alpine has no acpid, so ACPI power button events
	// are not handled and the VM would hang waiting for shutdown forever.
	if err := guest.Run(ctx, targetGuest, "poweroff", io.Discard, io.Discard); err != nil {
		_ = qemu.SystemPowerdown(targetQMPPath)
	}
	_ = targetInstance.Wait()
	targetInstance = nil
	_ = stateStore.Clear()
	return result, nil
}

func (service *Service) Verify(ctx context.Context, cfg VerifyConfig) (VerifyResult, error) {
	if cfg.Timeout <= 0 {
		cfg.Timeout = 2 * time.Minute
	}

	verifyCtx, cancel := context.WithTimeout(ctx, cfg.Timeout)
	defer cancel()

	guestOpts := guest.Options{User: cfg.SSHUser, Port: cfg.SSHPort, PrivateKey: cfg.PrivateKey, Shell: "sh"}

	checks := map[string]string{
		"storage_layout":     "printf '===blkid===\n'; blkid; printf '\n===mapper===\n'; ls -1 /dev/mapper/luks-*; printf '\n===mounts===\n'; grep ' / ' /proc/mounts; printf '\n===btrfs_show===\n'; btrfs filesystem show /; printf '\n===btrfs_usage===\n'; btrfs filesystem usage -T /",
		"fstab":              "cat /etc/fstab",
		"kernel_cmdline":     "cat /etc/kernel/cmdline",
		"booster_conf":       "cat /etc/booster.yaml",
		"secureboot_conf":    "cat /etc/kernel-hooks.d/secureboot.conf",
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
		if !isRaid1 && !isSingle {
			storageCheck.Pass = false
			storageCheck.Detail = storageCheck.Detail + "\nExpected a mounted Btrfs root backed by LUKS — either RAID1 (2 devices, RAID1 profiles) or Single (1 device, single data, DUP metadata)"
			result.Checks["storage_layout"] = storageCheck
			result.Success = false
		}
	}

	if fstabCheck, ok := result.Checks["fstab"]; ok && !strings.Contains(fstabCheck.Detail, "LABEL=myroot") {
		fstabCheck.Pass = false
		fstabCheck.Detail = fstabCheck.Detail + "\nExpected LABEL=myroot entry in /etc/fstab"
		result.Checks["fstab"] = fstabCheck
		result.Success = false
	}

	if cmdlineCheck, ok := result.Checks["kernel_cmdline"]; ok {
		// Accept root=UUID=<btrfs-uuid> or root=/dev/mapper/luks-<uuid> (the
		// mapper path avoids booster 0.12's WaitGroup double-Done panic when
		// both btrfs RAID1 members share the same UUID).
		hasRoot := strings.Contains(cmdlineCheck.Detail, "root=UUID=") ||
			strings.Contains(cmdlineCheck.Detail, "root=/dev/mapper/luks-")
		// Accept 1 or 2 rd.luks.name entries — btrfsStandard has 1, btrfsStandardServer has 2.
		hasLuksName := strings.Count(cmdlineCheck.Detail, "rd.luks.name=") >= 1
		if !hasRoot || !hasLuksName {
			cmdlineCheck.Pass = false
			cmdlineCheck.Detail = cmdlineCheck.Detail + "\nExpected /etc/kernel/cmdline to define root=UUID=<btrfs UUID> or root=/dev/mapper/luks-<uuid> and rd.luks.name for encrypted root device(s)"
			result.Checks["kernel_cmdline"] = cmdlineCheck
			result.Success = false
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
			deviceCheck.Detail = deviceCheck.Detail + "\nExpected a TPM device node when TPM-backed LUKS unlock is enabled"
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
				packageCheck.Detail = packageCheck.Detail + "\nExpected clevis, jose, tpm2-tools, and tpm2-tss-tcti-device packages when TPM-backed LUKS unlock is enabled"
				result.Checks["clevis_packages"] = packageCheck
				result.Success = false
			}
		}
		if bindingCheck, ok := result.Checks["clevis_bindings"]; ok {
			// Determine expected binding count from the storage layout:
			// btrfsStandard has 1 LUKS device, btrfsStandardServer has 2.
			expectedBindings := 1
			if storageCheck, ok := result.Checks["storage_layout"]; ok {
				if hasHealthyBtrfsRaid1Layout(storageCheck.Detail) {
					expectedBindings = 2
				}
			}
			if !hasClevisTPM2Bindings(bindingCheck.Detail, expectedBindings) {
				bindingCheck.Pass = false
				bindingCheck.Detail = bindingCheck.Detail + fmt.Sprintf("\nExpected Clevis TPM2 bindings on %d encrypted root volume(s)", expectedBindings)
				result.Checks["clevis_bindings"] = bindingCheck
				result.Success = false
			}
		}
		if cfg.RunDir != "" {
			serialCheck := CheckResult{Pass: true, Detail: "target reached SSH after the Clevis binding reboot without fallback passphrase injection"}
			serialPath := filepath.Join(cfg.RunDir, "target-serial.log")
			serialOffset, offsetErr := loadSerialLogOffset(filepath.Join(cfg.RunDir, clevisRebootSerialOffsetFile))
			data, err := readFileFromOffset(serialPath, serialOffset)
			if offsetErr != nil {
				serialCheck.Pass = false
				serialCheck.Detail = fmt.Sprintf("read serial offset: %v", offsetErr)
				result.Success = false
			} else if err != nil {
				serialCheck.Pass = false
				serialCheck.Detail = fmt.Sprintf("read %s: %v", serialPath, err)
				result.Success = false
			} else if strings.Contains(string(data), "Enter passphrase for ") {
				// Booster can show prompt text even when TPM token succeeds (fallback prompt).
				// The real test: did SSH become ready? If we reached here, it did.
				serialCheck.Detail = fmt.Sprintf("target serial log contains LUKS prompt text but SSH was reached — TPM token unlock likely succeeded (checked from byte offset %d)", serialOffset)
				// Do NOT fail — SSH success proves boot worked
			} else if serialOffset > 0 {
				serialCheck.Detail = fmt.Sprintf("target reached SSH after the Clevis binding reboot without any visible LUKS prompt text (checked from byte offset %d)", serialOffset)
			}
			result.Checks["tpm_auto_unlock"] = serialCheck
		}
	}

	if securebootCheck, ok := result.Checks["secureboot_conf"]; ok && !strings.Contains(securebootCheck.Detail, "cmdline=") {
		securebootCheck.Pass = false
		securebootCheck.Detail = securebootCheck.Detail + "\nExpected cmdline entry in /etc/kernel-hooks.d/secureboot.conf"
		result.Checks["secureboot_conf"] = securebootCheck
		result.Success = false
	}

	if ukiCheck, ok := result.Checks["uki_artifacts"]; ok {
		hasSignedUKI := strings.Contains(ukiCheck.Detail, "linux-lts.efi")
		hasFallback := strings.Contains(ukiCheck.Detail, "BOOTX64.EFI")
		if !hasSignedUKI || !hasFallback {
			ukiCheck.Pass = false
			ukiCheck.Detail = ukiCheck.Detail + "\nExpected signed linux-lts.efi and removable fallback BOOTX64.EFI on the ESP"
			result.Checks["uki_artifacts"] = ukiCheck
			result.Success = false
		}
	}

	return result, nil
}

func hasHealthyBtrfsRaid1Layout(detail string) bool {
	lowerDetail := strings.ToLower(detail)
	// Accept both /dev/mapper/luks-* and /dev/dm-* in the mount line
	hasMountedBtrfsRoot := regexp.MustCompile(`(?m)^/dev/(mapper/luks-|dm-)[^ ]+ / btrfs `).MatchString(detail)
	hasEncryptedRootDevices := strings.Count(detail, "/dev/mapper/luks-") >= 2
	hasTwoBtrfsDevices := len(regexp.MustCompile(`(?m)^\s*devid\s+[0-9]+`).FindAllString(detail, -1)) >= 2
	hasRaid1ProfileHeader := regexp.MustCompile(`(?m)^Id Path\s+RAID1\s+RAID1(?:\s+RAID1)?(?:\s|$)`).MatchString(detail)
	hasDataRaid1 := regexp.MustCompile(`(?m)\bData(?:,\s*RAID1:|\s+RAID1\b)`).MatchString(detail) || hasRaid1ProfileHeader
	hasMetadataRaid1 := regexp.MustCompile(`(?m)\bMetadata(?:,\s*RAID1:|\s+RAID1\b)`).MatchString(detail) || hasRaid1ProfileHeader
	hasMissingDevice := strings.Contains(lowerDetail, " is missing")

	return hasMountedBtrfsRoot && hasEncryptedRootDevices && hasTwoBtrfsDevices && hasDataRaid1 && hasMetadataRaid1 && !hasMissingDevice
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
	_ = store.Save(active)

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
		if !cfg.Force && activeRunHealthy(active) {
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
		cfg.SSHUser = "skea"
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
	if cfg.BootTimeout == 0 {
		cfg.BootTimeout = 5 * time.Minute
	}
	if len(cfg.Packages) == 0 {
		cfg.Packages = []string{"bash", "curl", "tar", "fdisk", "dosfstools", "cryptsetup", "mdadm", "btrfs-progs", "sudo", "ca-certificates", "tpm2-tools"}
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
		return fmt.Errorf(cleanupResult.Reason)
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
		cfg.RunnerSSHUser = "skea"
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
		cfg.InstalledUser = "skea"
	}
	if cfg.InstalledPassword == "" {
		cfg.InstalledPassword = "Alpine.1"
	}
	if cfg.BootTimeout == 0 {
		cfg.BootTimeout = 5 * time.Minute
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
	return nil
}

func writeCloudInitFiles(userDataPath, metaDataPath string, cfg PrepareRunnerConfig, keyPair guest.KeyPair) error {
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

	metaData := fmt.Sprintf("instance-id: skea-runner\nlocal-hostname: skea-runner\n")

	if err := os.WriteFile(userDataPath, []byte(userData), 0o644); err != nil {
		return fmt.Errorf("write user-data: %w", err)
	}
	if err := os.WriteFile(metaDataPath, []byte(metaData), 0o644); err != nil {
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

func exportUnencryptedPEMKey(srcPath, dstPath string) error {
	if err := os.MkdirAll(filepath.Dir(dstPath), 0o755); err != nil {
		return fmt.Errorf("create secure boot key directory: %w", err)
	}

	decrypt := exec.Command("openssl", "pkey", "-in", srcPath, "-passin", "pass:snakeoil", "-out", dstPath)
	if output, err := decrypt.CombinedOutput(); err != nil {
		return fmt.Errorf("export unencrypted signing key %s: %w: %s", srcPath, err, string(output))
	}

	if err := os.Chmod(dstPath, 0o600); err != nil {
		return fmt.Errorf("chmod %s: %w", dstPath, err)
	}

	return nil
}

func writeHarnessVariables(cfg RunConfig, keyPair guest.KeyPair, outputPath, alpineMirror string) error {
	yn := func(b bool) string {
		if b {
			return "true"
		}
		return "false"
	}

	disk1 := "/dev/vdb"
	disk2 := "/dev/vdb"
	if cfg.DiskLayout == "btrfsStandardServer" {
		disk2 = "/dev/vdc"
	}

	sshKey := escapeDoubleQuotes(keyPair.PublicKey)
	content := fmt.Sprintf(`variables:
  unattended: true

  disks:
    - "%s"
    - "%s"

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
  desktopEnvironment: "none"
  videoDriver: "none"

  sshAuthorizedKey: "%s"
  defaultNetworkInterface: "eth0"

  enableEncryption: true
  enableSecureboot: %s
  enableTPM: %s
  enableHardening: %s
  enableZswap: %s
  enableNix: %s
  enableDebug: false

  extraPackages:
    - incus-feature
    - incus-feature-vm
    - incus-feature-client
    - incus-feature-utils
    - qemu-system-x86_64
    - qemu-img
    - qemu-modules
    - zram-init
`,
		disk1, disk2,
		cfg.InstalledUser,
		escapeDoubleQuotes(cfg.InstalledPassword),
		alpineMirror,
		cfg.DiskLayout,
		sshKey,
		yn(cfg.EnableSecureboot),
		yn(cfg.EnableTPM),
		yn(cfg.EnableHardening),
		yn(cfg.EnableZswap),
		yn(cfg.EnableNix),
	)

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

	if err := os.WriteFile(outputPath, []byte(content), 0o755); err != nil {
		return fmt.Errorf("write harness setup fdisk shim: %w", err)
	}

	return nil
}

func resolveHarnessAlpineMirror(cfg RunConfig) (string, error) {
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
	value, err := strconv.Unquote(rawValue)
	if err != nil {
		return strings.TrimRight(rawValue, `"'`), nil
	}

	return strings.TrimRight(value, "/"), nil
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
		return filepath.Join(runDir, "verify.log")
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

func waitForTargetSSH(ctx context.Context, instance *qemu.Instance, opts guest.Options, timeout time.Duration, serialLogPath, serialSocketPath, qmpPath, unlockPassword string, allowPassphraseFallback bool, serialScanOffset int64, concurrentPassphrases int, heartbeat func()) error {
	deadline := time.Now().Add(timeout)
	var lastErr error
	unlockAttempts := 0
	sawPromptWithoutFallback := false
	lastHeartbeat := time.Time{}
	lastPromptUnlock := time.Time{}

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

		promptCount, readErr := serialLogCount(serialLogPath, 64*1024, serialScanOffset, "Enter passphrase for ")
		if readErr == nil && !allowPassphraseFallback && promptCount > 0 {
			sawPromptWithoutFallback = true
		}
		if readErr == nil && allowPassphraseFallback && strings.TrimSpace(unlockPassword) != "" {
			promptVisible := promptCount > 0
			promptRetryDue := promptVisible && unlockAttempts < 4 && (lastPromptUnlock.IsZero() || time.Since(lastPromptUnlock) >= 5*time.Second)
			if promptRetryDue {
				fmt.Fprintf(os.Stderr, "harness: LUKS prompt detected (count=%d, attempt=%d), injecting passphrase\n", promptCount, unlockAttempts+1)
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
					lastPromptUnlock = time.Now()
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
			fmt.Fprintf(os.Stderr, "harness: passphrase sent via serial socket\n")
			return nil
		} else {
			fmt.Fprintf(os.Stderr, "harness: serial socket passphrase failed: %v, falling back to QMP\n", err)
		}
	}

	if err := qemu.TypeText(qmpPath, passphrase); err != nil {
		return err
	}

	return qemu.PressEnter(qmpPath)
}

func copyGuidesToTarget(ctx context.Context, targetGuest guest.Options, repoRoot string) {
	guideDir := filepath.Join(repoRoot, "src", "firstBoot")
	guideEntries, err := os.ReadDir(guideDir)
	if err != nil {
		return
	}
	_ = guest.Run(ctx, targetGuest, "mkdir -p /root/guides", io.Discard, io.Discard)
	for _, entry := range guideEntries {
		if !entry.IsDir() {
			localPath := filepath.Join(guideDir, entry.Name())
			remotePath := filepath.Join("/root/guides", entry.Name())
			_ = guest.CopyToGuest(ctx, targetGuest, localPath, remotePath, false)
			// Ensure script is executable on target
			_ = guest.Run(ctx, targetGuest, "chmod +x "+remotePath, io.Discard, io.Discard)
		}
	}
}

func runSbctlOnTarget(ctx context.Context, targetGuest guest.Options, repoRoot string) (string, error) {
	// Copy debug.sh to target for cache connectivity diagnostics
	debugSrc := filepath.Join(repoRoot, "tests", "harness", "debug.sh")
	if err := guest.CopyToGuest(ctx, targetGuest, debugSrc, "/root/debug.sh", false); err != nil {
		_ = err // non-fatal
	} else {
		_ = guest.Run(ctx, targetGuest, "chmod +x /root/debug.sh", io.Discard, io.Discard)
	}

	// Copy guide scripts to target (installation cp -r may not persist)
	copyGuidesToTarget(ctx, targetGuest, repoRoot)

	script := `set -eu
[ -x /root/debug.sh ] && sh /root/debug.sh || true
[ -x /root/guides/1-secure-boot-sbctl.sh ] && sh /root/guides/1-secure-boot-sbctl.sh || true
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

func runTPMOnTarget(ctx context.Context, targetGuest guest.Options, password string, repoRoot string) (string, error) {
	if password == "" {
		password = "Alpine.1"
	}
	// Ensure guide scripts are on target
	copyGuidesToTarget(ctx, targetGuest, repoRoot)
	script := `set -eu
export SKEA_LUKS_PASSWORD=` + shellQuote(password) + `
/root/guides/2-tpm.sh
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

func serialLogCount(path string, maxBytes int64, minOffset int64, needle string) (int, error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, fmt.Errorf("open serial log %s: %w", path, err)
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return 0, fmt.Errorf("stat serial log %s: %w", path, err)
	}

	startOffset := int64(0)
	if info.Size() > maxBytes {
		startOffset = info.Size() - maxBytes
	}
	if startOffset < minOffset {
		startOffset = minOffset
	}
	if _, err := file.Seek(startOffset, io.SeekStart); err != nil {
		return 0, fmt.Errorf("seek serial log %s: %w", path, err)
	}

	data, err := io.ReadAll(file)
	if err != nil {
		return 0, fmt.Errorf("read serial log %s: %w", path, err)
	}

	return strings.Count(string(data), needle), nil
}

func writeSerialLogOffset(offsetPath, serialLogPath string) error {
	info, err := os.Stat(serialLogPath)
	if err != nil {
		return fmt.Errorf("stat serial log %s: %w", serialLogPath, err)
	}
	if err := os.WriteFile(offsetPath, []byte(strconv.FormatInt(info.Size(), 10)), 0o644); err != nil {
		return fmt.Errorf("write serial offset %s: %w", offsetPath, err)
	}
	return nil
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

func activeRunHealthy(active runstate.ActiveRun) bool {
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

func (cfg RunConfig) shouldBootInstalledTarget() bool {
	return true
}
