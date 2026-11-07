package internal

import (
	"reflect"
	"testing"
)

// deduplicate (firmware.go, unexported) collapses ADJACENT duplicates — the
// caller must pass a sorted slice (DetectFirmwarePackages sorts first).
func TestDeduplicate(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		in   []string
		want []string
	}{
		{"empty input", []string{}, []string{}},
		{"no duplicates", []string{"a", "b", "c"}, []string{"a", "b", "c"}},
		{"adjacent duplicates collapse", []string{"a", "a", "b"}, []string{"a", "b"}},
		{"all identical", []string{"a", "a", "a"}, []string{"a"}},
		{"order preserved", []string{"a", "b", "b", "c", "d", "d"}, []string{"a", "b", "c", "d"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := deduplicate(tt.in); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("deduplicate(%v) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

// mapFirmwareToPackages (firmware.go, unexported) resolves firmware file
// names to owning apk packages via the fetched index; unmatched files only
// warn, matched packages are emitted deduplicated.
func TestMapFirmwareToPackages(t *testing.T) {
	t.Parallel()

	t.Run("matched files yield owning packages", func(t *testing.T) {
		index := map[string]string{
			"i915/guc/adl.guc":   "linux-firmware-i915",
			"i915/guc/tgl.guc":   "linux-firmware-i915",
			"amdgpu/si58_me.bin": "linux-firmware-amdgpu",
		}
		got := mapFirmwareToPackages(
			[]string{"i915/guc/adl.guc", "i915/guc/tgl.guc", "amdgpu/si58_me.bin"}, index)
		// map iteration order — compare as a set
		want := []string{"linux-firmware-i915", "linux-firmware-amdgpu"}
		if len(got) != len(want) {
			t.Fatalf("got %v, want %v", got, want)
		}
		for _, p := range want {
			found := false
			for _, g := range got {
				if g == p {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("missing %v from output %v", p, got)
			}
		}
	})

	t.Run("unmatched files are skipped without error", func(t *testing.T) {
		index := map[string]string{"known.bin": "linux-firmware-known"}
		got := mapFirmwareToPackages([]string{"known.bin", "ghost.bin"}, index)
		want := []string{"linux-firmware-known"}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v (unmatched only warns)", got, want)
		}
	})

	t.Run("nothing matched yields empty list", func(t *testing.T) {
		got := mapFirmwareToPackages([]string{"ghost.bin"}, map[string]string{})
		if len(got) != 0 {
			t.Errorf("got %v, want empty", got)
		}
	})
}
