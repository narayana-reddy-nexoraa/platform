package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"

	"github.com/narayana-platform/execution-engine/internal/contextkeys"
	"github.com/narayana-platform/execution-engine/internal/handler"
)

// ErrorHandler is a recovery middleware that catches panics and returns 500.
func ErrorHandler(logger zerolog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				correlationID := contextkeys.CorrelationIDFromContext(c.Request.Context())
				logger.Error().
					Interface("panic", r).
					Str("correlation_id", correlationID).
					Str("path", c.Request.URL.Path).
					Msg("panic recovered")

				c.AbortWithStatusJSON(http.StatusInternalServerError, handler.ErrorResponse{
					Error: "internal server error",
					Code:  "INTERNAL_ERROR",
				})
			}
		}()
		c.Next()
	}
}
