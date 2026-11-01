package guest

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type Options struct {
	User       string
	Host       string
	Port       int
	PrivateKey string
	Shell      string
}

type KeyPair struct {
	PrivateKeyPath string `json:"privateKeyPath"`
	PublicKeyPath  string `json:"publicKeyPath"`
	PublicKey      string `json:"publicKey"`
}

func EnsureKeyPair(dir string) (KeyPair, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return KeyPair{}, fmt.Errorf("create ssh key directory: %w", err)
	}

	privateKeyPath := filepath.Join(dir, "id_ed25519")
	publicKeyPath := privateKeyPath + ".pub"

	if _, err := os.Stat(privateKeyPath); os.IsNotExist(err) {
		cmd := exec.Command("ssh-keygen", "-q", "-t", "ed25519", "-N", "", "-f", privateKeyPath)
		if output, err := cmd.CombinedOutput(); err != nil {
			return KeyPair{}, fmt.Errorf("generate ssh key pair: %w: %s", err, string(output))
		}
	}

	publicKey, err := os.ReadFile(publicKeyPath)
	if err != nil {
		return KeyPair{}, fmt.Errorf("read ssh public key: %w", err)
	}

	return KeyPair{
		PrivateKeyPath: privateKeyPath,
		PublicKeyPath:  publicKeyPath,
		PublicKey:      string(bytes.TrimSpace(publicKey)),
	}, nil
}

func WaitForSSH(ctx context.Context, opts Options, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		attemptCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		err := Run(attemptCtx, opts, "true", io.Discard, io.Discard)
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
		return fmt.Errorf("ssh did not become ready within %s: %w", timeout, lastErr)
	}

	return fmt.Errorf("ssh did not become ready within %s", timeout)
}

func Run(ctx context.Context, opts Options, command string, stdout io.Writer, stderr io.Writer) error {
	args := sshArgs(opts)
	args = append(args, fmt.Sprintf("%s@%s", opts.User, hostOrDefault(opts.Host)), remoteCommand(opts, command))

	cmd := exec.CommandContext(ctx, "ssh", args...)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ssh command failed: %w", err)
	}

	return nil
}

// Output runs the command and returns its COMBINED stdout+stderr.
//
// Legacy contract (kept): ~77 existing harness checks judge the returned
// text with strings.Contains, and several rely on stderr fragments (e.g.
// busybox pgrep quiet messages, ssh known-hosts warnings stripped from the
// middle of results). Do NOT change this behavior; new checks should prefer
// OutputOK, which keeps the two streams separate.
func Output(ctx context.Context, opts Options, command string) ([]byte, error) {
	args := sshArgs(opts)
	args = append(args, fmt.Sprintf("%s@%s", opts.User, hostOrDefault(opts.Host)), remoteCommand(opts, command))

	cmd := exec.CommandContext(ctx, "ssh", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("ssh command failed: %w: %s", err, string(output))
	}

	return output, nil
}

// OutputOK runs the same ssh command as Output but captures ONLY stdout.
// Stderr is dropped on success and folded into the error on failure, so
// the returned text is never polluted by diagnostics (ssh known-hosts
// warnings, busybox chatter) and Contains-based assertions stay clean.
//
// Output = combined (legacy contract, kept). OutputOK = clean separation
// for new checks.
func OutputOK(ctx context.Context, opts Options, command string) (string, error) {
	args := sshArgs(opts)
	args = append(args, fmt.Sprintf("%s@%s", opts.User, hostOrDefault(opts.Host)), remoteCommand(opts, command))

	cmd := exec.CommandContext(ctx, "ssh", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			return "", fmt.Errorf("ssh command failed: %w", err)
		}

		return "", fmt.Errorf("ssh command failed: %w: %s", err, msg)
	}

	return stdout.String(), nil
}

func CopyToGuest(ctx context.Context, opts Options, srcPath, dstPath string, recursive bool) error {
	args := scpArgs(opts)
	if recursive {
		args = append(args, "-r")
	}
	args = append(args, srcPath, fmt.Sprintf("%s@%s:%s", opts.User, hostOrDefault(opts.Host), dstPath))

	cmd := exec.CommandContext(ctx, "scp", args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("scp to guest failed: %w: %s", err, string(output))
	}

	return nil
}

func CopyFromGuest(ctx context.Context, opts Options, srcPath, dstPath string, recursive bool) error {
	args := scpArgs(opts)
	if recursive {
		args = append(args, "-r")
	}
	args = append(args, fmt.Sprintf("%s@%s:%s", opts.User, hostOrDefault(opts.Host), srcPath), dstPath)

	cmd := exec.CommandContext(ctx, "scp", args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("scp from guest failed: %w: %s", err, string(output))
	}

	return nil
}

func sshArgs(opts Options) []string {
	return []string{
		"-i", opts.PrivateKey,
		"-p", fmt.Sprintf("%d", opts.Port),
		"-o", "BatchMode=yes",
		"-o", "IdentitiesOnly=yes",
		"-o", "IdentityAgent=none",
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "ConnectTimeout=5",
		"-o", "ServerAliveInterval=5",
		"-o", "ServerAliveCountMax=3",
	}
}

func scpArgs(opts Options) []string {
	return []string{
		"-i", opts.PrivateKey,
		"-P", fmt.Sprintf("%d", opts.Port),
		"-o", "BatchMode=yes",
		"-o", "IdentitiesOnly=yes",
		"-o", "IdentityAgent=none",
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
	}
}

func hostOrDefault(host string) string {
	if host == "" {
		return "127.0.0.1"
	}

	return host
}

func remoteCommand(opts Options, command string) string {
	if opts.Shell == "" {
		return command
	}

	return fmt.Sprintf("%s -lc %s", opts.Shell, singleQuote(command))
}

func singleQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}
