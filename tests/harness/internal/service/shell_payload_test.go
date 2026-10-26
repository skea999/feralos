package service

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

// P3-4: the harness payloads embedded as Go raw strings are invisible to
// every shell tool until a VM runs them. This test extracts every raw string
// literal from the non-test sources of this package, keeps the ones that
// look like shell scripts, and syntax-checks each with `sh -n` (or `bash -n`
// when bash-isms are present). A syntax error fails the test with the
// payload's file + byte offset + first line.
//
// Sprintf placeholders (%s/%d/...) are replaced with the EMPTY string before
// the check: placeholders keep quoting balanced, but a verb may be glued to
// a shell keyword in the template (real case: "%sfor f in ...", where the
// runtime argument is an empty-or-newline prefix). Empty substitution turns
// "%sfor" back into the "for" keyword and stays syntax-valid everywhere else
// ("" in place of a word argument is always parseable; sh -n checks syntax,
// not runtime argument counts).
//
// oo7_pam_unlock.go is EXCLUDED: it embeds a 130+ line PYTHON payload, not
// shell — feeding it to sh -n would fail by construction. The python blob is
// validated separately (py_compile, checked in past rounds); making this test
// run python3 keeps a host-tool dependency out of the shell gate.
func TestShellPayloadsAreSyntacticallyValid(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	dir := filepath.Dir(thisFile)

	sources := []string{"service.go", "verify_builders.go", "de_data.go"}

	rawRe := regexp.MustCompile("(?s)`([^`]*)`")
	// Matches printf-style verbs: %s %d %q %02d %-v ... (flags/width/precision
	// then a verb letter). %% is left alone; sh -n does not parse formats.
	verbRe := regexp.MustCompile(`%[#\-+ 0]*[0-9.]*(?:\.[0-9]+)?[a-zA-Z]`)

	shellStarters := []string{
		"set ", "echo ", "printf", "if ", "for ", "while ", "case ", "until ",
		"export ", "#!", "chmod", "mkdir", "cat ", "apk ", "fish ", "su ",
		"fail=", "broker=", "install ", "rm -f", "ls -l",
	}
	shellMarkers := []string{"if ", "for ", "then", "$(", "grep ", "dinitctl "}

	extracted := 0
	for _, name := range sources {
		path := filepath.Join(dir, name)
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		for _, loc := range rawRe.FindAllSubmatchIndex(src, -1) {
			literal := string(src[loc[2]:loc[3]])
			if !looksLikeShell(literal, shellStarters, shellMarkers) {
				continue
			}
			extracted++
			payload := verbRe.ReplaceAllString(literal, "")
			checkPayload(t, name, loc[2], literal, payload)
		}
	}

	// Guard against silent degradation: today ~20 payloads qualify. If the
	// filter ever matches nothing, the test must fail loudly instead of
	// passing vacuously.
	if extracted < 10 {
		t.Fatalf("shell-payload filter extracted only %d payloads — heuristic or sources changed", extracted)
	}
}

func looksLikeShell(literal string, starters, markers []string) bool {
	if !strings.Contains(literal, "\n") {
		return false
	}
	trimmed := strings.TrimSpace(literal)
	for _, s := range starters {
		if strings.HasPrefix(trimmed, s) {
			return true
		}
	}
	for _, m := range markers {
		if strings.Contains(literal, m) {
			return true
		}
	}
	return false
}

func checkPayload(t *testing.T, file string, offset int, literal, payload string) {
	t.Helper()

	interpreter := "sh"
	if strings.Contains(payload, "[[") {
		interpreter = "bash" // bash-isms present
	}

	bin, err := exec.LookPath(interpreter)
	if err != nil {
		t.Skipf("%s not in PATH", interpreter)
	}

	tmp := t.TempDir()
	script := filepath.Join(tmp, "payload.sh")
	if err := os.WriteFile(script, []byte(payload), 0o600); err != nil {
		t.Fatalf("write temp payload: %v", err)
	}

	cmd := exec.Command(bin, "-n", script)
	if out, err := cmd.CombinedOutput(); err != nil {
		firstLine := literal
		if i := strings.IndexByte(firstLine, '\n'); i >= 0 {
			firstLine = firstLine[:i]
		}
		firstLine = strings.TrimSpace(firstLine)
		t.Errorf("shell syntax error in payload %s @byte %d (first line %q): %v\n%s",
			file, offset, firstLine, err, strings.TrimSpace(string(out)))
	}
}
