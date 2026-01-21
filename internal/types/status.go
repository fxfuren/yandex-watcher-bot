package types

import "time"

// VMStatus represents the current status of a VM
type VMStatus string

const (
	StatusUnknown      VMStatus = "Unknown"
	StatusRunning      VMStatus = "Running"
	StatusStopped      VMStatus = "Stopped"
	StatusStarting     VMStatus = "Starting"
	StatusStopping     VMStatus = "Stopping"
	StatusCrashed      VMStatus = "Crashed"
	StatusError        VMStatus = "Error"
	StatusProvisioning VMStatus = "Provisioning"
	StatusRestarting   VMStatus = "Restarting"
	StatusUpdating     VMStatus = "Updating"
	StatusDeleting     VMStatus = "Deleting"
)

// IsCritical returns true if the status requires immediate action
func (s VMStatus) IsCritical() bool {
	switch s {
	case StatusStopped, StatusCrashed, StatusError:
		return true
	default:
		return false
	}
}

// IsTransitional returns true if the VM is in a transitional state
func (s VMStatus) IsTransitional() bool {
	switch s {
	case StatusStarting, StatusStopping, StatusProvisioning, StatusRestarting, StatusUpdating:
		return true
	default:
		return false
	}
}

// ShouldStartVM returns true if we should attempt to start the VM in this status
func (s VMStatus) ShouldStartVM() bool {
	switch s {
	case StatusStopped, StatusCrashed, StatusError:
		return true
	default:
		return false
	}
}

// GetCheckInterval returns the recommended check interval for this status
func (s VMStatus) GetCheckInterval(min, max time.Duration) time.Duration {
	switch s {
	case StatusStopped, StatusCrashed, StatusError:
		// Check quickly to start recovery, but not too often
		return 5 * time.Second
	case StatusStarting, StatusRestarting, StatusProvisioning:
		// VM takes ~1-2 minutes to start, check every 15 seconds
		// This reduces API calls by 75% compared to 4-second interval
		return 15 * time.Second
	case StatusStopping:
		// Stopping is usually fast, check every 10 seconds
		return 10 * time.Second
	case StatusUpdating:
		// Updates take longer, check every 30 seconds
		return 30 * time.Second
	case StatusRunning:
		return max
	case StatusDeleting:
		return max
	default:
		return (min + max) / 2
	}
}

// GetTimeout returns the duration after which a transitional state is considered stuck
func (s VMStatus) GetTimeout() time.Duration {
	switch s {
	case StatusStarting, StatusRestarting:
		return 5 * time.Minute
	case StatusStopping:
		return 3 * time.Minute
	case StatusProvisioning:
		return 5 * time.Minute
	case StatusUpdating:
		return 10 * time.Minute
	default:
		return 0
	}
}
