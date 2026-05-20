package domain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestCalculateDelay_ExponentialGrowth(t *testing.T) {
	policy := RetryPolicy{BaseDelayMs: 1000, MaxDelayMs: 60000}

	for attempt := int32(1); attempt <= 5; attempt++ {
		maxExpected := float64(policy.BaseDelayMs) * float64(int64(1)<<(attempt-1))
		if maxExpected > float64(policy.MaxDelayMs) {
			maxExpected = float64(policy.MaxDelayMs)
		}

		for i := 0; i < 100; i++ {
			delay := policy.CalculateDelay(attempt)
			assert.GreaterOrEqual(t, delay, time.Duration(0))
			assert.LessOrEqual(t, delay, time.Duration(maxExpected)*time.Millisecond)
		}
	}
}

func TestCalculateDelay_CappedAtMax(t *testing.T) {
	policy := RetryPolicy{BaseDelayMs: 1000, MaxDelayMs: 5000}

	for i := 0; i < 100; i++ {
		delay := policy.CalculateDelay(10)
		assert.LessOrEqual(t, delay, 5000*time.Millisecond)
	}
}

func TestCalculateDelay_FullJitterSpread(t *testing.T) {
	policy := RetryPolicy{BaseDelayMs: 1000, MaxDelayMs: 60000}

	var min, max time.Duration
	min = time.Hour
	for i := 0; i < 1000; i++ {
		d := policy.CalculateDelay(1)
		if d < min {
			min = d
		}
		if d > max {
			max = d
		}
	}
	assert.Less(t, min, 200*time.Millisecond)
	assert.Greater(t, max, 700*time.Millisecond)
}

func TestIsRetryableError(t *testing.T) {
	assert.True(t, IsRetryableError("DOWNSTREAM_TIMEOUT"))
	assert.True(t, IsRetryableError("DOWNSTREAM_5XX"))
	assert.True(t, IsRetryableError("CONNECTION_REFUSED"))
	assert.True(t, IsRetryableError("CONNECTION_RESET"))
	assert.True(t, IsRetryableError("RESOURCE_EXHAUSTED"))
	assert.True(t, IsRetryableError("DATABASE_UNAVAILABLE"))

	assert.False(t, IsRetryableError("VALIDATION_FAILED"))
	assert.False(t, IsRetryableError("NOT_FOUND"))
	assert.False(t, IsRetryableError("PERMISSION_DENIED"))
	assert.False(t, IsRetryableError(""))
	assert.False(t, IsRetryableError("UNKNOWN_ERROR"))
}
