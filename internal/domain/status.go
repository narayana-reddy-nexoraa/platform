package domain

// ExecutionStatus represents the lifecycle state of an execution.
type ExecutionStatus string

const (
	StatusCreated  ExecutionStatus = "CREATED"
	StatusClaimed  ExecutionStatus = "CLAIMED"
	StatusRunning  ExecutionStatus = "RUNNING"
	StatusSucceeded ExecutionStatus = "SUCCEEDED"
	StatusFailed   ExecutionStatus = "FAILED"
	StatusCanceled ExecutionStatus = "CANCELED"
	StatusTimedOut ExecutionStatus = "TIMED_OUT"
)

// validTransitions defines the allowed state machine transitions.
// Key = current status, Value = set of valid next statuses.
var validTransitions = map[ExecutionStatus]map[ExecutionStatus]bool{
	StatusCreated: {
		StatusClaimed:  true,
		StatusCanceled: true,
	},
	StatusClaimed: {
		StatusRunning:  true,
		StatusCanceled: true,
		StatusTimedOut: true,
	},
	StatusRunning: {
		StatusSucceeded: true,
		StatusFailed:    true,
		StatusCanceled:  true,
		StatusTimedOut:  true,
	},
	StatusFailed: {
		StatusClaimed:  true, // retry: FAILED → CLAIMED
		StatusCanceled: true,
	},
	// Terminal states: no transitions out
	StatusSucceeded: {},
	StatusCanceled:  {},
	StatusTimedOut:  {},
}

// CanTransitionTo checks if the transition from current to next is valid.
func (s ExecutionStatus) CanTransitionTo(next ExecutionStatus) bool {
	targets, ok := validTransitions[s]
	if !ok {
		return false
	}
	return targets[next]
}

// IsTerminal returns true if no further transitions are possible.
func (s ExecutionStatus) IsTerminal() bool {
	targets, ok := validTransitions[s]
	if !ok {
		return true
	}
	return len(targets) == 0
}

// AllStatuses returns all valid execution statuses.
func AllStatuses() []ExecutionStatus {
	return []ExecutionStatus{
		StatusCreated, StatusClaimed, StatusRunning,
		StatusSucceeded, StatusFailed, StatusCanceled, StatusTimedOut,
	}
}

// IsValidStatus checks if a string is a valid execution status.
func IsValidStatus(s string) bool {
	for _, status := range AllStatuses() {
		if string(status) == s {
			return true
		}
	}
	return false
}
