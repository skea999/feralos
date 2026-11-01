package shell

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Environment knobs driving the failure-handling branches (constants to keep
// goconst quiet and the tests readable).
const (
	envIgnoreErrors           = "SKEA_IGNORE_ERRORS"
	envNonInteractive         = "SKEA_NONINTERACTIVE"
	envNonInteractiveDecision = "SKEA_NONINTERACTIVE_DECISION"

	// promptText is the interactive ask-loop prompt (asserted to prove the
	// branch was actually reached).
	promptText = "Options: [s]top, [i]gnore, [r]etry"
)

// captureOutput swaps os.Stdout for a pipe and returns a snapshot function.
// The code under test writes its logs via fmt.Printf (dynamic os.Stdout
// lookup), while child-process stdout/stderr go to internal bytes.Buffer —
// so only library logging lands in the pipe.
func captureOutput(t *testing.T) func() string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	old := os.Stdout
	os.Stdout = w
	t.Cleanup(func() {
		os.Stdout = old
		_ = w.Close()
		_ = r.Close()
	})
	return func() string {
		_ = w.Close()
		out, err := io.ReadAll(r)
		if err != nil {
			t.Fatalf("read captured output: %v", err)
		}
		return string(out)
	}
}

// setTestStdin replaces the package-global stdinReader with a strings.Reader
// and returns the underlying reader (for "nothing consumed" assertions).
func setTestStdin(t *testing.T, input string) *strings.Reader {
	t.Helper()
	old := stdinReader
	inputReader := strings.NewReader(input)
	stdinReader = bufio.NewReader(inputReader)
	t.Cleanup(func() { stdinReader = old })
	return inputReader
}

// retryCounter returns a shell command that appends one line per attempt and
// exits 3, plus a function counting the recorded attempts (t.TempDir-backed).
func retryCounter(t *testing.T) (cmd string, attempts func() int) {
	t.Helper()
	file := filepath.Join(t.TempDir(), "attempts")
	cmd = fmt.Sprintf("printf 'x\\n' >> %q; exit 3", file)
	return cmd, func() int {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("read attempts file: %v", err)
		}
		return strings.Count(string(data), "x")
	}
}

// timeoutCounter is retryCounter's hanging twin: appends one line per attempt
// then blocks in `exec sleep 5` (the exec makes sleep the process itself, so
// the timeout kill reaps it immediately instead of leaving a grandchild
// holding the stdout pipe).
func timeoutCounter(t *testing.T) (cmd string, attempts func() int) {
	t.Helper()
	file := filepath.Join(t.TempDir(), "attempts")
	cmd = fmt.Sprintf("printf 'x\\n' >> %q; exec sleep 5", file)
	return cmd, func() int {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("read attempts file: %v", err)
		}
		return strings.Count(string(data), "x")
	}
}

// setShortTimeout overrides the package command timeout (P1-2 hook) for the
// test duration.
func setShortTimeout(t *testing.T) {
	t.Helper()
	old := commandTimeout
	commandTimeout = 100 * time.Millisecond
	t.Cleanup(func() { commandTimeout = old })
}

func TestExecuteCmdSuccess(t *testing.T) {
	snapshot := captureOutput(t)
	out, err := ExecuteCmd("", "true")
	if err != nil {
		t.Fatalf("ExecuteCmd(true) = %v, want nil", err)
	}
	if out != "" {
		t.Errorf("ExecuteCmd(true) stdout = %q, want empty", out)
	}
	out, err = ExecuteCmd("", "echo hello")
	if err != nil {
		t.Fatalf("ExecuteCmd(echo hello) = %v, want nil", err)
	}
	if out != "hello\n" {
		t.Errorf("ExecuteCmd(echo hello) stdout = %q, want %q", out, "hello\n")
	}
	if got := snapshot(); !strings.Contains(got, "CMD: echo hello") {
		t.Errorf("log output missing command line, got: %s", got)
	}
}

func TestExecuteCmdRetryThenError(t *testing.T) {
	t.Setenv(envIgnoreErrors, "")
	t.Setenv(envNonInteractive, "")
	// No env short-circuit applies: after MaxRetries failed attempts the
	// code reaches the interactive ask loop — answer [i]gnore like a user.
	stdin := setTestStdin(t, "i\n")
	snapshot := captureOutput(t)
	cmd, attempts := retryCounter(t)

	out, err := ExecuteCmd("", cmd)
	if err != nil {
		t.Fatalf("interactive ignore after retries = %v, want nil", err)
	}
	if out != "" {
		t.Errorf("stdout = %q, want empty", out)
	}
	if n := attempts(); n != MaxRetries {
		t.Errorf("attempts = %d, want %d (MaxRetries honored)", n, MaxRetries)
	}
	log := snapshot()
	if !strings.Contains(log, "[RETRY]") {
		t.Errorf("log output missing retry notice, got: %s", log)
	}
	if !strings.Contains(log, promptText) {
		t.Errorf("log output missing interactive prompt, got: %s", log)
	}
	if stdin.Len() != 0 {
		t.Error("stdin not consumed — the interactive answer was not read")
	}
}

func TestExecuteCmdArgsStdout(t *testing.T) {
	snapshot := captureOutput(t)
	out, err := ExecuteCmdArgs("", "/bin/echo", "hello")
	if err != nil {
		t.Fatalf("ExecuteCmdArgs(echo hello) = %v, want nil", err)
	}
	if out != "hello\n" {
		t.Errorf("ExecuteCmdArgs stdout = %q, want %q", out, "hello\n")
	}
	if got := snapshot(); !strings.Contains(got, "[stdout] hello") {
		t.Errorf("log output missing captured stdout, got: %s", got)
	}
}

func TestExecuteCmdIgnoreErrors(t *testing.T) {
	t.Setenv(envIgnoreErrors, "1")
	t.Setenv(envNonInteractive, "")
	snapshot := captureOutput(t)
	cmd, attempts := retryCounter(t)

	out, err := ExecuteCmd("", cmd)
	if err != nil {
		t.Fatalf("ExecuteCmd with ignoreErrors = %v, want nil (ignore path)", err)
	}
	if out != "" {
		t.Errorf("stdout = %q, want empty", out)
	}
	if n := attempts(); n != MaxRetries {
		t.Errorf("attempts = %d, want %d (retries still happen before ignore)", n, MaxRetries)
	}
	if got := snapshot(); !strings.Contains(got, "[IGNORED]") {
		t.Errorf("log output missing ignore marker, got: %s", got)
	}
}

func TestExecuteCmdNonInteractiveAbort(t *testing.T) {
	t.Setenv(envIgnoreErrors, "")
	t.Setenv(envNonInteractive, "1")
	t.Setenv(envNonInteractiveDecision, "")
	stdin := setTestStdin(t, "SHOULD-NOT-BE-READ\n")
	snapshot := captureOutput(t)
	cmd, attempts := retryCounter(t)

	_, err := ExecuteCmd("", cmd)
	if err == nil {
		t.Fatal("ExecuteCmd in non-interactive mode (default decision) = nil, want error")
	}
	if !strings.Contains(err.Error(), fmt.Sprintf("after %d attempts", MaxRetries)) {
		t.Errorf("error %q missing attempt count", err)
	}
	if n := attempts(); n != MaxRetries {
		t.Errorf("attempts = %d, want %d", n, MaxRetries)
	}
	if remaining := stdin.Len(); remaining == 0 {
		t.Error("stdin was consumed — non-interactive path must not read stdin")
	}
	if got := snapshot(); strings.Contains(got, promptText) {
		t.Error("interactive prompt shown in non-interactive mode")
	}
}

func TestExecuteCmdNonInteractiveIgnoreDecision(t *testing.T) {
	t.Setenv(envIgnoreErrors, "")
	t.Setenv(envNonInteractive, "1")
	t.Setenv(envNonInteractiveDecision, "ignore")
	captureOutput(t)
	cmd, attempts := retryCounter(t)

	if _, err := ExecuteCmd("", cmd); err != nil {
		t.Fatalf("ExecuteCmd with decision=ignore = %v, want nil", err)
	}
	if n := attempts(); n != MaxRetries {
		t.Errorf("attempts = %d, want %d", n, MaxRetries)
	}
}

func TestExecuteCmdAllowFail(t *testing.T) {
	t.Setenv(envIgnoreErrors, "")
	t.Setenv(envNonInteractive, "")
	captureOutput(t)

	// allowFail returns on the FIRST failure — no retries, nil error.
	out, err := ExecuteCmdWithAllowFail("", "echo out; exit 5", true, false)
	if err != nil {
		t.Fatalf("ExecuteCmdWithAllowFail(allowFail) = %v, want nil", err)
	}
	if out != "out\n" {
		t.Errorf("stdout = %q, want %q", out, "out\n")
	}

	out, err = ExecuteCmdArgsWithAllowFail("", "/bin/sh", []string{"-c", "echo out; exit 5"}, true, false)
	if err != nil {
		t.Fatalf("ExecuteCmdArgsWithAllowFail(allowFail) = %v, want nil", err)
	}
	if out != "out\n" {
		t.Errorf("stdout = %q, want %q", out, "out\n")
	}
}

func TestInteractiveInvalidThenIgnore(t *testing.T) {
	t.Setenv(envIgnoreErrors, "")
	t.Setenv(envNonInteractive, "")
	setTestStdin(t, "junk\ni\n")
	captureOutput(t)
	cmd, _ := retryCounter(t)

	if _, err := ExecuteCmd("", cmd); err != nil {
		t.Fatalf("interactive invalid-then-ignore = %v, want nil", err)
	}
}

// TestChrootBranchAllowFail covers the chroot exec branch WITHOUT running
// chroot(2) for real: a nonexistent chroot dir fails instantly (start error,
// deterministic as root and non-root alike) and allowFail swallows it on the
// first attempt. Executing a real chroot inside unit tests is out of scope.
func TestChrootBranchAllowFail(t *testing.T) {
	t.Setenv(envIgnoreErrors, "")
	t.Setenv(envNonInteractive, "")
	captureOutput(t)

	if _, err := ExecuteCmdWithAllowFail("/nonexistent-feralos-test-chroot", "true", true, false); err != nil {
		t.Fatalf("chroot branch with allowFail = %v, want nil", err)
	}
	if _, err := ExecuteCmdArgsWithAllowFail("/nonexistent-feralos-test-chroot", "/bin/true", nil, true, false); err != nil {
		t.Fatalf("chroot args branch with allowFail = %v, want nil", err)
	}
}

func TestSetCaller(t *testing.T) {
	snapshot := captureOutput(t)
	SetCaller("testpkg")
	defer SetCaller("")
	if GetCaller() != "testpkg" {
		t.Errorf("GetCaller() = %q, want %q", GetCaller(), "testpkg")
	}
	if _, err := ExecuteCmd("", "true"); err != nil {
		t.Fatalf("ExecuteCmd(true) = %v, want nil", err)
	}
	if got := snapshot(); !strings.Contains(got, "[testpkg]") {
		t.Errorf("log output missing caller prefix, got: %s", got)
	}
}

// TestExecuteCmdStopAbortsViaHook covers the P2-1 exit hook: answering 's'
// in the ask loop reaches the single abort point and calls osExit(1) —
// captured via the overridable hook instead of killing the test process.
func TestExecuteCmdStopAbortsViaHook(t *testing.T) {
	t.Setenv(envIgnoreErrors, "")
	t.Setenv(envNonInteractive, "")
	setTestStdin(t, "s\n")
	snapshot := captureOutput(t)
	cmd, _ := retryCounter(t)

	oldExit := osExit
	exitCode := -1
	aborted := false
	osExit = func(code int) {
		exitCode = code
		aborted = true
		panic("osExit-under-test")
	}
	t.Cleanup(func() { osExit = oldExit })

	func() {
		defer func() {
			if r := recover(); r == nil {
				t.Error("osExit hook did not abort the flow (no panic propagated)")
			}
		}()
		_, _ = ExecuteCmd("", cmd)
	}()

	if !aborted {
		t.Fatal("osExit hook never called — 'stop' did not reach the abort point")
	}
	if exitCode != 1 {
		t.Errorf("osExit code = %d, want 1", exitCode)
	}
	if got := snapshot(); !strings.Contains(got, "[CRITICAL] Script aborted by user.") {
		t.Errorf("log output missing abort notice, got: %s", got)
	}
}

// TestExecuteCmdTimeoutRetryThenError covers P1-2: a hanging command is
// killed at the command timeout, the wrapped error flows through the standard
// retry path (3 attempts), and the final non-interactive abort error mentions
// the timeout.
func TestExecuteCmdTimeoutRetryThenError(t *testing.T) {
	t.Setenv(envIgnoreErrors, "")
	t.Setenv(envNonInteractive, "1")
	t.Setenv(envNonInteractiveDecision, "")
	setShortTimeout(t)
	snapshot := captureOutput(t)
	cmd, attempts := timeoutCounter(t)

	_, err := ExecuteCmd("", cmd)
	if err == nil {
		t.Fatal("ExecuteCmd(hanging, non-interactive abort) = nil, want error")
	}
	if !strings.Contains(err.Error(), "command timed out after 100ms") {
		t.Errorf("error %q missing timeout wrap", err)
	}
	if !strings.Contains(err.Error(), fmt.Sprintf("after %d attempts", MaxRetries)) {
		t.Errorf("error %q missing attempt count", err)
	}
	if n := attempts(); n != MaxRetries {
		t.Errorf("attempts = %d, want %d (timeout flows through the retry path)", n, MaxRetries)
	}
	if got := snapshot(); !strings.Contains(got, "[RETRY]") {
		t.Errorf("log output missing retry notice, got: %s", got)
	}
}

// TestExecuteCmdTimeoutAllowFail proves the allow-fail short-circuit fires on
// a timeout-killed attempt (returns nil on the FIRST attempt).
func TestExecuteCmdTimeoutAllowFail(t *testing.T) {
	t.Setenv(envIgnoreErrors, "")
	t.Setenv(envNonInteractive, "")
	setShortTimeout(t)
	captureOutput(t)
	cmd, attempts := timeoutCounter(t)

	out, err := ExecuteCmdWithAllowFail("", cmd, true, false)
	if err != nil {
		t.Fatalf("ExecuteCmdWithAllowFail(timeout) = %v, want nil (allowFail swallows the kill)", err)
	}
	if out != "" {
		t.Errorf("stdout = %q, want empty", out)
	}
	if n := attempts(); n != 1 {
		t.Errorf("attempts = %d, want 1 (allowFail returns on first failure)", n)
	}
}

// TestExecuteCmdTimeoutIgnoreErrors proves ignoreErrors still wins after the
// timeout-driven retries: nil error, 3 attempts, [IGNORED] marker.
func TestExecuteCmdTimeoutIgnoreErrors(t *testing.T) {
	t.Setenv(envIgnoreErrors, "1")
	t.Setenv(envNonInteractive, "")
	setShortTimeout(t)
	snapshot := captureOutput(t)
	cmd, attempts := timeoutCounter(t)

	out, err := ExecuteCmd("", cmd)
	if err != nil {
		t.Fatalf("ExecuteCmd(timeout, ignoreErrors) = %v, want nil (ignore path)", err)
	}
	if out != "" {
		t.Errorf("stdout = %q, want empty", out)
	}
	if n := attempts(); n != MaxRetries {
		t.Errorf("attempts = %d, want %d", n, MaxRetries)
	}
	log := snapshot()
	if !strings.Contains(log, "[IGNORED]") {
		t.Errorf("log output missing ignore marker, got: %s", log)
	}
	if !strings.Contains(log, "command timed out after 100ms") {
		t.Errorf("log output missing timeout detail in ignored error, got: %s", log)
	}
}

// TestExecuteCmdArgsTimeout proves the no-shell (args) flavor uses the same
// context-bounded execution path.
func TestExecuteCmdArgsTimeout(t *testing.T) {
	t.Setenv(envIgnoreErrors, "")
	t.Setenv(envNonInteractive, "1")
	t.Setenv(envNonInteractiveDecision, "")
	setShortTimeout(t)
	captureOutput(t)

	attemptsFile := filepath.Join(t.TempDir(), "attempts")
	_, err := ExecuteCmdArgs("", "/bin/sh", "-c", fmt.Sprintf("printf 'x\\n' >> %q; exec sleep 5", attemptsFile))
	if err == nil {
		t.Fatal("ExecuteCmdArgs(hanging) = nil, want error")
	}
	if !strings.Contains(err.Error(), "command timed out after 100ms") {
		t.Errorf("error %q missing timeout wrap", err)
	}
	data, readErr := os.ReadFile(attemptsFile)
	if readErr != nil {
		t.Fatalf("read attempts file: %v", readErr)
	}
	if n := strings.Count(string(data), "x"); n != MaxRetries {
		t.Errorf("attempts = %d, want %d", n, MaxRetries)
	}
}
