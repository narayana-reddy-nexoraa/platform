package contextkeys

import "context"

type ctxKey int

const correlationIDKey ctxKey = iota

// WithCorrelationID returns a new context with the given correlation ID.
func WithCorrelationID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, correlationIDKey, id)
}

// CorrelationIDFromContext extracts the correlation ID from the context.
// Returns an empty string if not present.
func CorrelationIDFromContext(ctx context.Context) string {
	if id, ok := ctx.Value(correlationIDKey).(string); ok {
		return id
	}
	return ""
}
