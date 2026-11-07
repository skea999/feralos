package createConfig

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	"feralos/src/utils/config"
)

// newTestScanner builds a scanner over an in-memory input, mirroring the
// PromptForConfig buffer sizing.
func newTestScanner(t *testing.T, input string) *bufio.Scanner {
	t.Helper()
	sc := bufio.NewScanner(strings.NewReader(input))
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	return sc
}

// captureStdout redirects package stdout (fmt.Printf prompts) into a
// buffer while fn runs, so tests neither spam the runner output nor get
// their prompt assertions cut short. The script-side write end is closed
// and the copy goroutine joined BEFORE returning, so the caller sees all
// bytes (a Go return value evaluates before defers — both steps must be
// in the body).
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	var buf bytes.Buffer
	done := make(chan struct{})
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("stdout pipe: %v", err)
	}
	os.Stdout = w
	defer func() { os.Stdout = old }()
	go func() {
		_, _ = io.Copy(&buf, r)
		_ = r.Close()
		close(done)
	}()
	fn()
	_ = w.Close()
	<-done
	return buf.String()
}

func okOnly(s string) error {
	if s == "ok" {
		return nil
	}
	return errors.New("invalid value")
}

func TestAskValid(t *testing.T) {
	cases := []struct {
		name     string
		input    string
		current  string
		options  []string
		validate func(string) error
		want     string
		wantErr  error
	}{
		{name: "valid input", input: "ok\n", current: "ok", validate: okOnly, want: "ok"},
		{name: "invalid then valid", input: "bad\nok\n", current: "", validate: okOnly, want: "ok"},
		{name: "empty keeps default (validate nil)", input: "\n", current: "cur", validate: nil, want: "cur"},
		{name: "empty keeps default (validated)", input: "\n", current: "ok", validate: okOnly, want: "ok"},
		{name: "input trimmed", input: "  ok  \n", current: "", validate: okOnly, want: "ok"},
		{name: "EOF closes chain", input: "", current: "", validate: okOnly, want: "", wantErr: errInputClosed},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var got string
			var err error
			out := captureStdout(t, func() {
				got, err = askValid(newTestScanner(t, tc.input), "Username", tc.current, tc.options, tc.validate)
			})
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("err = %v, want %v", err, tc.wantErr)
			}
			if got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
			if tc.name == "invalid then valid" && strings.Count(out, "Username [") != 2 {
				t.Fatalf("expected 2 prompts after an invalid answer, got %d:\n%s",
					strings.Count(out, "Username ["), out)
			}
		})
	}
}

func TestAskValidShowsOptions(t *testing.T) {
	var got string
	out := captureStdout(t, func() {
		got, _ = askValid(newTestScanner(t, "ok\n"), "Choice", "a", []string{"a", "b"}, okOnly)
	})
	if got != "ok" {
		t.Fatalf("got %q, want %q", got, "ok")
	}
	if want := "Choice (options: a, b) [a]: "; !strings.Contains(out, want) {
		t.Fatalf("prompt missing %q:\n%s", want, out)
	}
}

func TestAskValidRealValidators(t *testing.T) {
	var got string
	captureStdout(t, func() {
		got, _ = askValid(newTestScanner(t, "SKEIN\nskein\n"), "Username", "", nil, config.ValidateUserName)
	})
	if got != "skein" {
		t.Fatalf("got %q, want %q (real validator chain)", got, "skein")
	}
}

func TestAskDisks(t *testing.T) {
	cases := []struct {
		name     string
		input    string
		layout   string
		current  []string
		want     []string
		wantErr  error
		checkOut func(output string) error
	}{
		{
			name:   "valid comma list",
			input:  "/dev/sda,/dev/sdb\n",
			layout: "btrfsStandardServer",
			want:   []string{"/dev/sda", "/dev/sdb"},
		},
		{
			name:   "unclean comma list accepted",
			input:  " /dev/sda ,  /dev/sdb \n",
			layout: "btrfsStandardServer",
			want:   []string{"/dev/sda", "/dev/sdb"},
		},
		{
			name:   "wrong count re-asks",
			input:  "/dev/sda\n/dev/sda,/dev/sdb\n",
			layout: "btrfsStandardServer",
			want:   []string{"/dev/sda", "/dev/sdb"},
			checkOut: func(output string) error {
				if !strings.Contains(output, "btrfsStandardServer requires exactly 2 disk(s)") {
					return errors.New("missing layout count error")
				}
				return nil
			},
		},
		{
			name:   "bad path re-asks",
			input:  "/dev/notadisk\n/dev/sda\n",
			layout: "btrfsStandard",
			want:   []string{"/dev/sda"},
			checkOut: func(output string) error {
				if !strings.Contains(output, "invalid disk path") {
					return errors.New("missing path error")
				}
				return nil
			},
		},
		{
			name:    "EOF",
			input:   "",
			layout:  "btrfsStandard",
			current: nil,
			wantErr: errInputClosed,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var got []string
			var err error
			out := captureStdout(t, func() {
				got, err = askDisks(newTestScanner(t, tc.input), tc.layout, tc.current)
			})
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("err = %v, want %v", err, tc.wantErr)
			}
			if err == nil && strings.Join(got, ",") != strings.Join(tc.want, ",") {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
			if tc.checkOut != nil {
				if cErr := tc.checkOut(out); cErr != nil {
					t.Fatal(cErr)
				}
			}
		})
	}
}

func TestAskBool(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		current bool
		want    bool
		wantErr error
	}{
		{name: "y", input: "y\n", current: false, want: true},
		{name: "yes", input: "yes\n", current: false, want: true},
		{name: "YES", input: "YES\n", current: false, want: true},
		{name: "1", input: "1\n", current: false, want: true},
		{name: "true", input: "true\n", current: false, want: true},
		{name: "n", input: "n\n", current: false, want: false},
		{name: "no", input: "no\n", current: false, want: false},
		{name: "No", input: "No\n", current: false, want: false},
		{name: "0", input: "0\n", current: false, want: false},
		{name: "false", input: "false\n", current: false, want: false},
		{name: "empty keeps current", input: "\n", current: true, want: true},
		{name: "empty keeps current false", input: "\n", current: false, want: false},
		{name: "invalid then valid", input: "junk\ny\n", current: false, want: true},
		{name: "EOF", input: "", current: false, want: false, wantErr: errInputClosed},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var got bool
			var err error
			out := captureStdout(t, func() {
				got, err = askBool(newTestScanner(t, tc.input), "Toggle", tc.current)
			})
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("err = %v, want %v", err, tc.wantErr)
			}
			if err == nil && got != tc.want {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
			if tc.name == "invalid then valid" && strings.Count(out, "Toggle [true/false]") != 2 {
				t.Fatalf("expected 2 prompts, got %d:\n%s", strings.Count(out, "Toggle [true/false]"), out)
			}
		})
	}
}

func TestAskConfirm(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		want    bool
		wantErr error
	}{
		{name: "yes", input: "yes\n", want: true},
		{name: "y", input: "y\n", want: true},
		{name: "no", input: "no\n", want: false},
		{name: "n", input: "n\n", want: false},
		{name: "NO", input: "NO\n", want: false},
		{name: "invalid then no", input: "maybe\nno\n", want: false},
		{name: "EOF", input: "", want: false, wantErr: errInputClosed},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var got bool
			var err error
			out := captureStdout(t, func() {
				got, err = askConfirm(newTestScanner(t, tc.input))
			})
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("err = %v, want %v", err, tc.wantErr)
			}
			if err == nil && got != tc.want {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
			if tc.name == "invalid then no" && strings.Count(out, "Proceed with this configuration?") != 2 {
				t.Fatalf("expected 2 prompts, got %d:\n%s",
					strings.Count(out, "Proceed with this configuration?"), out)
			}
		})
	}
}

func TestAskSecret(t *testing.T) {
	longEnough := func(s string) error {
		if len(s) >= 6 {
			return nil
		}
		return errors.New("too short")
	}
	cases := []struct {
		name       string
		input      string
		allowEmpty bool
		validate   func(string) error
		want       string
		wantErr    error
		checkOut   func(output string) error
	}{
		{name: "match", input: "hunter2\nhunter2\n", validate: longEnough, want: "hunter2"},
		{
			name:     "mismatch asks both again",
			input:    "secret1\nnotmatch\nsecret1\nsecret1\n",
			validate: longEnough,
			want:     "secret1",
			checkOut: func(output string) error {
				for _, marker := range []string{"passwords do not match", "Confirm User Password: "} {
					if !strings.Contains(output, marker) {
						return fmt.Errorf("output missing %q:\n%s", marker, output)
					}
				}
				if strings.Count("\n"+output, "\nUser Password: ") != 2 {
					return fmt.Errorf("expected both re-asked (2 prompts), got:\n%s", output)
				}
				return nil
			},
		},
		{
			name:     "validation error re-asks entry",
			input:    "short\nsecret1\nsecret1\n",
			validate: longEnough,
			want:     "secret1",
			checkOut: func(output string) error {
				if !strings.Contains(output, "too short") {
					return errors.New("missing validation error prompt")
				}
				return nil
			},
		},
		{name: "allowEmpty skips confirmation", input: "\n", allowEmpty: true, validate: longEnough, want: ""},
		{name: "empty denied without allowEmpty", input: "\nshort1\nshort1\n", validate: longEnough, want: "short1"},
		{name: "EOF at entry", input: "", validate: longEnough, wantErr: errInputClosed},
		{name: "EOF at confirmation", input: "secret1\n", validate: longEnough, wantErr: errInputClosed},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var got string
			var err error
			out := captureStdout(t, func() {
				got, err = askSecret(newTestScanner(t, tc.input), "User Password", tc.allowEmpty, tc.validate)
			})
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("err = %v, want %v", err, tc.wantErr)
			}
			if err == nil && got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
			if tc.checkOut != nil {
				if cErr := tc.checkOut(out); cErr != nil {
					t.Fatal(cErr)
				}
			}
		})
	}
}

func TestAskSecretRealValidators(t *testing.T) {
	var got string
	captureStdout(t, func() {
		got, _ = askSecret(newTestScanner(t, "hunter2b\nhunter2b\n"), "User Password", false, config.ValidatePassword)
	})
	if got != "hunter2b" {
		t.Fatalf("got %q, want %q (real config.ValidatePassword)", got, "hunter2b")
	}
}

// captureStdout redirects package stdout (fmt.Printf prompts) into a

func TestEnumOptions(t *testing.T) {
	m := map[string]int{"b": 1, "a": 2, "c": 3}
	opts := enumOptions(m)
	if len(opts) != 3 {
		t.Fatalf("len = %d, want 3", len(opts))
	}
	seen := map[string]bool{}
	for _, k := range opts {
		seen[k] = true
	}
	for k := range m {
		if !seen[k] {
			t.Fatalf("missing key %q in %v", k, opts)
		}
	}
	if got := enumOptions(map[string]string{}); len(got) != 0 {
		t.Fatalf("empty map gave %v", got)
	}
}

func TestMaskDefault(t *testing.T) {
	if got := maskDefault(""); got != "(empty)" {
		t.Fatalf("got %q, want %q", got, "(empty)")
	}
	if got := maskDefault("abc"); got != "***" {
		t.Fatalf("got %q, want ***", got)
	}
	if got := maskDefault("héllo"); got != "*****" {
		t.Fatalf("multibyte padding got %q, want ***** (rune count)", got)
	}
}

func TestTruncDefault(t *testing.T) {
	short := "abc"
	if got := truncDefault(short); got != short {
		t.Fatalf("short value changed: %q", got)
	}
	w40 := strings.Repeat("a", 40)
	if got := truncDefault(w40); got != w40 {
		t.Fatalf("40 bytes kept verbatim failed: %q", got)
	}
	long := strings.Repeat("a", 41)
	want := strings.Repeat("a", 37) + "..."
	if got := truncDefault(long); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestReadLine(t *testing.T) {
	sc := newTestScanner(t, "  spaced line  \nplain\n")
	first, err := readLine(sc)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if first != "spaced line" {
		t.Fatalf("trim failed: %q", first)
	}
	second, err := readLine(sc)
	if err != nil || second != "plain" {
		t.Fatalf("second = (%q, %v)", second, err)
	}
	if _, err := readLine(sc); !errors.Is(err, errInputClosed) {
		t.Fatalf("EOF err = %v, want errInputClosed", err)
	}
}
