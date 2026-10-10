package swap

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
)

// ResolveSwapSizeMiB returns the total swap size in MiB.
// "auto" or "" → 2x system RAM (from /proc/meminfo MemTotal).
// Explicit values like "8G", "4096M", "16G" are parsed as-is.
func ResolveSwapSizeMiB(swapSize string) (int, error) {
	size := strings.TrimSpace(swapSize)
	if size == "" || strings.EqualFold(size, "auto") {
		return autoSwapSizeMiB()
	}
	return parseSwapSizeMiB(size)
}

// ValidSwapSize reports whether swapSize is "auto"/empty or a valid explicit
// size (8G, 4096M, ...). Does NOT read /proc/meminfo — safe for validation.
func ValidSwapSize(swapSize string) error {
	size := strings.TrimSpace(swapSize)
	if size == "" || strings.EqualFold(size, "auto") {
		return nil
	}
	_, err := parseSwapSizeMiB(size)
	return err
}

// autoSwapSizeMiB reads MemTotal from /proc/meminfo and returns 2x RAM in MiB.
func autoSwapSizeMiB() (int, error) {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0, fmt.Errorf("failed to read /proc/meminfo: %w", err)
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "MemTotal:") {
			fields := strings.Fields(line)
			if len(fields) < 2 {
				return 0, fmt.Errorf("unexpected MemTotal line: %q", line)
			}
			kb, err := strconv.Atoi(fields[1])
			if err != nil {
				return 0, fmt.Errorf("invalid MemTotal value %q: %w", fields[1], err)
			}
			// 2x RAM, converted from KiB to MiB
			return kb / 1024 * 2, nil
		}
	}
	return 0, fmt.Errorf("MemTotal not found in /proc/meminfo")
}

var sizeRe = regexp.MustCompile(`(?i)^(\d+)\s*([kmgtp]?)$`)

// maxSwapMiB caps explicit swap sizes to prevent DoS (e.g. 999999P = petabyte).
const maxSwapMiB = 32768 // 32 GiB

// parseSwapSizeMiB parses an explicit size like "8G" / "4096M" into MiB.
func parseSwapSizeMiB(size string) (int, error) {
	m := sizeRe.FindStringSubmatch(size)
	if m == nil {
		return 0, fmt.Errorf("invalid swap size %q (use e.g. 8G, 4096M, 16G or auto)", size)
	}
	n, err := strconv.Atoi(m[1])
	if err != nil {
		return 0, fmt.Errorf("invalid swap size %q: %w", size, err)
	}
	unit := strings.ToLower(m[2])
	var mib int
	switch unit {
	case "", "m":
		mib = n
	case "k":
		if n == 0 {
			return 0, nil
		}
		mib = (n + 1023) / 1024
		if mib == 0 {
			mib = 1
		}
	case "g":
		mib = n * 1024
	case "t":
		mib = n * 1024 * 1024
	case "p":
		mib = n * 1024 * 1024 * 1024
	default:
		return 0, fmt.Errorf("invalid swap size %q", size)
	}
	if mib > maxSwapMiB {
		return 0, fmt.Errorf("swap size %d MiB exceeds maximum %d MiB (32 GiB)", mib, maxSwapMiB)
	}
	return mib, nil
}
