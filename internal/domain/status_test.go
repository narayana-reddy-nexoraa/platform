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
		{StatusClaimed, StatusRunning},
		{StatusClaimed, StatusCanceled},
		{StatusClaimed, StatusTimedOut},
		{StatusRunning, StatusSucceeded},
		{StatusRunning, StatusFailed},
		{StatusRunning, StatusCanceled},
		{StatusRunning, StatusTimedOut},
		{StatusFailed, StatusClaimed}, // retry
		{StatusFailed, StatusCanceled},
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
		{StatusTimedOut, StatusCreated},    // terminal state
		{StatusRunning, StatusCreated},     // can't go backwards
		{StatusClaimed, StatusCreated},     // can't go backwards
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
	assert.True(t, StatusTimedOut.IsTerminal())

	assert.False(t, StatusCreated.IsTerminal())
	assert.False(t, StatusClaimed.IsTerminal())
	assert.False(t, StatusRunning.IsTerminal())
	assert.False(t, StatusFailed.IsTerminal())
}

func TestIsValidStatus(t *testing.T) {
	assert.True(t, IsValidStatus("CREATED"))
	assert.True(t, IsValidStatus("SUCCEEDED"))
	assert.True(t, IsValidStatus("TIMED_OUT"))
	assert.False(t, IsValidStatus("INVALID"))
	assert.False(t, IsValidStatus(""))
}
