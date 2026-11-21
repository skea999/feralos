package service

import "testing"

func TestRetriedAttempt(t *testing.T) {
	cases := []struct {
		name  string
		lines []string
		i     int
		want  bool
	}{
		{
			name: "fail followed by retry marker",
			lines: []string{
				"[[packages]][RESULT] FAIL exit=1 time=5ms",
				"",
				"[[packages]][RETRY] Attempt 1/3 failed, retrying in 3s...",
				"[[packages]] CHROOT: apk add chrony-dinit[stdout] OK",
			},
			i:    0,
			want: true,
		},
		{
			name: "final failure without retry",
			lines: []string{
				"[[uki]][RESULT] FAIL exit=1 time=3ms",
				"[ERROR][IGNORED] command failed (ignoreErrors=true)",
			},
			i:    0,
			want: false,
		},
		{
			name: "retry marker too far away (other output in between)",
			lines: []string{
				"[[packages]][RESULT] FAIL exit=1",
				"[stderr] something else happened",
				"[[packages]][RETRY] Attempt 1/3",
			},
			i:    0,
			want: false,
		},
		{
			name: "blank lines are skipped before the retry marker",
			lines: []string{
				"[[packages]][RESULT] FAIL exit=1",
				"",
				"",
				"[[packages]][RETRY] Attempt 2/3",
			},
			i:    0,
			want: true,
		},
		{
			name: "last line, nothing follows",
			lines: []string{
				"[[packages]][RESULT] FAIL exit=1",
			},
			i:    0,
			want: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := retriedAttempt(tc.lines, tc.i); got != tc.want {
				t.Fatalf("retriedAttempt = %v, want %v", got, tc.want)
			}
		})
	}
}
