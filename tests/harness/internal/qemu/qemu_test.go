package qemu

import (
	"strings"
	"testing"
)

func TestTextQCodesMapping(t *testing.T) {
	tests := []struct {
		name string
		text string
		want [][]string
	}{
		{"letters", "abc", [][]string{{"a"}, {"b"}, {"c"}}},
		{"digits", "0129", [][]string{{"0"}, {"1"}, {"2"}, {"9"}}},
		{"space", "a b", [][]string{{"a"}, {"spc"}, {"b"}}},
		{"enter", "x\n", [][]string{{"x"}, {"ret"}}},
		{"carriage return", "x\r", [][]string{{"x"}, {"ret"}}},
		{"mixed", "ab1 2\n", [][]string{{"a"}, {"b"}, {"1"}, {"spc"}, {"2"}, {"ret"}}},
		{"uppercase uses shift", "Ab", [][]string{{"shift", "a"}, {"b"}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := TextQCodes(tt.text)
			if err != nil {
				t.Fatalf("TextQCodes(%q) unexpected error: %v", tt.text, err)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("TextQCodes(%q) = %v, want %v", tt.text, got, tt.want)
			}
			for i := range got {
				if strings.Join(got[i], ",") != strings.Join(tt.want[i], ",") {
					t.Errorf("TextQCodes(%q)[%d] = %v, want %v", tt.text, i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestTextQCodesUnsupportedChar(t *testing.T) {
	// § is genuinely unmapped (non-ASCII); # and ! ARE mapped (shift 3/1)
	got, err := TextQCodes("ab§")
	if err == nil {
		t.Fatalf(`TextQCodes("ab§") expected error, got batches %v`, got)
	}
	if !strings.Contains(err.Error(), "§") {
		t.Errorf("error %q must list the unsupported char (fail loud, no guessing)", err.Error())
	}
	if got != nil {
		t.Errorf("TextQCodes must return no batches on unsupported chars, got %v", got)
	}
}

func TestSendTextRejectsUnsupportedBeforeDialing(t *testing.T) {
	// Missing socket: SendText must fail on validation BEFORE dialing, so
	// the error is the mapping error, not a dial error. § is unmapped.
	err := SendText("/nonexistent/qmp.sock", "pa§§")
	if err == nil {
		t.Fatal("SendText expected error for unsupported chars")
	}
	if !strings.Contains(err.Error(), "unsupported character") {
		t.Errorf("SendText error = %q, want the unsupported-character validation error", err.Error())
	}
}

func TestTextQCodesPunctuation(t *testing.T) {
	batches, err := TextQCodes(".-_")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := [][]string{{"dot"}, {"minus"}, {"shift", "minus"}}
	if len(batches) != len(want) {
		t.Fatalf("got %d batches, want %d", len(batches), len(want))
	}
	for i, b := range want {
		if strings.Join(b, "+") != strings.Join(batches[i], "+") {
			t.Errorf("batch %d: got %v, want %v", i, batches[i], b)
		}
	}
	shifted, err := TextQCodes("@_")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(shifted) != 2 || !strings.Contains(strings.Join(shifted[0], "+"), "shift+2") || !strings.Contains(strings.Join(shifted[1], "+"), "shift+minus") {
		t.Errorf("shifted mapping wrong: %v", shifted)
	}
	if _, err := TextQCodes("\u00e8"); err == nil {
		t.Error("non-ASCII should fail loud")
	}
}
