package internal

// microcodeByCPU — the CPU-vendor microcode FACTS (user decision
// 2026-11-28: the values are ALWAYS the same — the standard Alpine
// microcode package/image pairs — so the internal_rules `microcode` map is
// GONE; these constants ARE the contract). The CPU vendor itself is still
// detected at install from /proc/cpuinfo. The booster-hook PACKAGE runs
// the same detection at every kernel upgrade via [ -f ] tests on the
// standard image paths (aports booster-hook).
type microcodeEntry struct {
	Package string // apk package installed at setup
	Image   string // initramfs image path included in the UKI
}

var microcodeByCPU = map[string]microcodeEntry{
	"amd":   {Package: "amd-ucode", Image: "/boot/amd-ucode.img"},
	"intel": {Package: "intel-ucode", Image: "/boot/intel-ucode.img"},
}
