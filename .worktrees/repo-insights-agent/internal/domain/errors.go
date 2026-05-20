package domain

import "fmt"

// ErrNotFound indicates the requested resource does not exist.
type ErrNotFound struct {
	Entity string
	ID     string
}

func (e *ErrNotFound) Error() string {
	return fmt.Sprintf("%s not found: %s", e.Entity, e.ID)
}

// ErrIdempotencyConflict indicates same idempotency key with different payload.
type ErrIdempotencyConflict struct {
	IdempotencyKey string
}

func (e *ErrIdempotencyConflict) Error() string {
	return fmt.Sprintf("idempotency conflict: key %q already exists with different payload", e.IdempotencyKey)
}

// ErrInvalidStateTransition indicates an illegal status change.
type ErrInvalidStateTransition struct {
	From ExecutionStatus
	To   ExecutionStatus
}

func (e *ErrInvalidStateTransition) Error() string {
	return fmt.Sprintf("invalid state transition: %s -> %s", e.From, e.To)
}

// ErrOptimisticLock indicates the row was modified by another process.
type ErrOptimisticLock struct {
	ExecutionID string
}

func (e *ErrOptimisticLock) Error() string {
	return fmt.Sprintf("optimistic lock conflict for execution %s", e.ExecutionID)
}

// ErrClaimFailed indicates no executions could be claimed.
type ErrClaimFailed struct{}

func (e *ErrClaimFailed) Error() string {
	return "no executions available to claim"
}

// ErrValidation indicates invalid input data.
type ErrValidation struct {
	Field   string
	Message string
}

func (e *ErrValidation) Error() string {
	return fmt.Sprintf("validation error on %s: %s", e.Field, e.Message)
}

// ErrMissingHeader indicates a required HTTP header is absent.
type ErrMissingHeader struct {
	Header string
}

func (e *ErrMissingHeader) Error() string {
	return fmt.Sprintf("missing required header: %s", e.Header)
}

// ErrLostLease indicates the worker's lease expired and was reclaimed.
type ErrLostLease struct {
	ExecutionID string
	WorkerID    string
}

func (e *ErrLostLease) Error() string {
	return fmt.Sprintf("lost lease on execution %s (worker %s)", e.ExecutionID, e.WorkerID)
}

// retryableErrors defines error codes that are safe to retry.
var retryableErrors = map[string]bool{
	"DOWNSTREAM_TIMEOUT":   true,
	"DOWNSTREAM_5XX":       true,
	"CONNECTION_REFUSED":   true,
	"CONNECTION_RESET":     true,
	"RESOURCE_EXHAUSTED":   true,
	"DATABASE_UNAVAILABLE": true,
}

// IsRetryableError checks if an error code is transient and safe to retry.
func IsRetryableError(code string) bool {
	return retryableErrors[code]
}
