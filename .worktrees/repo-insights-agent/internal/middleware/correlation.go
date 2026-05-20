package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/narayana-platform/execution-engine/internal/contextkeys"
)

const CorrelationIDHeader = "X-Correlation-ID"

// CorrelationID extracts or generates a correlation ID for request tracing.
func CorrelationID() gin.HandlerFunc {
	return func(c *gin.Context) {
		correlationID := c.GetHeader(CorrelationIDHeader)
		if correlationID == "" {
			correlationID = uuid.New().String()
		}

		c.Request = c.Request.WithContext(contextkeys.WithCorrelationID(c.Request.Context(), correlationID))
		c.Header(CorrelationIDHeader, correlationID)

		c.Next()
	}
}
