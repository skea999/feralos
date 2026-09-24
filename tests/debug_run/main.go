package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"gopkg.in/yaml.v3"
)

type variablesYAML struct {
	Variables struct {
		UserName                string   `yaml:"userName"`
		Password                string   `yaml:"password"`
		DiskLayout              string   `yaml:"diskLayout"`
		EnableTPM               bool     `yaml:"enableTPM"`
		EnableSecureboot        bool     `yaml:"enableSecureboot"`
		EnableHardening         bool     `yaml:"enableHardening"`
		EnableZswap             bool     `yaml:"enableZswap"`
		EnableNix               bool     `yaml:"enableNix"`
		EnableDinit             bool     `yaml:"enableDinit"`
		ExtraPackages           []string `yaml:"extraPackages"`
		SSHAuthorizedKey        string   `yaml:"sshAuthorizedKey"`
		DefaultNetworkInterface string   `yaml:"defaultNetworkInterface"`
	} `yaml:"variables"`
}

type runConfig struct {
	RunDir        string
	SSHPort       int
	MemoryMiB     int
	CPUs          int
	EnableKVM     bool
	QemuBinary    string
	SSHUser       string
	SSHPrivateKey string
	Password      string
	BootTimeout   time.Duration
}

// QEMU instance management

type qemuInstance struct {
	cmd     *exec.Cmd
	logFile *os.File
	waitCh  chan error
	waitMu  sync.Mutex
	waitErr error
	waited  bool
	qmpPath string
}

func qemuStart(ctx context.Context, binary string, args []string, logPath, qmpPath, serialLog string, persistent bool) (*qemuInstance, error) {
	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
		return nil, fmt.Errorf("create log directory: %w", err)
	}

	logFile, err := os.Create(logPath)
	if err != nil {
		return nil, fmt.Errorf("create log file: %w", err)
	}

	var cmd *exec.Cmd
	if persistent {
		cmd = exec.Command(binary, args...)
	} else {
		cmd = exec.CommandContext(ctx, binary, args...)
	}
	cmd.Stdout = logFile
	cmd.Stderr = logFile

	if err := cmd.Start(); err != nil {
		logFile.Close()
		return nil, fmt.Errorf("start qemu: %w", err)
	}

	waitCh := make(chan error, 1)
	inst := &qemuInstance{
		cmd:     cmd,
		logFile: logFile,
		waitCh:  waitCh,
		qmpPath: qmpPath,
	}
	go func() {
		err := cmd.Wait()
		inst.waitMu.Lock()
		inst.waitErr = err
		inst.waited = true
		inst.waitMu.Unlock()
		waitCh <- err
		close(waitCh)
		_ = logFile.Close()
	}()

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		select {
		case err := <-waitCh:
			return nil, fmt.Errorf("qemu exited: %w", err)
		default:
		}
		if qmpPath == "" {
			break
		}
		if _, err := os.Stat(qmpPath); err == nil {
			return inst, nil
		}
		time.Sleep(250 * time.Millisecond)
	}

	select {
	case err := <-waitCh:
		return nil, fmt.Errorf("qemu exited: %w", err)
	default:
	}

	return inst, nil
}

func (inst *qemuInstance) kill() error {
	if inst == nil || inst.cmd == nil || inst.cmd.Process == nil {
		return nil
	}
	err := inst.cmd.Process.Kill()
	if err != nil && !errors.Is(err, os.ErrProcessDone) {
		return err
	}
	inst.wait()
	return nil
}

func (inst *qemuInstance) wait() error {
	if inst == nil || inst.cmd == nil || inst.waitCh == nil {
		if inst == nil {
			return nil
		}
		inst.waitMu.Lock()
		defer inst.waitMu.Unlock()
		return inst.waitErr
	}
	err := <-inst.waitCh
	inst.waitCh = nil
	return err
}

// QMP commands

func qmpExecute(socketPath string, request map[string]any, target any) error {
	conn, err := net.DialTimeout("unix", socketPath, 5*time.Second)
	if err != nil {
		return fmt.Errorf("dial qmp: %w", err)
	}
	defer conn.Close()

	if err := conn.SetDeadline(time.Now().Add(30 * time.Second)); err != nil {
		return fmt.Errorf("set qmp deadline: %w", err)
	}

	reader := bufio.NewReader(conn)
	decoder := json.NewDecoder(reader)
	encoder := json.NewEncoder(conn)

	var greeting map[string]any
	if err := decoder.Decode(&greeting); err != nil {
		return fmt.Errorf("read qmp greeting: %w", err)
	}

	if err := encoder.Encode(map[string]any{"execute": "qmp_capabilities"}); err != nil {
		return fmt.Errorf("send qmp capabilities: %w", err)
	}

	if err := qmpReadResponse(decoder, nil); err != nil {
		return err
	}

	if err := encoder.Encode(request); err != nil {
		return fmt.Errorf("send qmp request: %w", err)
	}

	return qmpReadResponse(decoder, target)
}

func qmpReadResponse(decoder *json.Decoder, target any) error {
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
		if target == nil {
			return nil
		}
		blob, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("marshal qmp response: %w", err)
		}
		return json.Unmarshal(blob, target)
	}
}

func qmpSystemPowerdown(socketPath string) error {
	return qmpExecute(socketPath, map[string]any{"execute": "system_powerdown"}, nil)
}

func qmpQuit(socketPath string) error {
	return qmpExecute(socketPath, map[string]any{"execute": "quit"}, nil)
}

// OVMF detection

type ovmfPaths struct {
	Code string
	Vars string
}

func findOVMF() (ovmfPaths, error) {
	candidates := []ovmfPaths{
		{Code: "/usr/share/OVMF/OVMF_CODE.fd", Vars: "/usr/share/OVMF/OVMF_VARS.fd"},
		{Code: "/usr/share/OVMF/OVMF_CODE_4M.fd", Vars: "/usr/share/OVMF/OVMF_VARS_4M.fd"},
		{Code: "/usr/share/ovmf/OVMF.fd", Vars: "/usr/share/OVMF/OVMF_VARS_4M.fd"},
		{Code: "/usr/share/edk2/ovmf/OVMF_CODE.fd", Vars: "/usr/share/edk2/ovmf/OVMF_VARS.fd"},
	}

	for _, c := range candidates {
		if _, err := os.Stat(c.Code); err != nil {
			continue
		}
		if _, err := os.Stat(c.Vars); err != nil {
			continue
		}
		return c, nil
	}

	return ovmfPaths{}, errors.New("could not locate OVMF firmware files")
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("read %s: %w", src, err)
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return fmt.Errorf("create destination directory: %w", err)
	}
	return os.WriteFile(dst, data, 0o644)
}

// swtpm management

type swtpmConfig struct {
	socketPath   string
	controlPath  string
	stateDir     string
	pidFile      string
	logPath      string
	startTimeout time.Duration
}

func startSwtpm(cfg swtpmConfig) error {
	if cfg.startTimeout <= 0 {
		cfg.startTimeout = 10 * time.Second
	}

	for _, path := range []string{cfg.socketPath, cfg.controlPath, cfg.pidFile} {
		_ = os.Remove(path)
	}
	if err := os.MkdirAll(cfg.stateDir, 0o755); err != nil {
		return fmt.Errorf("create tpm state directory: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(cfg.logPath), 0o755); err != nil {
		return fmt.Errorf("create tpm log directory: %w", err)
	}

	args := []string{
		"socket",
		"--tpm2",
		"--daemon",
		"--tpmstate", "dir=" + cfg.stateDir,
		"--ctrl", "type=unixio,path=" + cfg.controlPath,
		"--pid", "file=" + cfg.pidFile,
		"--log", "level=5,file=" + cfg.logPath,
		"--flags", "not-need-init,startup-clear",
	}
	cmd := exec.Command("swtpm", args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("start swtpm: %w: %s", err, string(output))
	}

	deadline := time.Now().Add(cfg.startTimeout)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(cfg.controlPath); err == nil {
			if _, err := os.Stat(cfg.pidFile); err == nil {
				return nil
			}
		}
		time.Sleep(100 * time.Millisecond)
	}

	return fmt.Errorf("swtpm did not become ready within %s", cfg.startTimeout)
}

func stopSwtpm(pidFile string) error {
	if strings.TrimSpace(pidFile) == "" {
		return nil
	}
	blob, err := os.ReadFile(pidFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read swtpm pid file: %w", err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(blob)))
	if err != nil {
		return fmt.Errorf("parse swtpm pid: %w", err)
	}
	if err := syscall.Kill(pid, syscall.SIGTERM); err != nil && !errors.Is(err, syscall.ESRCH) {
		return fmt.Errorf("signal swtpm: %w", err)
	}
	_ = os.Remove(pidFile)
	return nil
}

// SSH operations

type sshOptions struct {
	user       string
	host       string
	port       int
	privateKey string
	shell      string
}

func sshArgs(opts sshOptions) []string {
	host := opts.host
	if host == "" {
		host = "127.0.0.1"
	}
	return []string{
		"-i", opts.privateKey,
		"-p", fmt.Sprintf("%d", opts.port),
		"-o", "BatchMode=yes",
		"-o", "IdentitiesOnly=yes",
		"-o", "IdentityAgent=none",
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "ConnectTimeout=5",
		"-o", "ServerAliveInterval=5",
		"-o", "ServerAliveCountMax=3",
		fmt.Sprintf("%s@%s", opts.user, host),
	}
}

func sshRun(ctx context.Context, opts sshOptions, command string) error {
	args := sshArgs(opts)
	args = append(args, command)

	cmd := exec.CommandContext(ctx, "ssh", args...)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ssh command failed: %w", err)
	}
	return nil
}

// Serial writes

func writeSerial(socketPath, text string) error {
	conn, err := net.DialTimeout("unix", socketPath, 5*time.Second)
	if err != nil {
		return fmt.Errorf("dial serial socket: %w", err)
	}
	defer conn.Close()
	time.Sleep(200 * time.Millisecond)
	if err := conn.SetWriteDeadline(time.Now().Add(5 * time.Second)); err != nil {
		return fmt.Errorf("set serial write deadline: %w", err)
	}
	if _, err := conn.Write([]byte(text)); err != nil {
		return fmt.Errorf("write serial data: %w", err)
	}
	time.Sleep(200 * time.Millisecond)
	if _, err := conn.Write([]byte("\r")); err != nil {
		return fmt.Errorf("write serial Enter: %w", err)
	}
	time.Sleep(500 * time.Millisecond)
	return nil
}

func getSerialLogSize(path string) int64 {
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return info.Size()
}

// serialLogHasPrompt checks if the serial log contains a CURRENT LUKS prompt
// after minOffset. Old prompts from previous boots are ignored.
func serialLogHasPrompt(path, needle string, minOffset int64) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	if int64(len(data)) <= minOffset {
		return false
	}
	// Only check new data after minOffset, capped to last 4KB
	start := int(minOffset)
	if len(data)-start > 4096 {
		start = len(data) - 4096
	}
	return strings.Contains(string(data[start:]), needle)
}

func waitForSSH(ctx context.Context, opts sshOptions, timeout time.Duration, serialLogPath, serialSocketPath, password string) error {
	deadline := time.Now().Add(timeout)
	var lastErr error
	var unlockAttempts int
	serialStartOffset := getSerialLogSize(serialLogPath)
	for time.Now().Before(deadline) {
		attemptCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		err := sshRun(attemptCtx, opts, "true")
		cancel()
		if err == nil {
			return nil
		}
		lastErr = err

		if password != "" {
			luksPrompt := serialLogHasPrompt(serialLogPath, "Enter passphrase for luks-", serialStartOffset)
			// Inject only once per boot — if cat succeeds here, LUKS will unlock.
			if luksPrompt && unlockAttempts < 1 {
				fmt.Fprintf(os.Stderr, "debug-run: LUKS prompt detected (attempt=%d), injecting passphrase\n", unlockAttempts+1)
				if err := writeSerial(serialSocketPath, password); err != nil {
					fmt.Fprintf(os.Stderr, "debug-run: serial write failed: %v\n", err)
				}
				unlockAttempts++
			}
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}

	if lastErr != nil {
		return fmt.Errorf("ssh did not become ready within %s: %w", timeout, lastErr)
	}
	return fmt.Errorf("ssh did not become ready within %s", timeout)
}

// SSH key generation

type keyPair struct {
	privateKeyPath string
	publicKeyPath  string
}

func ensureKeyPair(dir string) (keyPair, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return keyPair{}, fmt.Errorf("create ssh key directory: %w", err)
	}

	privateKeyPath := filepath.Join(dir, "id_ed25519")
	publicKeyPath := privateKeyPath + ".pub"

	if _, err := os.Stat(privateKeyPath); os.IsNotExist(err) {
		cmd := exec.Command("ssh-keygen", "-q", "-t", "ed25519", "-N", "", "-f", privateKeyPath)
		if output, err := cmd.CombinedOutput(); err != nil {
			return keyPair{}, fmt.Errorf("generate ssh key pair: %w: %s", err, string(output))
		}
	}

	return keyPair{
		privateKeyPath: privateKeyPath,
		publicKeyPath:  publicKeyPath,
	}, nil
}

// QEMU argument builder

type targetArgsInput struct {
	Name           string
	MemoryMiB      int
	CPUs           int
	SerialLog      string
	SerialSocket   string
	QMPPath        string
	FirmwareLog    string
	OVMFCode       string
	OVMFVars       string
	SSHPort        int
	Disk1Path      string
	DiskLayout     string
	Disk2Path      string
	BtrfsDisk1Path string
	BtrfsDisk2Path string
	EnableTPM      bool
	EnableKVM      bool
	TPMControlPath string
}

func buildTargetArgs(cfg targetArgsInput) []string {
	args := []string{
		"-name", cfg.Name,
		"-machine", "q35,smm=on",
		"-m", fmt.Sprintf("%d", cfg.MemoryMiB),
		"-smp", fmt.Sprintf("%d", cfg.CPUs),
		"-nographic",
		"-chardev", "socket,id=targetserial,path=" + cfg.SerialSocket + ",server=on,wait=off,logfile=" + cfg.SerialLog + ",logappend=on",
		"-serial", "chardev:targetserial",
		"-qmp", "unix:" + cfg.QMPPath + ",server=on,wait=off",
		"-global", "driver=cfi.pflash01,property=secure,value=off",
		"-global", "isa-debugcon.iobase=0x402",
		"-debugcon", "file:" + cfg.FirmwareLog,
		"-drive", "if=pflash,format=raw,readonly=on,file=" + cfg.OVMFCode,
		"-drive", "if=pflash,format=raw,file=" + cfg.OVMFVars,
		"-netdev", fmt.Sprintf("user,id=net0,hostfwd=tcp::%d-:22", cfg.SSHPort),
		"-device", "virtio-net-pci,netdev=net0",
		"-drive", "if=none,id=targetdisk1,format=qcow2,file=" + cfg.Disk1Path,
		"-device", "virtio-blk-pci,drive=targetdisk1,bootindex=1",
	}

	if cfg.DiskLayout == "btrfsStandardServer" && cfg.Disk2Path != "" {
		args = append(args,
			"-drive", "if=none,id=targetdisk2,format=qcow2,file="+cfg.Disk2Path,
			"-device", "virtio-blk-pci,drive=targetdisk2,bootindex=2",
		)
		if cfg.BtrfsDisk1Path != "" {
			args = append(args,
				"-drive", "if=none,id=btrfsdisk1,format=qcow2,file="+cfg.BtrfsDisk1Path,
				"-device", "virtio-blk-pci,drive=btrfsdisk1",
			)
		}
		if cfg.BtrfsDisk2Path != "" {
			args = append(args,
				"-drive", "if=none,id=btrfsdisk2,format=qcow2,file="+cfg.BtrfsDisk2Path,
				"-device", "virtio-blk-pci,drive=btrfsdisk2",
			)
		}
	}

	// Always pass swtpm — binaries need /dev/tpmrm0
	args = append(args,
		"-chardev", "socket,id=chrtpm,path="+cfg.TPMControlPath,
		"-tpmdev", "emulator,id=tpm0,chardev=chrtpm",
		"-device", "tpm-tis,tpmdev=tpm0",
	)

	if cfg.EnableKVM {
		args = append(args, "-enable-kvm")
	}

	return args
}

// Utility functions

func loadVariables(path string) (variablesYAML, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return variablesYAML{}, fmt.Errorf("read %s: %w", path, err)
	}
	var vars variablesYAML
	if err := yaml.Unmarshal(data, &vars); err != nil {
		return variablesYAML{}, fmt.Errorf("parse %s: %w", path, err)
	}
	return vars, nil
}

func findDisk(runDir, filename string) string {
	path := filepath.Join(runDir, filename)
	if _, err := os.Stat(path); err == nil {
		return path
	}
	return ""
}

func latestRunDir(runsDir string) string {
	entries, err := os.ReadDir(runsDir)
	if err != nil {
		return ""
	}
	var names []string
	for _, entry := range entries {
		if entry.IsDir() {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)
	if len(names) == 0 {
		return ""
	}
	return filepath.Join(runsDir, names[len(names)-1])
}

func detectArtifactsRoot() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("get working directory: %w", err)
	}

	current := cwd
	for {
		if _, err := os.Stat(filepath.Join(current, "setup.sh")); err == nil {
			return filepath.Join(current, "tests", "artifacts", "harness"), nil
		}
		if _, err := os.Stat(filepath.Join(current, "tests", "artifacts", "harness")); err == nil {
			return filepath.Join(current, "tests", "artifacts", "harness"), nil
		}
		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}

	if strings.Contains(cwd, "tests/") {
		return filepath.Join(cwd, "..", "..", "tests", "artifacts", "harness"), nil
	}

	return "", fmt.Errorf("could not detect artifacts root from %s", cwd)
}

// Main

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	artifactsRoot, err := detectArtifactsRoot()
	if err != nil {
		return err
	}

	cfg := runConfig{
		SSHPort:     2224,
		MemoryMiB:   4096,
		CPUs:        2,
		EnableKVM:   true,
		QemuBinary:  "qemu-system-x86_64",
		SSHUser:     "root",
		BootTimeout: 5 * time.Minute,
	}

	fs := flag.NewFlagSet("debug-run", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.StringVar(&cfg.RunDir, "run-dir", "", "specific run directory (auto-detect latest if empty)")
	fs.IntVar(&cfg.SSHPort, "ssh-port", cfg.SSHPort, "host port forwarded to guest SSH")
	fs.IntVar(&cfg.MemoryMiB, "memory", cfg.MemoryMiB, "guest memory in MiB")
	fs.IntVar(&cfg.CPUs, "cpus", cfg.CPUs, "guest vCPU count")
	fs.BoolVar(&cfg.EnableKVM, "enable-kvm", cfg.EnableKVM, "enable KVM acceleration")
	fs.StringVar(&cfg.QemuBinary, "qemu-binary", cfg.QemuBinary, "QEMU system binary")
	fs.StringVar(&cfg.Password, "password", "Alpine.1", "LUKS passphrase for encrypted root")
	bootTimeoutSeconds := 300
	fs.IntVar(&bootTimeoutSeconds, "boot-timeout-seconds", bootTimeoutSeconds, "seconds to wait for SSH")
	if err := fs.Parse(args); err != nil {
		return err
	}
	cfg.BootTimeout = time.Duration(bootTimeoutSeconds) * time.Second

	if cfg.RunDir == "" {
		cfg.RunDir = latestRunDir(filepath.Join(artifactsRoot, "runs"))
		if cfg.RunDir == "" {
			return fmt.Errorf("no run directories found in %s/runs", artifactsRoot)
		}
		fmt.Fprintf(os.Stderr, "auto-detected latest run: %s\n", cfg.RunDir)
	}

	if _, err := os.Stat(cfg.RunDir); err != nil {
		return fmt.Errorf("run directory %s: %w", cfg.RunDir, err)
	}

	vars, err := loadVariables(filepath.Join(cfg.RunDir, "variables.yaml"))
	if err != nil {
		return err
	}

	if vars.Variables.UserName != "" {
		cfg.SSHUser = vars.Variables.UserName
	}

	keysDir := filepath.Join(artifactsRoot, "cache", "keys")
	kp, err := ensureKeyPair(keysDir)
	if err != nil {
		return fmt.Errorf("ensure SSH key pair: %w", err)
	}
	cfg.SSHPrivateKey = kp.privateKeyPath

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	disk1Path := findDisk(cfg.RunDir, "install-disk-1.qcow2")
	if disk1Path == "" {
		return fmt.Errorf("no install-disk-1.qcow2 found in %s", cfg.RunDir)
	}

	ovmfPaths, err := findOVMF()
	if err != nil {
		return fmt.Errorf("find OVMF: %w", err)
	}

	ovmfVarsPath := filepath.Join(cfg.RunDir, "OVMF_VARS.fd")
	if _, err := os.Stat(ovmfVarsPath); err != nil {
		if err := copyFile(ovmfPaths.Vars, ovmfVarsPath); err != nil {
			return fmt.Errorf("copy OVMF VARS: %w", err)
		}
	}

	targetSerialLog := filepath.Join(cfg.RunDir, "d-serial.log")
	targetSerialSocketPath := filepath.Join(cfg.RunDir, "d-tser.sock")
	targetQemuLog := filepath.Join(cfg.RunDir, "d-qemu.log")
	targetQMPPath := filepath.Join(cfg.RunDir, "d-qmp.sock")
	targetFirmwareLog := filepath.Join(cfg.RunDir, "d-firmware.log")
	tpmControlPath := filepath.Join(cfg.RunDir, "tpm.sock.ctrl")
	tpmStateDir := filepath.Join(cfg.RunDir, "tpm-state")
	tpmPIDPath := filepath.Join(cfg.RunDir, "tpm.pid")
	tpmLogPath := filepath.Join(cfg.RunDir, "tpm.log")

	stopTPM := false
	tpmSocketPath := filepath.Join(cfg.RunDir, "tpm.sock")
	if err := startSwtpm(swtpmConfig{
		socketPath:   tpmSocketPath,
		controlPath:  tpmControlPath,
		stateDir:     tpmStateDir,
		pidFile:      tpmPIDPath,
		logPath:      tpmLogPath,
		startTimeout: 10 * time.Second,
	}); err != nil {
		return fmt.Errorf("start swtpm: %w", err)
	}
	stopTPM = true
	defer func() {
		if stopTPM {
			_ = stopSwtpm(tpmPIDPath)
		}
	}()

	qemuArgs := buildTargetArgs(targetArgsInput{
		Name:           "skea-debug-target",
		MemoryMiB:      cfg.MemoryMiB,
		CPUs:           cfg.CPUs,
		SerialLog:      targetSerialLog,
		SerialSocket:   targetSerialSocketPath,
		QMPPath:        targetQMPPath,
		FirmwareLog:    targetFirmwareLog,
		OVMFCode:       ovmfPaths.Code,
		OVMFVars:       ovmfVarsPath,
		SSHPort:        cfg.SSHPort,
		Disk1Path:      disk1Path,
		DiskLayout:     vars.Variables.DiskLayout,
		Disk2Path:      findDisk(cfg.RunDir, "install-disk-2.qcow2"),
		BtrfsDisk1Path: findDisk(cfg.RunDir, "btrfs-disk-1.qcow2"),
		BtrfsDisk2Path: findDisk(cfg.RunDir, "btrfs-disk-2.qcow2"),
		EnableTPM:      vars.Variables.EnableTPM,
		EnableKVM:      cfg.EnableKVM,
		TPMControlPath: tpmControlPath,
	})

	fmt.Fprintf(os.Stderr, "booting target VM from %s...\n", cfg.RunDir)
	fmt.Fprintf(os.Stderr, "disk: %s\n", disk1Path)
	fmt.Fprintf(os.Stderr, "ssh user: %s\n", cfg.SSHUser)
	fmt.Fprintf(os.Stderr, "ssh port: %d\n", cfg.SSHPort)

	instance, err := qemuStart(ctx, cfg.QemuBinary, qemuArgs, targetQemuLog, targetQMPPath, targetSerialLog, true)
	if err != nil {
		return fmt.Errorf("start QEMU: %w", err)
	}
	defer func() {
		_ = instance.kill()
	}()

	sshOpts := sshOptions{
		user:       cfg.SSHUser,
		port:       cfg.SSHPort,
		privateKey: cfg.SSHPrivateKey,
		shell:      "sh",
	}

	fmt.Fprintf(os.Stderr, "waiting for SSH (timeout: %s)...\n", cfg.BootTimeout)
	if err := waitForSSH(ctx, sshOpts, cfg.BootTimeout, targetSerialLog, targetSerialSocketPath, cfg.Password); err != nil {
		return fmt.Errorf("wait for SSH: %w", err)
	}

	fmt.Println()
	fmt.Println("=== SSH ready ===")
	fmt.Printf("ssh -i %s -p %d %s@127.0.0.1\n", cfg.SSHPrivateKey, cfg.SSHPort, cfg.SSHUser)
	fmt.Println()
	fmt.Println("Press Ctrl+C to shut down the VM.")

	<-ctx.Done()
	fmt.Fprintf(os.Stderr, "\nshutting down...\n")

	// Init-aware shutdown: busybox `poweroff` sends SIGUSR2, which dinit
	// ignores (and `dinitctl shutdown` only HALTs) — dinit needs
	// `dinit-poweroff` from the dinit-shutdown package.
	shutdownCommand := "poweroff"
	if vars.Variables.EnableDinit {
		shutdownCommand = "/usr/sbin/dinit-poweroff"
	}
	if err := sshRun(context.Background(), sshOpts, shutdownCommand); err != nil {
		_ = qmpSystemPowerdown(targetQMPPath)
	}
	_ = instance.wait()

	return nil
}
