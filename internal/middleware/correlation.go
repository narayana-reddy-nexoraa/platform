package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const CorrelationIDHeader = "X-Correlation-ID"
const CorrelationIDKey = "correlation_id"

// CorrelationID extracts or generates a correlation ID for request tracing.
func CorrelationID() gin.HandlerFunc {
	return func(c *gin.Context) {
		correlationID := c.GetHeader(CorrelationIDHeader)
		if correlationID == "" {
			correlationID = uuid.New().String()
		}

		c.Set(CorrelationIDKey, correlationID)
		c.Header(CorrelationIDHeader, correlationID)

		c.Next()
	}
}
