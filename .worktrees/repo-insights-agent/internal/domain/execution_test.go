package domain

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestComputePayloadHash_Deterministic(t *testing.T) {
	payload := json.RawMessage(`{"order_id": "order-123", "amount": 99.99}`)

	hash1, err := ComputePayloadHash(payload)
	require.NoError(t, err)

	hash2, err := ComputePayloadHash(payload)
	require.NoError(t, err)

	assert.Equal(t, hash1, hash2, "same payload should produce same hash")
	assert.Len(t, hash1, 64, "SHA-256 hex should be 64 characters")
}

func TestComputePayloadHash_KeyOrderIndependent(t *testing.T) {
	// Keys in different order should produce the same hash
	payload1 := json.RawMessage(`{"amount": 99.99, "order_id": "order-123"}`)
	payload2 := json.RawMessage(`{"order_id": "order-123", "amount": 99.99}`)

	hash1, err := ComputePayloadHash(payload1)
	require.NoError(t, err)

	hash2, err := ComputePayloadHash(payload2)
	require.NoError(t, err)

	assert.Equal(t, hash1, hash2, "different key ordering should produce same hash")
}

func TestComputePayloadHash_DifferentPayloadsDifferentHash(t *testing.T) {
	payload1 := json.RawMessage(`{"order_id": "order-123", "amount": 99.99}`)
	payload2 := json.RawMessage(`{"order_id": "order-456", "amount": 50.00}`)

	hash1, err := ComputePayloadHash(payload1)
	require.NoError(t, err)

	hash2, err := ComputePayloadHash(payload2)
	require.NoError(t, err)

	assert.NotEqual(t, hash1, hash2, "different payloads should produce different hashes")
}

func TestComputePayloadHash_NestedJSON(t *testing.T) {
	payload := json.RawMessage(`{"user": {"name": "John", "id": 1}, "items": [1, 2, 3]}`)

	hash, err := ComputePayloadHash(payload)
	require.NoError(t, err)
	assert.Len(t, hash, 64)
}

func TestComputePayloadHash_InvalidJSON(t *testing.T) {
	payload := json.RawMessage(`{invalid json}`)

	_, err := ComputePayloadHash(payload)
	assert.Error(t, err)
}
