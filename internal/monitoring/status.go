package monitoring

import "github.com/fxfuren/yandex-watcher-bot/internal/types"

// IsValidTransition checks if transitioning from 'from' to 'to' status is valid
func IsValidTransition(from, to types.VMStatus) bool {
	// Allow any transition from Unknown (first check)
	if from == types.StatusUnknown {
		return true
	}

	// Same status is always valid
	if from == to {
		return true
	}

	validTransitions := map[types.VMStatus][]types.VMStatus{
		types.StatusStopped: {
			types.StatusStarting,
			types.StatusProvisioning, // VM can go to provisioning when starting
			types.StatusDeleting,
		},
		types.StatusStarting: {
			types.StatusRunning,
			types.StatusError,
			types.StatusStopped,
		},
		types.StatusRunning: {
			types.StatusStopping,
			types.StatusRestarting,
			types.StatusUpdating,
			types.StatusCrashed,
			types.StatusError,
			types.StatusStopped,
		},
		types.StatusStopping: {
			types.StatusStopped,
			types.StatusError,
		},
		types.StatusRestarting: {
			types.StatusRunning,
			types.StatusError,
			types.StatusStopped,
		},
		types.StatusUpdating: {
			types.StatusRunning,
			types.StatusError,
		},
		types.StatusProvisioning: {
			types.StatusStopped,
			types.StatusStarting,  // Provisioning -> Starting is valid
			types.StatusRunning,
			types.StatusError,
		},
		types.StatusCrashed: {
			types.StatusStarting,
			types.StatusProvisioning,
			types.StatusStopped,
		},
		types.StatusError: {
			types.StatusStopped,
			types.StatusStarting,
			types.StatusProvisioning,
			types.StatusDeleting,
		},
		types.StatusDeleting: {},
	}

	allowed, exists := validTransitions[from]
	if !exists {
		return false
	}

	for _, allowedStatus := range allowed {
		if to == allowedStatus {
			return true
		}
	}

	return false
}
