package monitoring

import (
	"testing"
	"time"

	"github.com/fxfuren/yandex-watcher-bot/internal/types"
)

func TestVMStatus_IsCritical(t *testing.T) {
	tests := []struct {
		status   types.VMStatus
		expected bool
	}{
		{types.StatusStopped, true},
		{types.StatusCrashed, true},
		{types.StatusError, true},
		{types.StatusRunning, false},
		{types.StatusStarting, false},
		{types.StatusUpdating, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			if got := tt.status.IsCritical(); got != tt.expected {
				t.Errorf("IsCritical() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestVMStatus_IsTransitional(t *testing.T) {
	tests := []struct {
		status   types.VMStatus
		expected bool
	}{
		{types.StatusStarting, true},
		{types.StatusStopping, true},
		{types.StatusProvisioning, true},
		{types.StatusRestarting, true},
		{types.StatusUpdating, true},
		{types.StatusRunning, false},
		{types.StatusStopped, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			if got := tt.status.IsTransitional(); got != tt.expected {
				t.Errorf("IsTransitional() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestVMStatus_ShouldStartVM(t *testing.T) {
	tests := []struct {
		status   types.VMStatus
		expected bool
	}{
		{types.StatusStopped, true},
		{types.StatusCrashed, true},
		{types.StatusError, true},
		{types.StatusRunning, false},
		{types.StatusStarting, false},
		{types.StatusStopping, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			if got := tt.status.ShouldStartVM(); got != tt.expected {
				t.Errorf("ShouldStartVM() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestVMStatus_GetCheckInterval(t *testing.T) {
	min := 5 * time.Second
	max := 60 * time.Second

	tests := []struct {
		status   types.VMStatus
		expected time.Duration
	}{
		{types.StatusStopped, min},
		{types.StatusCrashed, min},
		{types.StatusError, min * 2},
		{types.StatusStarting, min * 2},
		{types.StatusRunning, max},
		{types.StatusProvisioning, min * 3},
		{types.StatusUpdating, min * 6},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			got := tt.status.GetCheckInterval(min, max)
			if got != tt.expected {
				t.Errorf("GetCheckInterval() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestVMStatus_GetTimeout(t *testing.T) {
	tests := []struct {
		status   types.VMStatus
		expected time.Duration
	}{
		{types.StatusStarting, 5 * time.Minute},
		{types.StatusRestarting, 5 * time.Minute},
		{types.StatusStopping, 3 * time.Minute},
		{types.StatusProvisioning, 5 * time.Minute},
		{types.StatusUpdating, 10 * time.Minute},
		{types.StatusRunning, 0},
		{types.StatusStopped, 0},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			if got := tt.status.GetTimeout(); got != tt.expected {
				t.Errorf("GetTimeout() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestIsValidTransition(t *testing.T) {
	tests := []struct {
		from     types.VMStatus
		to       types.VMStatus
		expected bool
	}{
		{types.StatusUnknown, types.StatusRunning, true},
		{types.StatusUnknown, types.StatusStopped, true},
		{types.StatusStopped, types.StatusStarting, true},
		{types.StatusStarting, types.StatusRunning, true},
		{types.StatusRunning, types.StatusStopping, true},
		{types.StatusStopping, types.StatusStopped, true},
		{types.StatusRunning, types.StatusCrashed, true},
		{types.StatusCrashed, types.StatusStarting, true},
		{types.StatusStopped, types.StatusRunning, false},
		{types.StatusRunning, types.StatusStarting, false},
		{types.StatusDeleting, types.StatusRunning, false},
		{types.StatusRunning, types.StatusRunning, true},
		{types.StatusStopped, types.StatusStopped, true},
	}

	for _, tt := range tests {
		t.Run(string(tt.from)+"_to_"+string(tt.to), func(t *testing.T) {
			if got := IsValidTransition(tt.from, tt.to); got != tt.expected {
				t.Errorf("IsValidTransition(%v, %v) = %v, want %v", tt.from, tt.to, got, tt.expected)
			}
		})
	}
}

func BenchmarkGetCheckInterval(b *testing.B) {
	min := 5 * time.Second
	max := 60 * time.Second
	status := types.StatusRunning

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = status.GetCheckInterval(min, max)
	}
}
