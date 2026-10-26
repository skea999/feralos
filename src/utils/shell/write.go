package shell

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

// WriteFile writes content to path with logging — the sanctioned way for
// hostPhase/chrootPhase to write files (logging-rule amendment, polishCode
// P2-5). It replaces the fragile `echo '<escaped>' > path` shell pattern:
// single-quote escaping is error-prone with arbitrary content and very large
// payloads can hit ARG_MAX, while a direct write is exact and only logs the
// path + size (never the content).
//
// Contract:
//   - chroot is the mount ROOT (e.g. /mnt); path is the in-guest absolute
//     path. The file is written DIRECTLY on the host at
//     filepath.Join(chroot, path) — no chroot(2) involved. Empty chroot =
//     host-rooted path (also what tests use).
//   - MkdirAll of the parent directory (0755) runs first.
//   - perm is forced via chmod when the file is newly created (defeats
//     umask; matches what `echo > file` produced under umask 022). An
//     EXISTING file keeps its mode (same as `echo > file` truncating in
//     place); run a chmod command afterwards when a reset is wanted.
//   - Logging mirrors the command style: a `[WRITE]` line (path + size)
//     followed by a `[RESULT]` line. Content is NEVER logged.
//   - Failure honors SKEA_IGNORE_ERRORS=1 (logs `[ERROR][IGNORED]` and
//     returns nil) — same best-effort semantics as ExecuteCmd, so harness
//     runs stay non-fatal.
//
// shell.ExecuteCmd remains the tool for COMMANDS; file WRITES go through
// here (WriteFile / WriteFileAppend). Documented exec carve-outs (passwd,
// cryptsetup stdin) are unchanged.
func WriteFile(chroot, path string, content []byte, perm os.FileMode) error {
	return writeFileLogging(chroot, path, content, perm, "WRITE")
}

// WriteFileAppend appends content to path with logging — the append analogue
// of WriteFile (same contract; O_APPEND, creates the file when missing).
// Use it for unconditional `echo '...' >> file` sites. CONDITIONAL appends
// (a `grep -q ... || echo >> file` idempotency guard) stay on ExecuteCmd —
// the guard is part of the operation and is not expressible as a plain
// append.
func WriteFileAppend(chroot, path string, content []byte, perm os.FileMode) error {
	return writeFileLogging(chroot, path, content, perm, "APPEND")
}

// writeFileLogging is the single implementation behind WriteFile and
// WriteFileAppend.
func writeFileLogging(chroot, path string, content []byte, perm os.FileMode, verb string) error {
	target := path
	if chroot != "" {
		target = filepath.Join(chroot, path)
	}

	caller := GetCaller()
	prefix := ""
	if caller != "" {
		prefix = fmt.Sprintf("[%s]", caller)
	}

	timestamp := time.Now().Format("2006-01-02 15:04:05")
	fmt.Printf("\n%s[%s] %s: %s (%d bytes)\n", prefix, timestamp, verb, target, len(content))

	start := time.Now()
	err := writeFileDirect(target, content, perm, verb == "APPEND")
	duration := time.Since(start)

	if err == nil {
		fmt.Printf("[%s][RESULT] %s ok time=%v\n", prefix, verb, duration.Round(time.Microsecond))
		return nil
	}

	fmt.Printf("[%s][RESULT] %s FAIL time=%v: %v\n", prefix, verb, duration.Round(time.Microsecond), err)

	if os.Getenv("SKEA_IGNORE_ERRORS") == "1" {
		fmt.Printf("[ERROR][IGNORED] write failed (ignoreErrors=true): %s: %v\n", target, err)
		return nil
	}

	return fmt.Errorf("%s %s: %w", verb, path, err)
}

// writeFileDirect performs the parent MkdirAll + open/write/chmod. An
// existing file keeps its mode (echo > truncates in place); a newly created
// one gets perm enforced (echo > under umask 022 would produce 0644 — the
// explicit chmod makes the result umask-independent).
func writeFileDirect(target string, content []byte, perm os.FileMode, appendMode bool) error {
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return fmt.Errorf("mkdir parent: %w", err)
	}

	existed := true
	if _, err := os.Stat(target); err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("stat: %w", err)
		}
		existed = false
	}

	flags := os.O_WRONLY | os.O_CREATE
	if appendMode {
		flags |= os.O_APPEND
	} else {
		flags |= os.O_TRUNC
	}

	f, err := os.OpenFile(target, flags, perm)
	if err != nil {
		return fmt.Errorf("open: %w", err)
	}
	if _, err := f.Write(content); err != nil {
		f.Close()
		return fmt.Errorf("write: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("close: %w", err)
	}

	if !existed {
		if err := os.Chmod(target, perm); err != nil {
			return fmt.Errorf("chmod: %w", err)
		}
	}
	return nil
}
