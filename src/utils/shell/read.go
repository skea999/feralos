package shell

import (
	"fmt"
	"os"
	"time"
)

// ReadFile reads a HOST-side file with logging — the read counterpart of
// WriteFile. hostPhase/chrootPhase use it to load assets that live in the
// repo tree (e.g. branding/) before writing them into the target via
// WriteFile; reads of files INSIDE the chroot stay on ExecuteCmd (the
// command runs in the guest). Logging mirrors the write style: a `[READ]`
// line (path + size) and a `[RESULT]` line; content is NEVER logged.
//
// Unlike WriteFile there is no chroot parameter — reads are host-side by
// definition. Failures return the error; callers decide fatality (assets
// are cosmetic → WARNING, config → error).
func ReadFile(path string) ([]byte, error) {
	caller := GetCaller()
	prefix := ""
	if caller != "" {
		prefix = fmt.Sprintf("[%s]", caller)
	}

	timestamp := time.Now().Format("2006-01-02 15:04:05")
	fmt.Printf("\n%s[%s] READ: %s\n", prefix, timestamp, path)

	start := time.Now()
	content, err := os.ReadFile(path)
	duration := time.Since(start)

	if err == nil {
		fmt.Printf("[%s][RESULT] READ ok size=%d time=%v\n", prefix, len(content), duration.Round(time.Microsecond))
		return content, nil
	}

	fmt.Printf("[%s][RESULT] READ FAIL time=%v: %v\n", prefix, duration.Round(time.Microsecond), err)
	return nil, fmt.Errorf("read %s: %w", path, err)
}
