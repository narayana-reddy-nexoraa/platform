package broker

import (
	"sync"
	"time"
)

// --- Token Bucket Rate Limiter (per-tenant) ---

// TokenBucket implements a simple token bucket rate limiter.
type TokenBucket struct {
	mu         sync.Mutex
	tokens     float64
	maxTokens  float64
	refillRate float64 // tokens per second
	lastRefill time.Time
}

// NewTokenBucket creates a bucket with the given capacity and refill rate.
func NewTokenBucket(maxTokens, refillRate float64) *TokenBucket {
	return &TokenBucket{
		tokens:     maxTokens,
		maxTokens:  maxTokens,
		refillRate: refillRate,
		lastRefill: time.Now(),
	}
}

// Allow checks if a token is available and consumes it. Returns false if rate limited.
func (b *TokenBucket) Allow() bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.refill()

	if b.tokens >= 1 {
		b.tokens--
		return true
	}
	return false
}

func (b *TokenBucket) refill() {
	now := time.Now()
	elapsed := now.Sub(b.lastRefill).Seconds()
	b.tokens += elapsed * b.refillRate
	if b.tokens > b.maxTokens {
		b.tokens = b.maxTokens
	}
	b.lastRefill = now
}

// TenantRateLimiter manages per-tenant token buckets.
type TenantRateLimiter struct {
	mu         sync.RWMutex
	buckets    map[string]*TokenBucket
	maxTokens  float64
	refillRate float64
}

// NewTenantRateLimiter creates a limiter that gives each tenant the specified capacity.
func NewTenantRateLimiter(maxTokensPerTenant, refillRatePerTenant float64) *TenantRateLimiter {
	return &TenantRateLimiter{
		buckets:    make(map[string]*TokenBucket),
		maxTokens:  maxTokensPerTenant,
		refillRate: refillRatePerTenant,
	}
}

// Allow checks if the tenant is within rate limits.
func (r *TenantRateLimiter) Allow(tenantID string) bool {
	r.mu.RLock()
	bucket, exists := r.buckets[tenantID]
	r.mu.RUnlock()

	if !exists {
		r.mu.Lock()
		bucket, exists = r.buckets[tenantID]
		if !exists {
			bucket = NewTokenBucket(r.maxTokens, r.refillRate)
			r.buckets[tenantID] = bucket
		}
		r.mu.Unlock()
	}

	return bucket.Allow()
}

// --- Circuit Breaker ---

// CircuitState represents the state of a circuit breaker.
type CircuitState int

const (
	CircuitClosed   CircuitState = iota // Normal operation
	CircuitOpen                         // Failing, reject requests
	CircuitHalfOpen                     // Testing recovery
)

func (s CircuitState) String() string {
	switch s {
	case CircuitClosed:
		return "CLOSED"
	case CircuitOpen:
		return "OPEN"
	case CircuitHalfOpen:
		return "HALF_OPEN"
	default:
		return "UNKNOWN"
	}
}

// CircuitBreaker implements the circuit breaker pattern for downstream services.
type CircuitBreaker struct {
	mu               sync.RWMutex
	state            CircuitState
	failureCount     int
	successCount     int
	failureThreshold int           // consecutive failures to trip open
	successThreshold int           // consecutive successes in half-open to close
	openTimeout      time.Duration // time to stay open before trying half-open
	lastFailureAt    time.Time
}

// NewCircuitBreaker creates a circuit breaker with the given thresholds.
func NewCircuitBreaker(failureThreshold, successThreshold int, openTimeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		state:            CircuitClosed,
		failureThreshold: failureThreshold,
		successThreshold: successThreshold,
		openTimeout:      openTimeout,
	}
}

// Allow checks if the circuit allows a request through.
func (cb *CircuitBreaker) Allow() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case CircuitClosed:
		return true

	case CircuitOpen:
		// Check if enough time has passed to try half-open
		if time.Since(cb.lastFailureAt) >= cb.openTimeout {
			cb.state = CircuitHalfOpen
			cb.successCount = 0
			circuitBreakerStateGauge.WithLabelValues("half_open").Set(1)
			circuitBreakerStateGauge.WithLabelValues("open").Set(0)
			return true
		}
		return false

	case CircuitHalfOpen:
		return true

	default:
		return false
	}
}

// RecordSuccess records a successful call.
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case CircuitHalfOpen:
		cb.successCount++
		if cb.successCount >= cb.successThreshold {
			cb.state = CircuitClosed
			cb.failureCount = 0
			cb.successCount = 0
			circuitBreakerStateGauge.WithLabelValues("closed").Set(1)
			circuitBreakerStateGauge.WithLabelValues("half_open").Set(0)
			circuitBreakerTransitionsTotal.WithLabelValues("closed").Inc()
		}
	case CircuitClosed:
		cb.failureCount = 0 // reset consecutive failures on success
	}
}

// RecordFailure records a failed call.
func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.lastFailureAt = time.Now()

	switch cb.state {
	case CircuitClosed:
		cb.failureCount++
		if cb.failureCount >= cb.failureThreshold {
			cb.state = CircuitOpen
			circuitBreakerStateGauge.WithLabelValues("open").Set(1)
			circuitBreakerStateGauge.WithLabelValues("closed").Set(0)
			circuitBreakerTransitionsTotal.WithLabelValues("open").Inc()
		}
	case CircuitHalfOpen:
		// Any failure in half-open immediately trips back to open
		cb.state = CircuitOpen
		cb.successCount = 0
		circuitBreakerStateGauge.WithLabelValues("open").Set(1)
		circuitBreakerStateGauge.WithLabelValues("half_open").Set(0)
		circuitBreakerTransitionsTotal.WithLabelValues("open").Inc()
	}
}

// State returns the current circuit state.
func (cb *CircuitBreaker) State() CircuitState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}
