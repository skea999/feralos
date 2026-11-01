package guest

import (
	"context"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// OutputOK runs ssh, so the failure-path unit test uses a guaranteed-closed
// local port: ssh prints "Connection refused" to stderr, which lets us prove
// the stderr-folding contract without an ssh server. The success path needs a
// real guest and stays covered by the harness runs.
func TestOutputOKFoldsStderrIntoError(t *testing.T) {
	ssh, err := exec.LookPath("ssh")
	if err != nil {
		t.Skip("ssh not in PATH")
	}
	_ = ssh

	opts := Options{User: "nobody", Host: "127.0.0.1", Port: 1, PrivateKey: "/dev/null"}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	_, err = OutputOK(ctx, opts, "true")
	if err == nil {
		t.Fatal("expected error for closed port")
	}
	if !strings.Contains(err.Error(), "ssh command failed") {
		t.Fatalf("error missing wrapper prefix: %v", err)
	}
	if !strings.Contains(err.Error(), "Connection refused") {
		t.Fatalf("stderr not folded into error: %v", err)
	}
}

// Output keeps the legacy combined contract: on failure the combined text
// (stderr included) is appended to the error.
func TestOutputLegacyCombinedContract(t *testing.T) {
	if _, err := exec.LookPath("ssh"); err != nil {
		t.Skip("ssh not in PATH")
	}

	opts := Options{User: "nobody", Host: "127.0.0.1", Port: 1, PrivateKey: "/dev/null"}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	_, err := Output(ctx, opts, "true")
	if err == nil {
		t.Fatal("expected error for closed port")
	}
	if !strings.Contains(err.Error(), "Connection refused") {
		t.Fatalf("combined stderr missing from error: %v", err)
	}
}
