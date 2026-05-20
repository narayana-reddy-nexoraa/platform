package broker

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestAdaptiveConsumer_StartsUnthrottled(t *testing.T) {
	ac := NewAdaptiveConsumer(DefaultAdaptiveConsumerConfig())
	assert.False(t, ac.ShouldThrottle())
}

func TestAdaptiveConsumer_BurstBuffer(t *testing.T) {
	cfg := DefaultAdaptiveConsumerConfig()
	cfg.BurstBufferSize = 3
	ac := NewAdaptiveConsumer(cfg)

	assert.Equal(t, 3, ac.BurstBufferAvailable())
	assert.True(t, ac.AddToBurstBuffer())
	assert.True(t, ac.AddToBurstBuffer())
	assert.True(t, ac.AddToBurstBuffer())
	assert.False(t, ac.AddToBurstBuffer(), "should reject when buffer full")
	assert.Equal(t, 0, ac.BurstBufferAvailable())

	assert.True(t, ac.ConsumeFromBurstBuffer())
	assert.Equal(t, 1, ac.BurstBufferAvailable())
}

func TestAdaptiveConsumer_SpikeDetectionWithSyntheticRates(t *testing.T) {
	cfg := AdaptiveConsumerConfig{
		WindowSize:       5,
		SpikeThreshold:   3.0,
		SampleInterval:   1 * time.Millisecond,
		CooldownDuration: 1 * time.Millisecond,
		BurstBufferSize:  100,
	}
	ac := NewAdaptiveConsumer(cfg)

	// Fill moving average with baseline rate (10 events/sample)
	for i := 0; i < 5; i++ {
		ac.mu.Lock()
		ac.samples[i] = 10.0
		ac.mu.Unlock()
	}

	// Inject a spike: 50 events in a tiny window (>>3x avg of 10)
	ac.mu.Lock()
	ac.currentCount = 50
	ac.lastSampleAt = time.Now().Add(-1 * time.Millisecond)
	ac.mu.Unlock()

	assert.True(t, ac.IsSpike(), "should detect spike at 50 vs avg 10")
}

func TestAdaptiveConsumer_NoSpikeWhenBelowThreshold(t *testing.T) {
	cfg := DefaultAdaptiveConsumerConfig()
	cfg.SampleInterval = 1 * time.Millisecond
	ac := NewAdaptiveConsumer(cfg)

	// Fill with baseline of 10
	for i := 0; i < ac.windowSize; i++ {
		ac.mu.Lock()
		ac.samples[i] = 10.0
		ac.mu.Unlock()
	}

	// Current rate is 20 (2x avg), below 3x threshold
	ac.mu.Lock()
	ac.currentCount = 20
	ac.lastSampleAt = time.Now().Add(-1 * time.Second)
	ac.mu.Unlock()

	assert.False(t, ac.IsSpike(), "2x should not trigger spike at 3x threshold")
}

func TestAdaptiveConsumer_RecordEventIncrementsCount(t *testing.T) {
	cfg := DefaultAdaptiveConsumerConfig()
	cfg.SampleInterval = 1 * time.Hour // prevent auto-sampling
	ac := NewAdaptiveConsumer(cfg)

	ac.RecordEvent()
	ac.RecordEvent()
	ac.RecordEvent()

	ac.mu.RLock()
	count := ac.currentCount
	ac.mu.RUnlock()

	assert.Equal(t, int64(3), count)
}

func TestAdaptiveConsumer_MovingAverageIgnoresZeros(t *testing.T) {
	cfg := DefaultAdaptiveConsumerConfig()
	ac := NewAdaptiveConsumer(cfg)

	// Only first 3 samples have values
	ac.mu.Lock()
	ac.samples[0] = 10
	ac.samples[1] = 20
	ac.samples[2] = 30
	// rest are 0 (unfilled)
	ac.mu.Unlock()

	ac.mu.RLock()
	avg := ac.movingAverage()
	ac.mu.RUnlock()

	assert.InDelta(t, 20.0, avg, 0.01, "should average only non-zero samples")
}
