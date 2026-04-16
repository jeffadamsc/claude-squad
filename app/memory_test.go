package app

import (
	"fmt"
	"testing"
)

func TestGetSystemMemoryMB(t *testing.T) {
	mb := GetSystemMemoryMB()
	if mb <= 0 {
		t.Fatalf("expected positive system memory, got %d MB", mb)
	}
	if mb < 2048 {
		t.Fatalf("expected at least 2048 MB, got %d MB", mb)
	}
}

func TestComputeMemoryLimits(t *testing.T) {
	tests := []struct {
		ramMB           int
		wantGOMEMLIMIT  int64
		wantIdleTimeout int // minutes, 0 = disabled
	}{
		{8192, 128 * 1024 * 1024, 15},
		{16384, 256 * 1024 * 1024, 30},
		{32768, 384 * 1024 * 1024, 60},
		{65536, 512 * 1024 * 1024, 0},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%dMB", tt.ramMB), func(t *testing.T) {
			limits := ComputeMemoryLimits(tt.ramMB)
			if limits.GOMEMLIMIT != tt.wantGOMEMLIMIT {
				t.Errorf("GOMEMLIMIT: got %d, want %d", limits.GOMEMLIMIT, tt.wantGOMEMLIMIT)
			}
			if limits.IdleTimeoutMinutes != tt.wantIdleTimeout {
				t.Errorf("IdleTimeout: got %d min, want %d min", limits.IdleTimeoutMinutes, tt.wantIdleTimeout)
			}
		})
	}
}
