package domain

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"
)

// Execution is the core domain entity.
type Execution struct {
	ExecutionID    uuid.UUID       `json:"execution_id"`
	TenantID       uuid.UUID       `json:"tenant_id"`
	IdempotencyKey string          `json:"idempotency_key"`
	Status         ExecutionStatus `json:"status"`
	AttemptCount   int32           `json:"attempt_count"`
	MaxAttempts    int32           `json:"max_attempts"`
	LockedBy       *string         `json:"locked_by,omitempty"`
	LockExpiresAt  *time.Time      `json:"lock_expires_at,omitempty"`
	LastHeartbeatAt *time.Time     `json:"last_heartbeat_at,omitempty"`
	ErrorCode      *string         `json:"error_code,omitempty"`
	ErrorMessage   *string         `json:"error_message,omitempty"`
	Payload        json.RawMessage `json:"payload"`
	PayloadHash    string          `json:"payload_hash"`
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`
	Version        int32           `json:"version"`
}

// CreateExecutionRequest is the input for creating a new execution.
type CreateExecutionRequest struct {
	Payload     json.RawMessage `json:"payload" binding:"required"`
	MaxAttempts *int32          `json:"max_attempts,omitempty"`
}

// ExecutionResponse is the API output for a single execution.
type ExecutionResponse struct {
	ExecutionID    uuid.UUID       `json:"execution_id"`
	TenantID       uuid.UUID       `json:"tenant_id"`
	IdempotencyKey string          `json:"idempotency_key"`
	Status         ExecutionStatus `json:"status"`
	AttemptCount   int32           `json:"attempt_count"`
	MaxAttempts    int32           `json:"max_attempts"`
	LockedBy       *string         `json:"locked_by,omitempty"`
	LockExpiresAt  *time.Time      `json:"lock_expires_at,omitempty"`
	ErrorCode      *string         `json:"error_code,omitempty"`
	ErrorMessage   *string         `json:"error_message,omitempty"`
	Payload        json.RawMessage `json:"payload"`
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`
	Version        int32           `json:"version"`
}

// PaginatedResponse wraps a list response with pagination metadata.
type PaginatedResponse struct {
	Data       []ExecutionResponse `json:"data"`
	TotalCount int64               `json:"total_count"`
	Limit      int32               `json:"limit"`
	Offset     int32               `json:"offset"`
}

// ToResponse converts a domain Execution to an API response.
func (e *Execution) ToResponse() ExecutionResponse {
	return ExecutionResponse{
		ExecutionID:    e.ExecutionID,
		TenantID:       e.TenantID,
		IdempotencyKey: e.IdempotencyKey,
		Status:         e.Status,
		AttemptCount:   e.AttemptCount,
		MaxAttempts:    e.MaxAttempts,
		LockedBy:       e.LockedBy,
		LockExpiresAt:  e.LockExpiresAt,
		ErrorCode:      e.ErrorCode,
		ErrorMessage:   e.ErrorMessage,
		Payload:        e.Payload,
		CreatedAt:      e.CreatedAt,
		UpdatedAt:      e.UpdatedAt,
		Version:        e.Version,
	}
}

// ComputePayloadHash produces a deterministic SHA-256 hex string from JSON payload.
// It normalizes the JSON by unmarshalling into a map and re-marshalling with sorted keys.
func ComputePayloadHash(payload json.RawMessage) (string, error) {
	var data interface{}
	if err := json.Unmarshal(payload, &data); err != nil {
		return "", fmt.Errorf("invalid JSON payload: %w", err)
	}

	canonical, err := canonicalJSON(data)
	if err != nil {
		return "", fmt.Errorf("failed to canonicalize payload: %w", err)
	}

	hash := sha256.Sum256(canonical)
	return fmt.Sprintf("%x", hash), nil
}

// canonicalJSON produces sorted-key JSON bytes for deterministic hashing.
func canonicalJSON(v interface{}) ([]byte, error) {
	switch val := v.(type) {
	case map[string]interface{}:
		keys := make([]string, 0, len(val))
		for k := range val {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		ordered := make([]byte, 0, 128)
		ordered = append(ordered, '{')
		for i, k := range keys {
			if i > 0 {
				ordered = append(ordered, ',')
			}
			keyBytes, _ := json.Marshal(k)
			ordered = append(ordered, keyBytes...)
			ordered = append(ordered, ':')
			valBytes, err := canonicalJSON(val[k])
			if err != nil {
				return nil, err
			}
			ordered = append(ordered, valBytes...)
		}
		ordered = append(ordered, '}')
		return ordered, nil

	case []interface{}:
		ordered := make([]byte, 0, 64)
		ordered = append(ordered, '[')
		for i, item := range val {
			if i > 0 {
				ordered = append(ordered, ',')
			}
			itemBytes, err := canonicalJSON(item)
			if err != nil {
				return nil, err
			}
			ordered = append(ordered, itemBytes...)
		}
		ordered = append(ordered, ']')
		return ordered, nil

	default:
		return json.Marshal(v)
	}
}
