package middleware

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"

	"github.com/narayana-platform/execution-engine/internal/contextkeys"
)

// RequestLogger logs each HTTP request with structured fields.
func RequestLogger(logger zerolog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		c.Next()

		latency := time.Since(start)
		correlationID := contextkeys.CorrelationIDFromContext(c.Request.Context())

		logger.Info().
			Str("method", c.Request.Method).
			Str("path", c.Request.URL.Path).
			Int("status", c.Writer.Status()).
			Dur("latency", latency).
			Str("correlation_id", correlationID).
			Str("client_ip", c.ClientIP()).
			Msg("request completed")
	}
}
