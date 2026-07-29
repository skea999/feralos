package qemu

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

type LaunchConfig struct {
	Binary       string
	Args         []string
	LogPath      string
	QMPPath      string
	SerialLog    string
	StartTimeout time.Duration
	Persistent   bool
}

type Instance struct {
	cmd       *exec.Cmd
	logFile   *os.File
	waitCh    chan error
	waitMu    sync.Mutex
	waitErr   error
	waited    bool
	QMPPath   string
	LogPath   string
	SerialLog string
	Args      []string
}

type Status struct {
	Running bool   `json:"running"`
	State   string `json:"state"`
}

type OVMFPaths struct {
	Code string `json:"code"`
	Vars string `json:"vars"`
}

type OVMFSnakeoil struct {
	OVMFPaths
	KeyPath  string `json:"keyPath"`
	CertPath string `json:"certPath"`
}

type SoftwareTPMConfig struct {
	SocketPath  string
	ControlPath string
	StateDir    string
	PIDFile     string
	LogPath     string
	StartTimeout time.Duration
}

func Start(ctx context.Context, cfg LaunchConfig) (*Instance, error) {
	if err := os.MkdirAll(filepath.Dir(cfg.LogPath), 0o755); err != nil {
		return nil, fmt.Errorf("create qemu log directory: %w", err)
	}

	logFile, err := os.Create(cfg.LogPath)
	if err != nil {
		return nil, fmt.Errorf("create qemu log file: %w", err)
	}

	var cmd *exec.Cmd
	if cfg.Persistent {
		cmd = exec.Command(cfg.Binary, cfg.Args...)
	} else {
		cmd = exec.CommandContext(ctx, cfg.Binary, cfg.Args...)
	}
	cmd.Stdout = logFile
	cmd.Stderr = logFile

	if err := cmd.Start(); err != nil {
		logFile.Close()
		return nil, fmt.Errorf("start qemu: %w", err)
	}

	waitCh := make(chan error, 1)
	instance := &Instance{
		cmd:       cmd,
		logFile:   logFile,
		waitCh:    waitCh,
		QMPPath:   cfg.QMPPath,
		LogPath:   cfg.LogPath,
		SerialLog: cfg.SerialLog,
		Args:      append([]string(nil), cfg.Args...),
	}
	go func() {
		err := cmd.Wait()
		instance.waitMu.Lock()
		instance.waitErr = err
		instance.waited = true
		instance.waitMu.Unlock()
		waitCh <- err
		close(waitCh)
		_ = logFile.Close()
	}()

	if cfg.StartTimeout > 0 {
		deadline := time.Now().Add(cfg.StartTimeout)
		for time.Now().Before(deadline) {
			select {
			case err := <-waitCh:
				return nil, startupError(cfg.LogPath, err)
			default:
			}

			if cfg.QMPPath == "" {
				break
			}
			if _, err := os.Stat(cfg.QMPPath); err == nil {
				return instance, nil
			}
			time.Sleep(250 * time.Millisecond)
		}
	}

	select {
	case err := <-waitCh:
		return nil, startupError(cfg.LogPath, err)
	default:
	}

	return instance, nil
}

func (instance *Instance) PID() int {
	if instance == nil || instance.cmd == nil || instance.cmd.Process == nil {
		return 0
	}

	return instance.cmd.Process.Pid
}

func (instance *Instance) Wait() error {
	if instance == nil || instance.cmd == nil || instance.waitCh == nil {
		if instance == nil {
			return nil
		}
		instance.waitMu.Lock()
		defer instance.waitMu.Unlock()
		return instance.waitErr
	}

	err := <-instance.waitCh
	instance.waitCh = nil
	return err
}

func (instance *Instance) Exited() (bool, error) {
	if instance == nil {
		return false, nil
	}

	instance.waitMu.Lock()
	defer instance.waitMu.Unlock()
	return instance.waited, instance.waitErr
}

func (instance *Instance) Kill() error {
	if instance == nil || instance.cmd == nil || instance.cmd.Process == nil {
		return nil
	}

	err := instance.cmd.Process.Kill()
	if err != nil && !errors.Is(err, os.ErrProcessDone) {
		return err
	}

	_ = instance.Wait()
	return nil
}

func CreateDisk(qemuImgBinary, diskPath, size string) error {
	if err := os.MkdirAll(filepath.Dir(diskPath), 0o755); err != nil {
		return fmt.Errorf("create disk directory: %w", err)
	}

	cmd := exec.Command(qemuImgBinary, "create", "-f", "qcow2", diskPath, size)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("create disk %s: %w: %s", diskPath, err, string(output))
	}

	return nil
}

func CreateOverlay(qemuImgBinary, backingFile, overlayPath string) error {
	if err := os.MkdirAll(filepath.Dir(overlayPath), 0o755); err != nil {
		return fmt.Errorf("create overlay directory: %w", err)
	}

	cmd := exec.Command(qemuImgBinary, "create", "-f", "qcow2", "-F", "qcow2", "-b", backingFile, overlayPath)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("create overlay %s: %w: %s", overlayPath, err, string(output))
	}

	return nil
}

func ResizeDisk(qemuImgBinary, diskPath, size string) error {
	cmd := exec.Command(qemuImgBinary, "resize", diskPath, size)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("resize disk %s to %s: %w: %s", diskPath, size, err, string(output))
	}
	return nil
}

func StartSoftwareTPM(cfg SoftwareTPMConfig) error {
	if cfg.StartTimeout <= 0 {
		cfg.StartTimeout = 10 * time.Second
	}
	if cfg.SocketPath == "" || cfg.ControlPath == "" || cfg.StateDir == "" || cfg.PIDFile == "" || cfg.LogPath == "" {
		return fmt.Errorf("software TPM socket, control path, state dir, pid file, and log path are required")
	}
	if _, err := exec.LookPath("swtpm"); err != nil {
		return fmt.Errorf("find swtpm: %w", err)
	}

	for _, path := range []string{cfg.SocketPath, cfg.ControlPath, cfg.PIDFile} {
		_ = os.Remove(path)
	}
	if err := os.MkdirAll(cfg.StateDir, 0o755); err != nil {
		return fmt.Errorf("create TPM state directory: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(cfg.LogPath), 0o755); err != nil {
		return fmt.Errorf("create TPM log directory: %w", err)
	}

	// QEMU 10 uses the CMD_SET_DATAFD protocol: QEMU connects to the --ctrl
	// socket and passes its own data fd via SCM_RIGHTS. Do NOT pass --server:
	// when --server is present, swtpm pre-sets mlp->fd from the server socket,
	// causing CMD_SET_DATAFD to fail with TPM_IOERROR (mlp->fd != -1 guard).
	args := []string{
		"socket",
		"--tpm2",
		"--daemon",
		"--tpmstate", "dir=" + cfg.StateDir,
		"--ctrl", "type=unixio,path=" + cfg.ControlPath,
		"--pid", "file=" + cfg.PIDFile,
		"--log", "level=5,file=" + cfg.LogPath,
		"--flags", "not-need-init,startup-clear",
	}
	cmd := exec.Command("swtpm", args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("start swtpm: %w: %s", err, string(output))
	}

	deadline := time.Now().Add(cfg.StartTimeout)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(cfg.ControlPath); err == nil {
			if _, err := os.Stat(cfg.PIDFile); err == nil {
				return nil
			}
		}
		time.Sleep(100 * time.Millisecond)
	}

	return fmt.Errorf("swtpm did not become ready within %s", cfg.StartTimeout)
}

func StopSoftwareTPM(pidFile string) error {
	if strings.TrimSpace(pidFile) == "" {
		return nil
	}
	blob, err := os.ReadFile(pidFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read swtpm pid file %s: %w", pidFile, err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(blob)))
	if err != nil {
		return fmt.Errorf("parse swtpm pid file %s: %w", pidFile, err)
	}
	if err := syscall.Kill(pid, syscall.SIGTERM); err != nil && !errors.Is(err, syscall.ESRCH) {
		return fmt.Errorf("signal swtpm pid %d: %w", pid, err)
	}
	_ = os.Remove(pidFile)
	return nil
}

func FindOVMF() (OVMFPaths, error) {
	candidates := []OVMFPaths{
		{Code: "/usr/share/OVMF/OVMF_CODE.fd", Vars: "/usr/share/OVMF/OVMF_VARS.fd"},
		{Code: "/usr/share/OVMF/OVMF_CODE_4M.fd", Vars: "/usr/share/OVMF/OVMF_VARS_4M.fd"},
		{Code: "/usr/share/ovmf/OVMF.fd", Vars: "/usr/share/OVMF/OVMF_VARS_4M.fd"},
		{Code: "/usr/share/edk2/ovmf/OVMF_CODE.fd", Vars: "/usr/share/edk2/ovmf/OVMF_VARS.fd"},
	}

	for _, candidate := range candidates {
		if _, err := os.Stat(candidate.Code); err != nil {
			continue
		}
		if _, err := os.Stat(candidate.Vars); err != nil {
			continue
		}
		return candidate, nil
	}

	return OVMFPaths{}, errors.New("could not locate OVMF firmware files")
}

func FindOVMFSnakeoil() (OVMFSnakeoil, error) {
	candidates := []OVMFSnakeoil{
		{
			OVMFPaths: OVMFPaths{Code: "/usr/share/OVMF/OVMF_CODE_4M.snakeoil.fd", Vars: "/usr/share/OVMF/OVMF_VARS_4M.snakeoil.fd"},
			KeyPath:   "/usr/share/ovmf/PkKek-1-snakeoil.key",
			CertPath:  "/usr/share/ovmf/PkKek-1-snakeoil.pem",
		},
	}

	for _, candidate := range candidates {
		if _, err := os.Stat(candidate.Code); err != nil {
			continue
		}
		if _, err := os.Stat(candidate.Vars); err != nil {
			continue
		}
		if _, err := os.Stat(candidate.KeyPath); err != nil {
			continue
		}
		if _, err := os.Stat(candidate.CertPath); err != nil {
			continue
		}
		return candidate, nil
	}

	return OVMFSnakeoil{}, errors.New("could not locate OVMF snakeoil Secure Boot assets")
}

func CopyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("read %s: %w", src, err)
	}

	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return fmt.Errorf("create destination directory: %w", err)
	}

	if err := os.WriteFile(dst, data, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", dst, err)
	}

	return nil
}

func QueryStatus(socketPath string) (Status, error) {
	var response struct {
		Return struct {
			Running bool   `json:"running"`
			Status  string `json:"status"`
		} `json:"return"`
	}

	if err := executeQMP(socketPath, map[string]any{"execute": "query-status"}, &response); err != nil {
		return Status{}, err
	}

	return Status{Running: response.Return.Running, State: response.Return.Status}, nil
}

func SystemPowerdown(socketPath string) error {
	return executeQMP(socketPath, map[string]any{"execute": "system_powerdown"}, nil)
}

func Quit(socketPath string) error {
	return executeQMP(socketPath, map[string]any{"execute": "quit"}, nil)
}

func SendKeys(socketPath string, keys []string) error {
	for _, key := range keys {
		if err := humanMonitorCommand(socketPath, "sendkey "+key); err != nil {
			return err
		}
		time.Sleep(100 * time.Millisecond)
	}

	return nil
}

func TypeText(socketPath, text string) error {
	keys := make([]string, 0, len(text))
	for _, character := range text {
		key, err := qemuKeyForRune(character)
		if err != nil {
			return err
		}
		keys = append(keys, key)
	}

	return SendKeys(socketPath, keys)
}

func PressEnter(socketPath string) error {
	return SendKeys(socketPath, []string{"ret"})
}

func WriteSerial(socketPath, text string) error {
	conn, err := net.DialTimeout("unix", socketPath, 5*time.Second)
	if err != nil {
		return fmt.Errorf("dial serial socket: %w", err)
	}
	defer conn.Close()

	if err := conn.SetWriteDeadline(time.Now().Add(5 * time.Second)); err != nil {
		return fmt.Errorf("set serial write deadline: %w", err)
	}

	if _, err := conn.Write([]byte(text)); err != nil {
		return fmt.Errorf("write serial data: %w", err)
	}

	return nil
}

func WriteSerialChunks(socketPath string, pause time.Duration, chunks ...string) error {
	conn, err := net.DialTimeout("unix", socketPath, 5*time.Second)
	if err != nil {
		return fmt.Errorf("dial serial socket: %w", err)
	}
	defer conn.Close()

	// Let the chardev settle after accepting the connection before sending data.
	time.Sleep(200 * time.Millisecond)

	for index, chunk := range chunks {
		if err := conn.SetWriteDeadline(time.Now().Add(5 * time.Second)); err != nil {
			return fmt.Errorf("set serial write deadline: %w", err)
		}
		if _, err := conn.Write([]byte(chunk)); err != nil {
			return fmt.Errorf("write serial data chunk %d: %w", index, err)
		}
		if pause > 0 && index < len(chunks)-1 {
			time.Sleep(pause)
		}
	}

	// Keep the connection open so QEMU's event loop has time to read the
	// data from the socket buffer and forward it to the serial port input.
	// Without this delay the deferred conn.Close() can fire before QEMU
	// drains the socket, silently losing the keystrokes.
	time.Sleep(500 * time.Millisecond)

	return nil
}

func humanMonitorCommand(socketPath, command string) error {
	request := map[string]any{
		"execute": "human-monitor-command",
		"arguments": map[string]any{
			"command-line": command,
		},
	}

	return executeQMP(socketPath, request, nil)
}

func qemuKeyForRune(character rune) (string, error) {
	switch {
	case character >= 'a' && character <= 'z':
		return string(character), nil
	case character >= 'A' && character <= 'Z':
		return "shift-" + strings.ToLower(string(character)), nil
	case character >= '0' && character <= '9':
		return string(character), nil
	}

	specialKeys := map[rune]string{
		'.': "dot",
		'-': "minus",
		'_': "shift-minus",
		'/': "slash",
		' ': "spc",
		':': "shift-semicolon",
		';': "semicolon",
		'!': "shift-1",
		'@': "shift-2",
		'#': "shift-3",
		'$': "shift-4",
		'%': "shift-5",
		'^': "shift-6",
		'&': "shift-7",
		'*': "shift-8",
		'(': "shift-9",
		')': "shift-0",
	}

	key, ok := specialKeys[character]
	if !ok {
		return "", fmt.Errorf("unsupported key %q for qemu sendkey", character)
	}

	return key, nil
}

func executeQMP(socketPath string, request map[string]any, target any) error {
	conn, err := net.DialTimeout("unix", socketPath, 5*time.Second)
	if err != nil {
		return fmt.Errorf("dial qmp socket: %w", err)
	}
	defer conn.Close()

	// Hard deadline: QEMU may be alive (socket accepts) but hang before starting
	// its event loop (e.g. during OVMF TPM init). Without this, Decode blocks forever.
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

	if err := readQMPResponse(decoder, nil); err != nil {
		return err
	}

	if err := encoder.Encode(request); err != nil {
		return fmt.Errorf("send qmp request: %w", err)
	}

	return readQMPResponse(decoder, target)
}

func readQMPResponse(decoder *json.Decoder, target any) error {
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

		if err := json.Unmarshal(blob, target); err != nil {
			return fmt.Errorf("unmarshal qmp response: %w", err)
		}

		return nil
	}
}

func startupError(logPath string, err error) error {
	if err == nil {
		err = errors.New("qemu exited before startup completed")
	}

	logData, readErr := os.ReadFile(logPath)
	if readErr != nil || len(logData) == 0 {
		return fmt.Errorf("qemu startup failed: %w", err)
	}

	return fmt.Errorf("qemu startup failed: %w: %s", err, string(logData))
}
