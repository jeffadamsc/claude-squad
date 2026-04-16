package app

import (
	"encoding/binary"

	"golang.org/x/sys/unix"
)

// GetSystemMemoryMB returns the total system RAM in megabytes.
// Falls back to 8192 (8 GB) if detection fails.
func GetSystemMemoryMB() int {
	raw, err := unix.SysctlRaw("hw.memsize")
	if err != nil || len(raw) < 8 {
		return 8192
	}
	memBytes := binary.LittleEndian.Uint64(raw)
	return int(memBytes / (1024 * 1024))
}

// MemoryLimits holds adaptive configuration derived from system RAM.
type MemoryLimits struct {
	GOMEMLIMIT         int64 // bytes, for debug.SetMemoryLimit
	IdleTimeoutMinutes int   // 0 = disabled
}

// ComputeMemoryLimits returns scaled memory configuration for the given RAM (in MB).
func ComputeMemoryLimits(ramMB int) MemoryLimits {
	switch {
	case ramMB <= 8192:
		return MemoryLimits{
			GOMEMLIMIT:         128 * 1024 * 1024,
			IdleTimeoutMinutes: 15,
		}
	case ramMB <= 16384:
		return MemoryLimits{
			GOMEMLIMIT:         256 * 1024 * 1024,
			IdleTimeoutMinutes: 30,
		}
	case ramMB <= 32768:
		return MemoryLimits{
			GOMEMLIMIT:         384 * 1024 * 1024,
			IdleTimeoutMinutes: 60,
		}
	default:
		return MemoryLimits{
			GOMEMLIMIT:         512 * 1024 * 1024,
			IdleTimeoutMinutes: 0,
		}
	}
}
