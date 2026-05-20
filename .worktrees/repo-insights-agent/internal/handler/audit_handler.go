package handler

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	sopdomain "github.com/narayana-platform/execution-engine/internal/sop/domain"
)

// AuditServiceInterface defines the methods the audit handler needs from the service layer.
type AuditServiceInterface interface {
	GetAuditTrail(ctx context.Context, executionID, tenantID uuid.UUID) (*sopdomain.AuditTrailResponse, error)
}

// AuditHandler handles HTTP requests for audit trail queries.
type AuditHandler struct {
	service AuditServiceInterface
}

// NewAuditHandler creates a new audit handler.
func NewAuditHandler(svc AuditServiceInterface) *AuditHandler {
	return &AuditHandler{service: svc}
}

// GetAuditTrail handles GET /api/v2/audit/executions/:id
func (h *AuditHandler) GetAuditTrail(c *gin.Context) {
	tenantID := c.MustGet("tenant_id").(uuid.UUID)

	executionID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error: "invalid execution ID format",
			Code:  "INVALID_ID",
		})
		return
	}

	trail, err := h.service.GetAuditTrail(c.Request.Context(), executionID, tenantID)
	if err != nil {
		mapDomainError(c, err)
		return
	}

	c.JSON(http.StatusOK, trail)
}
