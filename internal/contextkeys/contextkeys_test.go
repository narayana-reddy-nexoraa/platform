package contextkeys

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCorrelationIDFromContext_EmptyContext(t *testing.T) {
	ctx := context.Background()
	assert.Equal(t, "", CorrelationIDFromContext(ctx))
}

func TestCorrelationID_RoundTrip(t *testing.T) {
	ctx := context.Background()
	id := "abc-123-def-456"
	ctx = WithCorrelationID(ctx, id)
	assert.Equal(t, id, CorrelationIDFromContext(ctx))
}
