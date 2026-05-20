package broker

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestTokenBucket_AllowWithinLimit(t *testing.T) {
	b := NewTokenBucket(5, 10) // 5 tokens, refills 10/sec
	for i := 0; i < 5; i++ {
		assert.True(t, b.Allow(), "should allow request %d", i)
	}
	assert.False(t, b.Allow(), "should deny after exhausting tokens")
}

func TestTokenBucket_Refill(t *testing.T) {
	b := NewTokenBucket(2, 100) // 2 tokens, refills 100/sec
	b.Allow()
	b.Allow()
	assert.False(t, b.Allow(), "should be empty")

	// Simulate time passing for refill
	b.mu.Lock()
	b.lastRefill = time.Now().Add(-50 * time.Millisecond) // 0.05s * 100/s = 5 tokens
	b.mu.Unlock()

	assert.True(t, b.Allow(), "should have refilled")
}

func TestTenantRateLimiter_IsolatesTenants(t *testing.T) {
	limiter := NewTenantRateLimiter(2, 10)

	assert.True(t, limiter.Allow("tenant-a"))
	assert.True(t, limiter.Allow("tenant-a"))
	assert.False(t, limiter.Allow("tenant-a"), "tenant-a should be exhausted")

	assert.True(t, limiter.Allow("tenant-b"), "tenant-b should have its own bucket")
	assert.True(t, limiter.Allow("tenant-b"))
	assert.False(t, limiter.Allow("tenant-b"), "tenant-b should be exhausted")
}

func TestCircuitBreaker_StartsClosedAndTrips(t *testing.T) {
	cb := NewCircuitBreaker(3, 2, 5*time.Second)

	assert.Equal(t, CircuitClosed, cb.State())
	assert.True(t, cb.Allow())

	// Record failures below threshold
	cb.RecordFailure()
	cb.RecordFailure()
	assert.Equal(t, CircuitClosed, cb.State(), "should still be closed after 2 failures")

	// Third failure trips the breaker
	cb.RecordFailure()
	assert.Equal(t, CircuitOpen, cb.State(), "should be open after 3 failures")
	assert.False(t, cb.Allow(), "should deny when open")
}

func TestCircuitBreaker_SuccessResetsFaiureCount(t *testing.T) {
	cb := NewCircuitBreaker(3, 2, 5*time.Second)

	cb.RecordFailure()
	cb.RecordFailure()
	cb.RecordSuccess() // resets failure count

	cb.RecordFailure()
	assert.Equal(t, CircuitClosed, cb.State(), "should still be closed — success reset the counter")
}

func TestCircuitBreaker_HalfOpenTransition(t *testing.T) {
	cb := NewCircuitBreaker(2, 2, 10*time.Millisecond)

	// Trip to open
	cb.RecordFailure()
	cb.RecordFailure()
	assert.Equal(t, CircuitOpen, cb.State())

	// Wait for open timeout
	time.Sleep(15 * time.Millisecond)

	// Should transition to half-open
	assert.True(t, cb.Allow(), "should allow in half-open")
	assert.Equal(t, CircuitHalfOpen, cb.State())

	// Successes close the circuit
	cb.RecordSuccess()
	cb.RecordSuccess()
	assert.Equal(t, CircuitClosed, cb.State(), "should close after success threshold in half-open")
}

func TestCircuitBreaker_HalfOpenFailureReOpens(t *testing.T) {
	cb := NewCircuitBreaker(2, 2, 10*time.Millisecond)

	cb.RecordFailure()
	cb.RecordFailure()
	time.Sleep(15 * time.Millisecond)
	cb.Allow() // transition to half-open

	cb.RecordFailure() // any failure in half-open re-opens
	assert.Equal(t, CircuitOpen, cb.State(), "should re-open on failure in half-open")
}

func TestCircuitBreaker_StateString(t *testing.T) {
	assert.Equal(t, "CLOSED", CircuitClosed.String())
	assert.Equal(t, "OPEN", CircuitOpen.String())
	assert.Equal(t, "HALF_OPEN", CircuitHalfOpen.String())
}
