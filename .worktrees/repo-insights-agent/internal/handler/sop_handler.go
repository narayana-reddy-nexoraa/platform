package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/narayana-platform/execution-engine/internal/domain"
	sopdomain "github.com/narayana-platform/execution-engine/internal/sop/domain"
)

// SOPServiceInterface defines the methods the SOP handler needs from the service layer.
type SOPServiceInterface interface {
	StartExecution(ctx context.Context, sopID string, tenantID uuid.UUID, payload json.RawMessage) (*sopdomain.SOPExecution, error)
	GetExecution(ctx context.Context, executionID, tenantID uuid.UUID) (*sopdomain.SOPExecution, error)
	ListExecutions(ctx context.Context, tenantID uuid.UUID, sopID, status, industry *string, limit, offset int32) (*sopdomain.SOPPaginatedResponse, error)
}

// SOPHandler handles HTTP requests for SOP executions.
type SOPHandler struct {
	service SOPServiceInterface
}

// NewSOPHandler creates a new SOP handler.
func NewSOPHandler(svc SOPServiceInterface) *SOPHandler {
	return &SOPHandler{service: svc}
}

// StartExecution handles POST /api/v2/sops/:id/execute
func (h *SOPHandler) StartExecution(c *gin.Context) {
	tenantID := c.MustGet("tenant_id").(uuid.UUID)
	sopID := c.Param("id")

	var req sopdomain.StartSOPExecutionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error: "invalid request body: " + err.Error(),
			Code:  "INVALID_REQUEST",
		})
		return
	}

	exec, err := h.service.StartExecution(c.Request.Context(), sopID, tenantID, req.Payload)
	if err != nil {
		mapDomainError(c, err)
		return
	}

	c.JSON(http.StatusCreated, exec.ToResponse())
}

// GetExecution handles GET /api/v2/sop-executions/:id
func (h *SOPHandler) GetExecution(c *gin.Context) {
	tenantID := c.MustGet("tenant_id").(uuid.UUID)

	executionID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error: "invalid execution ID format",
			Code:  "INVALID_ID",
		})
		return
	}

	exec, err := h.service.GetExecution(c.Request.Context(), executionID, tenantID)
	if err != nil {
		mapDomainError(c, err)
		return
	}

	c.JSON(http.StatusOK, exec.ToResponse())
}

// ListExecutions handles GET /api/v2/sop-executions
func (h *SOPHandler) ListExecutions(c *gin.Context) {
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

	var sopID, status, industry *string
	if s := c.Query("sop_id"); s != "" {
		sopID = &s
	}
	if s := c.Query("status"); s != "" {
		st := domain.ExecutionStatus(s)
		_ = st // validate below via service
		status = &s
	}
	if s := c.Query("industry"); s != "" {
		industry = &s
	}

	result, err := h.service.ListExecutions(c.Request.Context(), tenantID, sopID, status, industry, limit, offset)
	if err != nil {
		mapDomainError(c, err)
		return
	}

	c.JSON(http.StatusOK, result)
}
