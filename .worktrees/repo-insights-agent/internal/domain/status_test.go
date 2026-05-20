package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCanTransitionTo_ValidTransitions(t *testing.T) {
	validCases := []struct {
		from ExecutionStatus
		to   ExecutionStatus
	}{
		{StatusCreated, StatusClaimed},
		{StatusCreated, StatusCanceled},
		{StatusClaimed, StatusCreated},  // reaper reclaim
		{StatusClaimed, StatusRunning},
		{StatusClaimed, StatusCanceled},
		{StatusClaimed, StatusTimedOut},
		{StatusRunning, StatusSucceeded},
		{StatusRunning, StatusFailed},
		{StatusRunning, StatusCanceled},
		{StatusRunning, StatusTimedOut},
		{StatusFailed, StatusCreated},  // retry re-queue
		{StatusFailed, StatusClaimed},  // direct retry claim
		{StatusFailed, StatusCanceled},
		{StatusTimedOut, StatusCreated}, // retry re-queue
	}

	for _, tc := range validCases {
		t.Run(string(tc.from)+"->"+string(tc.to), func(t *testing.T) {
			assert.True(t, tc.from.CanTransitionTo(tc.to), "%s should be able to transition to %s", tc.from, tc.to)
		})
	}
}

func TestCanTransitionTo_InvalidTransitions(t *testing.T) {
	invalidCases := []struct {
		from ExecutionStatus
		to   ExecutionStatus
	}{
		{StatusCreated, StatusRunning},     // must go through CLAIMED
		{StatusCreated, StatusSucceeded},   // can't skip to terminal
		{StatusSucceeded, StatusCreated},   // terminal state
		{StatusSucceeded, StatusFailed},    // terminal state
		{StatusCanceled, StatusCreated},    // terminal state
		{StatusRunning, StatusCreated},     // can't go backwards
	}

	for _, tc := range invalidCases {
		t.Run(string(tc.from)+"->"+string(tc.to), func(t *testing.T) {
			assert.False(t, tc.from.CanTransitionTo(tc.to), "%s should NOT be able to transition to %s", tc.from, tc.to)
		})
	}
}

func TestIsTerminal(t *testing.T) {
	assert.True(t, StatusSucceeded.IsTerminal())
	assert.True(t, StatusCanceled.IsTerminal())

	assert.False(t, StatusCreated.IsTerminal())
	assert.False(t, StatusClaimed.IsTerminal())
	assert.False(t, StatusRunning.IsTerminal())
	assert.False(t, StatusFailed.IsTerminal())
	assert.False(t, StatusTimedOut.IsTerminal())
}

func TestIsValidStatus(t *testing.T) {
	assert.True(t, IsValidStatus("CREATED"))
	assert.True(t, IsValidStatus("SUCCEEDED"))
	assert.True(t, IsValidStatus("TIMED_OUT"))
	assert.False(t, IsValidStatus("INVALID"))
	assert.False(t, IsValidStatus(""))
}
