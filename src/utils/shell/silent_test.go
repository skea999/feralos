package shell

import (
	"strings"
	"testing"
	"time"
)

func TestExecSilentSuccess(t *testing.T) {
	out, err := ExecSilent("", "echo out")
	if err != nil {
		t.Fatalf("ExecSilent(echo out) = %v, want nil", err)
	}
	if out != "out" {
		t.Errorf("ExecSilent stdout = %q, want %q (trimmed)", out, "out")
	}
}

func TestExecSilentTrimsStderr(t *testing.T) {
	_, err := ExecSilent("", "echo '  boom  ' >&2; exit 1")
	if err == nil {
		t.Fatal("ExecSilent(failing) = nil error, want error")
	}
	if !strings.Contains(err.Error(), "boom") {
		t.Errorf("error %q missing trimmed stderr snippet", err)
	}
	if !strings.Contains(err.Error(), "exit status 1") {
		t.Errorf("error %q missing exit code", err)
	}
}

func TestExecSilentStderrBounded(t *testing.T) {
	big := strings.Repeat("a", 600)
	// Failure with oversized stderr AND stdout: the error carries the
	// trimmed stderr bounded to maxSilentStderr, stdout still returned.
	out, err := ExecSilent("", "printf '%s\\n' '"+big+"' >&2; echo OUT; exit 1")
	if err == nil {
		t.Fatal("ExecSilent(failing) = nil error, want error")
	}
	if out != "OUT" {
		t.Errorf("stdout = %q, want %q (returned alongside the error)", out, "OUT")
	}
	if !strings.Contains(err.Error(), strings.Repeat("a", maxSilentStderr)) {
		t.Errorf("error %q missing bounded stderr", err)
	}
	if strings.Contains(err.Error(), strings.Repeat("a", maxSilentStderr+1)) {
		t.Errorf("error %q exceeds maxSilentStderr bound", err)
	}
}

func TestExecSilentNoStderr(t *testing.T) {
	out, err := ExecSilent("", "exit 7")
	if err == nil {
		t.Fatal("ExecSilent(exit 7) = nil error, want error")
	}
	if out != "" {
		t.Errorf("stdout = %q, want empty", out)
	}
	if err.Error() != "exit status 7" {
		t.Errorf("error = %q, want %q (no stderr, no snippet)", err, "exit status 7")
	}
}

// TestExecSilentTimeout covers P1-2 for the silent path: ExecSilent is bounded
// by the same command timeout and the error mentions the timeout.
func TestExecSilentTimeout(t *testing.T) {
	old := commandTimeout
	commandTimeout = 100 * time.Millisecond
	t.Cleanup(func() { commandTimeout = old })

	out, err := ExecSilent("", "echo out; exec sleep 5")
	if err == nil {
		t.Fatal("ExecSilent(hanging) = nil error, want timeout error")
	}
	if !strings.Contains(err.Error(), "command timed out after 100ms") {
		t.Errorf("error %q missing timeout wrap", err)
	}
	if out != "out" {
		t.Errorf("stdout = %q, want %q (returned alongside the error)", out, "out")
	}
}
