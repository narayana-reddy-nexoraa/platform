package middleware

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"

	"github.com/narayana-platform/execution-engine/internal/contextkeys"
	"github.com/narayana-platform/execution-engine/internal/metrics"
)

// RequestLogger logs each HTTP request with structured fields.
func RequestLogger(logger zerolog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		c.Next()

		latency := time.Since(start)
		status := c.Writer.Status()
		correlationID := contextkeys.CorrelationIDFromContext(c.Request.Context())

		metrics.HTTPRequestDurationSeconds.WithLabelValues(c.Request.Method, strconv.Itoa(status)).Observe(latency.Seconds())

		logger.Info().
			Str("method", c.Request.Method).
			Str("path", c.Request.URL.Path).
			Int("status", status).
			Dur("latency", latency).
			Str("correlation_id", correlationID).
			Str("client_ip", c.ClientIP()).
			Msg("request completed")
	}
}
