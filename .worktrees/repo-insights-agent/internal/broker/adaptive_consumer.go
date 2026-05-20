package broker

import (
	"sync"
	"time"
)

// AdaptiveConsumer detects traffic spikes and applies backpressure
// to prevent the consumer from being overwhelmed.
//
// Spike detection: if the current rate exceeds 3x the moving average,
// a spike is detected and the consumer enters throttled mode.
//
// Flipflop cooldown: after entering or leaving throttled mode, the
// consumer must wait for the cooldown period before changing state
// again. This prevents rapid oscillation between throttled/unthrottled.
type AdaptiveConsumer struct {
	mu sync.RWMutex

	// Spike detection
	windowSize     int           // number of samples in the moving average
	samples        []float64     // circular buffer of rate samples
	sampleIdx      int           // next write position
	spikeThreshold float64       // multiplier over moving average to detect spike (default: 3.0)
	sampleInterval time.Duration // how often to sample the rate

	// Rate tracking
	currentCount int64     // events in current sample window
	lastSampleAt time.Time // when the last sample was taken

	// Flipflop cooldown
	cooldownDuration time.Duration // min time between state changes
	lastStateChange  time.Time     // when throttled state last changed
	throttled        bool          // whether we're in throttled mode

	// Burst buffer
	burstBufferSize int // max events to buffer during spike
	burstBufferUsed int // current events in burst buffer
}

// AdaptiveConsumerConfig holds configuration for the adaptive consumer.
type AdaptiveConsumerConfig struct {
	WindowSize       int           // moving average window (default: 10)
	SpikeThreshold   float64       // spike detection multiplier (default: 3.0)
	SampleInterval   time.Duration // rate sampling interval (default: 5s)
	CooldownDuration time.Duration // flipflop cooldown (default: 60s)
	BurstBufferSize  int           // max burst buffer size (default: 1000)
}

// DefaultAdaptiveConsumerConfig returns sensible defaults.
func DefaultAdaptiveConsumerConfig() AdaptiveConsumerConfig {
	return AdaptiveConsumerConfig{
		WindowSize:       10,
		SpikeThreshold:   3.0,
		SampleInterval:   5 * time.Second,
		CooldownDuration: 60 * time.Second,
		BurstBufferSize:  1000,
	}
}

// NewAdaptiveConsumer creates an adaptive consumer with the given config.
func NewAdaptiveConsumer(cfg AdaptiveConsumerConfig) *AdaptiveConsumer {
	if cfg.WindowSize <= 0 {
		cfg.WindowSize = 10
	}
	if cfg.SpikeThreshold <= 0 {
		cfg.SpikeThreshold = 3.0
	}
	if cfg.SampleInterval <= 0 {
		cfg.SampleInterval = 5 * time.Second
	}
	if cfg.CooldownDuration <= 0 {
		cfg.CooldownDuration = 60 * time.Second
	}
	if cfg.BurstBufferSize <= 0 {
		cfg.BurstBufferSize = 1000
	}

	return &AdaptiveConsumer{
		windowSize:       cfg.WindowSize,
		samples:          make([]float64, cfg.WindowSize),
		spikeThreshold:   cfg.SpikeThreshold,
		sampleInterval:   cfg.SampleInterval,
		cooldownDuration: cfg.CooldownDuration,
		burstBufferSize:  cfg.BurstBufferSize,
		lastSampleAt:     time.Now(),
		lastStateChange:  time.Now(),
	}
}

// RecordEvent records that an event was received. Call this for every consumed event.
func (ac *AdaptiveConsumer) RecordEvent() {
	ac.mu.Lock()
	defer ac.mu.Unlock()

	ac.currentCount++

	// Check if it's time to take a sample
	if time.Since(ac.lastSampleAt) >= ac.sampleInterval {
		ac.takeSample()
	}
}

// ShouldThrottle returns true if the consumer should slow down processing.
func (ac *AdaptiveConsumer) ShouldThrottle() bool {
	ac.mu.RLock()
	defer ac.mu.RUnlock()
	return ac.throttled
}

// IsSpike returns true if the current rate is a spike relative to the moving average.
func (ac *AdaptiveConsumer) IsSpike() bool {
	ac.mu.RLock()
	defer ac.mu.RUnlock()

	avg := ac.movingAverage()
	if avg == 0 {
		return false
	}

	currentRate := float64(ac.currentCount) / time.Since(ac.lastSampleAt).Seconds()
	return currentRate > avg*ac.spikeThreshold
}

// BurstBufferAvailable returns how many events can still be buffered.
func (ac *AdaptiveConsumer) BurstBufferAvailable() int {
	ac.mu.RLock()
	defer ac.mu.RUnlock()
	return ac.burstBufferSize - ac.burstBufferUsed
}

// ConsumeFromBurstBuffer marks one event as consumed from the burst buffer.
func (ac *AdaptiveConsumer) ConsumeFromBurstBuffer() bool {
	ac.mu.Lock()
	defer ac.mu.Unlock()
	if ac.burstBufferUsed > 0 {
		ac.burstBufferUsed--
		return true
	}
	return false
}

// AddToBurstBuffer adds an event to the burst buffer during a spike.
// Returns false if the buffer is full.
func (ac *AdaptiveConsumer) AddToBurstBuffer() bool {
	ac.mu.Lock()
	defer ac.mu.Unlock()
	if ac.burstBufferUsed >= ac.burstBufferSize {
		adaptiveShedTotal.Inc()
		return false
	}
	ac.burstBufferUsed++
	return true
}

// takeSample records the current rate and updates spike detection. Caller must hold mu.
func (ac *AdaptiveConsumer) takeSample() {
	elapsed := time.Since(ac.lastSampleAt).Seconds()
	if elapsed <= 0 {
		return
	}

	rate := float64(ac.currentCount) / elapsed
	ac.samples[ac.sampleIdx] = rate
	ac.sampleIdx = (ac.sampleIdx + 1) % ac.windowSize

	// Update spike detection
	avg := ac.movingAverage()
	isSpike := avg > 0 && rate > avg*ac.spikeThreshold

	// Apply flipflop cooldown
	if time.Since(ac.lastStateChange) >= ac.cooldownDuration {
		if isSpike && !ac.throttled {
			ac.throttled = true
			ac.lastStateChange = time.Now()
			adaptiveThrottledGauge.Set(1)
			adaptiveSpikeDetectedTotal.Inc()
		} else if !isSpike && ac.throttled {
			ac.throttled = false
			ac.lastStateChange = time.Now()
			ac.burstBufferUsed = 0 // drain burst buffer on recovery
			adaptiveThrottledGauge.Set(0)
		}
	}

	adaptiveRateGauge.Set(rate)
	adaptiveMovingAvgGauge.Set(avg)

	// Reset for next sample
	ac.currentCount = 0
	ac.lastSampleAt = time.Now()
}

// movingAverage computes the average of non-zero samples. Caller must hold mu.
func (ac *AdaptiveConsumer) movingAverage() float64 {
	var sum float64
	var count int
	for _, s := range ac.samples {
		if s > 0 {
			sum += s
			count++
		}
	}
	if count == 0 {
		return 0
	}
	return sum / float64(count)
}
