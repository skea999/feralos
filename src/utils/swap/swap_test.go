package swap

import "testing"

func TestParseSwapSizeMiB(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    int
		wantErr bool
	}{
		{"auto lower", "auto", -1, false}, // -1 = skip exact, just no error (depends on /proc/meminfo)
		{"auto upper", "AUTO", -1, false},
		{"auto whitespace", "  auto  ", -1, false},
		{"empty", "", -1, false},
		{"8G", "8G", 8192, false},
		{"8g lower", "8g", 8192, false},
		{"8 G with space", "8 G", 8192, false},
		{"16G", "16G", 16384, false},
		{"4096M", "4096M", 4096, false},
		{"8192M", "8192M", 8192, false},
		{"m no unit", "1024", 1024, false},
		{"M explicit", "1024M", 1024, false},
		{"1T exceeds max", "1T", 0, true},
		{"1P exceeds max", "1P", 0, true},
		{"512K ceil to 1", "512K", 1, false},
		{"1024K exact 1", "1024K", 1, false},
		{"2048K 2", "2048K", 2, false},
		{"0K", "0K", 0, false},
		{"0", "0", 0, false},
		{"invalid foo", "foo", 0, true},
		{"invalid 8X", "8X", 0, true},
		{"invalid -1G", "-1G", 0, true},
		{"invalid 8.5G", "8.5G", 0, true},
		{"invalid empty G", "G", 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ResolveSwapSizeMiB(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error for %q, got %d", tt.input, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error for %q: %v", tt.input, err)
			}
			if tt.want == -1 {
				// auto: just check >0
				if got <= 0 {
					t.Fatalf("auto expected >0, got %d", got)
				}
				return
			}
			if got != tt.want {
				t.Fatalf("ResolveSwapSizeMiB(%q)=%d want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestResolveSwapSizeMiBExplicit(t *testing.T) {
	// Direct parse tests without /proc/meminfo
	cases := map[string]int{
		"8G":    8192,
		"4G":    4096,
		"2G":    2048,
		"1024M": 1024,
		"2048M": 2048,
	}
	for in, want := range cases {
		got, err := parseSwapSizeMiB(in)
		if err != nil {
			t.Fatalf("%q err %v", in, err)
		}
		if got != want {
			t.Fatalf("%q got %d want %d", in, got, want)
		}
	}
}
