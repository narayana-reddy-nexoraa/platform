package handler

import (
	"context"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	sopdomain "github.com/narayana-platform/execution-engine/internal/sop/domain"
)

// HITLServiceInterface defines the methods the HITL handler needs from the service layer.
type HITLServiceInterface interface {
	GetRequest(ctx context.Context, requestID, tenantID uuid.UUID) (*sopdomain.HITLRequest, error)
	ListPending(ctx context.Context, tenantID uuid.UUID, limit, offset int32) ([]sopdomain.HITLRequest, int64, error)
	Decide(ctx context.Context, requestID, tenantID uuid.UUID, req sopdomain.HITLDecideRequest) (*sopdomain.HITLRequest, error)
}

// HITLHandler handles HTTP requests for HITL approvals.
type HITLHandler struct {
	service HITLServiceInterface
}

// NewHITLHandler creates a new HITL handler.
func NewHITLHandler(svc HITLServiceInterface) *HITLHandler {
	return &HITLHandler{service: svc}
}

// ListPending handles GET /api/v2/hitl/pending
func (h *HITLHandler) ListPending(c *gin.Context) {
	tenantID := c.MustGet("tenant_id").(uuid.UUID)

	limit := int32(20)
	if l := c.Query("limit"); l != "" {
		if parsed, err := strconv.ParseInt(l, 10, 32); err == nil {
			limit = int32(parsed)
		}
	}

	offset := int32(0)
	if o := c.Query("offset"); o != "" {
		if parsed, err := strconv.ParseInt(o, 10, 32); err == nil {
			offset = int32(parsed)
		}
	}

	requests, totalCount, err := h.service.ListPending(c.Request.Context(), tenantID, limit, offset)
	if err != nil {
		mapDomainError(c, err)
		return
	}

	data := make([]sopdomain.HITLRequestResponse, len(requests))
	for i := range requests {
		data[i] = requests[i].ToResponse()
	}

	c.JSON(http.StatusOK, gin.H{
		"data":        data,
		"total_count": totalCount,
		"limit":       limit,
		"offset":      offset,
	})
}

// GetRequest handles GET /api/v2/hitl/:id
func (h *HITLHandler) GetRequest(c *gin.Context) {
	tenantID := c.MustGet("tenant_id").(uuid.UUID)

	requestID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error: "invalid request ID format",
			Code:  "INVALID_ID",
		})
		return
	}

	req, err := h.service.GetRequest(c.Request.Context(), requestID, tenantID)
	if err != nil {
		mapDomainError(c, err)
		return
	}

	c.JSON(http.StatusOK, req.ToResponse())
}

// Decide handles POST /api/v2/hitl/:id/decide
func (h *HITLHandler) Decide(c *gin.Context) {
	tenantID := c.MustGet("tenant_id").(uuid.UUID)

	requestID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error: "invalid request ID format",
			Code:  "INVALID_ID",
		})
		return
	}

	var req sopdomain.HITLDecideRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error: "invalid request body: " + err.Error(),
			Code:  "INVALID_REQUEST",
		})
		return
	}

	updated, err := h.service.Decide(c.Request.Context(), requestID, tenantID, req)
	if err != nil {
		mapDomainError(c, err)
		return
	}

	c.JSON(http.StatusOK, updated.ToResponse())
}
