package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/narayana-platform/execution-engine/internal/handler"
)

const TenantIDHeader = "X-Tenant-ID"
const TenantIDKey = "tenant_id"

// TenantExtractor validates and extracts the X-Tenant-ID header.
func TenantExtractor() gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantHeader := c.GetHeader(TenantIDHeader)
		if tenantHeader == "" {
			c.AbortWithStatusJSON(http.StatusBadRequest, handler.ErrorResponse{
				Error: "missing required header: X-Tenant-ID",
				Code:  "MISSING_HEADER",
			})
			return
		}

		tenantID, err := uuid.Parse(tenantHeader)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, handler.ErrorResponse{
				Error: "invalid X-Tenant-ID: must be a valid UUID",
				Code:  "INVALID_HEADER",
			})
			return
		}

		c.Set(TenantIDKey, tenantID)
		c.Next()
	}
}
