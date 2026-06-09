package llamaserve

import (
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
)

// totalRAMBytes reports the host's total physical RAM. It is a package var so
// tests can inject a fixed value. Returns 0 on detection failure.
var totalRAMBytes = detectTotalRAM

const giB = 1 << 30

// ContextForHost returns the adaptive "Auto" context size (tokens) for the
// host, tiered by total RAM. Weights are mmap'd, so the KV cache is the RAM
// lever we control; tiering on total RAM is robust and needs no GGUF parsing.
// On detection failure it falls back to the smallest, safest tier.
func ContextForHost() int {
	bytes := totalRAMBytes()
	gib := bytes / giB
	switch {
	case bytes == 0:
		return CtxFast // detection failed → safe fallback
	case gib < 10: // ~8 GB class
		return CtxFast
	case gib < 20: // ~16 GB class
		return CtxBalanced
	case gib < 40: // ~32 GB class
		return 65536
	default: // 64 GB+
		return CtxMax
	}
}

// detectTotalRAM reads total physical memory. Linux: /proc/meminfo MemTotal.
// macOS: sysctl hw.memsize. Returns 0 if it cannot be determined.
func detectTotalRAM() uint64 {
	switch runtime.GOOS {
	case "linux":
		data, err := os.ReadFile("/proc/meminfo")
		if err != nil {
			return 0
		}
		for _, line := range strings.Split(string(data), "\n") {
			if !strings.HasPrefix(line, "MemTotal:") {
				continue
			}
			fields := strings.Fields(line) // "MemTotal:  16384000 kB"
			if len(fields) < 2 {
				return 0
			}
			kb, err := strconv.ParseUint(fields[1], 10, 64)
			if err != nil {
				return 0
			}
			return kb * 1024
		}
		return 0
	case "darwin":
		out, err := exec.Command("sysctl", "-n", "hw.memsize").Output()
		if err != nil {
			return 0
		}
		n, err := strconv.ParseUint(strings.TrimSpace(string(out)), 10, 64)
		if err != nil {
			return 0
		}
		return n
	default:
		return 0
	}
}
