package domain

import (
	"math"
	"math/rand"
	"time"
)

// DefaultRetryPolicy is the hardcoded retry policy for all executions.
var DefaultRetryPolicy = RetryPolicy{
	BaseDelayMs: 1000,  // 1 second
	MaxDelayMs:  60000, // 60 seconds
}

// RetryPolicy defines the backoff strategy for retrying failed executions.
type RetryPolicy struct {
	BaseDelayMs int64
	MaxDelayMs  int64
}

// CalculateDelay computes the retry delay with exponential backoff and full jitter.
// attemptCount is 1-based (the attempt that just failed).
func (p RetryPolicy) CalculateDelay(attemptCount int32) time.Duration {
	rawDelay := float64(p.BaseDelayMs) * math.Pow(2, float64(attemptCount-1))
	cappedDelay := math.Min(rawDelay, float64(p.MaxDelayMs))
	jitteredDelay := rand.Float64() * cappedDelay
	return time.Duration(jitteredDelay) * time.Millisecond
}
