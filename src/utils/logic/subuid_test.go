package logic

import (
	"testing"
)

func TestSubuidConsts(t *testing.T) {
	t.Parallel()
	if SubuidStart != 100000 {
		t.Errorf("SubuidStart = %d, want 100000 (pin the subordinate range)", SubuidStart)
	}
	if SubuidCount != 65536 {
		t.Errorf("SubuidCount = %d, want 65536 (2^16 subids)", SubuidCount)
	}
}

func TestSubuidLines(t *testing.T) {
	tests := []struct {
		name   string
		user   string
		subuid string
		subgid string
	}{
		{
			name:   "root shape",
			user:   "root",
			subuid: "root:100000:65536\n",
			subgid: "root:100000:65536\n",
		},
		{
			name:   "primary user shape",
			user:   "skea999",
			subuid: "skea999:100000:65536\n",
			subgid: "skea999:100000:65536\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			subuid, subgid := SubuidLines(tt.user)
			if subuid != tt.subuid {
				t.Errorf("subuid = %q, want %q", subuid, tt.subuid)
			}
			if subgid != tt.subgid {
				t.Errorf("subgid = %q, want %q (identical shape)", subgid, tt.subgid)
			}
			// Trailing newline: writers append with WriteFileAppend — the
			// range must self-terminate or the next line glues on.
			if subuid[len(subuid)-1] != '\n' || subgid[len(subgid)-1] != '\n' {
				t.Errorf("lines must self-terminate with \\n: %q / %q", subuid, subgid)
			}
		})
	}
}
